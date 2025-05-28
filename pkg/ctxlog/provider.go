package ctxlog

import (
	"context"
	"net/http"

	"github.com/go-kit/log"
	"github.com/grafana/mimir-graphite/v2/pkg/server/middleware"
)

type key int

const contextKey key = 0

type Provider interface {
	For(ctx context.Context) LevelLogger
	ContextWith(ctx context.Context, keyvals ...interface{}) context.Context
	ContextWithRequest(r *http.Request) context.Context
	Logger() log.Logger
}

func NewProvider(logger log.Logger) Provider {
	return goKitProvider{logger: logger}
}

type goKitProvider struct {
	logger log.Logger
}

// ContextWith provides a context with given keyvals as additional baggage
func (p goKitProvider) ContextWith(ctx context.Context, keyvals ...interface{}) context.Context {
	values, ok := ctx.Value(contextKey).([]interface{})
	if !ok {
		return context.WithValue(ctx, contextKey, keyvals)
	}
	clone := make([]interface{}, len(values)+len(keyvals))
	copy(clone, values)
	copy(clone[len(values):], keyvals)
	return context.WithValue(ctx, contextKey, clone)
}

// ContextWithRequest provides a logging context with request details as additional baggage
func (p goKitProvider) ContextWithRequest(r *http.Request) context.Context {
	ctx := r.Context()
	traceID, ok := middleware.ExtractTraceID(ctx)
	if ok {
		ctx = p.ContextWith(ctx, "traceID", traceID)
	}
	ctx = p.ContextWith(ctx, "request_uri", r.RequestURI, "method", r.Method)
	return ctx
}

// For provides a logger for the given context
func (p goKitProvider) For(ctx context.Context) LevelLogger {
	return goKitLevelLogger{log.With(p.logger, BaggageFrom(ctx)...)}
}

// BaggageFrom is used to extract the baggage from a context.
// Useful for testing
func BaggageFrom(ctx context.Context) []interface{} {
	values, ok := ctx.Value(contextKey).([]interface{})
	if !ok {
		return nil
	}
	return values
}

// Logger returns the underlying logger for the context log provider. This can
// be used when a context is not available.
func (p goKitProvider) Logger() log.Logger {
	return p.logger
}
