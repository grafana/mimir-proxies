package remoteread

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	tests := []struct {
		name                  string
		clientFactory         func(*url.URL, *testing.T) Client
		httpHandler           http.HandlerFunc
		readContextTimeout    time.Duration
		query                 *prompb.Query
		expectedLabels        map[string]string
		expectedSamples       []model.SamplePair
		expectedErrorContains string
	}{
		{
			name: "sampled response w/ stream client",
			clientFactory: func(u *url.URL, _ *testing.T) Client {
				return NewStreamClient("test", u, http.DefaultClient)
			},
			httpHandler:    sampledResponseHTTPHandler(t),
			expectedLabels: map[string]string{"foo": "bar"},
			expectedSamples: []model.SamplePair{
				{Timestamp: model.Time(0), Value: model.SampleValue(1)},
				{Timestamp: model.Time(5), Value: model.SampleValue(2)},
			},
			expectedErrorContains: "",
		},
		{
			name: "streaming response w/ stream client",
			clientFactory: func(u *url.URL, _ *testing.T) Client {
				return NewStreamClient("test", u, http.DefaultClient)
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", StreamedContentTypePrefix)

				flusher, ok := w.(http.Flusher)
				require.True(t, ok)

				cw := remote.NewChunkedWriter(w, flusher)
				l := []prompb.Label{
					{Name: "foo", Value: "bar"},
				}

				chunks := buildTestChunks(t)
				for i, c := range chunks {
					cSeries := prompb.ChunkedSeries{Labels: l, Chunks: []prompb.Chunk{c}}
					readResp := prompb.ChunkedReadResponse{
						ChunkedSeries: []*prompb.ChunkedSeries{&cSeries},
						QueryIndex:    int64(i),
					}

					b, err := proto.Marshal(&readResp)
					require.NoError(t, err)

					_, err = cw.Write(b)
					require.NoError(t, err)
				}
			}),
			query:          &prompb.Query{StartTimestampMs: 4000, EndTimestampMs: 12000},
			expectedLabels: map[string]string{"foo": "bar"},
			expectedSamples: []model.SamplePair{
				// This is the output of buildTestChunks minus the samples that fall outside the query range.
				{Timestamp: model.Time(4000), Value: model.SampleValue(4)},
				{Timestamp: model.Time(5000), Value: model.SampleValue(1)},
				{Timestamp: model.Time(6000), Value: model.SampleValue(2)},
				{Timestamp: model.Time(7000), Value: model.SampleValue(3)},
				{Timestamp: model.Time(8000), Value: model.SampleValue(4)},
				{Timestamp: model.Time(9000), Value: model.SampleValue(5)},
				{Timestamp: model.Time(10000), Value: model.SampleValue(2)},
				{Timestamp: model.Time(11000), Value: model.SampleValue(3)},
				{Timestamp: model.Time(12000), Value: model.SampleValue(4)},
			},
			expectedErrorContains: "",
		},
		{
			name: "non-2xx http status",
			clientFactory: func(u *url.URL, _ *testing.T) Client {
				return NewStreamClient("test", u, http.DefaultClient)
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
			expectedErrorContains: "returned http status 500",
		},
		{
			name: "unsupported content type",
			clientFactory: func(u *url.URL, _ *testing.T) Client {
				return NewStreamClient("test", u, http.DefaultClient)
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "foobar")
			}),
			expectedErrorContains: "unsupported content type",
		},
		{
			name: "context timeout for stream client",
			clientFactory: func(u *url.URL, _ *testing.T) Client {
				return NewStreamClient("test", u, http.DefaultClient)
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(2 * time.Second)
			}),
			readContextTimeout:    1 * time.Second,
			expectedErrorContains: "context deadline exceeded",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.httpHandler)
			defer server.Close()

			u, err := url.Parse(server.URL)
			require.NoError(t, err)

			c := test.clientFactory(u, t)

			query := &prompb.Query{}
			if test.query != nil {
				query = test.query
			}

			var ctx context.Context
			var cancel context.CancelFunc
			if test.readContextTimeout > 0 {
				ctx, cancel = context.WithTimeout(context.Background(), test.readContextTimeout)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			ss, err := c.Read(ctx, query)
			if test.expectedErrorContains != "" {
				require.ErrorContains(t, err, test.expectedErrorContains)
				return
			}

			require.NoError(t, err)

			i := 0

			for ss.Next() {
				require.NoError(t, ss.Err())
				s := ss.At()

				l := s.Labels()
				require.Equal(t, len(test.expectedLabels), l.Len())
				for k, v := range test.expectedLabels {
					require.True(t, l.Has(k))
					require.Equal(t, v, l.Get(k))
				}

				it := s.Iterator(nil)
				for res := it.Next(); res != chunkenc.ValNone; res = it.Next() {
					require.NoError(t, it.Err())
					require.Equal(t, chunkenc.ValFloat, res)

					ts, v := it.At()
					expectedSample := test.expectedSamples[i]

					require.Equal(t, int64(expectedSample.Timestamp), ts)
					require.Equal(t, float64(expectedSample.Value), v)

					i++
				}
			}

			require.Equal(t, len(test.expectedSamples), i)
			require.NoError(t, ss.Err())
		})
	}
}

func sampledResponseHTTPHandler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", SampledContentTypePrefix)

		resp := prompb.ReadResponse{
			Results: []*prompb.QueryResult{
				{
					Timeseries: []*prompb.TimeSeries{
						{
							Labels: []prompb.Label{
								{Name: "foo", Value: "bar"},
							},
							Samples: []prompb.Sample{
								{Value: float64(1), Timestamp: int64(0)},
								{Value: float64(2), Timestamp: int64(5)},
							},
							Exemplars: []prompb.Exemplar{},
						},
					},
				},
			},
		}
		b, err := proto.Marshal(&resp)
		require.NoError(t, err)

		_, err = w.Write(snappy.Encode(nil, b))
		require.NoError(t, err)
	}
}
