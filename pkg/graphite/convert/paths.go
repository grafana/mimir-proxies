package convert

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/mimir-graphite/v2/pkg/tsdb"
)

// PathsForWorker returns a subset of the paths slice for the worker with the
// given index.  If there are more workers than there are paths, and the worker
// ID is greater than the number of paths, the returned slice will be empty.
// If the worker ID is greater than the workerCount, returns empty.
func PathsForWorker(paths []string, workerCount, workerID int) []string {
	if workerCount > len(paths) {
		if workerID >= len(paths) {
			return []string{}
		}
		return paths[workerID:workerID]
	}

	// Starting with our ID, take every workerCount path and add it to the subset
	// until we hit the end.
	subset := []string{}
	for i := workerID; i < len(paths); i += workerCount {
		subset = append(subset, paths[i])
	}
	return subset
}

// GetFinishedBlockDates walks the blocks directory and builds a list of dates
// that have already been processed.
func GetFinishedBlockDates(blocksDirectory string) (map[time.Time]bool, error) {
	metaFilter := regexp.MustCompile(`/meta.json$`)
	blockPaths := []string{}
	err := filepath.Walk(
		blocksDirectory,
		func(path string, info os.FileInfo, err error) error {
			if metaFilter.MatchString(path) {
				blockPaths = append(blockPaths, strings.Replace(path, "meta.json", "", 1))
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	skippableDates := make(map[time.Time]bool)
	for _, p := range blockPaths {
		meta, err := tsdb.ReadMetaFile(p)
		if err != nil {
			continue
		}
		t := time.UnixMilli(meta.MinTime).UTC()
		rounded := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		skippableDates[rounded] = true
	}

	return skippableDates, nil
}
