package convert

import (
	"fmt"
	"sync/atomic"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Progress is a concurrent-safe type for tracking the progress of the file
// conversion. It can keep track of the number of processed and skipped records.
// Every 100 processed files it emits an info log line.
type Progress struct {
	processedCount uint64
	skippedCount   uint64

	logger log.Logger
}

func NewProgress(logger log.Logger) *Progress {
	return &Progress{
		processedCount: 0,
		skippedCount:   0,
		logger:         logger,
	}
}

// IncProcessed atomically increases the processed count and prints a message if processing is complete.
func (p *Progress) IncProcessed() {
	processed := atomic.AddUint64(&p.processedCount, 1)
	if processed%100 == 0 {
		skipped := p.GetSkippedCount()
		_ = level.Info(p.logger).Log("msg", fmt.Sprintf("Processed %d files, %d skipped", processed, skipped))
	}
}

// IncSkipped atomically increases the skipped file count.
func (p *Progress) IncSkipped() {
	atomic.AddUint64(&p.skippedCount, 1)
}

// GetProcessedCount atomically loads and returns the current processed count.
func (p *Progress) GetProcessedCount() uint64 {
	return atomic.LoadUint64(&p.processedCount)
}

// GetSkippedCount atomically loads and returns the current skipped count.
func (p *Progress) GetSkippedCount() uint64 {
	return atomic.LoadUint64(&p.skippedCount)
}
