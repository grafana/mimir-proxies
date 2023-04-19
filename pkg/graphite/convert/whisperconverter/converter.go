package whisperconverter

import (
	"regexp"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/mimir-proxies/pkg/graphite/convert"
	"github.com/prometheus/prometheus/model/labels"
)

// WhisperConverter is an object for performing various steps of the conversion
// from Whisper files to Mimir blocks.
type WhisperConverter struct {
	// namePrefix is the tprefix to prepend before every metric name, should
	// include the '.' if necessary.
	namePrefix string
	// whisperDirectory contains the whisper file structure.
	whisperDirectory string
	// fileFilter will be applied to all incoming files to determine if they
	// should be converted.
	fileFilter *regexp.Regexp
	// threads is the number of goroutines to use when executing.
	threads int
	// workerCount is the total number of separate binaries running at once.
	workerCount int
	// workerID is the 0-workerCount index of this particular binary.
	workerID int
	// customLabels will be applied to all metrics when generating blocks.
	customLabels labels.Labels
	// dates is the list of all dates to process.
	dates []time.Time

	logger   log.Logger
	progress *convert.Progress
}

func NewWhisperConverter(
	namePrefix,
	whisperDirectory string,
	fileFilter *regexp.Regexp,
	threads,
	workerCount,
	workerID int,
	customLabels labels.Labels,
	dates []time.Time,
	logger log.Logger) *WhisperConverter {
	return &WhisperConverter{
		namePrefix:       namePrefix,
		whisperDirectory: whisperDirectory,
		fileFilter:       fileFilter,
		threads:          threads,
		workerCount:      workerCount,
		workerID:         workerID,
		customLabels:     customLabels,
		dates:            dates,
		logger:           logger,
		progress:         convert.NewProgress(logger),
	}
}

func (c *WhisperConverter) GetProcessedCount() uint64 {
	return c.progress.GetProcessedCount()
}

func (c *WhisperConverter) GetSkippedCount() uint64 {
	return c.progress.GetSkippedCount()
}
