package remotewrite

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

//go:generate mockery --inpackage --testonly --case underscore --name Recorder
type Recorder interface {
	measureOutOfOrderSamples(count int)
	measure(string, time.Duration, error)
}

// NewRecorder returns a new Prometheus metrics Recorder.
// It ensures that the ingester metrics are properly registered.
func NewRecorder(prefix string, reg prometheus.Registerer) Recorder {
	r := &prometheusRecorder{
		outOfOrderWrites: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "out_of_order_writes_total",
			Help:      "The total number of out of order writes to Cortex.",
		}, []string{}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: prefix + "_remote_write_client",
			Name:      "request_duration_seconds",
			Help:      "Client-side duration of remote write calls.",
		}, []string{"operation", "result"}),
	}

	reg.MustRegister(r.outOfOrderWrites)
	reg.MustRegister(r.requestDuration)

	return r
}

// prometheusRecorder knows the metrics of the ingester and how to measure them for
// Prometheus.
type prometheusRecorder struct {
	outOfOrderWrites *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
}

func (r prometheusRecorder) measureOutOfOrderSamples(count int) {
	r.outOfOrderWrites.WithLabelValues().Add(float64(count))
}

func (r prometheusRecorder) measure(op string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	r.requestDuration.WithLabelValues(op, result).Observe(duration.Seconds())
}
