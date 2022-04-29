package memcached

import (
	"errors"
	"os"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"github.com/prometheus/client_golang/prometheus"
)

type Recorder interface {
	measure(string, time.Duration, error)
}

func NewRecorder(reg prometheus.Registerer) Recorder {
	hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "datadog_proxy",
		Name:      "memcached_operation_duration_seconds",
		Help:      "Duration of different memcached operations.",
		Buckets:   []float64{.0005, .001, .0025, .005, .0075, .010, .015, .025, .050, .100, 1},
	}, []string{"operation", "result"})
	reg.MustRegister(hist)
	return prometheusRecorder{histogram: hist}
}

//go:generate mockery --case underscore --inpackage --testonly --name Recorder
var _ Recorder = prometheusRecorder{}

type prometheusRecorder struct {
	histogram *prometheus.HistogramVec
}

func (p prometheusRecorder) measure(op string, duration time.Duration, err error) {
	p.histogram.WithLabelValues(op, p.errToResult(err)).Observe(duration.Seconds())
}

func (prometheusRecorder) errToResult(err error) string {
	if err == nil {
		return "success"
	}

	switch {
	case errors.Is(err, memcache.ErrCacheMiss):
		return "cache_miss"
	case errors.Is(err, memcache.ErrCASConflict):
		return "cas_conflict"
	case errors.Is(err, memcache.ErrNotStored):
		return "not_stored"
	case errors.Is(err, memcache.ErrServerError):
		return "server_error"
	case errors.Is(err, memcache.ErrNoStats):
		return "no_stats"
	case errors.Is(err, memcache.ErrMalformedKey):
		return "malformed_key"
	case errors.Is(err, memcache.ErrNoServers):
		return "no_servers"
	case errors.Is(err, os.ErrDeadlineExceeded):
		return "deadline_exceeded"
	default:
		return "unmapped_error"
	}
}
