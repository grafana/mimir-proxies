package remoteread

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

//go:generate mockery --case underscore --inpackage --testonly --name Recorder
type Recorder interface {
	measure(string, time.Duration, error)
}

func NewRecorder(namespacePrefix string, reg prometheus.Registerer) Recorder {
	hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespacePrefix + "_remote_read_client",
		Name:      "request_duration_seconds",
		Help:      "Client-side duration of remote read calls.",
	}, []string{"operation", "result"})
	reg.MustRegister(hist)

	return &prometheusRecorder{histogram: hist}
}

type prometheusRecorder struct {
	histogram *prometheus.HistogramVec
}

func (p prometheusRecorder) measure(op string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	p.histogram.WithLabelValues(op, result).Observe(duration.Seconds())
}
