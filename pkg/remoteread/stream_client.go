package remoteread

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	cortexseries "github.com/grafana/mimir/pkg/storage/series"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"

	labelsutil "github.com/grafana/mimir-proxies/pkg/util/labels"
)

const (
	UserAgent                   = "Grafana"
	InitialBufSize              = 32 * 1024 // 32KB
	SampledContentTypePrefix    = "application/x-protobuf"
	StreamedContentTypePrefix   = "application/x-streamed-protobuf; proto=prometheus.ChunkedReadResponse"
	PrometheusRemoteReadVersion = "0.1.0"
	HTTPSuccessStatusPrefix     = 2
)

var (
	AcceptedResponseTypes = []prompb.ReadRequest_ResponseType{
		prompb.ReadRequest_STREAMED_XOR_CHUNKS,
		prompb.ReadRequest_SAMPLES,
	}

	remoteReadQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "prometheus",
			Subsystem: "remote_read",
			Name:      "read_queries_total",
			Help:      "The total number of remote read queries.",
		},
		[]string{"remote_name", "url", "response_type", "code"},
	)
	readQueriesInflight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "prometheus",
			Subsystem: "remote_read",
			Name:      "remote_read_queries",
			Help:      "The number of in-flight remote read queries.",
		},
		[]string{"remote_name", "url"},
	)
	remoteReadQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "prometheus",
			Subsystem: "remote_read",
			Name:      "read_request_duration_seconds",
			Help:      "Histogram of the latency for remote read requests.",
			//nolint:gomnd
			Buckets: append(prometheus.DefBuckets, 25, 60),
		},
		[]string{"remote_name", "url", "response_type"},
	)
)

func init() {
	prometheus.MustRegister(remoteReadQueriesTotal, readQueriesInflight, remoteReadQueryDuration)
}

// streamClient implements Client
type streamClient struct {
	url        *url.URL
	httpClient *http.Client
	bufPool    *sync.Pool

	readQueriesTotal    *prometheus.CounterVec
	readQueries         prometheus.Gauge
	readQueriesDuration prometheus.ObserverVec
}

func NewStreamClient(name string, url *url.URL, httpClient *http.Client) Client {
	return &streamClient{
		url:        url,
		httpClient: httpClient,
		bufPool: &sync.Pool{New: func() interface{} {
			b := make([]byte, 0, InitialBufSize)
			return &b
		}},
		readQueriesTotal:    remoteReadQueriesTotal.MustCurryWith(prometheus.Labels{"remote_name": name, "url": url.String()}),
		readQueries:         readQueriesInflight.WithLabelValues(name, url.String()),
		readQueriesDuration: remoteReadQueryDuration.MustCurryWith(prometheus.Labels{"remote_name": name, "url": url.String()}),
	}
}

func (c *streamClient) Type() string {
	return "stream"
}

func (c *streamClient) Read(ctx context.Context, query *prompb.Query) (storage.SeriesSet, error) {
	c.readQueries.Inc()

	req := prompb.ReadRequest{
		Queries:               []*prompb.Query{query},
		AcceptedResponseTypes: AcceptedResponseTypes,
	}

	reqb, err := proto.Marshal(&req)
	if err != nil {
		c.readQueries.Dec()
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.url.String(), bytes.NewReader(snappy.Encode(nil, reqb)))
	if err != nil {
		c.readQueries.Dec()
		return nil, err
	}

	httpReq.Header.Add("Content-Encoding", "snappy")
	httpReq.Header.Add("Accept-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", SampledContentTypePrefix)
	httpReq.Header.Set("User-Agent", UserAgent)
	httpReq.Header.Set("X-Prometheus-Remote-Read-Version", PrometheusRemoteReadVersion)

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq.WithContext(ctx))
	if err != nil {
		c.readQueries.Dec()
		return nil, err
	}

	if httpResp.StatusCode/100 != HTTPSuccessStatusPrefix {
		// Make an attempt at getting an error message.
		body, _ := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()

		c.readQueries.Dec()
		return nil, fmt.Errorf("remote server %s returned http status %s: %s", c.url, httpResp.Status, string(body))
	}

	contentType := httpResp.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, SampledContentTypePrefix) {
		c.readQueriesDuration.WithLabelValues("sampled").Observe(time.Since(start).Seconds())
		c.readQueriesTotal.WithLabelValues("sampled", strconv.Itoa(httpResp.StatusCode)).Inc()
		c.readQueries.Dec()

		return c.handleSampledResponse(httpResp)
	} else if strings.HasPrefix(contentType, StreamedContentTypePrefix) {
		c.readQueriesTotal.WithLabelValues("streamed", strconv.Itoa(httpResp.StatusCode)).Inc()
		ss := c.handleStreamedResponse(httpResp, start, query.StartTimestampMs, query.EndTimestampMs)

		return ss, nil
	} else {
		c.readQueriesDuration.WithLabelValues("unknown").Observe(time.Since(start).Seconds())
		c.readQueriesTotal.WithLabelValues("unknown", strconv.Itoa(httpResp.StatusCode)).Inc()
		c.readQueries.Dec()

		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

func (c *streamClient) getBuf() *[]byte {
	b := c.bufPool.Get()
	return b.(*[]byte)
}

func (c *streamClient) putBuf(b *[]byte) {
	c.bufPool.Put(b)
}

func (c *streamClient) handleSampledResponse(httpResp *http.Response) (storage.SeriesSet, error) {
	compressed, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "problem reading http response (status code: %s)", httpResp.Status)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, httpResp.Body)
		_ = httpResp.Body.Close()
	}()

	buf := c.getBuf()
	defer c.putBuf(buf)

	decompressed, err := snappy.Decode(*buf, compressed)
	if err != nil {
		return nil, err
	}

	var resp prompb.ReadResponse
	err = proto.Unmarshal(decompressed, &resp)
	if err != nil {
		return nil, err
	}

	// This client does not batch queries, so there's always only 1 result.
	res := resp.Results[0]

	return fromQueryResult(res), nil
}

// fromQueryResult is copied from remote.FromQueryResult and it has the `validateLabelsAndMetricName` call removed
// because that's not a storage.Querier's responsibility (and actually Graphite needs this to provide invalid labels)
func fromQueryResult(res *prompb.QueryResult) storage.SeriesSet {
	series := make([]storage.Series, 0, len(res.Timeseries))
	for _, ts := range res.Timeseries {
		lbls := labelsutil.LabelProtosToLabels(ts.Labels)
		series = append(series, cortexseries.NewConcreteSeries(lbls, sampleProtosToSamples(ts.Samples), nil))
	}

	return cortexseries.NewConcreteSeriesSetFromUnsortedSeries(series)
}

func sampleProtosToSamples(in []prompb.Sample) []model.SamplePair {
	if len(in) == 0 {
		return nil
	}

	out := make([]model.SamplePair, len(in))
	for i := range in {
		out[i] = model.SamplePair{
			Timestamp: model.Time(in[i].Timestamp),
			Value:     model.SampleValue(in[i].Value),
		}
	}
	return out
}

func (c *streamClient) handleStreamedResponse(httpResp *http.Response, start time.Time, queryStartMs, queryEndMs int64) storage.SeriesSet {
	s := remote.NewChunkedReader(httpResp.Body, config.DefaultChunkedReadLimit, nil)
	return NewStreamingSeriesSet(s, httpResp.Body, queryStartMs, queryEndMs, func() {
		c.readQueries.Dec()
		c.readQueriesDuration.WithLabelValues("streamed").Observe(time.Since(start).Seconds())
	})
}
