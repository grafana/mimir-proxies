package tsdb

import (
	"fmt"
	"io"
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/stretchr/testify/require"
)

func TestSeriesBatchAndIteration(t *testing.T) {
	dir := t.TempDir()

	b := newSeriesBatcher(100, dir)

	allSeries := map[string]struct{}{}

	const max = 500
	for i := 0; i < max; i++ {
		lbls := labels.FromStrings("y", fmt.Sprintf("%d", max-i), "x", fmt.Sprintf("%d", i%10), "a", fmt.Sprintf("%d", i/10))

		require.NoError(t, b.addSeries(lbls, []chunks.Meta{{Ref: 123456, MinTime: 10, MaxTime: 100}}))
		allSeries[lbls.String()] = struct{}{}
	}

	require.NoError(t, b.flushSeries(true))
	require.NoError(t, b.flushSeries(true)) // call again, this should do nothing, and not create new empty file.

	files := b.getSeriesFiles()

	it, err := newSeriesIterator(files)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, it.Close())
	})

	first := true
	var s, prev series
	for s, err = it.NextSeries(); err == nil; s, err = it.NextSeries() {
		require.NotNil(t, s.Chunks)
		if !first {
			require.True(t, labels.Compare(prev.Metric, s.Metric) < 0)
		}

		first = false

		_, known := allSeries[s.Metric.String()]
		require.True(t, known)
		delete(allSeries, s.Metric.String())
		prev = s
	}
	require.Equal(t, io.EOF, err)
	require.Equal(t, 0, len(allSeries))
}

func TestDuplicateSeries(t *testing.T) {
	dir := t.TempDir()

	b := newSeriesBatcher(100, dir)
	require.NoError(t, b.addSeries(labels.FromStrings("a", "b"), nil))
	require.NoError(t, b.addSeries(labels.FromStrings("a", "b"), nil))

	require.NoError(t, b.flushSeries(true))
	require.NoError(t, b.flushSeries(true)) // call again, this should do nothing, and not create new empty file.

	files := b.getSeriesFiles()

	it, err := newSeriesIterator(files)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, it.Close())
	})

	_, err = it.NextSeries()
	require.NoError(t, err)
	_, err = it.NextSeries()
	require.ErrorContains(t, err, `duplicate series: {a="b"}, {a="b"}`)
}
