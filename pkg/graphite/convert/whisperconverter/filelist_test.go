package whisperconverter

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestCommandFileList(t *testing.T) {
	tests := []struct {
		name           string
		outputFile     string
		testFiles      []string
		expectedFiles  []string
		processedCount uint64
		skippedCount   uint64
	}{
		{
			name:           "discoverTargetWhisperFiles",
			outputFile:     "asdf.out",
			testFiles:      []string{"test1.skip", "test2.skip", "desc/test3.skip", "test1.wsp", "desc/test2.wsp"},
			expectedFiles:  []string{"test1.wsp", "desc/test2.wsp"},
			processedCount: 2,
			skippedCount:   4, // The extra skipped file is targetWhisperFiles
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("/tmp", "testCommandFileList*")
			require.NoError(t, err)
			defer func() {
				_ = os.RemoveAll(tmpDir)
			}()

			for _, filename := range test.testFiles {
				fullPath := tmpDir + "/" + filename
				dirPath := filepath.Dir(fullPath)

				err = os.MkdirAll(dirPath, 0700)
				require.NoError(t, err)

				_, err = os.Create(fullPath)
				require.NoError(t, err)
			}

			whisperDirectory := tmpDir
			fileFilter := regexp.MustCompile(`\.wsp$`)
			namePrefix := ""

			targetWhisperFiles := ""
			if test.outputFile != "" {
				targetWhisperFiles = tmpDir + "/" + test.outputFile
			}

			c := NewWhisperConverter(
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

			err = c.CommandFileList(targetWhisperFiles)
			require.NoError(t, err)

			actualFiles, err := getFileContents(targetWhisperFiles)
			require.NoError(t, err)

			// Add the full path to test.expectedFiles (we have to do it now
			// the path of tmpDir isn't known until it's created).
			expectedFiles := make([]string, 0)
			for _, file := range test.expectedFiles {
				expectedFiles = append(expectedFiles, tmpDir+"/"+file)
			}

			require.ElementsMatch(t, expectedFiles, actualFiles)

			require.Equal(t, test.processedCount, c.GetProcessedCount())
			require.Equal(t, test.skippedCount, c.GetSkippedCount())
		})
	}
}

func getFileContents(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	output := make([]string, 0)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		output = append(output, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return output, nil
}
