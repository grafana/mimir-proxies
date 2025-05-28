package whisperconverter

import (
	"io/fs"
	"os"
	"regexp"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestGetWhisperListIntoChan(t *testing.T) {
	tests := []struct {
		name               string
		testFiles          []string
		elementsMatch      []string
		targetWhisperFiles []string
		skippedCount       uint64
	}{
		{
			name:               "discoverTargetWhisperFiles",
			testFiles:          []string{"test1.skip", "test2.skip", "test1.wsp", "test2.wsp"},
			elementsMatch:      []string{"test1.wsp", "test2.wsp"},
			targetWhisperFiles: []string{},
			skippedCount:       2,
		},
		{
			name:               "specifiedTargetWhisperFiles",
			testFiles:          []string{"test1.skip", "test2.skip", "test1.wsp", "test2.wsp"},
			elementsMatch:      []string{"test1.wsp"},
			targetWhisperFiles: []string{"test1.wsp"},
			skippedCount:       0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("/tmp", test.name+"*")
			require.NoError(t, err)
			defer func() {
				_ = os.RemoveAll(tmpDir)
			}()

			for _, filename := range test.testFiles {
				_, err = os.Create(tmpDir + "/" + filename)
				require.NoError(t, err)
			}

			whisperDirectory := tmpDir
			fileFilter := regexp.MustCompile(`\.wsp$`)
			namePrefix := ""
			targetWhisperFiles := ""

			if len(test.targetWhisperFiles) > 0 {
				targetWhisperFiles = tmpDir + "/targetWhisperFiles.out"
				f, err := os.Create(targetWhisperFiles)
				require.NoError(t, err)
				defer func() {
					_ = f.Close()
				}()

				for _, file := range test.targetWhisperFiles {
					_, err = f.WriteString(tmpDir + "/" + file + "\n")
					require.NoError(t, err)
				}

				err = f.Sync()
				require.NoError(t, err)
			}

			fileChan := make(chan string, 3)

			converter := NewWhisperConverter(
				namePrefix,
				whisperDirectory,
				fileFilter,
				1,
				1,
				0,
				labels.FromStrings(),
				nil,
				log.NewNopLogger(),
			)

			converter.getWhisperListIntoChan(targetWhisperFiles, fileChan)

			files := make([]string, 0)

			for fname := range fileChan {
				files = append(files, fname)
			}

			expected := make([]string, 0)
			for _, element := range test.elementsMatch {
				expected = append(expected, tmpDir+"/"+element)
			}

			require.ElementsMatch(t, expected, files)
			require.Equal(t, test.skippedCount, converter.GetSkippedCount())
		})
	}
}

func TestGetMetricName(t *testing.T) {
	tests := []struct {
		name               string
		namePrefix         string
		whisperDirectory   string
		file               string
		expectedMetricName string
	}{
		{
			name:               "whisperDirectoryPathRemoved",
			namePrefix:         "",
			whisperDirectory:   "/output",
			file:               "/output/asdf/qwer/test.wsp",
			expectedMetricName: "asdf.qwer.test",
		},
		{
			name:               "whisperDirectoryNotRoot",
			namePrefix:         "",
			whisperDirectory:   "/output",
			file:               "/notoutput/asdf/qwer/test.wsp",
			expectedMetricName: "notoutput.asdf.qwer.test",
		},
		{
			name:               "specifyNamePrefix",
			namePrefix:         "namePrefix.",
			whisperDirectory:   "/output",
			file:               "/output/asdf/qwer/test.wsp",
			expectedMetricName: "namePrefix.asdf.qwer.test",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converter := NewWhisperConverter(
				test.namePrefix,
				test.whisperDirectory,
				regexp.MustCompile(`.+\.wsp$`),
				1,
				1,
				0,
				labels.FromStrings(),
				nil,
				log.NewNopLogger(),
			)
			metricName := converter.getMetricName(test.file)

			require.Equal(t, test.expectedMetricName, metricName)
		})
	}
}

type mockDirEntry struct {
	isDir bool
}

func (d *mockDirEntry) Name() string {
	return ""
}

func (d *mockDirEntry) IsDir() bool {
	return d.isDir
}

func (d *mockDirEntry) Type() fs.FileMode {
	return fs.FileMode(0)
}

func (d *mockDirEntry) Info() (fs.FileInfo, error) {
	return nil, nil
}

func TestIsMatchingFile(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		d                fs.DirEntry
		whisperDirectory string
		fileFilter       *regexp.Regexp
		skippedCount     uint64
		isMatching       bool
	}{
		{
			name:             "skipDirectory",
			path:             "/output/directory",
			d:                &mockDirEntry{isDir: true},
			whisperDirectory: "/output",
			fileFilter:       regexp.MustCompile(`\.wsp$`),
			skippedCount:     0,
			isMatching:       false,
		},
		{
			name:             "matchesFileFilter",
			path:             "/output/directory/asdf.wsp",
			d:                &mockDirEntry{isDir: false},
			whisperDirectory: "/output",
			fileFilter:       regexp.MustCompile(`\.wsp$`),
			skippedCount:     0,
			isMatching:       true,
		},
		{
			name:         "doesNotMatchFileFilter",
			path:         "/output/directory/asdf.spw",
			d:            &mockDirEntry{isDir: false},
			fileFilter:   regexp.MustCompile(`\.wsp$`),
			skippedCount: 1,
			isMatching:   false,
		},
		{
			name:         "Default file filter does not match no filename whisper file",
			path:         "/output/directory/.wsp",
			d:            &mockDirEntry{isDir: false},
			fileFilter:   regexp.MustCompile(`.+\.wsp$`),
			skippedCount: 1,
			isMatching:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converter := NewWhisperConverter(
				"",
				test.whisperDirectory,
				test.fileFilter,
				1,
				1,
				0,
				labels.FromStrings(),
				nil,
				log.NewNopLogger(),
			)

			isMatching := converter.isMatchingFile(test.path, test.d)

			require.Equal(t, test.isMatching, isMatching)
			require.Equal(t, test.skippedCount, converter.GetSkippedCount())
		})
	}
}
