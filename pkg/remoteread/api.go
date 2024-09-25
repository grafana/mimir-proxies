package remoteread

import (
	"context"
	"errors"
	"flag"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/mimir/pkg/frontend/querymiddleware"
	"github.com/mwitkow/go-conntrack"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/grafana/mimir-proxies/pkg/appcommon"
	"github.com/grafana/mimir-proxies/pkg/errorx"
)

type Config struct {
	Endpoint     string        `yaml:"endpoint"`
	Timeout      time.Duration `yaml:"timeout"`
	KeepAlive    time.Duration `yaml:"keep_alive"`
	MaxIdleConns int           `yaml:"max_idle_conns"`
	MaxConns     int           `yaml:"max_conns"`
}

// RegisterFlags implements flagext.Registerer
func (c *Config) RegisterFlags(flags *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("", flags)
}

// RegisterFlagsWithPrefix registers flags, adding the provided prefix if
// needed. If the prefix is not blank and doesn't end with '.', a '.' is
// appended to it.
//
//nolint:gomnd
func (c *Config) RegisterFlagsWithPrefix(prefix string, flags *flag.FlagSet) {
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix += "."
	}
	flags.StringVar(&c.Endpoint, prefix+"query-endpoint", "", "URL for queries from upstream Prometheus API.")
	flags.DurationVar(&c.Timeout, prefix+"query-timeout", 60*time.Second, "Timeout for queries to upstream Prometheus API.")
	flags.DurationVar(&c.KeepAlive, prefix+"query-keep-alive", 30*time.Second, "KeepAlive for queries to upstream Prometheus API.")
	flags.IntVar(&c.MaxIdleConns, prefix+"query-max-idle-conns", 10, "Max idle conns for queries to upstream Prometheus API.")
	flags.IntVar(&c.MaxConns, prefix+"query-max-conns", 100, "Max conns per host for queries to upstream Prometheus API.")
}

//go:generate mockery --output apimock --outpkg apimock --case underscore --name API
type API interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error)
	QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error)
	Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error)
	LabelNames(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) ([]string, v1.Warnings, error)
	LabelValues(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error)
}

type errorMappingAPI struct {
	api API
}

func mapAPIError(err error) error {
	if err == nil {
		return nil
	}
	// Map specific Prometheus API errors to internal errors that make more sense
	if v1Err := (&v1.Error{}); errors.As(err, &v1Err) && v1Err.Type == v1.ErrClient && strings.Contains(v1Err.Msg, "client error: 429") {
		return errorx.TooManyRequests{Msg: err.Error()}
	}

	return err
}

func (e errorMappingAPI) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (_ model.Value, _ v1.Warnings, err error) {
	defer func() { err = mapAPIError(err) }()
	return e.api.Query(ctx, query, ts, opts...)
}

func (e errorMappingAPI) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (_ model.Value, _ v1.Warnings, err error) {
	defer func() { err = mapAPIError(err) }()
	return e.api.QueryRange(ctx, query, r, opts...)
}

func (e errorMappingAPI) Series(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (_ []model.LabelSet, _ v1.Warnings, err error) {
	defer func() { err = mapAPIError(err) }()
	return e.api.Series(ctx, matches, startTime, endTime, opts...)
}

func (e errorMappingAPI) LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (_ []string, _ v1.Warnings, err error) {
	defer func() { err = mapAPIError(err) }()
	return e.api.LabelNames(ctx, matches, startTime, endTime, opts...)
}

func (e errorMappingAPI) LabelValues(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (_ model.LabelValues, _ v1.Warnings, err error) {
	defer func() { err = mapAPIError(err) }()
	return e.api.LabelValues(ctx, label, matches, startTime, endTime, opts...)
}

func NewAPI(cfg Config, options ...APIOption) (API, error) {
	var o apiOptions
	for _, option := range options {
		option(&o)
	}

	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, err
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

	for _, t := range o.tripperware {
		transport = t(transport)
	}

	queryClient, err := api.NewClient(api.Config{
		Address:      endpoint.String(),
		RoundTripper: transport,
	})

	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(queryClient)
	return errorMappingAPI{api: v1api}, nil
}

type APIOption func(a *apiOptions)

type apiOptions struct {
	tripperware []querymiddleware.Tripperware
}

// WithTripperware adds a tripperware to the http client's roundtripper.
// If there are multiple tripperwares, they are applied in order - e.g.
// NewAPI(cfg Config, WithTripperware(a), WithTripperware(b), WithTripperware(c))
// would result in c(b(a(httpClientRoundTripper).
func WithTripperware(tripperware querymiddleware.Tripperware) APIOption {
	return func(a *apiOptions) {
		a.tripperware = append(a.tripperware, tripperware)
	}
}
