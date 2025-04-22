package remoteread

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/mimir/pkg/frontend/querymiddleware"

	"github.com/grafana/mimir-proxies/pkg/errorx"
	remotereadstorage "github.com/grafana/mimir-proxies/pkg/remoteread/storage"

	"github.com/mwitkow/go-conntrack"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/util/annotations"

	"github.com/grafana/mimir-proxies/pkg/appcommon"
)

const (
	apiPrefix           = "/api/v1"
	endpointLabels      = apiPrefix + "/labels"
	endpointLabelValues = apiPrefix + "/label/:name/values"
	endpointRemoteRead  = apiPrefix + "/read"
	endpointSeries      = apiPrefix + "/series"

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
//
//nolint:gomnd
func (c *StorageQueryableConfig) RegisterFlagsWithPrefix(prefix string, flags *flag.FlagSet) {
	flags.StringVar(&c.Address, prefix+".query-address", "http://localhost:80/prometheus", "Base URL for queries from upstream Prometheus API. The /api/v1 suffix will be appended to this address. Defaults to http://localhost:80/prometheus.")
	flags.DurationVar(&c.Timeout, prefix+".query-timeout", 30*time.Second, "Timeout for queries to upstream Prometheus API.")
	flags.DurationVar(&c.KeepAlive, prefix+".query-keep-alive", 30*time.Second, "KeepAlive for queries to upstream Prometheus API.")
	flags.IntVar(&c.MaxIdleConns, prefix+".query-max-idle-conns", 10, "Max idle conns for queries to upstream Prometheus API.")
	flags.IntVar(&c.MaxConns, prefix+".query-max-conns", 100, "Max conns per host for queries to upstream Prometheus API.")

	flags.StringVar(&c.ClientName, prefix+".query-client-name", "graphite-querier", "Client name to use when identifying requests in Prometheus API.")
}

func NewStorageQueryable(cfg StorageQueryableConfig, tripperware querymiddleware.Tripperware) (remotereadstorage.Queryable, error) {
	readURL, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("can't parse endpoint: %w", err)
	}
	readURL.Path = path.Join(readURL.Path, endpointRemoteRead)

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

	apiClient, err := api.NewClient(api.Config{
		Address:      cfg.Address,
		RoundTripper: transport,
	})
	if err != nil {
		return nil, err
	}

	return &storageQueryable{
		client:     NewStreamClient(cfg.ClientName, readURL, httpClient),
		api:        apiClient,
		httpClient: httpClient,
		endpoint:   cfg.Address,
	}, nil
}

type storageQueryable struct {
	client     Client
	api        api.Client
	httpClient *http.Client
	endpoint   string
}

func (q *storageQueryable) Querier(mint, maxt int64) (remotereadstorage.Querier, error) {
	return &storageQuerier{mint, maxt, q.client, q.api, q.httpClient, q.endpoint}, nil
}

var _ storage.Querier = &storageQuerier{}

type storageQuerier struct {
	mint, maxt int64

	client     Client
	api        api.Client
	httpClient *http.Client
	endpoint   string
}

func (q *storageQuerier) Select(ctx context.Context, sortSeries bool, hints *storage.SelectHints, matchers ...*labels.Matcher) (set storage.SeriesSet) {
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, parentSpan.Tracer(), "remoteread.StorageQuerier.Select")
		defer span.Finish()
		span.LogKV("mint", q.mint, "maxt", q.maxt, "matchers", MatchersString(matchers), "sortSeries", sortSeries)
		defer func() {
			if set.Err() != nil {
				span.LogKV("err", set.Err())
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
			err = errorx.TooManyRequests{Err: err}
		}

		return storage.ErrSeriesSet(fmt.Errorf("can't perform remote read: %w", err))
	}

	return res
}

func (q *storageQuerier) LabelValues(ctx context.Context, label string, _ *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, parentSpan.Tracer(), "remoteread.StorageQuerier.LabelValues")
		defer span.Finish()
		span.LogKV("mint", q.mint, "maxt", q.maxt, "name", label, "matchers", MatchersString(matchers))
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

func (q *storageQuerier) LabelNames(ctx context.Context, _ *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, parentSpan.Tracer(), "remoteread.StorageQuerier.LabelNames")
		defer span.Finish()
		span.LogKV("mint", q.mint, "maxt", q.maxt, "matchers", MatchersString(matchers))
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

func (q *storageQuerier) Series(ctx context.Context, matchers []string) ([]map[string]string, error) {
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, parentSpan.Tracer(), "remoteread.StorageQuerier.Series")
		defer span.Finish()
		span.LogKV("matchers", matchers)
	}

	data := url.Values{}
	for _, m := range matchers {
		data.Add("match[]", m)
	}
	if q.mint >= 0 {
		data.Set("start", strconv.FormatInt(time.UnixMilli(q.mint).Unix(), 10))
	}
	if q.maxt >= 0 {
		data.Set("end", strconv.FormatInt(time.UnixMilli(q.maxt).Unix(), 10))
	}

	u, err := url.Parse(q.endpoint)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpointSeries)

	// User POST method and Content-Type: application/x-www-form-urlencoded header so we can specify series selectors
	// that may breach server-side URL character limits.
	req, err := http.NewRequest("POST", u.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode/100 != HTTPSuccessStatusPrefix {
		// Make an attempt at getting an error message.
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		return nil, fmt.Errorf("remote server %s returned http status %s: %s", u.String(), resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var seriesResponse struct {
		Data []map[string]string
	}
	err = json.Unmarshal(body, &seriesResponse)
	if err != nil {
		return nil, err
	}

	return seriesResponse.Data, nil
}

func formatTime(t int64) string {
	const (
		millisPerSecond = int64(time.Second / time.Millisecond)
		bitSize         = 64
	)
	return strconv.FormatFloat(float64(t)/float64(millisPerSecond), 'f', -1, bitSize)
}

//go:generate mockery --output remotereadmock --outpkg remotereadmock --case underscore --name StorageQueryableInterface
type StorageQueryableInterface interface {
	remotereadstorage.Queryable
}

//go:generate mockery --output remotereadmock --outpkg remotereadmock --case underscore --name StorageQuerierInterface
type StorageQuerierInterface interface {
	remotereadstorage.Querier
}

func MatchersString(matchers []*labels.Matcher) string {
	if len(matchers) == 0 {
		return ""
	}
	return (&parser.VectorSelector{LabelMatchers: matchers}).String()
}
