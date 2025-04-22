package whisperconverter

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
)

// CommandFileList traverses the file structure starting from whisperDirectory, discovers
// files that match fileFilter, and outputs those files to the destination specified by targetWhisperFiles.
func (c *WhisperConverter) CommandFileList(targetWhisperFiles string) error {
	if targetWhisperFiles == "" {
		return fmt.Errorf("must specify output file using --target-whisper-files")
	}

	err := c.discoverTargetWhisperFiles(targetWhisperFiles)
	if err != nil {
		return errors.Wrap(err, "problem discovering target files")
	}
	return nil
}

// discoverTargetWhisperFiles discovers whisper files and writes them to targetWhisperFiles
func (c *WhisperConverter) discoverTargetWhisperFiles(targetWhisperFiles string) error {
	_ = level.Info(c.logger).Log("msg", "discovering target files", "outputFilename", targetWhisperFiles)

	f, err := os.Create(targetWhisperFiles)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	w := bufio.NewWriter(f)

	err = filepath.WalkDir(c.whisperDirectory, func(path string, d fs.DirEntry, pathErr error) error {
		if !c.isMatchingFile(path, d) {
			return nil
		}

		_, err = w.WriteString(path + "\n")
		if err != nil {
			return err
		}

		c.progress.IncProcessed()
		return nil
	})
	if err != nil {
		return err
	}

	err = w.Flush()
	if err != nil {
		return err
	}

	return nil
}
