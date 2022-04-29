package ingester

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecorder(t *testing.T) {
	tests := map[string]struct {
		measure        func(r Recorder)
		expMetricNames []string
		expMetrics     string
	}{
		"Measure proxy metrics parsed": {
			measure: func(r Recorder) {
				r.measureMetricsParsed(8)
			},
			expMetricNames: []string{
				"datadog_proxy_ingester_metrics_parsed_total",
			},
			expMetrics: "" +
				"# HELP datadog_proxy_ingester_metrics_parsed_total The total number of metrics that have been parsed.\n" +
				"# TYPE datadog_proxy_ingester_metrics_parsed_total counter\n" +
				"datadog_proxy_ingester_metrics_parsed_total 8\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// Init metric Recorder
			reg := prometheus.NewRegistry()

			rec := NewRecorder(reg)

			// Measure metrics
			test.measure(rec)

			err := testutil.GatherAndCompare(reg, strings.NewReader(test.expMetrics), test.expMetricNames...)
			assert.NoError(err)
		})
	}
}
