package remoteread

import (
	"bytes"
	"io"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/stretchr/testify/require"
)

func TestStreamingSeriesIterator(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		chunks := buildTestChunks(t)

		it := newStreamingSeriesIterator(chunks, 0, 15000)

		require.Nil(t, it.err)
		require.NotNil(t, it.cur)

		// Initial next; advance to first sample of first chunk.
		res := it.Next()
		require.Equal(t, chunkenc.ValFloat, res)
		require.Nil(t, it.Err())

		ts, v := it.At()
		require.Equal(t, int64(0), ts)
		require.Equal(t, float64(0), v)

		// Next to the second sample of the first chunk.
		res = it.Next()
		require.Equal(t, chunkenc.ValFloat, res)
		require.Nil(t, it.Err())

		ts, v = it.At()
		require.Equal(t, int64(1000), ts)
		require.Equal(t, float64(1), v)

		// Attempt to seek to the first sample of the first chunk (should return current sample).
		res = it.Seek(0)
		require.Equal(t, chunkenc.ValFloat, res)

		ts, v = it.At()
		require.Equal(t, int64(1000), ts)
		require.Equal(t, float64(1), v)

		// Seek to the end of the first chunk.
		res = it.Seek(4000)
		require.Equal(t, chunkenc.ValFloat, res)

		ts, v = it.At()
		require.Equal(t, int64(4000), ts)
		require.Equal(t, float64(4), v)

		// Next to the first sample of the second chunk.
		res = it.Next()
		require.Equal(t, chunkenc.ValFloat, res)
		require.Nil(t, it.Err())

		ts, v = it.At()
		require.Equal(t, int64(5000), ts)
		require.Equal(t, float64(1), v)

		// Seek to the second sample of the third chunk.
		res = it.Seek(10999)
		require.Equal(t, chunkenc.ValFloat, res)
		require.Nil(t, it.Err())

		ts, v = it.At()
		require.Equal(t, int64(11000), ts)
		require.Equal(t, float64(3), v)

		// Attempt to seek to something past the last sample (should return false and do nothing otherwise).
		res = it.Seek(99999)
		require.Equal(t, chunkenc.ValNone, res)
		require.Nil(t, it.Err())

		// Next to the last sample.
		for i := 0; i < 3; i++ {
			res = it.Next()
			require.Equal(t, chunkenc.ValFloat, res)
			require.Nil(t, it.Err())
		}

		// Attempt to next past the last sample (should return false).
		res = it.Next()
		require.Equal(t, chunkenc.ValNone, res)
		require.Nil(t, it.Err())
	})

	t.Run("invalid chunk encoding error", func(t *testing.T) {
		chunks := buildTestChunks(t)

		// Set chunk type to an invalid value.
		chunks[0].Type = 8

		it := newStreamingSeriesIterator(chunks, 0, 15000)

		res := it.Next()
		require.Equal(t, chunkenc.ValNone, res)

		res = it.Seek(1000)
		require.Equal(t, chunkenc.ValNone, res)

		require.ErrorContains(t, it.err, "invalid chunk encoding")
		require.Nil(t, it.cur)
	})

	t.Run("empty chunks", func(t *testing.T) {
		emptyChunks := make([]prompb.Chunk, 0)

		it1 := newStreamingSeriesIterator(emptyChunks, 0, 15000)
		require.Equal(t, chunkenc.ValNone, it1.Next())
		require.Equal(t, chunkenc.ValNone, it1.Seek(1000))
		require.NoError(t, it1.Err())

		var nilChunks []prompb.Chunk

		it2 := newStreamingSeriesIterator(nilChunks, 0, 15000)
		require.Equal(t, chunkenc.ValNone, it2.Next())
		require.Equal(t, chunkenc.ValNone, it2.Seek(1000))
		require.NoError(t, it2.Err())
	})

	t.Run("query time range", func(t *testing.T) {
		chunks := buildTestChunks(t)

		it1 := newStreamingSeriesIterator(chunks, 4000, 12000)

		require.Equal(t, chunkenc.ValFloat, it1.Seek(1000))
		ts, v := it1.At()
		require.Equal(t, int64(4000), ts)
		require.Equal(t, float64(4), v)

		require.Equal(t, chunkenc.ValFloat, it1.Seek(12000))
		ts, v = it1.At()
		require.Equal(t, int64(12000), ts)
		require.Equal(t, float64(4), v)

		// Try to Seek or Next on an exhausted iterator.
		require.Equal(t, chunkenc.ValNone, it1.Seek(15000))
		require.Equal(t, chunkenc.ValNone, it1.Seek(20000))
		require.Equal(t, chunkenc.ValNone, it1.Next())

		it2 := newStreamingSeriesIterator(chunks, 4000, 12000)

		require.Equal(t, chunkenc.ValFloat, it2.Next())
		ts, v = it2.At()
		require.Equal(t, int64(4000), ts)
		require.Equal(t, float64(4), v)

		// Next to the last sample in the query range.
		for i := 0; i < 8; i++ {
			require.Equal(t, chunkenc.ValFloat, it2.Next())
		}

		// Try to Seek or Next on an exhausted iterator.
		require.Equal(t, chunkenc.ValNone, it2.Next())
		require.Equal(t, chunkenc.ValNone, it2.Seek(15000))
	})
}

func TestStreamingSeries(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		chunks := buildTestChunks(t)

		s := streamingSeries{
			labels: []prompb.Label{
				{Name: "foo", Value: "bar"},
				{Name: "asdf", Value: "zxcv"},
			},
			chunks: chunks,
		}

		require.Equal(t, labels.Labels{
			{Name: "asdf", Value: "zxcv"},
			{Name: "foo", Value: "bar"},
		}, s.Labels())

		it := s.Iterator(nil)
		res := it.Next() // Behavior is undefined w/o the initial call to Next.

		require.Equal(t, chunkenc.ValFloat, res)
		require.Nil(t, it.Err())

		ts, v := it.At()
		require.Equal(t, int64(0), ts)
		require.Equal(t, float64(0), v)
	})
}

func TestStreamingSeriesSet(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		buf := &bytes.Buffer{}
		flusher := &mockFlusher{}

		w := remote.NewChunkedWriter(buf, flusher)
		r := remote.NewChunkedReader(buf, config.DefaultChunkedReadLimit, nil)

		chunks := buildTestChunks(t)
		l := []prompb.Label{
			{Name: "foo", Value: "bar"},
		}

		for i, c := range chunks {
			cSeries := prompb.ChunkedSeries{Labels: l, Chunks: []prompb.Chunk{c}}
			readResp := prompb.ChunkedReadResponse{
				ChunkedSeries: []*prompb.ChunkedSeries{&cSeries},
				QueryIndex:    int64(i),
			}

			b, err := proto.Marshal(&readResp)
			require.NoError(t, err)

			_, err = w.Write(b)
			require.NoError(t, err)
		}

		ss := NewStreamingSeriesSet(r, io.NopCloser(buf), 0, 15000, func() {})
		require.Nil(t, ss.Err())
		require.Nil(t, ss.Warnings())

		res := ss.Next()
		require.True(t, res)
		require.Nil(t, ss.Err())

		s := ss.At()
		require.Equal(t, 1, s.Labels().Len())
		require.True(t, s.Labels().Has("foo"))
		require.Equal(t, "bar", s.Labels().Get("foo"))

		it := s.Iterator(nil)
		it.Next()
		ts, v := it.At()
		require.Equal(t, int64(0), ts)
		require.Equal(t, float64(0), v)

		numResponses := 1
		for ss.Next() {
			numResponses++
		}
		require.Equal(t, numTestChunks, numResponses)
		require.Nil(t, ss.Err())
	})

	t.Run("chunked reader error", func(t *testing.T) {
		buf := &bytes.Buffer{}
		flusher := &mockFlusher{}

		w := remote.NewChunkedWriter(buf, flusher)
		r := remote.NewChunkedReader(buf, config.DefaultChunkedReadLimit, nil)

		chunks := buildTestChunks(t)
		l := []prompb.Label{
			{Name: "foo", Value: "bar"},
		}

		for i, c := range chunks {
			cSeries := prompb.ChunkedSeries{Labels: l, Chunks: []prompb.Chunk{c}}
			readResp := prompb.ChunkedReadResponse{
				ChunkedSeries: []*prompb.ChunkedSeries{&cSeries},
				QueryIndex:    int64(i),
			}

			b, err := proto.Marshal(&readResp)
			require.NoError(t, err)

			b[0] = 0xFF // Corruption!

			_, err = w.Write(b)
			require.NoError(t, err)
		}

		ss := NewStreamingSeriesSet(r, io.NopCloser(buf), 0, 15000, func() {})
		require.Nil(t, ss.Err())
		require.Nil(t, ss.Warnings())

		res := ss.Next()
		require.False(t, res)
		require.ErrorContains(t, ss.Err(), "proto: illegal wireType 7")
	})
}

// mockFlusher implements http.Flusher
type mockFlusher struct{}

func (f *mockFlusher) Flush() {}

const (
	numTestChunks          = 3
	numSamplesPerTestChunk = 5
)

func buildTestChunks(t *testing.T) []prompb.Chunk {
	startTime := int64(0)
	chunks := make([]prompb.Chunk, 0, numTestChunks)

	time := startTime

	for i := 0; i < numTestChunks; i++ {
		c := chunkenc.NewXORChunk()

		a, err := c.Appender()
		require.NoError(t, err)

		minTimeMs := time

		for j := 0; j < numSamplesPerTestChunk; j++ {
			a.Append(time, float64(i+j))
			time += int64(1000)
		}

		chunks = append(chunks, prompb.Chunk{
			MinTimeMs: minTimeMs,
			MaxTimeMs: time,
			Type:      prompb.Chunk_XOR,
			Data:      c.Bytes(),
		})
	}

	return chunks
}
