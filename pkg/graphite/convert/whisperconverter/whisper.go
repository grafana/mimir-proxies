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

	totalPoints := 0
	for _, a := range w.GetArchives() {
		totalPoints += int(a.Points)
	}

	// Preallocate space for all allPoints in one slice.
	allPoints := make([]pointWithPrecision, totalPoints)
	pIdx := 0
	// Dump one precision level at a time and write into the output slice.
	for i, a := range w.GetArchives() {
		archivePoints, err := w.DumpArchive(i)
		if err != nil {
			return nil, fmt.Errorf("failed to dump archive %d from whisper metric %s", i, name)
		}
		for j, p := range archivePoints {
			allPoints[pIdx+j] = pointWithPrecision{p, a.SecondsPerPoint}
		}
		pIdx += len(archivePoints)
	}

	// Points must be in time order.
	sort.Slice(allPoints, func(i, j int) bool {
		return allPoints[i].Timestamp < allPoints[j].Timestamp
	})

	trimmedPoints := []whisper.Point{}
	for i := 0; i < len(allPoints); i++ {
		// Remove all points of time = 0.
		if allPoints[i].Timestamp == 0 {
			continue
		}
		// There might be duplicate timestamps in different archives. Take the
		// higher-precision archive value since it's unaggregated.
		if i > 0 && allPoints[i].Timestamp == allPoints[i-1].Timestamp {
			if allPoints[i].secondsPerPoint == allPoints[i-1].secondsPerPoint {
				return nil, fmt.Errorf("duplicate timestamp at same precision in archive %s: %d", name, allPoints[i].Timestamp)
			}
			if allPoints[i].secondsPerPoint < allPoints[i-1].secondsPerPoint {
				trimmedPoints[len(trimmedPoints)-1] = allPoints[i].Point
			}
			// If the previous point is higher precision, just continue.
			continue
		}
		trimmedPoints = append(trimmedPoints, allPoints[i].Point)
	}

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
