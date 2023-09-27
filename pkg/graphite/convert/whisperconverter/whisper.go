package whisperconverter

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/kisielk/whisper-go/whisper"
)

// WhisperToMimirSamples opens the given whisper file, applying the given metric
// name, and writes it to the given block directory with blocks covering the
// given duration.
func WhisperToMimirSamples(whisperFile, name string) ([]mimirpb.Sample, error) {
	fd, err := os.Open(whisperFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open whisper file: %v", err)
	}
	defer fd.Close()
	w, err := newIOReaderArchive(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to open whisper archive: %v", err)
	}

	points, err := ReadPoints(w, name)
	if err != nil {
		return nil, fmt.Errorf("error dumping metric from whisper: %v", err)
	}

	return ToMimirSamples(points)
}

// Archive provides a testable interface for converting whisper databases.
type Archive interface {
	GetArchives() []whisper.ArchiveInfo
	DumpArchive(int) ([]whisper.Point, error)
}

// pointWithPrecision is a whisper Point with the precision of the archive it
// came from. This is used to differentiate when we have duplicate timestamps at
// different precisions.
type pointWithPrecision struct {
	whisper.Point
	secondsPerPoint uint32
}

// ReadPoints reads and concatenates all of the points in a whisper Archive.
func ReadPoints(w Archive, name string) ([]whisper.Point, error) {
	archives := w.GetArchives()
	if len(archives) == 0 {
		return nil, fmt.Errorf("whisper file contains no archives for metric: %q", name)
	}

	// Ensure archives are sorted by precision just in case.
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].SecondsPerPoint < archives[j].SecondsPerPoint
	})

	// Dump one precision level at a time and write into the output slice.
	// Its important to remember that the archive with index 0 (first archive)
	// has the raw data and the highest precision https://graphite.readthedocs.io/en/latest/whisper.html#archives-retention-and-precision
	archivePoints := make([][]whisper.Point, len(archives))
	var keptPoints []whisper.Point
	// We want to track the max timestamp of the archives because we know
	// it virtually represents now() and we wont have newer points.
	// Then the min timestamp of the archive would be maxTs - each archive
	// retention.
	var maxTs uint32
	for i, a := range archives {
		points, err := w.DumpArchive(i)
		if err != nil {
			return nil, fmt.Errorf("failed to dump archive %d from whisper metric %s", i, name)
		}

		// All archives share the same maxArchiveTs, so only calculate it once.
		if i == 0 {
			for _, p := range points {
				if p.Timestamp > maxTs {
					maxTs = p.Timestamp
				}
			}
		}

		var minArchiveTs uint32
		if maxTs < a.Retention() { // very big retention
			minArchiveTs = 0
		} else {
			minArchiveTs = maxTs - a.Retention()
		}

		// Sort this archive.
		sort.Slice(points, func(i, j int) bool {
			return points[i].Timestamp < points[j].Timestamp
		})
		archivePoints[i] = points

		// Store a number of indexes so we can look for duplicate points efficiently.
		archiveIdx := make([]int, i)

POINTLOOP:
		for _, p := range archivePoints[i] {
			// Skip points with time = 0
			if p.Timestamp == 0 {
				continue
			}
			if p.Timestamp < minArchiveTs {
				continue
			}

			// For each of the previous archives, check to see if a point already
			// exists at this timestamp. If it does, we don't add this point and
			// keep the higher resolution point.
			for x := range archiveIdx {
				for {
					if archiveIdx[x] >= len(archivePoints[x]) {
						break
					}
					// We found a match, so skip this point
					if archivePoints[x][archiveIdx[x]].Timestamp == p.Timestamp {
						continue POINTLOOP
					}
					// The previous archive does not have this point, so stop.
					if archivePoints[x][archiveIdx[x]].Timestamp > p.Timestamp {
						break
					}
					archiveIdx[x]++
				}
			}

			keptPoints = append(keptPoints, whisper.Point{Timestamp: p.Timestamp, Value: p.Value})
		}
	}

	// We need to finally sort the kept points again because different archives
	// may overlap and have older points
	sort.Slice(keptPoints, func(i, j int) bool {
		return keptPoints[i].Timestamp < keptPoints[j].Timestamp
	})

	return keptPoints, nil
}

// ToMimirSamples converts a Whisper metric with the given name to a slice of
// labels and series of mimir samples.  Returns error if no points.
func ToMimirSamples(points []whisper.Point) ([]mimirpb.Sample, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("no points to convert for metric")
	}

	samples := make([]mimirpb.Sample, 0, len(points))

	for _, md := range points {
		s := mimirpb.Sample{
			Value:       md.Value,
			TimestampMs: md.Time().UnixMilli(),
		}
		samples = append(samples, s)
	}

	return samples, nil
}

// SplitSamplesByDays separates a slice of samples over the midnight UTC
// boundary.
func SplitSamplesByDays(samples []mimirpb.Sample) [][]mimirpb.Sample {
	blocks := [][]mimirpb.Sample{}
	if len(samples) == 0 {
		return blocks
	}

	// Create blocks from midnight UTC to midnight UTC
	blockStartIdx := 0
	idx := 0
	blockStartTime := time.UnixMilli(samples[0].TimestampMs).UTC()
	blockYear, blockMonth, blockDay := blockStartTime.Date()
	for ; idx < len(samples); idx++ {
		sampleTime := time.UnixMilli(samples[idx].TimestampMs).UTC()
		sampleYear, sampleMonth, sampleDay := sampleTime.Date()
		if sampleDay != blockDay || sampleMonth != blockMonth || sampleYear != blockYear {
			blocks = append(blocks, samples[blockStartIdx:idx])
			blockStartIdx = idx
			blockDay = sampleDay
			blockMonth = sampleMonth
			blockYear = sampleYear
		}
	}
	blocks = append(blocks, samples[blockStartIdx:])
	return blocks
}

type ioReaderArchive struct {
	*whisper.Whisper
}

// newIOReaderArchive opens a whisper archive and returns a new
// Whisper Archive-compatible pointer.
func newIOReaderArchive(fd io.ReadWriteSeeker) (*ioReaderArchive, error) {
	w, err := whisper.OpenWhisper(fd)
	if err != nil {
		return nil, err
	}
	return &ioReaderArchive{w}, nil
}

func (w *ioReaderArchive) GetArchives() []whisper.ArchiveInfo {
	return w.Header.Archives
}
