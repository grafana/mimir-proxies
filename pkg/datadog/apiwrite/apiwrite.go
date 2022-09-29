package apiwrite

import (
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/mimir-proxies/pkg/ctxlog"
	"github.com/grafana/mimir-proxies/pkg/datadog/ddstructs"
	"github.com/grafana/mimir-proxies/pkg/datadog/ingester"
	"github.com/grafana/mimir-proxies/pkg/errorx"
	"github.com/grafana/mimir-proxies/pkg/route"

	"github.com/opentracing/opentracing-go"

	"github.com/gorilla/mux"
)

const (
	httpStatusCanceled = 499
	defaultHTTPTimeout = 5 * time.Second
)

type Config struct {
	Timeouts struct {
		V1 struct {
			// Write path
			Series   time.Duration `yaml:"series"`
			CheckRun time.Duration `yaml:"check_run"`
			Sketches time.Duration `yaml:"sketches"`
		} `yaml:"v1"`
		Intake time.Duration
	} `yaml:"timeouts"`
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (cfg *Config) RegisterFlags(flags *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", flags)
}

// RegisterFlagsWithPrefix registers flags, adding the provided prefix if
// needed. If the prefix is not blank and doesn't end with '.', a '.' is
// appended to it.
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, flags *flag.FlagSet) {
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix += "."
	}
	flags.DurationVar(&cfg.Timeouts.V1.Series, prefix+"api.v1-series-timeout", defaultHTTPTimeout, "Sets api/v1/series timeout, by default 5 seconds")
	flags.DurationVar(&cfg.Timeouts.V1.CheckRun, prefix+"api.v1-check-run-timeout", defaultHTTPTimeout, "Sets api/v1/check_run timeout, by default 5 seconds")
	flags.DurationVar(&cfg.Timeouts.Intake, prefix+"api.intake-timeout", defaultHTTPTimeout, "Sets /intake timeout, by default 5 seconds")
	flags.DurationVar(&cfg.Timeouts.V1.Sketches, prefix+"api.v1-sketches-timeout", defaultHTTPTimeout, "Sets api/v1/sketches and api/beta/sketches timeout, by default 5 seconds")
}

type API struct {
	log    ctxlog.Provider
	tracer opentracing.Tracer

	ingester ingester.Ingester

	cfg Config
}

func NewAPI(cfg Config, ctxlogProvider ctxlog.Provider, ingester ingester.Ingester, tracer opentracing.Tracer) *API {
	return &API{
		ingester: ingester,

		log:    ctxlogProvider,
		tracer: tracer,

		cfg: cfg,
	}
}

// RegisterAll registers all the routes that the API provides. As well as
// registering the Datadog APIs, it also registers a not-found handler.
// This method is used by the Cloud binaries.
func (a *API) RegisterAll(router *mux.Router) {
	registerer := route.NewMuxRegisterer(router)
	a.RegisterWritePath(registerer)
	a.RegisterValidatePath(registerer)

	// catch the rest of the requests and log them
	router.NotFoundHandler = http.HandlerFunc(a.handleNotFound)
}

func (a *API) RegisterWritePath(registerer route.Registerer) {
	registerer.RegisterRoute("/api/v1/series", http.HandlerFunc(a.handleSeriesPush), http.MethodPost)
	registerer.RegisterRoute("/api/v1/check_run", http.HandlerFunc(a.handleCheckRun), http.MethodPost)
	registerer.RegisterRoute("/intake/", http.HandlerFunc(a.handleIntake), http.MethodPost)

	// sketches are the distribution metrics from datadog, we don't support them
	registerer.RegisterRoute("/api/v1/sketches", http.HandlerFunc(a.handleSketches), http.MethodPost)
	registerer.RegisterRoute("/api/beta/sketches", http.HandlerFunc(a.handleSketches), http.MethodPost)
}

func (a *API) RegisterValidatePath(registerer route.Registerer) {
	// validate is used by both datadog-agent and datasource plugin to check that they have valid credentials configured
	registerer.RegisterRoute("/api/v1/validate", http.HandlerFunc(a.handleValidate), http.MethodGet)
}

type SeriesPushPayload struct {
	Series ddstructs.Series `json:"series"`
}

func (a *API) handleSeriesPush(w http.ResponseWriter, r *http.Request) {
	// Sent data:
	// https://github.com/DataDog/datadog-agent/blob/66c7bed107756702e043f3d9f004fa7967437044/pkg/metrics/series.go#L138-L140
	//  - some metric names start with `n_o_i_n_d_e_x.` ??
	var payload SeriesPushPayload
	ctx, ok := a.readJSONRequest(w, r, &payload)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeouts.V1.Series)
	defer cancel()

	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, a.tracer, "api.handleSeriesPush")
	defer sp.Finish()

	sp.LogKV("series_count", len(payload.Series))
	if len(payload.Series) > 0 {
		sp.LogKV("example_metric", payload.Series[0].Name)
	}

	if err := a.ingester.StoreMetrics(ctx, payload.Series); err != nil {
		a.handleError(ctx, w, err)
		return
	}

	sp.LogKV("msg", "successfully stored series")
	a.log.For(ctx).Debug("msg", "successfully stored series")
}

func (a *API) handleIntake(w http.ResponseWriter, r *http.Request) {
	payload := ddstructs.Payload{}
	ctx, ok := a.readJSONRequest(w, r, &payload)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeouts.Intake)
	defer cancel()

	if payload.HostTags != nil {
		ctx = a.log.ContextWith(ctx, "hostname", payload.InternalHostname, "host_tags_count", len(payload.HostTags.System))
		err := a.ingester.StoreHostTags(ctx, payload.InternalHostname, payload.HostTags.System)
		if err != nil {
			a.handleError(ctx, w, err)
			return
		}
		a.log.For(ctx).Debug("msg", "successfully handled intake with host tags")
	} else {
		a.log.For(ctx).Debug("msg", "successfully handled intake without host tags")
	}
}

func (a *API) handleCheckRun(w http.ResponseWriter, r *http.Request) {
	var checks ddstructs.ServiceChecks
	ctx, ok := a.readJSONRequest(w, r, &checks)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeouts.V1.CheckRun)
	defer cancel()

	err := a.ingester.StoreCheckRun(ctx, checks)
	if err != nil {
		a.handleError(ctx, w, err)
		return
	}

	a.log.For(ctx).Debug("msg", "successfully stored series")
}

func (a *API) readJSONRequest(w http.ResponseWriter, r *http.Request, body interface{}) (context.Context, bool) {
	ctx := a.log.ContextWithRequest(r)

	buff, err := readAll(r.Body, r.Header.Get("Content-Encoding") == "deflate")
	if err != nil {
		code := http.StatusBadRequest
		a.log.For(ctx).Warn("msg", "can't read body", "response_code", code, "err", err)
		httpError(ctx, w, err.Error(), code)
		return ctx, false
	}

	if err := json.Unmarshal(buff, body); err != nil {
		code := http.StatusBadRequest
		a.log.For(ctx).Warn("msg", "can't unmarshal json", "response_code", code, "err", err)
		httpError(ctx, w, err.Error(), code)
		return ctx, false
	}

	return ctx, true
}

func (a *API) handleError(ctx context.Context, w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	var errx errorx.Error
	if errors.Is(err, context.Canceled) {
		code = httpStatusCanceled
		a.log.For(ctx).Error("msg", "canceled", "response_code", code, "err", err)
	} else if errors.As(err, &errx) {
		switch code = errx.HTTPStatusCode(); code {
		case http.StatusBadRequest:
			a.log.For(ctx).Warn("msg", errx.Message(), "response_code", code, "err", tryUnwrap(errx))
		default:
			a.log.For(ctx).Error("msg", errx.Message(), "response_code", code, "err", tryUnwrap(errx))
		}
	} else {
		a.log.For(ctx).Error("msg", "unknown error", "response_code", code, "err", err)
	}
	httpError(ctx, w, err.Error(), code)
}

func readAll(body io.Reader, encoded bool) ([]byte, error) {
	var err error
	if encoded {
		body, err = zlib.NewReader(body)
		if err != nil {
			return nil, err
		}
	}
	return io.ReadAll(body)
}

// handleValidate always returns 200 and an empty json object.
// this is used by both plugin and agent to validate their credentials
func (a *API) handleValidate(w http.ResponseWriter, r *http.Request) {
	ctx := a.log.ContextWithRequest(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(struct{}{})
	if err != nil {
		a.log.For(ctx).Error("msg", "can't write response", "response_code", http.StatusInternalServerError, "err", err)
		httpError(ctx, w, err.Error(), http.StatusInternalServerError)
	}
}

// handleSketches handles sketches by dropping them since they're not implemented yet but we don't want the noise in the agent.
func (a *API) handleSketches(w http.ResponseWriter, r *http.Request) {
	ctx := a.log.ContextWithRequest(r)

	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeouts.V1.Sketches)
	defer cancel()

	a.log.For(ctx).Info("msg", "received sketches, responding not found", "response_code", http.StatusNotFound)
	httpError(ctx, w, "sketches are not implemented", http.StatusNotFound)
}

// handleNotFound returns a 404 but also logs the request so we can see what is being requested
func (a *API) handleNotFound(w http.ResponseWriter, r *http.Request) {
	ctx := a.log.ContextWithRequest(r)
	a.log.For(ctx).Error("msg", "not found", "response_code", http.StatusNotFound)
	httpError(ctx, w, "not found", http.StatusNotFound)
}

func tryUnwrap(err error) error {
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return wrapped.Unwrap()
	}
	return err
}

// httpError is a wrapper around http.Error() that also logs the error in a trace.
func httpError(ctx context.Context, w http.ResponseWriter, err string, code int) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		span.LogKV("error_msg", err)
	}

	http.Error(w, err, code)
}
