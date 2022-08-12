package remotewrite

import (
	"errors"
	"strings"
	"testing"
	"time"

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
		"Measure out of order writes": {
			measure: func(r Recorder) {
				r.measureOutOfOrderSamples(2)
			},
			expMetricNames: []string{
				"my_proxy_out_of_order_writes_total",
			},
			expMetrics: "" +
				"# HELP my_proxy_out_of_order_writes_total The total number of out of order writes to Cortex.\n" +
				"# TYPE my_proxy_out_of_order_writes_total counter\n" +
				"my_proxy_out_of_order_writes_total 2\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// Init metric Recorder
			reg := prometheus.NewRegistry()

			rec := NewRecorder("my_proxy", reg)

			// Measure metrics
			test.measure(rec)

			err := testutil.GatherAndCompare(reg, strings.NewReader(test.expMetrics), test.expMetricNames...)
			assert.NoError(err)
		})
	}
}

func TestRecorder_measure(t *testing.T) {
	expectedTemplate := `
		# HELP my_proxy_remote_write_client_request_duration_seconds Client-side duration of remote write calls.
		# TYPE my_proxy_remote_write_client_request_duration_seconds histogram
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.005"} 0
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.01"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.025"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.05"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.1"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.25"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.5"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="1"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="2.5"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="5"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="10"} 1
		my_proxy_remote_write_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="+Inf"} 1
		my_proxy_remote_write_client_request_duration_seconds_sum{operation="foo",result="<RESULT>"} 0.01
		my_proxy_remote_write_client_request_duration_seconds_count{operation="foo",result="<RESULT>"} 1
	`
	for _, tc := range []struct {
		result string
		err    error
	}{
		{"success", nil},
		{"error", errors.New("whoopsie")},
	} {
		t.Run(tc.result, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			recorder := NewRecorder("my_proxy", reg)
			recorder.measure("foo", 10*time.Millisecond, tc.err)

			expected := strings.ReplaceAll(expectedTemplate, "<RESULT>", tc.result)
			err := testutil.GatherAndCompare(reg, strings.NewReader(expected),
				"my_proxy_remote_write_client_request_duration_seconds",
			)
			assert.NoError(t, err)
		})
	}
}
