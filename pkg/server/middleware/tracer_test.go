package middleware

import (
	"context"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"go.opentelemetry.io/otel"
	bridge "go.opentelemetry.io/otel/bridge/opentracing"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestExtractSampledTraceID(t *testing.T) {
	testCases := []struct {
		desc  string
		ctx   func(*testing.T) (context.Context, func())
		empty bool
	}{
		{
			desc: "OpenTracing with Jaeger",
			ctx:  getContextWithOpenTracing,
		},
		{
			desc: "OpenTelemetry",
			ctx:  getContextWithOpenTelemetry,
		},
		{
			desc: "OpenTelemetry with the OpentTracing bridge",
			ctx:  getContextWithOpenTelemetryWithBridge,
		},
		{
			desc: "No tracer",
			ctx: func(_ *testing.T) (context.Context, func()) {
				return context.Background(), func() {}
			},
			empty: true,
		},
		{
			desc:  "OpenTelemetry with the noop",
			ctx:   getContextWithOpenTelemetryNoop,
			empty: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx, closer := tC.ctx(t)
			defer closer()
			// test contract of ExtractSampledTraceID
			sampledTraceID, sampled := ExtractSampledTraceID(ctx)
			traceID, ok := ExtractTraceID(ctx)

			assert.Equal(t, sampledTraceID, traceID, "Expected sampledTraceID to equal traceID")
			if tC.empty {
				assert.Empty(t, traceID, "Expected traceID to be empty")
				assert.False(t, sampled, "Expected sampled to be false")
				assert.False(t, ok, "Expected ok to be false")
			} else {
				assert.NotEmpty(t, traceID, "Expected traceID to be non-empty")
				assert.True(t, sampled, "Expected sampled to be true")
				assert.True(t, ok, "Expected ok to be true")
			}
		})
	}
}

func getContextWithOpenTracing(t *testing.T) (context.Context, func()) {
	jCfg, err := config.FromEnv()
	require.NoError(t, err)

	jCfg.ServiceName = "test"
	jCfg.Sampler.Options = append(jCfg.Sampler.Options, jaeger.SamplerOptions.InitialSampler(jaeger.NewConstSampler(true)))

	tracer, closer, err := jCfg.NewTracer()
	require.NoError(t, err)

	opentracing.SetGlobalTracer(tracer)

	sp := opentracing.GlobalTracer().StartSpan("test")
	return opentracing.ContextWithSpan(context.Background(), sp), func() {
		sp.Finish()
		_ = closer.Close()
	}
}

func getContextWithOpenTelemetryWithBridge(t *testing.T) (context.Context, func()) {
	previous := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tr := tp.Tracer("test")

	otTracer, _ := bridge.NewTracerPair(tr)
	opentracing.SetGlobalTracer(otTracer)

	sp := opentracing.GlobalTracer().StartSpan("test")
	return opentracing.ContextWithSpan(context.Background(), sp), func() {
		sp.Finish()
		otel.SetTracerProvider(previous)
	}
}

func getContextWithOpenTelemetry(t *testing.T) (context.Context, func()) {
	previous := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tr := tp.Tracer("test")
	ctx, sp := tr.Start(context.Background(), "test")
	return ctx, func() {
		sp.End()
		otel.SetTracerProvider(previous)
	}
}

func getContextWithOpenTelemetryNoop(t *testing.T) (context.Context, func()) {
	ctx, sp := noop.NewTracerProvider().Tracer("test").Start(context.Background(), "test")

	// sanity check
	require.False(t, sp.SpanContext().TraceID().IsValid())

	return ctx, func() {
		sp.End()
	}
}
