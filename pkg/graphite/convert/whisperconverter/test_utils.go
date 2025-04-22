package whisperconverter

import (
	"os"
	"time"

	"github.com/go-graphite/go-whisper"
)

const (
	layout              = "2006-01-02"
	defaultXFilesFactor = 0.5
)

func CreateWhisperFile(path string, timestamps []*time.Time) error {
	retentions, err := whisper.ParseRetentionDefs("1s:1d,1h:5w,1d:200y")
	if err != nil {
		return err
	}

	wsp, err := whisper.Create(path, retentions, whisper.Sum, defaultXFilesFactor)
	if err != nil {
		return err
	}
	defer func() {
		_ = wsp.Close()
	}()

	for _, t := range timestamps {
		err = wsp.Update(1.0, int(t.Unix()))
		if err != nil {
			return err
		}
	}

	return nil
}

func ToTimes(in []string) ([]*time.Time, error) {
	out := make([]*time.Time, 0)

	for _, tin := range in {
		tout, err := ToTime(tin)
		if err != nil {
			return nil, err
		}

		out = append(out, &tout)
	}

	return out, nil
}

func ToTime(in string) (time.Time, error) {
	return time.Parse(layout, in)
}

func ListFilesInDir(dirPath string) ([]string, error) {
	files := make([]string, 0)

	fileInfos, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, f := range fileInfos {
		files = append(files, f.Name())
	}

	return files, nil
}
