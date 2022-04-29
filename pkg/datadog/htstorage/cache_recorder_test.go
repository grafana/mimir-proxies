package htstorage

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestCacheRecorder(t *testing.T) {
	reg := prometheus.NewRegistry()
	recorder := NewCacheRecorder(reg)

	// call each of the functions a different amount of times
	for n, call := range []func(){
		nil,
		func() { recorder.mcGetTotal() },
		func() { recorder.mcGetMiss() },
		func() { recorder.mcGetErr() },
		func() { recorder.mcAddTotal() },
		func() { recorder.mcAddNotStored() },
		func() { recorder.mcAddErr() },
		func() { recorder.mcSetTotal() },
		func() { recorder.mcSetErr() },
		func() { recorder.mcDeleteAfterFailedSetTotal() },
		func() { recorder.mcDeleteAfterFailedSetMiss() },
		func() { recorder.mcDeleteAfterFailedSetErr() },
		func() { recorder.mcDeleteAfterFailedUnmarshalTotal() },
		func() { recorder.mcDeleteAfterFailedUnmarshalMiss() },
		func() { recorder.mcDeleteAfterFailedUnmarshalErr() },
		func() { recorder.storageGetTotal() },
		func() { recorder.storageGetErr() },
		func() { recorder.storageSetTotal() },
		func() { recorder.storageGetNotFound() },
		func() { recorder.storageSetErr() },
		func() { recorder.missingOrgID() },
		func() { recorder.unmarshalError() },
	} {
		for i := 0; i < n; i++ {
			call()
		}
	}

	expected := `
		# HELP datadog_proxy_htstorage_cache_mc_add_err_total Host tags storage cache: total number of failed memcached ADD operations.
		# TYPE datadog_proxy_htstorage_cache_mc_add_err_total counter
		datadog_proxy_htstorage_cache_mc_add_err_total 6
		# HELP datadog_proxy_htstorage_cache_mc_add_not_stored_total Host tags storage cache: total number of ignored memcached ADD operations (already stored data).
		# TYPE datadog_proxy_htstorage_cache_mc_add_not_stored_total counter
		datadog_proxy_htstorage_cache_mc_add_not_stored_total 5
		# HELP datadog_proxy_htstorage_cache_mc_add_total Host tags storage cache: total number of memcached ADD operations.
		# TYPE datadog_proxy_htstorage_cache_mc_add_total counter
		datadog_proxy_htstorage_cache_mc_add_total 4
		# HELP datadog_proxy_htstorage_cache_mc_delete_on_failed_set_err_total Host tags storage cache: total number of failed memcached DELETE operations after failed set.
		# TYPE datadog_proxy_htstorage_cache_mc_delete_on_failed_set_err_total counter
		datadog_proxy_htstorage_cache_mc_delete_on_failed_set_err_total 11
		# HELP datadog_proxy_htstorage_cache_mc_delete_on_failed_set_miss_total Host tags storage cache: total number of missed memcached DELETE operations after failed set.
		# TYPE datadog_proxy_htstorage_cache_mc_delete_on_failed_set_miss_total counter
		datadog_proxy_htstorage_cache_mc_delete_on_failed_set_miss_total 10
		# HELP datadog_proxy_htstorage_cache_mc_delete_on_failed_set_total Host tags storage cache: total number of memcached DELETE operations after failed set.
		# TYPE datadog_proxy_htstorage_cache_mc_delete_on_failed_set_total counter
		datadog_proxy_htstorage_cache_mc_delete_on_failed_set_total 9
		# HELP datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_err_total Host tags storage cache: total number of failed memcached DELETE operations after failed unmarshal.
		# TYPE datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_err_total counter
		datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_err_total 14
		# HELP datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_miss_total Host tags storage cache: total number of missed memcached DELETE operations after failed unmarshal.
		# TYPE datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_miss_total counter
		datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_miss_total 13
		# HELP datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_total Host tags storage cache: total number of memcached DELETE operations after failed unmarshal.
		# TYPE datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_total counter
		datadog_proxy_htstorage_cache_mc_delete_on_failed_unmarshal_total 12
		# HELP datadog_proxy_htstorage_cache_mc_get_err_total Host tags storage cache: total number of failed memcached GET operations.
		# TYPE datadog_proxy_htstorage_cache_mc_get_err_total counter
		datadog_proxy_htstorage_cache_mc_get_err_total 3
		# HELP datadog_proxy_htstorage_cache_mc_get_miss_total Host tags storage cache: total number of missed memcached GET operations.
		# TYPE datadog_proxy_htstorage_cache_mc_get_miss_total counter
		datadog_proxy_htstorage_cache_mc_get_miss_total 2
		# HELP datadog_proxy_htstorage_cache_mc_get_total Host tags storage cache: total number of cache memcached GET operations.
		# TYPE datadog_proxy_htstorage_cache_mc_get_total counter
		datadog_proxy_htstorage_cache_mc_get_total 1
		# HELP datadog_proxy_htstorage_cache_mc_set_err_total Host tags storage cache: total number of failed memcached SET operations.
		# TYPE datadog_proxy_htstorage_cache_mc_set_err_total counter
		datadog_proxy_htstorage_cache_mc_set_err_total 8
		# HELP datadog_proxy_htstorage_cache_mc_set_total Host tags storage cache: total number of memcached SET operations.
		# TYPE datadog_proxy_htstorage_cache_mc_set_total counter
		datadog_proxy_htstorage_cache_mc_set_total 7
		# HELP datadog_proxy_htstorage_cache_missing_org_id_total Host tags storage cache: total number of calls without org ID in the context.
		# TYPE datadog_proxy_htstorage_cache_missing_org_id_total counter
		datadog_proxy_htstorage_cache_missing_org_id_total 20
		# HELP datadog_proxy_htstorage_cache_storage_get_err_total Host tags storage cache: total number of failed storage Get() calls.
		# TYPE datadog_proxy_htstorage_cache_storage_get_err_total counter
		datadog_proxy_htstorage_cache_storage_get_err_total 16
		# HELP datadog_proxy_htstorage_cache_storage_get_not_found Host tags storage cache: total number of storage Get() calls where the host was not found.
		# TYPE datadog_proxy_htstorage_cache_storage_get_not_found counter
		datadog_proxy_htstorage_cache_storage_get_not_found 18
		# HELP datadog_proxy_htstorage_cache_storage_get_total Host tags storage cache: total number of storage Get() calls.
		# TYPE datadog_proxy_htstorage_cache_storage_get_total counter
		datadog_proxy_htstorage_cache_storage_get_total 15
		# HELP datadog_proxy_htstorage_cache_storage_set_err_total Host tags storage cache: total number of failed storage Set() calls.
		# TYPE datadog_proxy_htstorage_cache_storage_set_err_total counter
		datadog_proxy_htstorage_cache_storage_set_err_total 19
		# HELP datadog_proxy_htstorage_cache_storage_set_total Host tags storage cache: total number of storage Set() calls.
		# TYPE datadog_proxy_htstorage_cache_storage_set_total counter
		datadog_proxy_htstorage_cache_storage_set_total 17
		# HELP datadog_proxy_htstorage_cache_unmarshal_error_total Host tags storage cache: total number of entries that were not able to be unmarshaled.
		# TYPE datadog_proxy_htstorage_cache_unmarshal_error_total counter
		datadog_proxy_htstorage_cache_unmarshal_error_total 21
	`
	err := testutil.GatherAndCompare(reg, strings.NewReader(expected))
	assert.NoError(t, err)
}
