package whisperconverter

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log/level"
)

// CommandDateRange prints the minimum and maximum timestamps for the dataset in
// a format suitable for use as arguments to pass1 and pass2. targetWhisperFiles
// is a filename containing the list of files to process, or if blank, files
// will be walked using c.whisperDirectory.
func (c *WhisperConverter) CommandDateRange(targetWhisperFiles string) {
	fileChan := make(chan string)
	readsDoneCh := make(chan interface{})

	wgReads := &sync.WaitGroup{}
	wgReads.Add(c.threads)
	wgProcess := &sync.WaitGroup{}
	wgProcess.Add(1)
	tsChan := make(chan int64)
	for i := 0; i < c.threads; i++ {
		go c.getTimestampBounds(fileChan, tsChan, wgReads)
	}
	go c.collectTimestamps(tsChan, readsDoneCh, wgProcess)
	c.getWhisperListIntoChan(targetWhisperFiles, fileChan)

	wgReads.Wait()
	close(readsDoneCh)
	wgProcess.Wait()
}

// getTimestampBounds reads whisper files and determines the min and max
// timestamps for all the points in the archive.  It then passes these values
// to a processing channel to find the min and max over all of the archives.
func (c *WhisperConverter) getTimestampBounds(files chan string, tsChan chan<- int64, wg *sync.WaitGroup) {
	for fname := range files {
		metricName := c.getMetricName(fname)
		level.Info(c.logger).Log("file", fname, "metric", metricName, "msg", "processing file")
		samples, err := WhisperToMimirSamples(fname, metricName)
		if err != nil {
			level.Warn(c.logger).Log("file", fname, "metric", metricName, "msg", "error converting whisper metric", "err", err)
			c.progress.IncSkipped()
			continue
		}

		minTS := int64(math.MaxInt64)
		maxTS := int64(math.MinInt64)

		for _, s := range samples {
			if s.TimestampMs < minTS {
				minTS = s.TimestampMs
			}
			if s.TimestampMs > maxTS {
				maxTS = s.TimestampMs
			}
		}

		tsChan <- minTS
		tsChan <- maxTS

		c.progress.IncProcessed()
	}
	wg.Done()
}

// collectTimestamps listens for timestamps over the channel and determines the
// min and max of all it sees, printing out the results in a format suitable for
// passing to the first and second pass executions of the converter.
func (c *WhisperConverter) collectTimestamps(tsChan <-chan int64, doneCh <-chan interface{}, wg *sync.WaitGroup) {
	minTS := int64(math.MaxInt64)
	maxTS := int64(math.MinInt64)

READLOOP:
	for {
		select {
		case <-doneCh:
			break READLOOP
		case ts := <-tsChan:
			if ts < minTS {
				minTS = ts
			}
			if ts > maxTS {
				maxTS = ts
			}
		}
	}

	terms := []string{}

	if minTS != int64(math.MaxInt64) {
		terms = append(terms, fmt.Sprintf("--start-date %s", time.UnixMilli(minTS).UTC().Format("2006-01-02")))
	}
	if maxTS != int64(math.MinInt64) {
		terms = append(terms, fmt.Sprintf("--end-date %s", time.UnixMilli(maxTS).UTC().Format("2006-01-02")))
	}
	terms = append(terms, "\n")
	fmt.Printf("%s", strings.Join(terms, " "))

	wg.Done()
}
