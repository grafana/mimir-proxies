package writeproxy

import (
	"time"

	"github.com/grafana/dskit/instrument"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	prefix = "graphite_proxy_ingester"
)

//go:generate mockery --inpackage --testonly --case underscore --name Recorder
type Recorder interface {
	measureReceivedRequest(user string)
	measureIncomingRequest(user string)
	measureReceivedSamples(user string, count int)
	measureIncomingSamples(user string, count int)
	measureRejectedSamples(user, reason string)
	measureConversionDuration(user string, duration time.Duration)
}

// NewRecorder returns a new Prometheus metrics Recorder.
// It ensures that the graphite ingester metrics are properly registered.
func NewRecorder(reg prometheus.Registerer) Recorder {
	r := &prometheusRecorder{
		receivedRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "received_requests_total",
			Help:      "The total number of received requests, excluding rejected requests.",
		}, []string{"user"}),
		incomingRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "requests_in_total",
			Help: "The total number of requests that have come in to the graphite write proxy, including rejected " +
				"requests.",
		}, []string{"user"}),
		receivedSamples: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "received_samples_total",
			Help:      "The total number of received samples, excluding rejected and deduped samples.",
		}, []string{"user"}),
		incomingSamples: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "samples_in_total",
			Help: "The total number of samples that have come in to the graphite write proxy, including rejected " +
				"or deduped samples.",
		}, []string{"user"}),
		rejectedSamples: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "rejected_samples_total",
			Help:      "The total number of samples that were rejected.",
		}, []string{"user", "reason"}),
		conversionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: prefix,
			Name:      "data_conversion_seconds",
			Help:      "Time (in seconds) spent converting ingested Graphite data into Prometheus data.",
			Buckets:   instrument.DefBuckets,
		}, []string{"user"}),
	}

	reg.MustRegister(r.receivedRequests, r.incomingRequests, r.receivedSamples, r.incomingSamples, r.rejectedSamples,
		r.conversionDuration)

	return r
}

// prometheusRecorder knows the metrics of the ingester and how to measure them for
// Prometheus.
type prometheusRecorder struct {
	receivedRequests   *prometheus.CounterVec
	incomingRequests   *prometheus.CounterVec
	receivedSamples    *prometheus.CounterVec
	incomingSamples    *prometheus.CounterVec
	rejectedSamples    *prometheus.CounterVec
	conversionDuration *prometheus.HistogramVec
}

// measureReceivedRequests measures the total amount of received requests on Prometheus.
func (r prometheusRecorder) measureReceivedRequest(user string) {
	r.receivedRequests.WithLabelValues(user).Inc()
}

// measureIncomingRequests measures the total amount of incoming requests on Prometheus.
func (r prometheusRecorder) measureIncomingRequest(user string) {
	r.incomingRequests.WithLabelValues(user).Inc()
}

// measureMetricsParsed measures the total amount of received samples on Prometheus.
func (r prometheusRecorder) measureReceivedSamples(user string, count int) {
	r.receivedSamples.WithLabelValues(user).Add(float64(count))
}

// measureIncomingSamples measures the total amount of incoming samples on Prometheus.
func (r prometheusRecorder) measureIncomingSamples(user string, count int) {
	r.incomingSamples.WithLabelValues(user).Add(float64(count))
}

// measureRejectedSamples measures the total amount of rejected samples on Prometheus.
func (r prometheusRecorder) measureRejectedSamples(user, reason string) {
	r.rejectedSamples.WithLabelValues(user, reason).Add(1)
}

// measureConversionDuration measures the total time spent translating samples to Prometheus format
func (r prometheusRecorder) measureConversionDuration(user string, duration time.Duration) {
	r.conversionDuration.WithLabelValues(user).Observe(duration.Seconds())
}
