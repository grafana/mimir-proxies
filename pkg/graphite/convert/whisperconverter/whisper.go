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
		return nil, fmt.Errorf("failed to open whisper file: %w", err)
	}
	defer func() {
		_ = fd.Close()
	}()
	w, err := newIOReaderArchive(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to open whisper archive: %w", err)
	}

	points, err := ReadPoints(w, name)
	if err != nil {
		return nil, fmt.Errorf("error dumping metric from whisper: %w", err)
	}

	return ToMimirSamples(points)
}

// Archive provides a testable interface for converting whisper databases.
type Archive interface {
	GetArchives() []whisper.ArchiveInfo
	DumpArchive(int) ([]whisper.Point, error)
}

// ReadPoints reads and concatenates all of the points in a whisper Archive.
func ReadPoints(w Archive, name string) ([]whisper.Point, error) {
	archives := w.GetArchives()
	if len(archives) == 0 {
		return nil, fmt.Errorf("whisper file contains no archives for metric: %q", name)
	}

	// Dump one precision level at a time and write into the output slice.
	// Its important to remember that the archive with index 0 (first archive)
	// has the raw data and the highest precision https://graphite.readthedocs.io/en/latest/whisper.html#archives-retention-and-precision
	keptPoints := []whisper.Point{}

	// We want to track the max timestamp of the archives because we know it
	// virtually represents now() and we won't have newer points.
	var maxTs, maxTsOffset uint32
	for i := range archives {
		// All archives share the same maxTs, so only calculate it once.
		if maxTs == 0 {
			if i > 0 {
				// If there are no points in the high-res archives, we have to bump up
				// maxTs by the difference in retention to the next higher archive so
				// that this point is validly in the retention for this archive. This
				// can happen when the only points added to a whisper archive are
				// significantly older than "Now()" at the time of writing, as happens
				// during our e2e test.
				maxTsOffset = archives[i-1].Retention()
			}
			points, err := w.DumpArchive(i)
			if err != nil {
				return nil, fmt.Errorf("failed to dump archive %d from whisper metric %s", i, name)
			}
			for _, p := range points {
				if p.Timestamp > maxTs {
					maxTs = p.Timestamp
				}
			}
		}
	}
	maxTs += maxTsOffset

	// Also determine the boundaries between archives.
	lowerBoundTs := make([]uint32, len(archives))
	for i, a := range archives {
		if maxTs < a.Retention() {
			// very big retention, boundary would be < 0, therefore all points are
			// covered by this archive.
			lowerBoundTs[i] = 0
		} else {
			lowerBoundTs[i] = maxTs - a.Retention()
		}
	}

	// no maxTs means no points. This is not an error.
	if maxTs == 0 {
		return []whisper.Point{}, nil
	}

	// Iterate over archives backwards so we process oldest points first. Sort the
	// points, then determine the slice that is between the bounds for this
	// archive, and append those to the output array.
	for i := len(archives) - 1; i >= 0; i-- {
		points, err := w.DumpArchive(i)
		if err != nil {
			return nil, fmt.Errorf("failed to dump archive %d from whisper metric %s", i, name)
		}

		if len(points) == 0 {
			continue
		}

		// Sort this archive.
		sort.Slice(points, func(i, j int) bool {
			return points[i].Timestamp < points[j].Timestamp
		})

		startIdx := -1
		endIdx := len(points) - 1
		for j, p := range points {
			if p.Timestamp == 0 {
				continue
			}
			// Don't include any points in this archive that are older than the
			// retention period.
			if p.Timestamp <= lowerBoundTs[i] {
				continue
			}
			// Don't include any points in this archive that are covered in a higher
			// resolution archive. If the other boundary is zero, it is invalid
			// so we keep the point.
			if i > 0 && p.Timestamp > lowerBoundTs[i-1] {
				break
			}
			endIdx = j
			if startIdx == -1 {
				startIdx = j
			}
		}
		// if startIdx is -1, we did not find any valid points.
		if startIdx != -1 {
			keptPoints = append(keptPoints, points[startIdx:endIdx+1]...)
		}
	}

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
