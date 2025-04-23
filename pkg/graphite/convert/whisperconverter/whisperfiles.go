package whisperconverter

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kit/log/level"
)

// getWhisperListIntoChan scans a directory and feed the list of whisper files
// relative to base into the given channel.
func (c *WhisperConverter) getWhisperListIntoChan(targetWhisperFiles string, fileChan chan string) {
	if targetWhisperFiles == "" {
		_ = level.Info(c.logger).Log("msg", "discovering target files")

		err := filepath.WalkDir(c.whisperDirectory, func(path string, d fs.DirEntry, pathErr error) error {
			if !c.isMatchingFile(path, d) {
				return nil
			}

			fileChan <- path
			return nil
		})
		if err != nil {
			_ = level.Error(c.logger).Log("path", c.whisperDirectory, "msg", "error walking path", "err", err)
			return
		}
	} else {
		_ = level.Info(c.logger).Log("msg", "reading target files from file", "file", targetWhisperFiles)

		f, err := os.Open(targetWhisperFiles)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "problem opening input file list", "file", targetWhisperFiles, "err", err)
		}
		defer func() {
			_ = f.Close()
		}()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fileChan <- scanner.Text()
		}

		if err = scanner.Err(); err != nil {
			_ = level.Error(c.logger).Log("msg", "problem scanning input file list", "err", err)
		}
	}

	close(fileChan)
}

// getMetricName generates the metric name based on the file name and given
// prefix.
func (c *WhisperConverter) getMetricName(file string) string {
	// remove all leading '/' from file name
	file = strings.TrimPrefix(file, c.whisperDirectory)
	for file[0] == '/' {
		file = file[1:]
	}

	return c.namePrefix + strings.ReplaceAll(strings.TrimSuffix(file, filepath.Ext(file)), "/", ".")
}

func (c *WhisperConverter) isMatchingFile(path string, d fs.DirEntry) bool {
	if d.IsDir() {
		return false
	}

	filename := filepath.Base(path)

	if !c.fileFilter.MatchString(filename) {
		_ = level.Debug(c.logger).Log("file", path, "metricname", c.getMetricName(path), "msg", "skipping file")
		c.progress.IncSkipped()
		return false
	}

	return true
}
