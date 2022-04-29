package htstorage

import "github.com/prometheus/client_golang/prometheus"

//go:generate mockery --case underscore --inpackage --testonly --name CacheRecorder
type CacheRecorder interface {
	mcGetTotal()
	mcGetMiss()
	mcGetErr()

	mcAddTotal()
	mcAddNotStored()
	mcAddErr()

	mcSetTotal()
	mcSetErr()

	mcDeleteAfterFailedSetTotal()
	mcDeleteAfterFailedSetMiss()
	mcDeleteAfterFailedSetErr()

	mcDeleteAfterFailedUnmarshalTotal()
	mcDeleteAfterFailedUnmarshalMiss()
	mcDeleteAfterFailedUnmarshalErr()

	storageGetTotal()
	storageGetNotFound()
	storageGetErr()

	storageSetTotal()
	storageSetErr()

	missingOrgID()
	unmarshalError()
}

const (
	metricMCGetTotal = "mc_get_total"
	metricMCGetMiss  = "mc_get_miss_total"
	metricMCGetErr   = "mc_get_err_total"

	metricMCAddTotal     = "mc_add_total"
	metricMCAddNotStored = "mc_add_not_stored_total"
	metricMCAddErr       = "mc_add_err_total"

	metricMCSetTotal = "mc_set_total"
	metricMCSetErr   = "mc_set_err_total"

	metricMCDeleteAfterFailedUnmarshalTotal = "mc_delete_on_failed_unmarshal_total"
	metricMCDeleteAfterFailedUnmarshalMiss  = "mc_delete_on_failed_unmarshal_miss_total"
	metricMCDeleteAfterFailedUnmarshalErr   = "mc_delete_on_failed_unmarshal_err_total"

	metricMCDeleteAfterFailedSetTotal = "mc_delete_on_failed_set_total"
	metricMCDeleteAfterFailedSetMiss  = "mc_delete_on_failed_set_miss_total"
	metricMCDeleteAfterFailedSetErr   = "mc_delete_on_failed_set_err_total"

	metricStorageGetTotal    = "storage_get_total"
	metricStorageGetErr      = "storage_get_err_total"
	metricStorageGetNotFound = "storage_get_not_found"

	metricStorageSetTotal = "storage_set_total"
	metricStorageSetErr   = "storage_set_err_total"

	metricMissingOrgID   = "missing_org_id_total"
	metricUnmarshalError = "unmarshal_error_total"
)

var cacheCounterMetrics = map[string]string{
	metricMCGetTotal: "Host tags storage cache: total number of cache memcached GET operations.",
	metricMCGetMiss:  "Host tags storage cache: total number of missed memcached GET operations.",
	metricMCGetErr:   "Host tags storage cache: total number of failed memcached GET operations.",

	metricMCAddTotal:     "Host tags storage cache: total number of memcached ADD operations.",
	metricMCAddNotStored: "Host tags storage cache: total number of ignored memcached ADD operations (already stored data).",
	metricMCAddErr:       "Host tags storage cache: total number of failed memcached ADD operations.",

	metricMCSetTotal: "Host tags storage cache: total number of memcached SET operations.",
	metricMCSetErr:   "Host tags storage cache: total number of failed memcached SET operations.",

	metricMCDeleteAfterFailedSetTotal: "Host tags storage cache: total number of memcached DELETE operations after failed set.",
	metricMCDeleteAfterFailedSetMiss:  "Host tags storage cache: total number of missed memcached DELETE operations after failed set.",
	metricMCDeleteAfterFailedSetErr:   "Host tags storage cache: total number of failed memcached DELETE operations after failed set.",

	metricMCDeleteAfterFailedUnmarshalTotal: "Host tags storage cache: total number of memcached DELETE operations after failed unmarshal.",
	metricMCDeleteAfterFailedUnmarshalMiss:  "Host tags storage cache: total number of missed memcached DELETE operations after failed unmarshal.",
	metricMCDeleteAfterFailedUnmarshalErr:   "Host tags storage cache: total number of failed memcached DELETE operations after failed unmarshal.",

	metricStorageGetTotal:    "Host tags storage cache: total number of storage Get() calls.",
	metricStorageGetNotFound: "Host tags storage cache: total number of storage Get() calls where the host was not found.",
	metricStorageGetErr:      "Host tags storage cache: total number of failed storage Get() calls.",

	metricStorageSetTotal: "Host tags storage cache: total number of storage Set() calls.",
	metricStorageSetErr:   "Host tags storage cache: total number of failed storage Set() calls.",

	metricMissingOrgID:   "Host tags storage cache: total number of calls without org ID in the context.",
	metricUnmarshalError: "Host tags storage cache: total number of entries that were not able to be unmarshaled.",
}

const (
	prefix = "datadog_proxy_htstorage_cache"
)

func NewCacheRecorder(reg prometheus.Registerer) CacheRecorder {
	r := &prometheusRecorder{
		metrics: make(map[string]prometheus.Counter),
	}
	for name, help := range cacheCounterMetrics {
		counter := prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      name,
			Help:      help,
		})
		reg.MustRegister(counter)
		r.metrics[name] = counter
	}

	return r
}

// prometheusRecorder knows the metrics of the ingester and how to measure them for Prometheus.
type prometheusRecorder struct {
	metrics map[string]prometheus.Counter
}

func (p *prometheusRecorder) mcGetTotal() {
	p.metrics[metricMCGetTotal].Inc()
}

func (p *prometheusRecorder) mcGetMiss() {
	p.metrics[metricMCGetMiss].Inc()
}

func (p *prometheusRecorder) mcGetErr() {
	p.metrics[metricMCGetErr].Inc()
}

func (p *prometheusRecorder) mcAddTotal() {
	p.metrics[metricMCAddTotal].Inc()
}

func (p *prometheusRecorder) mcAddNotStored() {
	p.metrics[metricMCAddNotStored].Inc()
}

func (p *prometheusRecorder) mcAddErr() {
	p.metrics[metricMCAddErr].Inc()
}

func (p *prometheusRecorder) mcSetTotal() {
	p.metrics[metricMCSetTotal].Inc()
}

func (p *prometheusRecorder) mcSetErr() {
	p.metrics[metricMCSetErr].Inc()
}

func (p *prometheusRecorder) mcDeleteAfterFailedSetTotal() {
	p.metrics[metricMCDeleteAfterFailedSetTotal].Inc()
}

func (p *prometheusRecorder) mcDeleteAfterFailedSetMiss() {
	p.metrics[metricMCDeleteAfterFailedSetMiss].Inc()
}

func (p *prometheusRecorder) mcDeleteAfterFailedSetErr() {
	p.metrics[metricMCDeleteAfterFailedSetErr].Inc()
}

func (p *prometheusRecorder) mcDeleteAfterFailedUnmarshalTotal() {
	p.metrics[metricMCDeleteAfterFailedUnmarshalTotal].Inc()
}

func (p *prometheusRecorder) mcDeleteAfterFailedUnmarshalMiss() {
	p.metrics[metricMCDeleteAfterFailedUnmarshalMiss].Inc()
}

func (p *prometheusRecorder) mcDeleteAfterFailedUnmarshalErr() {
	p.metrics[metricMCDeleteAfterFailedUnmarshalErr].Inc()
}

func (p *prometheusRecorder) storageGetTotal() {
	p.metrics[metricStorageGetTotal].Inc()
}

func (p *prometheusRecorder) storageGetErr() {
	p.metrics[metricStorageGetErr].Inc()
}

func (p *prometheusRecorder) storageGetNotFound() {
	p.metrics[metricStorageGetNotFound].Inc()
}

func (p *prometheusRecorder) storageSetTotal() {
	p.metrics[metricStorageSetTotal].Inc()
}

func (p *prometheusRecorder) storageSetErr() {
	p.metrics[metricStorageSetErr].Inc()
}

func (p *prometheusRecorder) missingOrgID() {
	p.metrics[metricMissingOrgID].Inc()
}

func (p *prometheusRecorder) unmarshalError() {
	p.metrics[metricUnmarshalError].Inc()
}
