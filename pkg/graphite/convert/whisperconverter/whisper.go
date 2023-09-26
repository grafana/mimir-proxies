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
	if len(w.GetArchives()) == 0 {
		return nil, fmt.Errorf("whisper file contains no archives for metric: %q", name)
	}

	// Dump one precision level at a time and write into the output slice.
	// Its important to remember that the archive with index 0 (first archive)
	// has the raw data and the highest precision https://graphite.readthedocs.io/en/latest/whisper.html#archives-retention-and-precision
	seenTs := map[uint32]struct{}{}
	var allKeptPoints [][]pointWithPrecision
	for i, a := range w.GetArchives() {
		archivePoints, err := w.DumpArchive(i)
		if err != nil {
			return nil, fmt.Errorf("failed to dump archive %d from whisper metric %s", i, name)
		}

		var minArchiveTs, maxArchiveTs uint32
		for _, p := range archivePoints {
			// We want to track the max timestamp of the archive because we know
			// it virtually represents now() and we wont have newer points.
			// Then the min timestamp of the archive would be maxTs - the archive
			// retention.
			if p.Timestamp > maxArchiveTs {
				maxArchiveTs = p.Timestamp
			}
		}

		if a.Retention() > maxArchiveTs {
			minArchiveTs = 0
		} else {
			minArchiveTs = maxArchiveTs - a.Retention()
		}

		var keptPoints []pointWithPrecision
		for _, p := range archivePoints {
			if p.Timestamp < minArchiveTs {
				continue
			}
			// If we have already seen a point with the same timestamp it means
			// we already have a point from an archive with higher precision that
			// we want to keep. So we skip this point.
			if _, ok := seenTs[p.Timestamp]; ok {
				continue
			}
			keptPoints = append(keptPoints, pointWithPrecision{p, a.SecondsPerPoint})
			seenTs[p.Timestamp] = struct{}{}
		}

		// Points are not necessarily in order because the archive is a ring buffer
		// so we order the slice
		sort.Slice(keptPoints, func(i, j int) bool {
			return keptPoints[i].Timestamp < keptPoints[j].Timestamp
		})

		allKeptPoints = append(allKeptPoints, keptPoints)
	}

	trimmedPoints := []whisper.Point{}
	for _, points := range allKeptPoints {
		for _, p := range points {
			// Remove all points of time = 0.
			if p.Timestamp == 0 {
				continue
			}
			trimmedPoints = append(trimmedPoints, p.Point)
		}
	}

	// We need to finally sort the trimmed points again because different archives
	// may overlap and have older points
	sort.Slice(trimmedPoints, func(i, j int) bool {
		return trimmedPoints[i].Timestamp < trimmedPoints[j].Timestamp
	})

	return trimmedPoints, nil
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
