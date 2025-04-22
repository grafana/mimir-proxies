package remoteread

import (
	"errors"
	"io"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/util/annotations"

	labelsutil "github.com/grafana/mimir-proxies/pkg/util/labels"
)

// streamingSeriesSet implements storage.SeriesSet
type streamingSeriesSet struct {
	chunkedReader *remote.ChunkedReader
	respBody      io.ReadCloser
	finalizer     func()

	queryStartMs int64
	queryEndMs   int64

	current storage.Series
	err     error
}

func NewStreamingSeriesSet(chunkedReader *remote.ChunkedReader, respBody io.ReadCloser, queryStartMs, queryEndMs int64, finalizer func()) storage.SeriesSet {
	return &streamingSeriesSet{
		chunkedReader: chunkedReader,
		respBody:      respBody,
		finalizer:     finalizer,
		queryStartMs:  queryStartMs,
		queryEndMs:    queryEndMs,
	}
}

// Next return true if there is a next series and false otherwise. It will
// block until the next series is available.
func (s *streamingSeriesSet) Next() bool {
	res := &prompb.ChunkedReadResponse{}

	err := s.chunkedReader.NextProto(res)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			s.err = err
			_, _ = io.Copy(io.Discard, s.respBody)
		}

		_ = s.respBody.Close()
		s.finalizer()

		return false
	}

	s.current = &streamingSeries{
		labels:       res.ChunkedSeries[0].Labels,
		chunks:       res.ChunkedSeries[0].Chunks,
		queryStartMs: s.queryStartMs,
		queryEndMs:   s.queryEndMs,
	}

	return true
}

func (s *streamingSeriesSet) At() storage.Series {
	return s.current
}

func (s *streamingSeriesSet) Err() error {
	return s.err
}

func (s *streamingSeriesSet) Warnings() annotations.Annotations {
	return nil
}

// streamingSeries implements storage.Series
type streamingSeries struct {
	labels       []prompb.Label
	chunks       []prompb.Chunk
	queryStartMs int64
	queryEndMs   int64
}

func (s *streamingSeries) Labels() labels.Labels {
	return labelsutil.LabelProtosToLabels(s.labels)
}

func (s *streamingSeries) Iterator(_ chunkenc.Iterator) chunkenc.Iterator {
	return newStreamingSeriesIterator(s.chunks, s.queryStartMs, s.queryEndMs)
}

// streamingSeriesIterator implements chunkenc.Iterator
type streamingSeriesIterator struct {
	chunks []prompb.Chunk
	idx    int

	cur chunkenc.Iterator
	// curType is the type of the sample pointed to by cur
	curType chunkenc.ValueType

	queryStartMs int64
	queryEndMs   int64

	err error
}

func newStreamingSeriesIterator(chunks []prompb.Chunk, queryStartMs, queryEndMs int64) *streamingSeriesIterator {
	it := &streamingSeriesIterator{
		chunks:       chunks,
		idx:          0,
		curType:      chunkenc.ValNone,
		queryStartMs: queryStartMs,
		queryEndMs:   queryEndMs,
	}
	if len(chunks) > 0 {
		it.resetIterator()
	}

	return it
}

func (it *streamingSeriesIterator) Next() chunkenc.ValueType {
	if it.err != nil {
		return chunkenc.ValNone
	}
	if len(it.chunks) == 0 {
		return chunkenc.ValNone
	}

	for it.curType = it.cur.Next(); it.curType != chunkenc.ValNone; it.curType = it.cur.Next() {
		atT := it.AtT()
		if atT > it.queryEndMs {
			it.chunks = nil // Exhaust this iterator so follow-up calls to Next or Seek return fast.
			return chunkenc.ValNone
		}
		if atT >= it.queryStartMs {
			return it.curType
		}
	}

	if it.idx == len(it.chunks)-1 {
		return chunkenc.ValNone
	}

	it.idx++
	it.resetIterator()
	it.curType = it.Next()
	return it.curType
}

func (it *streamingSeriesIterator) Seek(t int64) chunkenc.ValueType { //nolint:govet
	if it.err != nil {
		return chunkenc.ValNone
	}
	if len(it.chunks) == 0 {
		return chunkenc.ValNone
	}

	iteratorAdvanced := false
	for it.chunks[it.idx].MaxTimeMs < t {
		if it.idx == len(it.chunks)-1 {
			return chunkenc.ValNone
		}
		it.idx++
		iteratorAdvanced = true
	}
	if iteratorAdvanced {
		it.resetIterator()
	}

	// We must check if the current sample is valid before advancing the iterator.
	if it.curType != chunkenc.ValNone && it.AtT() >= t {
		return it.curType
	}

	for it.curType = it.cur.Next(); it.curType != chunkenc.ValNone; it.curType = it.cur.Next() {
		atT := it.AtT()
		if atT > it.queryEndMs {
			it.chunks = nil // Exhaust this iterator so follow-up calls to Next or Seek return fast.
			break
		}
		if atT >= it.queryStartMs && atT >= t {
			return it.curType
		}
	}

	// Failed to find a corresponding sample.
	return chunkenc.ValNone
}

func (it *streamingSeriesIterator) resetIterator() {
	chunk := it.chunks[it.idx]

	decodedChunk, err := chunkenc.FromData(chunkenc.Encoding(chunk.Type), chunk.Data)
	if err != nil {
		it.err = err
		return
	}

	it.cur = decodedChunk.Iterator(nil)
	it.curType = chunkenc.ValNone
}

func (it *streamingSeriesIterator) At() (ts int64, v float64) {
	return it.cur.At()
}

func (it *streamingSeriesIterator) Err() error {
	return it.err
}

func (it *streamingSeriesIterator) AtFloatHistogram(fh *histogram.FloatHistogram) (int64, *histogram.FloatHistogram) {
	return it.cur.AtFloatHistogram(fh)
}

func (it *streamingSeriesIterator) AtHistogram(h *histogram.Histogram) (int64, *histogram.Histogram) {
	return it.cur.AtHistogram(h)
}

func (it *streamingSeriesIterator) AtT() int64 {
	return it.cur.AtT()
}
