package remoteread

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecorder_measure(t *testing.T) {
	expectedTemplate := `
		# HELP unit_test_remote_read_client_request_duration_seconds Client-side duration of remote read calls.
		# TYPE unit_test_remote_read_client_request_duration_seconds histogram
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.005"} 0
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.01"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.025"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.05"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.1"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.25"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.5"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="1"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="2.5"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="5"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="10"} 1
		unit_test_remote_read_client_request_duration_seconds_bucket{operation="foo",result="<RESULT>",le="+Inf"} 1
		unit_test_remote_read_client_request_duration_seconds_sum{operation="foo",result="<RESULT>"} 0.01
		unit_test_remote_read_client_request_duration_seconds_count{operation="foo",result="<RESULT>"} 1
	`
	for _, tc := range []struct {
		result string
		err    error
	}{
		{"success", nil},
		{"error", errors.New("yikes")},
	} {
		t.Run(tc.result, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			recorder := NewRecorder("unit_test", reg)
			recorder.measure("foo", 10*time.Millisecond, tc.err)

			expected := strings.ReplaceAll(expectedTemplate, "<RESULT>", tc.result)
			err := testutil.GatherAndCompare(reg, strings.NewReader(expected),
				"unit_test_remote_read_client_request_duration_seconds",
			)
			assert.NoError(t, err)
		})
	}
}
