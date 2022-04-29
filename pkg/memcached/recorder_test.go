package memcached

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecorder_measure(t *testing.T) {
	expectedTemplate := `
		# HELP datadog_proxy_memcached_operation_duration_seconds Duration of different memcached operations.
		# TYPE datadog_proxy_memcached_operation_duration_seconds histogram
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.0005"} 0
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.001"} 0
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.0025"} 0
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.005"} 0
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.0075"} 0
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.01"} 1
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.015"} 1
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.025"} 1
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.05"} 1
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="0.1"} 1
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="1"} 1
		datadog_proxy_memcached_operation_duration_seconds_bucket{operation="foo",result="<RESULT>",le="+Inf"} 1
		datadog_proxy_memcached_operation_duration_seconds_sum{operation="foo",result="<RESULT>"} 0.01
		datadog_proxy_memcached_operation_duration_seconds_count{operation="foo",result="<RESULT>"} 1
	`
	for _, tc := range []struct {
		result string
		err    error
	}{
		{"success", nil},
		{"cache_miss", memcache.ErrCacheMiss},
		{"cas_conflict", memcache.ErrCASConflict},
		{"not_stored", memcache.ErrNotStored},
		{"server_error", memcache.ErrServerError},
		{"no_stats", memcache.ErrNoStats},
		{"malformed_key", memcache.ErrMalformedKey},
		{"no_servers", memcache.ErrNoServers},
		{"deadline_exceeded", os.ErrDeadlineExceeded},
		{"unmapped_error", errors.New("foobar")},
	} {
		t.Run(tc.result, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			recorder := NewRecorder(reg)
			recorder.measure("foo", 10*time.Millisecond, tc.err)

			expected := strings.ReplaceAll(expectedTemplate, "<RESULT>", tc.result)
			err := testutil.GatherAndCompare(reg, strings.NewReader(expected),
				"datadog_proxy_memcached_operation_duration_seconds",
			)
			assert.NoError(t, err)
		})
	}
}
