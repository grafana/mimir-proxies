package writeproxy

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecorder(t *testing.T) {
	// Init metric Recorder
	reg := prometheus.NewRegistry()
	rec := NewRecorder(reg)

	tests := map[string]struct {
		measure        func(r Recorder)
		expMetricNames []string
		expMetrics     string
	}{
		"Measure received samples": {
			measure: func(r Recorder) {
				r.measureReceivedSamples("123", 1)
			},
			expMetricNames: []string{
				"graphite_proxy_ingester_received_samples_total",
			},
			expMetrics: `
# HELP graphite_proxy_ingester_received_samples_total The total number of received samples, excluding rejected and deduped samples.
# TYPE graphite_proxy_ingester_received_samples_total counter
graphite_proxy_ingester_received_samples_total{user="123"} 1
`,
		},
		"Measure incoming samples": {
			measure: func(r Recorder) {
				r.measureIncomingSamples("123", 1)
			},
			expMetricNames: []string{
				"graphite_proxy_ingester_samples_in_total",
			},
			expMetrics: `
# HELP graphite_proxy_ingester_samples_in_total The total number of samples that have come in to the graphite write proxy, including rejected or deduped samples.
# TYPE graphite_proxy_ingester_samples_in_total counter
graphite_proxy_ingester_samples_in_total{user="123"} 1
`,
		},
		"Measure rejected samples": {
			measure: func(r Recorder) {
				r.measureRejectedSamples("123", "foo_reason")
			},
			expMetricNames: []string{
				"graphite_proxy_ingester_rejected_samples_total",
			},
			expMetrics: `
# HELP graphite_proxy_ingester_rejected_samples_total The total number of samples that were rejected.
# TYPE graphite_proxy_ingester_rejected_samples_total counter
graphite_proxy_ingester_rejected_samples_total{reason="foo_reason", user="123"} 1
`,
		},
		"Measure conversion duration": {
			measure: func(r Recorder) {
				r.measureConversionDuration("123", 15*time.Second)
			},
			expMetricNames: []string{
				"graphite_proxy_ingester_data_conversion_seconds",
			},
			expMetrics: `
# HELP graphite_proxy_ingester_data_conversion_seconds Time (in seconds) spent converting ingested Graphite data into Prometheus data.
# TYPE graphite_proxy_ingester_data_conversion_seconds histogram
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.005"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.01"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.025"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.05"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.1"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.25"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.5"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="1"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="2.5"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="5"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="10"} 0
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="25"} 1
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="50"} 1
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="100"} 1
graphite_proxy_ingester_data_conversion_seconds_bucket{user="123",le="+Inf"} 1
graphite_proxy_ingester_data_conversion_seconds_sum{user="123"} 15
graphite_proxy_ingester_data_conversion_seconds_count{user="123"} 1
`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// Measure metrics
			test.measure(rec)

			err := testutil.GatherAndCompare(reg, strings.NewReader(test.expMetrics), test.expMetricNames...)
			assert.NoError(err)
		})
	}
}
