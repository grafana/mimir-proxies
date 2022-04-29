package middleware

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	mb = 1024 * 1024
	kb = 1024
)

// Instrument is a Middleware which records timings for every HTTP request
type Instrument struct {
	routeMatcher     RouteMatcher
	duration         *prometheus.HistogramVec
	requestBodySize  *prometheus.HistogramVec
	responseBodySize *prometheus.HistogramVec
	inflightRequests *prometheus.GaugeVec
}

var (

	// BodySizeBuckets defines buckets for request/response body sizes.
	BodySizeBuckets = []float64{1 * mb, 2.5 * mb, 5 * mb, 10 * mb, 25 * mb, 50 * mb, 100 * mb, 250 * mb}
	// DefBuckets are histogram buckets for the response time (in seconds)
	// of a network service, including one that is responding very slowly.
	DefBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 25, 50, 100}
)

func NewInstrument(routeMatcher RouteMatcher, defBuckets []float64, prefix string) (*Instrument, error) {
	if len(defBuckets) == 0 {
		defBuckets = DefBuckets
	}

	// Prometheus histograms for requests.
	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: prefix,
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
		Buckets:   defBuckets,
	}, []string{"method", "route", "status_code"})
	prometheus.MustRegister(requestDuration)

	receivedMessageSize := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: prefix,
		Name:      "request_message_bytes",
		Help:      "Size (in bytes) of messages received in the request.",
		Buckets:   BodySizeBuckets,
	}, []string{"method", "route"})
	prometheus.MustRegister(receivedMessageSize)

	sentMessageSize := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: prefix,
		Name:      "response_message_bytes",
		Help:      "Size (in bytes) of messages sent in response.",
		Buckets:   BodySizeBuckets,
	}, []string{"method", "route"})
	prometheus.MustRegister(sentMessageSize)

	inflightRequests := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prefix,
		Name:      "inflight_requests",
		Help:      "Current number of inflight requests.",
	}, []string{"method", "route"})
	prometheus.MustRegister(inflightRequests)

	return &Instrument{
		routeMatcher:     routeMatcher,
		duration:         requestDuration,
		requestBodySize:  receivedMessageSize,
		responseBodySize: sentMessageSize,
		inflightRequests: inflightRequests,
	}, nil

}

type instrumentContextKey int

const requestBeginContextKey instrumentContextKey = 0

func extractRequestBeginTime(ctx context.Context) (time.Time, bool) {
	begin := ctx.Value(requestBeginContextKey)
	if begin == nil {
		return time.Time{}, false
	}

	return begin.(time.Time), true

}

// Wrap implements middleware.Interface
func (i Instrument) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		r = r.WithContext(context.WithValue(r.Context(), requestBeginContextKey, begin))

		route := i.getRouteName(r)
		inflight := i.inflightRequests.WithLabelValues(r.Method, route)
		inflight.Inc()
		defer inflight.Dec()

		origBody := r.Body
		defer func() {
			// No need to leak our Body wrapper beyond the scope of this handler.
			r.Body = origBody
		}()

		rBody := &reqBody{b: origBody}
		r.Body = rBody

		respMetrics := httpsnoop.CaptureMetricsFn(w, func(ww http.ResponseWriter) {
			next.ServeHTTP(ww, r)
		})

		i.requestBodySize.WithLabelValues(r.Method, route).Observe(float64(rBody.read))
		i.responseBodySize.WithLabelValues(r.Method, route).Observe(float64(respMetrics.Written))

		histogram := i.duration.WithLabelValues(r.Method, route, strconv.Itoa(respMetrics.Code))
		if traceID, ok := ExtractSampledTraceID(r.Context()); ok {
			// Need to type-convert the Observer to an
			// ExemplarObserver. This will always work for a
			// HistogramVec.
			histogram.(prometheus.ExemplarObserver).ObserveWithExemplar(
				respMetrics.Duration.Seconds(), prometheus.Labels{"traceID": traceID},
			)
			return
		}
		histogram.Observe(respMetrics.Duration.Seconds())
	})
}

// Return a name identifier for ths request.  There are three options:
//   1. The request matches a gorilla mux route, with a name.  Use that.
//   2. The request matches an unamed gorilla mux router.  Munge the path
//      template such that templates like '/api/{org}/foo' come out as
//      'api_org_foo'.
//   3. The request doesn't match a mux route. Return "other"
// We do all this as we do not wish to emit high cardinality labels to
// prometheus.
func (i Instrument) getRouteName(r *http.Request) string {
	route := getRouteName(i.routeMatcher, r)
	if route == "" {
		route = "other"
	}

	return route
}

type reqBody struct {
	b    io.ReadCloser
	read int64
}

func (w *reqBody) Read(p []byte) (int, error) {
	n, err := w.b.Read(p)
	if n > 0 {
		w.read += int64(n)
	}
	return n, err
}

func (w *reqBody) Close() error {
	return w.b.Close()
}
