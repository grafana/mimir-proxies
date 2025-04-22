package convert

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/stretchr/testify/require"
)

// TestIntermediateHappyPaths confirms that intermediate files can be opened
// for append or read and behave in the expected ways.
func TestIntermediateHappyPaths(t *testing.T) {
	testDir, err := os.MkdirTemp("/tmp", "intermediatetest*")
	require.NoError(t, err)

	fname := filepath.Join(testDir, "test1")
	logger := log.NewNopLogger()
	i, err := NewUSTableForAppend(fname, true, NewMimirSeriesProto, logger)
	require.NoError(t, err)

	metricData := map[string]*mimirpb.TimeSeries{
		"metric1": {
			Labels: []mimirpb.LabelAdapter{
				{Name: "label1", Value: "__stuff__"},
				{Name: "label2", Value: "__stuff2__"},
			},
			Samples: []mimirpb.Sample{
				{TimestampMs: 1000, Value: 1},
				{TimestampMs: 1001, Value: 42},
				{TimestampMs: 1003, Value: 52},
			},
		},
		"another_metric": {
			Labels: []mimirpb.LabelAdapter{
				{Name: "label3", Value: "__something__"},
				{Name: "label4", Value: "__whatever__"},
			},
			Samples: []mimirpb.Sample{
				{TimestampMs: 2000, Value: 6.4},
				{TimestampMs: 2001, Value: 42},
				{TimestampMs: 2023, Value: 52},
			},
		},
		"empty": {},
		"empty samples": {
			Labels: []mimirpb.LabelAdapter{
				{Name: "label3", Value: "__something__"},
				{Name: "label4", Value: "__whatever__"},
			},
			Samples: nil,
		},
		"last_one": {
			Samples: []mimirpb.Sample{
				{TimestampMs: 20000, Value: 7},
				{TimestampMs: 20001, Value: 8},
				{TimestampMs: 20023, Value: 2},
			},
		},
	}

	// Write out a few basic metricdatas to empty file.
	orderedMetrics := []string{}
	for l := range metricData {
		orderedMetrics = append(orderedMetrics, l)
	}
	sort.Strings(orderedMetrics)
	for _, k := range orderedMetrics {
		err = i.Append(k, metricData[k])
		require.NoError(t, err)
	}
	require.NoError(t, i.Close())

	// Opening as read-only, we can check the data.
	i, err = NewUSTableForRead(fname, NewMimirSeriesProto, logger)
	require.NoError(t, err)
	gotRecords := make(map[string]*mimirpb.TimeSeries)
	for {
		var gotKey string
		var gotProto ProtoUnmarshaler
		gotKey, gotProto, err = i.Next()
		if err != nil {
			require.Equal(t, err, ErrAtSentinel)
			break
		}
		gotSeries, ok := gotProto.(*mimirpb.TimeSeries)
		require.True(t, ok)
		gotRecords[gotKey] = gotSeries
	}
	require.Equal(t, metricData, gotRecords)

	// Attempting to Append gets us an error.
	err = i.Append("won't work", metricData["metric1"])
	require.Equal(t, ErrInvalidForMode, err)
	require.NoError(t, i.Close())

	// Read them back in, confirm they are logged.
	i, index, err := NewUSTableForAppendWithIndex(fname, false, NewMimirSeriesProto, logger)
	require.NoError(t, err)
	wantIndex := map[string]int64{"metric1": 289, "another_metric": 0, "last_one": 220, "empty": 121, "empty samples": 142}
	require.Equal(t, wantIndex, index)

	// Attempting to READ gets us ErrInvalidForMode
	_, _, err = i.Next()
	require.Equal(t, ErrInvalidForMode, err)

	// A call to Next should get us ErrAtSentinel. Note we have to override the
	// mode to test this, users won't be doing this.
	i.mode = READ
	_, s, err := i.Next()
	require.Nil(t, s)
	require.Equal(t, ErrAtSentinel, err)
	i.mode = APPEND

	// We can Append
	resumedMetric := &mimirpb.TimeSeries{
		Labels: []mimirpb.LabelAdapter{
			{Name: "label5", Value: "foo"},
			{Name: "other_thing", Value: "bar"},
		},
		Samples: []mimirpb.Sample{
			{TimestampMs: 52, Value: 9},
			{TimestampMs: 57, Value: 10},
			{TimestampMs: 59, Value: 11},
		},
	}
	metricData["resumed_metric"] = resumedMetric

	require.NoError(t, i.Append("resumed_metric", resumedMetric))
	require.NoError(t, i.Close())

	// And once again we can reopen and it all works.
	i, index, err = NewUSTableForAppendWithIndex(fname, false, NewMimirSeriesProto, logger)
	require.NoError(t, err)
	wantIndex["resumed_metric"] = 397
	require.Equal(t, wantIndex, index)

	// override mode again
	i.mode = READ
	_, s, err = i.Next()
	require.Nil(t, s)
	require.Equal(t, ErrAtSentinel, err)
	require.NoError(t, i.Close())

	// Opening as read-only, we can check the data again and the appended data is
	// there.
	i, err = NewUSTableForRead(fname, NewMimirSeriesProto, logger)
	require.NoError(t, err)
	gotRecords = make(map[string]*mimirpb.TimeSeries)
	for {
		var gotKey string
		var gotProto ProtoUnmarshaler
		gotKey, gotProto, err = i.Next()
		if err != nil {
			require.Equal(t, err, ErrAtSentinel)
			break
		}
		gotSeries, ok := gotProto.(*mimirpb.TimeSeries)
		require.True(t, ok)
		gotRecords[gotKey] = gotSeries
	}
	require.Equal(t, metricData, gotRecords)

	// Check the index.
	gotIndex, err := i.Index()
	require.Nil(t, err)
	require.Equal(t, wantIndex, gotIndex)

	for k := range metricData {
		var gotKey string
		var d ProtoUnmarshaler
		gotKey, d, err = i.ReadAt(gotIndex[k])
		require.Nil(t, err)
		require.Equal(t, k, gotKey)
		require.Equal(t, metricData[k], d)
	}
	require.NoError(t, i.Close())

	// Reopen once more as overwrite, write one, that should be the only one.
	i, index, err = NewUSTableForAppendWithIndex(fname, true, NewMimirSeriesProto, logger)
	require.NoError(t, err)
	require.Empty(t, index)
	err = i.Append("resumed_metric", resumedMetric)
	require.NoError(t, err)
	require.NoError(t, i.Close())

	_ = os.RemoveAll(testDir)
}

// writeGoodFile, if true, will generate a new golden good.intermediate file.
const writeGoodFile = false

// The test files are all manually-created partials. In all cases we should have
// the list of good metrics recorded and be able to write more records and
// close.
func TestIntermediatePartials(t *testing.T) {
	logger := log.NewNopLogger()
	testDir, err := os.MkdirTemp("/tmp", "intermediatetest*")
	require.NoError(t, err)
	metricData := map[string]*mimirpb.TimeSeries{
		"metric1": {
			Labels: []mimirpb.LabelAdapter{
				{Name: "label1", Value: "__stuff__"},
				{Name: "label2", Value: "__stuff2__"},
			},
			Samples: []mimirpb.Sample{
				{TimestampMs: 1000, Value: 1},
				{TimestampMs: 1001, Value: 42},
				{TimestampMs: 1003, Value: 52},
			},
		},
		"another_metric": {
			Labels: []mimirpb.LabelAdapter{
				{Name: "label3", Value: "__thing1__"},
				{Name: "label4", Value: "__thing3__"},
			},
			Samples: []mimirpb.Sample{
				{TimestampMs: 2000, Value: 6.4},
				{TimestampMs: 2001, Value: 42},
				{TimestampMs: 2023, Value: 52},
			},
		},
		"last_one": {
			Labels: []mimirpb.LabelAdapter{
				{Name: "label1", Value: "__different__"},
				{Name: "label2", Value: "__also different__"},
			},
			Samples: []mimirpb.Sample{
				{TimestampMs: 20000, Value: 7},
				{TimestampMs: 20001, Value: 8},
				{TimestampMs: 20023, Value: 2},
			},
		},
		"resumed_metric": {
			Labels: []mimirpb.LabelAdapter{
				{Name: "label1", Value: "__really__"},
				{Name: "label2", Value: "doesn't"},
				{Name: "label3", Value: "__matter2__"},
			},
			Samples: []mimirpb.Sample{
				{TimestampMs: 52, Value: 9},
				{TimestampMs: 57, Value: 10},
				{TimestampMs: 59, Value: 11},
			},
		},
	}

	if writeGoodFile {
		i, err := NewUSTableForAppend("/tmp/good.intermediate", true, NewMimirSeriesProto, logger)
		require.NoError(t, err)

		// Write out a few basic metricdatas to empty file.
		err = i.Append("metric1", metricData["metric1"])
		require.NoError(t, err)
		err = i.Append("another_metric", metricData["another_metric"])
		require.NoError(t, err)
		err = i.Append("last_one", metricData["last_one"])
		require.NoError(t, err)
		require.NoError(t, i.Close())
	}

	testFilePath := "./whisperconverter/testdata/"

	tests := []struct {
		fname       string
		wantReadErr string
		// After opening a file for Append, we want to confirm that the write head
		// is at the correct position in the file.
		wantSeekPos        int64
		wantWrittenMetrics map[string]int64
		appendMetrics      []string
		wantFinalMetrics   []string
	}{
		{
			fname:              "good.intermediate",
			wantReadErr:        ErrAtSentinel.Error(),
			wantSeekPos:        348,
			wantWrittenMetrics: map[string]int64{"metric1": 0, "another_metric": 108, "last_one": 224},
			appendMetrics:      []string{"resumed_metric"},
			wantFinalMetrics:   []string{"metric1", "another_metric", "last_one", "resumed_metric"},
		},
		{
			// The int64 length value is truncated for the sentinel.
			fname:              "bad-sentinel-length.intermediate",
			wantReadErr:        "unexpected EOF",
			wantSeekPos:        348,
			wantWrittenMetrics: map[string]int64{"metric1": 0, "another_metric": 108, "last_one": 224},
			appendMetrics:      []string{"resumed_metric"},
			wantFinalMetrics:   []string{"metric1", "another_metric", "last_one", "resumed_metric"},
		},
		{
			// The int64 length for a regular piece of name data is truncated.
			fname:              "bad-data-length.intermediate",
			wantReadErr:        "unexpected EOF",
			wantSeekPos:        224,
			wantWrittenMetrics: map[string]int64{"metric1": 0, "another_metric": 108},
			appendMetrics:      []string{"resumed_metric"},
			wantFinalMetrics:   []string{"metric1", "another_metric", "resumed_metric"},
		},
		{
			// The int64 length for a regular piece of series data is truncated.
			fname:              "bad-data-length.intermediate",
			wantReadErr:        "unexpected EOF",
			wantSeekPos:        224,
			wantWrittenMetrics: map[string]int64{"metric1": 0, "another_metric": 108},
			appendMetrics:      []string{"resumed_metric"},
			wantFinalMetrics:   []string{"metric1", "another_metric", "resumed_metric"},
		},
		{
			// The word "SENTINEL" is misspelled.
			fname:              "bad-sentinel.intermediate",
			wantReadErr:        ErrBadSentinal.Error(),
			wantSeekPos:        348,
			wantWrittenMetrics: map[string]int64{"metric1": 0, "another_metric": 108, "last_one": 224},
			appendMetrics:      []string{"resumed_metric"},
			wantFinalMetrics:   []string{"metric1", "another_metric", "last_one", "resumed_metric"},
		},
		{
			// The proto chunk is shorter than the predicted length.
			fname:              "truncated-proto.intermediate",
			wantSeekPos:        224,
			wantReadErr:        "proto: Sample: illegal tag 0 (wire type 0)",
			wantWrittenMetrics: map[string]int64{"metric1": 0, "another_metric": 108},
			appendMetrics:      []string{"last_one", "resumed_metric"},
			wantFinalMetrics:   []string{"metric1", "another_metric", "last_one", "resumed_metric"},
		},
		// We no longer check for junk data if the sentinel is valid.
		// {
		// 	// The proto chunk has junk data in it.
		// 	fname:              "corrupt-proto.intermediate",
		// 	wantSeekPos:        224,
		// 	wantReadErr:        "proto: LabelPair: illegal tag 0 (wire type 0)",
		// 	wantWrittenMetrics: map[string]bool{"metric1": true, "another_metric": true},
		// 	appendMetrics:      []string{"last_one", "resumed_metric"},
		// 	wantFinalMetrics:   []string{"metric1", "another_metric", "last_one", "resumed_metric"},
		// },
		{
			// The SENTINEL string is cut off.
			fname:              "truncated-sentinel.intermediate",
			wantReadErr:        ErrBadSentinal.Error(),
			wantSeekPos:        348,
			wantWrittenMetrics: map[string]int64{"metric1": 0, "another_metric": 108, "last_one": 224},
			appendMetrics:      []string{"resumed_metric"},
			wantFinalMetrics:   []string{"metric1", "another_metric", "last_one", "resumed_metric"},
		},
	}

	for _, test := range tests {
		t.Run(test.fname, func(t *testing.T) {
			goldenPath := filepath.Join(testFilePath, test.fname)
			mutablePath := filepath.Join(testDir, test.fname)
			// Copy golden files before use because we will be appending to them.
			require.NoError(t, copyFile(goldenPath, mutablePath))

			// First do a read test so we can recognize bad files.
			i, err := NewUSTableForRead(mutablePath, NewMimirSeriesProto, logger)
			require.Nil(t, err)
			gotMetrics := []string{}
			for {
				var gotKey string
				var s ProtoUnmarshaler
				gotKey, s, err = i.Next()
				if err != nil {
					require.Equal(t, test.wantReadErr, err.Error())
					break
				}
				gotSeries := s.(*mimirpb.TimeSeries)
				gotMetrics = append(gotMetrics, gotKey)
				require.Equal(t, metricData[gotKey], gotSeries)
			}
			require.Equal(t, len(test.wantWrittenMetrics), len(gotMetrics), gotMetrics)
			require.NoError(t, i.Close())

			// Open for append without reading preexisting, we should be at a sentinel
			// in resumable cases.
			i, err = NewUSTableForAppend(mutablePath, false, NewMimirSeriesProto, logger)
			require.Nil(t, err)
			require.Equal(t, test.wantSeekPos, i.pos())
			_ = i.Close()

			// Now open for append. We should detect the good / resumable data.
			i, m, err := NewUSTableForAppendWithIndex(mutablePath, false, NewMimirSeriesProto, logger)
			require.Nil(t, err)
			require.Equal(t, test.wantWrittenMetrics, m)
			for _, m := range test.appendMetrics {
				err = i.Append(m, metricData[m])
				require.NoError(t, err)
			}
			require.NoError(t, i.Close())

			// Confirm that we can read it all back.
			i, err = NewUSTableForRead(mutablePath, NewMimirSeriesProto, logger)
			require.Nil(t, err)
			gotMetrics = []string{}
			for {
				var gotKey string
				var s ProtoUnmarshaler
				gotKey, s, err = i.Next()
				if err != nil {
					require.Equal(t, ErrAtSentinel, err)
					break
				}
				gotSeries := s.(*mimirpb.TimeSeries)
				gotMetrics = append(gotMetrics, gotKey)
				require.Equal(t, metricData[gotKey], gotSeries)
			}
			require.ElementsMatch(t, test.wantFinalMetrics, gotMetrics)
		})
	}

	_ = os.RemoveAll(testDir)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}
