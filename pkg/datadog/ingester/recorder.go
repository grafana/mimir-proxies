package ingester

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	prefix = "datadog_proxy_ingester"
)

//go:generate mockery --inpackage --testonly --case underscore --name Recorder
type Recorder interface {
	measureMetricsParsed(count int)
}

// NewRecorder returns a new Prometheus metrics Recorder.
// It ensures that the ingester metrics are properly registered.
func NewRecorder(reg prometheus.Registerer) Recorder {
	r := &prometheusRecorder{
		proxyMetricsParsed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "metrics_parsed_total",
			Help:      "The total number of metrics that have been parsed.",
		}, []string{}),
	}

	reg.MustRegister(r.proxyMetricsParsed)

	return r
}

// prometheusRecorder knows the metrics of the ingester and how to measure them for
// Prometheus.
type prometheusRecorder struct {
	proxyMetricsParsed *prometheus.CounterVec
}

// measureMetricsParsed measures the total amount of parsed metrics on Prometheus.
func (r prometheusRecorder) measureMetricsParsed(count int) {
	r.proxyMetricsParsed.WithLabelValues().Add(float64(count))
}
