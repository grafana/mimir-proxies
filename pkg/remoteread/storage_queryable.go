package remoteread

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/mimir/pkg/frontend/querymiddleware"

	labelsutil "github.com/grafana/mimir-proxies/pkg/util/labels"

	"github.com/grafana/influx2cortex/pkg/errorx"

	"github.com/grafana/mimir-proxies/pkg/appcommon"
	cortexseries "github.com/grafana/mimir/pkg/storage/series"
	conntrack "github.com/mwitkow/go-conntrack"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
)

const (
	apiPrefix           = "/api/v1"
	endpointLabels      = apiPrefix + "/labels"
	endpointLabelValues = apiPrefix + "/label/:name/values"
	endpointRemoteRead  = apiPrefix + "/read"

	tooManyRequestsErrorSubstr = "429 Too Many Requests: too many outstanding requests"
)

type StorageQueryableConfig struct {
	Address      string        `yaml:"query_address"`
	Timeout      time.Duration `yaml:"query_timeout" category:"advanced"`
	KeepAlive    time.Duration `yaml:"query_keep_alive" category:"advanced"`
	MaxIdleConns int           `yaml:"query_max_idle_conns" category:"advanced"`
	MaxConns     int           `yaml:"query_max_conns" category:"advanced"`

	ClientName string `yaml:"query_client_name" category:"advanced"`
}

// RegisterFlagsWithPrefix registers flags, prepending the provided prefix if needed (no separation is added between the flag and the prefix)
//nolint:gomnd
func (c *StorageQueryableConfig) RegisterFlagsWithPrefix(prefix string, flags *flag.FlagSet) {
	flags.StringVar(&c.Address, prefix+".query-address", "http://localhost:80/prometheus", "Base URL for queries from upstream Prometheus API. The /api/v1 suffix will be appended to this address. Defaults to http://localhost:80/prometheus.")
	flags.DurationVar(&c.Timeout, prefix+".query-timeout", 30*time.Second, "Timeout for queries to upstream Prometheus API.")
	flags.DurationVar(&c.KeepAlive, prefix+".query-keep-alive", 30*time.Second, "KeepAlive for queries to upstream Prometheus API.")
	flags.IntVar(&c.MaxIdleConns, prefix+".query-max-idle-conns", 10, "Max idle conns for queries to upstream Prometheus API.")
	flags.IntVar(&c.MaxConns, prefix+".query-max-conns", 100, "Max conns per host for queries to upstream Prometheus API.")

	flags.StringVar(&c.ClientName, prefix+".query-client-name", "graphite-querier", "Client name to use when identifying requests in Prometheus API.")
}

func NewStorageQueryable(cfg StorageQueryableConfig, tripperware querymiddleware.Tripperware) (storage.Queryable, error) {
	readURL, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("can't parse endpoint: %w", err)
	}
	readURL.Path = path.Join(readURL.Path, endpointRemoteRead)

	remoteReadClient, err := remote.NewReadClient(cfg.ClientName, &remote.ClientConfig{
		URL:     &config.URL{URL: readURL},
		Timeout: model.Duration(cfg.Timeout),
	})
	if err != nil {
		return nil, fmt.Errorf("can't instantiate remote read client: %w", err)
	}

	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport.MaxIdleConnsPerHost = cfg.MaxIdleConns
	httpTransport.MaxIdleConns = cfg.MaxIdleConns
	httpTransport.MaxConnsPerHost = cfg.MaxConns
	httpTransport.DialContext = conntrack.NewDialContextFunc(
		conntrack.DialWithName("v1.API"),
		conntrack.DialWithTracing(),
		conntrack.DialWithDialer(&net.Dialer{
			Timeout:   cfg.Timeout,
			KeepAlive: cfg.KeepAlive,
		}),
	)
	transport := appcommon.NewTracedAuthRoundTripper(httpTransport, "Remote Read")
	if tripperware != nil {
		transport = tripperware(transport)
	}

	httpClient := &http.Client{Transport: transport}

	remoteClientStruct, ok := remoteReadClient.(*remote.Client)
	if !ok {
		return nil, fmt.Errorf("remote.ReadClient is expected to be *remote.Client but it was %T", remoteReadClient)
	}

	remoteClientStruct.Client = httpClient

	apiClient, err := api.NewClient(api.Config{
		Address:      cfg.Address,
		RoundTripper: transport,
	})
	if err != nil {
		return nil, err
	}

	return &storageQueryable{
		client: remoteReadClient,
		api:    apiClient,
	}, nil
}

var _ storage.Queryable = &storageQueryable{}

type storageQueryable struct {
	client remote.ReadClient
	api    api.Client
}

func (q *storageQueryable) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return &storageQuerier{ctx, mint, maxt, q.client, q.api}, nil
}

var _ storage.Querier = &storageQuerier{}

type storageQuerier struct {
	ctx        context.Context
	mint, maxt int64

	client remote.ReadClient
	api    api.Client
}

func (q *storageQuerier) Select(sortSeries bool, hints *storage.SelectHints, matchers ...*labels.Matcher) (set storage.SeriesSet) {
	var timeseries, samples int
	ctx := q.ctx
	if parentSpan := opentracing.SpanFromContext(q.ctx); parentSpan != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(q.ctx, parentSpan.Tracer(), "remoteread.StorageQuerier.Select")
		defer span.Finish()
		span.LogKV("mint", q.mint, "maxt", q.maxt, "matchers", matchersString(matchers), "sortSeries", sortSeries)
		defer func() {
			if set.Err() != nil {
				span.LogKV("err", set.Err())
			} else {
				span.LogKV("timeseries_len", timeseries, "samples_len", samples)
			}
		}()
	}

	query, err := remote.ToQuery(q.mint, q.maxt, matchers, hints)
	if err != nil {
		return storage.ErrSeriesSet(fmt.Errorf("can't translate to remote query: %w", err))
	}

	res, err := q.client.Read(ctx, query)
	if err != nil {
		if strings.Contains(err.Error(), tooManyRequestsErrorSubstr) {
			err = errorx.RateLimited{Err: err}
		}

		return storage.ErrSeriesSet(fmt.Errorf("can't perform remote read: %w", err))
	}

	// Gather some stats for the span.
	timeseries = len(res.Timeseries)
	for _, ts := range res.Timeseries {
		samples += len(ts.Samples)
	}

	return fromQueryResult(sortSeries, res)
}

func (q *storageQuerier) LabelValues(label string, matchers ...*labels.Matcher) ([]string, storage.Warnings, error) {
	ctx := q.ctx
	if parentSpan := opentracing.SpanFromContext(q.ctx); parentSpan != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(q.ctx, parentSpan.Tracer(), "remoteread.StorageQuerier.LabelValues")
		defer span.Finish()
		span.LogKV("mint", q.mint, "maxt", q.maxt, "name", label, "matchers", matchersString(matchers))
	}

	u := q.api.URL(endpointLabelValues, map[string]string{"name": label})
	q.updateURLWithStartEndAndMatchers(u, matchers)

	var labelValues struct {
		Data []string
	}
	if err := q.doGet(ctx, u, &labelValues); err != nil {
		return nil, nil, err
	}
	return labelValues.Data, nil, nil
}

func (q *storageQuerier) LabelNames(matchers ...*labels.Matcher) ([]string, storage.Warnings, error) {
	ctx := q.ctx
	if parentSpan := opentracing.SpanFromContext(q.ctx); parentSpan != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(q.ctx, parentSpan.Tracer(), "remoteread.StorageQuerier.LabelNames")
		defer span.Finish()
		span.LogKV("mint", q.mint, "maxt", q.maxt, "matchers", matchersString(matchers))
	}

	u := q.api.URL(endpointLabels, nil)
	q.updateURLWithStartEndAndMatchers(u, matchers)

	var labelNames struct {
		Data []string
	}
	if err := q.doGet(ctx, u, &labelNames); err != nil {
		return nil, nil, err
	}
	return labelNames.Data, nil, nil
}

func (q *storageQuerier) updateURLWithStartEndAndMatchers(u *url.URL, matchers []*labels.Matcher) {
	query := u.Query()
	query.Set("start", formatTime(q.mint))
	query.Set("end", formatTime(q.maxt))
	if len(matchers) > 0 {
		var selector []string
		for _, m := range matchers {
			selector = append(selector, m.String())
		}
		query.Add("match[]", fmt.Sprintf("{%s}", strings.Join(selector, ",")))
	}
	u.RawQuery = query.Encode()
}

//nolint:interfacer // I don't want to replace url.URL with a fmt.Stringer
func (q *storageQuerier) doGet(ctx context.Context, u *url.URL, data interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return err
	}

	resp, body, err := q.api.Do(ctx, req) //nolint:bodyclose // already closed by api.httpClient when the entire body is read
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("failed performing request: status %q, body %q", resp.Status, string(body))
	}
	return json.Unmarshal(body, data)
}

func (q *storageQuerier) Close() error {
	return nil
}

func formatTime(t int64) string {
	const (
		millisPerSecond = int64(time.Second / time.Millisecond)
		bitSize         = 64
	)
	return strconv.FormatFloat(float64(t)/float64(millisPerSecond), 'f', -1, bitSize)
}

// fromQueryResult is copied from remote.FromQueryResult and it has the `validateLabelsAndMetricName` call removed
// because that's not a storage.Querier's responsibility (and actually Graphite needs this to provide invalid labels)
func fromQueryResult(sortSeries bool, res *prompb.QueryResult) storage.SeriesSet {
	series := make([]storage.Series, 0, len(res.Timeseries))
	for _, ts := range res.Timeseries {
		lbls := labelsutil.LabelProtosToLabels(ts.Labels)
		series = append(series, cortexseries.NewConcreteSeries(lbls, sampleProtosToSamples(ts.Samples)))
	}

	if sortSeries {
		sort.Sort(byLabel(series))
	}
	return cortexseries.NewConcreteSeriesSet(series)
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

type byLabel []storage.Series

func (a byLabel) Len() int           { return len(a) }
func (a byLabel) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byLabel) Less(i, j int) bool { return labels.Compare(a[i].Labels(), a[j].Labels()) < 0 }

//go:generate mockery --output remotereadmock --outpkg remotereadmock --case underscore --name StorageQueryableInterface
type StorageQueryableInterface interface {
	storage.Queryable
}

//go:generate mockery --output remotereadmock --outpkg remotereadmock --case underscore --name StorageQuerierInterface
type StorageQuerierInterface interface {
	storage.Querier
}

func matchersString(matchers []*labels.Matcher) string {
	if len(matchers) == 0 {
		return ""
	}
	return (&parser.VectorSelector{LabelMatchers: matchers}).String()
}
