package writeproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang/snappy"

	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/go-kit/log/level"

	"github.com/grafana/mimir/pkg/util/spanlogger"

	"github.com/grafana/mimir-proxies/pkg/errorx"

	"github.com/grafana/metrictank/schema"
	"github.com/grafana/metrictank/schema/msg"

	graphiteAuth "github.com/grafana/mimir-proxies/pkg/graphite/authentication"
	"github.com/grafana/mimir-proxies/pkg/remotewrite"
)

type RemoteWriteProxy struct {
	client   remotewrite.Client
	recorder Recorder
}

func NewRemoteWriteProxy(client remotewrite.Client, recorder Recorder) *RemoteWriteProxy {
	return &RemoteWriteProxy{
		client:   client,
		recorder: recorder,
	}
}

const (
	contentTypeMetricBinary       = "rt-metric-binary"
	contentTypeMetricBinarySnappy = "rt-metric-binary-snappy"
	contentTypeApplicationJSON    = "application/json"
)

type remoteWriteResponse struct {
	Published int `json:"published"`
}

func (wp *RemoteWriteProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log, _ := spanlogger.New(r.Context(), "graphiteWriter.ServeHTTP")
	startTime := time.Now()
	defer log.Finish()
	ctx, userID := graphiteAuth.ExtractOrgID(r.Context())

	metrics, err := extractMetricsFromRequest(r)
	if err != nil {
		var unsupportedMediaType errorx.UnsupportedMediaType
		if errors.As(err, &unsupportedMediaType) {
			level.Info(log).Log("msg", "failed to parse content-type", "err", unsupportedMediaType.Error())
			http.Error(w, unsupportedMediaType.Error(), unsupportedMediaType.HTTPStatusCode())
			return
		}
		wp.recorder.measureRejectedSamples(userID, "cant_parse_body")
		level.Error(log).Log("msg", "failed to parse metrics from body", "err", err)
		http.Error(w, fmt.Sprintf("failed to parse metrics from body: %s", err), http.StatusBadRequest)
		return
	}

	// Counting the request and number of samples before validation.
	wp.recorder.measureIncomingRequest(userID)
	wp.recorder.measureIncomingSamples(userID, len(metrics))

	var firstValidationError error
	for _, metric := range metrics {
		metricDataDefaults(metric)
		var validateErr error
		if validateErr = metric.Validate(); validateErr != nil {
			if firstValidationError == nil {
				firstValidationError = validateErr
			}
			wp.recorder.measureRejectedSamples(userID,
				strings.ReplaceAll(validateErr.Error(), " ", "_"))
		}
	}
	if firstValidationError != nil {
		level.Error(log).Log("msg", "invalid metric data received", "err", firstValidationError)
		http.Error(w, fmt.Sprintf("invalid metric data received: %s", firstValidationError), http.StatusBadRequest)
		return
	}

	beforeConversion := time.Now()

	series, err := MetricDataPayload(metrics).GeneratePreallocTimeseries(ctx)
	if err != nil {
		level.Error(log).Log("msg", "failed to generate prometheus series from metric payload", "err", err)
		http.Error(w, fmt.Sprintf("failed to generate prometheus series from metric payload: %s", err), http.StatusBadRequest)
		return
	}
	wp.recorder.measureConversionDuration(userID, time.Since(beforeConversion))

	req := mimirpb.WriteRequest{
		Timeseries:          series,
		SkipLabelValidation: true,
	}
	defer mimirpb.ReuseSlice(req.Timeseries)

	err = wp.client.Write(ctx, &req)
	if err != nil {
		if errors.As(err, &errorx.TooManyRequests{}) {
			level.Warn(log).Log("msg", "too many requests", "err", err)
			http.Error(w, fmt.Sprintf("too many requests: %s", err), http.StatusTooManyRequests)
			return
		}

		level.Error(log).Log("msg", "failed to push metric data", "err", err)
		http.Error(w, "failed to push metric data", http.StatusInternalServerError)
		return
	}

	// Counting the request and number of samples after validation.
	wp.recorder.measureReceivedRequest(userID)
	wp.recorder.measureReceivedSamples(userID, len(series))

	w.WriteHeader(http.StatusOK)
	body, _ := json.Marshal(remoteWriteResponse{
		Published: len(series),
	})
	_, _ = w.Write(body)

	level.Debug(log).Log("msg", "successful series write", "len", len(series), "duration", time.Since(startTime))
}

func extractMetricsFromRequest(r *http.Request) ([]*schema.MetricData, error) {
	var metrics []*schema.MetricData
	var err error
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	switch contentType {
	case contentTypeMetricBinary:
		metrics, err = metricsBinary(r, false)
	case contentTypeMetricBinarySnappy:
		metrics, err = metricsBinary(r, true)
	case contentTypeApplicationJSON:
		metrics, err = metricsJSON(r)
	default:
		return nil, errorx.UnsupportedMediaType{Msg: fmt.Sprintf("unknown content-type %q", contentType)}
	}
	return metrics, err
}

func metricsJSON(r *http.Request) ([]*schema.MetricData, error) {
	if r.Body == nil {
		return nil, fmt.Errorf("no data included in request")
	}

	defer func() {
		_ = r.Body.Close()
	}()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	metrics := make([]*schema.MetricData, 0)
	err = json.Unmarshal(body, &metrics)
	if err != nil {
		return nil, fmt.Errorf("invalid metric data received: %w", err)
	}

	return metrics, nil
}

func metricsBinary(r *http.Request, compressed bool) ([]*schema.MetricData, error) {
	if r.Body == nil {
		return nil, fmt.Errorf("no data included in request")
	}
	var bodyReadCloser io.ReadCloser
	if compressed {
		bodyReadCloser = io.NopCloser(snappy.NewReader(r.Body))
	} else {
		bodyReadCloser = r.Body
	}
	defer func() {
		_ = bodyReadCloser.Close()
	}()

	body, err := io.ReadAll(bodyReadCloser)
	if err != nil {
		return nil, err
	}
	metricData := new(msg.MetricData)
	err = metricData.InitFromMsg(body)
	if err != nil {
		return nil, fmt.Errorf("invalid metric data received: %w", err)
	}

	err = metricData.DecodeMetricData()
	if err != nil {
		return nil, fmt.Errorf("invalid metric data received:%w", err)
	}

	return metricData.Metrics, nil
}

// metricDataDefaults enforces orgID in the metric to be int(1) as we don't use it and some customers may not provide it,
// and makes sure that metric type is present, defaulting to "gauge" if not set
func metricDataDefaults(m *schema.MetricData) {
	m.OrgId = 1
	if m.Mtype == "" {
		m.Mtype = "gauge"
	}
}
