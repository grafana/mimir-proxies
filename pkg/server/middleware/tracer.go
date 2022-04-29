package middleware

import (
	"fmt"
	"net/http"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	jaeger "github.com/uber/jaeger-client-go"
	"golang.org/x/net/context"
)

// Tracer is a middleware which traces incoming requests.
type Tracer struct {
	routeMatcher RouteMatcher
}

func NewTracer(routeMatcher RouteMatcher, tracer opentracing.Tracer) *Tracer {
	if tracer != nil {
		opentracing.SetGlobalTracer(tracer)
	}
	return &Tracer{
		routeMatcher: routeMatcher,
	}
}

// Wrap implements Interface
func (t Tracer) Wrap(next http.Handler) http.Handler {
	options := []nethttp.MWOption{
		nethttp.OperationNameFunc(func(r *http.Request) string {
			op := getRouteName(t.routeMatcher, r)
			if op == "" {
				return "HTTP " + r.Method
			}

			return fmt.Sprintf("HTTP %s - %s", r.Method, op)
		}),
	}

	return nethttp.Middleware(opentracing.GlobalTracer(), next, options...)
}

// ExtractTraceID extracts the trace id, if any from the context.
func ExtractTraceID(ctx context.Context) (string, bool) {
	sp := opentracing.SpanFromContext(ctx)
	if sp == nil {
		return "", false
	}
	sctx, ok := sp.Context().(jaeger.SpanContext)
	if !ok {
		return "", false
	}

	return sctx.TraceID().String(), true
}

// ExtractSampledTraceID works like ExtractTraceID but the returned bool is only
// true if the returned trace id is sampled.
func ExtractSampledTraceID(ctx context.Context) (string, bool) {
	sp := opentracing.SpanFromContext(ctx)
	if sp == nil {
		return "", false
	}
	sctx, ok := sp.Context().(jaeger.SpanContext)
	if !ok {
		return "", false
	}

	return sctx.TraceID().String(), sctx.IsSampled()
}
