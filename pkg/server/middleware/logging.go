package middleware

import (
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/uber/jaeger-client-go"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/user"
)

type Log struct {
	logger log.Logger
}

func NewLoggingMiddleware(logger log.Logger) *Log {
	// If user doesn't supply a logging implementation, by default instantiate
	// go kit logger.
	if logger == nil {
		logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	}

	return &Log{logger: logger}
}

// Wrap implements commons middleware interface
func (l Log) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := newResponseWriterSniffer(w)
		next.ServeHTTP(wrapped, r)
		statusCode, writeErr := wrapped.statusCode, wrapped.writeError

		if writeErr != nil {
			logger := log.With(l.logger, "err", writeErr, "msg", "couldn't write response body")
			logRequest(logger, r, statusCode)
			return
		}
		logRequest(l.logger, r, statusCode)
	})
}

func logRequest(logger log.Logger, r *http.Request, statusCode int) {
	if begin, found := extractRequestBeginTime(r.Context()); found {
		logger = log.With(logger, "elapsed", time.Since(begin))
	}

	traceID, ok := ExtractSampledTraceID(r.Context())
	if traceID != "" { // we want to log the IDs for traces even if they aren't sampled
		logger = log.With(logger, "traceID", traceID, "sampled", ok)

		// Setting JaegerDebugHeader forces a trace to be sampled and tags it with the provided debug id
		// We are logging the debug id if it's present to allow us to search for it in logs, as Tempo doesn't support
		// searching by tag yet.
		// The debug id is present in the Jaeger span struct, but is package-private, therefore we have to extract it
		// from the request headers instead.
		if dh := r.Header.Get(jaeger.JaegerDebugHeader); dh != "" {
			logger = log.With(logger, "jaegerDebugID", dh)
		}
	}

	orgID, err := user.ExtractOrgID(r.Context())
	if err == nil {
		logger = log.With(logger, "orgID", orgID)
	}

	userID, err := user.ExtractUserID(r.Context())
	if err == nil {
		logger = log.With(logger, "userID", userID)
	}

	// Happy path, status codes that we like: status code between [100,500) or status code is 502 or 503
	if http.StatusContinue <= statusCode && statusCode < http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable {
		level.Info(logger).Log(
			"method", r.Method,
			"uri", redactAPIKey(r.URL).RequestURI(),
			"status", statusCode,
		)
	} else {
		level.Warn(logger).Log(
			"method", r.Method,
			"uri", redactAPIKey(r.URL).RequestURI(),
			"status", statusCode,
		)
	}
}

// redactAPIKey modifies the provided URL's query redacting the api_key param if it's not empty and it's not grafana-labs
// Then it returns the same URL (notice that the original URL is modified too)
func redactAPIKey(u *url.URL) *url.URL {
	reqQuery := u.Query()
	if reqQuery.Get("api_key") != "" && reqQuery.Get("api_key") != "grafana-labs" {
		reqQuery.Set("api_key", "redacted")
		u.RawQuery = reqQuery.Encode()
	}
	return u
}
