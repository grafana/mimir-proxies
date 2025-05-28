//go:build !race

package whisperconverter

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/go-graphite/go-whisper"
	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

// This test contains a data due to how we take over STDOUT but it should be harmless.
func TestCommandDateRange(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "testCommandDateRange*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	whisper.Now = func() time.Time {
		t, err := time.Parse("2006-01-02", "2022-06-01")
		if err != nil {
			panic(err)
		}
		return t
	}

	fooTimes, err := ToTimes([]string{
		"2022-05-01",
		"2022-05-02",
		"2022-05-03",
		"2022-05-04",
	})
	require.NoError(t, err)

	err = CreateWhisperFile(tmpDir+"/foo.wsp", fooTimes)
	require.NoError(t, err)

	barTimes, err := ToTimes([]string{
		"2018-02-01",
		"2018-02-02",
		"2018-02-03",
	})
	require.NoError(t, err)

	err = CreateWhisperFile(tmpDir+"/bar.wsp", barTimes)
	require.NoError(t, err)

	namePrefix := ""
	targetWhisperFiles := ""
	threads := 1
	c := NewWhisperConverter(
		namePrefix,
		tmpDir,
		regexp.MustCompile(`\.wsp$`),
		threads,
		1,
		0,
		labels.FromStrings(),
		nil,
		log.NewNopLogger(),
	)

	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	c.CommandDateRange(targetWhisperFiles)

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer

		_, err = io.Copy(&buf, r)
		require.NoError(t, err)

		outC <- buf.String()
	}()

	err = w.Close()
	require.NoError(t, err)
	os.Stdout = stdout

	out := <-outC
	require.Equal(t, "--start-date 2018-02-01 --end-date 2022-05-04 \n", out)

	require.Equal(t, uint64(2), c.GetProcessedCount())
	require.Equal(t, uint64(0), c.GetSkippedCount())
}
