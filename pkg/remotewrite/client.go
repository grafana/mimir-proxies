package remotewrite

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/mimir/pkg/distributor"
	"github.com/grafana/mimir/pkg/frontend/querymiddleware"

	"github.com/grafana/mimir-proxies/pkg/appcommon"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/grafana/dskit/user"
	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/mwitkow/go-conntrack"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/grafana/mimir-proxies/pkg/errorx"
)

const (
	// maxErrMsgLen is copied from github.com/prometheus/prometheus/storage/remote
	maxErrMsgLen = 512
)

const defaultWriteTimeout = 1 * time.Second

// Client provides Prometheus Remote Write API access functionality
//
//go:generate mockery --output remotewritemock --outpkg remotewritemock --case underscore --name Client
type Client interface {
	Write(ctx context.Context, req *mimirpb.WriteRequest) error
}

type Config struct {
	Endpoint            string        `yaml:"endpoint"`
	Timeout             time.Duration `yaml:"timeout"`
	KeepAlive           time.Duration `yaml:"keep_alive"`
	MaxIdleConns        int           `yaml:"max_idle_conns"`
	MaxConns            int           `yaml:"max_conns"`
	SkipLabelValidation bool          `yaml:"skip_label_validation"`
	UserAgent           string        `yaml:"user_agent"`
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
	flags.StringVar(&c.Endpoint, prefix+"write-endpoint", "", "URL for writes to upstream Prometheus remote write API (including the /push suffix if needed).")
	flags.DurationVar(&c.Timeout, prefix+"write-timeout", defaultWriteTimeout, "Timeout for writes to upstream Prometheus remote write API.")
	flags.DurationVar(&c.KeepAlive, prefix+"write-keep-alive", 30*time.Second, "KeepAlive for write to upstream Prometheus remote write API.")
	flags.IntVar(&c.MaxIdleConns, prefix+"write-max-idle-conns", 10, "Max idle conns per host for writes to upstream Prometheus remote write API.")
	flags.IntVar(&c.MaxConns, prefix+"write-max-conns", 100, "Max open conns per host for writes to upstream Prometheus remote write API.")
	flags.BoolVar(&c.SkipLabelValidation, prefix+"skip-label-validation", false, "If set to true sends requests with headers to skip label validation.")
	flags.StringVar(&c.UserAgent, prefix+"user-agent", "", "User agent for proxy ingester")
}

// NewClient creates the default http implementation of the Client
func NewClient(cfg Config, metricsRecorder Recorder, tripperware querymiddleware.Tripperware) (Client, error) {
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport.MaxIdleConnsPerHost = cfg.MaxIdleConns
	httpTransport.MaxIdleConns = cfg.MaxIdleConns
	httpTransport.MaxConnsPerHost = cfg.MaxConns
	httpTransport.DialContext = conntrack.NewDialContextFunc(
		conntrack.DialWithName("remotewrite"),
		conntrack.DialWithTracing(),
		conntrack.DialWithDialer(&net.Dialer{
			Timeout:   cfg.Timeout,
			KeepAlive: cfg.KeepAlive,
		}),
	)

	transport := appcommon.NewTracedAuthRoundTripper(httpTransport, "Remote Write")
	if tripperware != nil {
		transport = tripperware(transport)
	}

	httpClient := &http.Client{Transport: transport}

	return &client{
		cfg:        cfg,
		httpClient: httpClient,
		endpoint:   endpoint.String(),
		recorder:   metricsRecorder,
	}, nil
}

type client struct {
	cfg        Config
	httpClient *http.Client
	endpoint   string
	recorder   Recorder
}

// Write remote metrics into Prometheus Remote Write API
// Inspired by https://github.com/prometheus/prometheus/blob/7bf76af6dffc020fb7c4d489694bb8db0a223add/storage/remote/client.go#L162-L220
// Plus added proto marshaling and snappy encoding
func (c *client) Write(ctx context.Context, req *mimirpb.WriteRequest) error {
	// TODO: use a pool of proto.Buffer here when performance becomes a priority
	data, err := proto.Marshal(req)
	if err != nil {
		return errorx.Internal{Msg: "can't marshal write request", Err: err}
	}

	// TODO: use a pool of buffers here when performance becomes a priority
	compressed := snappy.Encode(nil, data)

	httpReq, err := http.NewRequest("POST", c.endpoint, bytes.NewReader(compressed))
	if err != nil {
		// Errors from NewRequest are from unparsable URLs, so are not
		// recoverable.
		return errorx.Internal{Msg: "can't create write request", Err: err}
	}

	httpReq.Header.Add("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("User-Agent", c.cfg.UserAgent)
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	if c.cfg.SkipLabelValidation {
		httpReq.Header.Set(distributor.SkipLabelNameValidationHeader, "true")
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	httpReq = httpReq.WithContext(ctx)
	if err := user.InjectOrgIDIntoHTTPRequest(ctx, httpReq); err != nil {
		return errorx.BadRequest{Msg: "can't set org ID on write request", Err: err}
	}

	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		var ht *nethttp.Tracer
		httpReq, ht = nethttp.TraceRequest(
			parentSpan.Tracer(),
			httpReq,
			nethttp.OperationName("Remote Store"),
			nethttp.ClientTrace(false),
		)
		defer ht.Finish()
	}

	return c.do(httpReq)
}

func (c *client) do(httpReq *http.Request) error {
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Errors from Client.Do are from (for example) network errors, so are
		// recoverable.
		return errorx.Internal{Msg: "can't perform metrics write request", Err: err}
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.errFromResp(resp)
	}
	return nil
}

func (c *client) errFromResp(resp *http.Response) error {
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
	line := ""
	if scanner.Scan() {
		line = scanner.Text()
	}
	err := errors.Errorf("remote write API returned HTTP status %s: %s", resp.Status, line)

	if resp.StatusCode == http.StatusBadRequest {
		if strings.Contains(line, "out of order sample") {
			c.recorder.measureOutOfOrderSamples(1)
		}
		return errorx.BadRequest{Msg: "bad metrics write request", Err: err}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return errorx.TooManyRequests{Msg: "too many write requests", Err: err}
	}

	return errorx.Internal{Msg: "failed writing metrics", Err: err}
}
