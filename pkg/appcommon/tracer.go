package appcommon

import (
	"fmt"
	"io"

	"github.com/uber/jaeger-client-go"

	"github.com/go-kit/log/level"

	"github.com/go-kit/log"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

// NewTracer creates a new tracer using environment variables.
// Setting the environment variable JAEGER_AGENT_HOST enables tracing.
func NewTracer(name string, logger log.Logger) (opentracing.Tracer, io.Closer, error) {
	logger = log.With(logger, "component", "jaeger")
	level.Info(logger).Log("msg", "Setting up tracing", "service_name", name)
	jCfg, err := config.FromEnv()
	if err != nil {
		return nil, nil, err
	}

	jLogger := newJaegerLogger(logger)

	jCfg.ServiceName = name
	jCfg.Sampler.Options = append(jCfg.Sampler.Options, jaeger.SamplerOptions.Logger(jLogger))

	tracer, closer, err := jCfg.NewTracer(config.Metrics(prometheus.New()), config.Logger(jLogger))
	if err != nil {
		return nil, nil, err
	}

	return tracer, closer, nil
}

func newJaegerLogger(logger log.Logger) jaeger.Logger {
	return &jaegerLogger{logger: logger}
}

type jaegerLogger struct {
	logger log.Logger
}

func (l jaegerLogger) Error(msg string) {
	level.Error(l.logger).Log("msg", msg)
}

func (l jaegerLogger) Infof(msg string, args ...interface{}) {
	level.Info(l.logger).Log("msg", fmt.Sprintf(msg, args...))
}

func (l jaegerLogger) Debugf(msg string, args ...interface{}) {
	level.Debug(l.logger).Log("msg", fmt.Sprintf(msg, args...))
}
