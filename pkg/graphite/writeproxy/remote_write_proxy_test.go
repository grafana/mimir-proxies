package writeproxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/mimir-graphite/v2/pkg/errorx"

	"github.com/grafana/mimir-graphite/v2/pkg/remotewrite/remotewritemock"

	"github.com/grafana/metrictank/schema"
	"github.com/grafana/metrictank/schema/msg"
	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRemoteWriteMetricsHandler(t *testing.T) {
	tests := []struct {
		name            string
		contentType     string
		metrics         []*schema.MetricData
		reqValidation   func(*mimirpb.WriteRequest) bool
		status          int
		recorderMock    func() *MockRecorder
		remoteWriteMock func() *remotewritemock.Client
	}{
		{
			name:        "happy path the handler attempts to write the timeseries",
			contentType: contentTypeMetricBinary,
			metrics: []*schema.MetricData{
				{
					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     time.Now().Unix() - 100,
					Value:    1,
				},
			},
			reqValidation: func(req *mimirpb.WriteRequest) bool {
				return req.SkipLabelValidation == true
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureIncomingRequest", "fake").Return(nil)
				recorderMock.On("measureIncomingSamples", "fake", 1).Return(nil)
				recorderMock.On("measureConversionDuration", "fake", mock.Anything).Return(nil)
				recorderMock.On("measureReceivedRequest", "fake").Return(nil)
				recorderMock.On("measureReceivedSamples", "fake", 1).Return(nil)
				return recorderMock
			},
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "graphite_tagged"},
									{Name: "foo", Value: "bar"},
									{Name: "name", Value: "some.test.metric"},
									{Name: "tag", Value: "value"},
								},
								Samples: []mimirpb.Sample{
									{Value: 1, TimestampMs: (time.Now().Unix() - 100) * 1000},
								},
								Exemplars: []mimirpb.Exemplar{},
							},
						},
					},
					SkipLabelValidation: true,
				}).Return(nil)
				return remoteWriteMock
			},
			status: http.StatusOK,
		},
		{
			name:        "handler allows empty org and mtype",
			contentType: contentTypeMetricBinary,
			metrics: []*schema.MetricData{
				{
					// omitted: OrgId:    1,
					Name: "some.test.metric",
					Tags: []string{"tag=value", "foo=bar"},
					// omitted: Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Value:    1,
					Time:     time.Now().Unix() - 100,
				},
			},
			reqValidation: func(req *mimirpb.WriteRequest) bool {
				return req.SkipLabelValidation == true
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureIncomingRequest", "fake").Return(nil)
				recorderMock.On("measureIncomingSamples", "fake", 1).Return(nil)
				recorderMock.On("measureConversionDuration", "fake", mock.Anything).Return(nil)
				recorderMock.On("measureReceivedRequest", "fake").Return(nil)
				recorderMock.On("measureReceivedSamples", "fake", 1).Return(nil)
				return recorderMock
			},
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "graphite_tagged"},
									{Name: "foo", Value: "bar"},
									{Name: "name", Value: "some.test.metric"},
									{Name: "tag", Value: "value"},
								},
								Samples: []mimirpb.Sample{
									{Value: 1, TimestampMs: (time.Now().Unix() - 100) * 1000},
								},
								Exemplars: []mimirpb.Exemplar{},
							},
						},
					},
					SkipLabelValidation: true,
				}).Return(nil)
				return remoteWriteMock
			},
			status: http.StatusOK,
		},
		{
			name:        "handler return 429 on 429 from remote write",
			contentType: contentTypeMetricBinary,
			metrics: []*schema.MetricData{
				{

					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Value:    1,
					Time:     time.Now().Unix() - 100,
				},
			},
			reqValidation: func(request *mimirpb.WriteRequest) bool {
				return true
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureIncomingRequest", "fake").Return(nil)
				recorderMock.On("measureIncomingSamples", "fake", 1).Return(nil)
				recorderMock.On("measureConversionDuration", "fake", mock.Anything).Return(nil)
				recorderMock.On("measureReceivedRequest", "fake").Return(nil)
				recorderMock.On("measureReceivedSamples", "fake", 1).Return(nil)
				return recorderMock
			},
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "graphite_tagged"},
									{Name: "foo", Value: "bar"},
									{Name: "name", Value: "some.test.metric"},
									{Name: "tag", Value: "value"},
								},
								Samples: []mimirpb.Sample{
									{Value: 1, TimestampMs: (time.Now().Unix() - 100) * 1000},
								},
								Exemplars: []mimirpb.Exemplar{},
							},
						},
					},
					SkipLabelValidation: true,
				}).Return(errorx.TooManyRequests{Msg: "simulating rate limited"})
				return remoteWriteMock
			},
			status: http.StatusTooManyRequests,
		},
		{
			name:        "handler return 500 on bad request",
			contentType: contentTypeMetricBinary,
			metrics: []*schema.MetricData{
				{

					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Value:    1,
					Time:     time.Now().Unix() - 100,
				},
			},
			reqValidation: func(request *mimirpb.WriteRequest) bool {
				return true
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureIncomingRequest", "fake").Return(nil)
				recorderMock.On("measureIncomingSamples", "fake", 1).Return(nil)
				recorderMock.On("measureConversionDuration", "fake", mock.Anything).Return(nil)
				recorderMock.On("measureReceivedRequest", "fake").Return(nil)
				recorderMock.On("measureReceivedSamples", "fake", 1).Return(nil)
				return recorderMock
			},
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "graphite_tagged"},
									{Name: "foo", Value: "bar"},
									{Name: "name", Value: "some.test.metric"},
									{Name: "tag", Value: "value"},
								},
								Samples: []mimirpb.Sample{
									{Value: 1, TimestampMs: (time.Now().Unix() - 100) * 1000},
								},
								Exemplars: []mimirpb.Exemplar{},
							},
						},
					},
					SkipLabelValidation: true,
				}).Return(errorx.BadRequest{Msg: "simulating bad request"})
				return remoteWriteMock
			},
			status: http.StatusInternalServerError,
		},
		{
			name:        "handler return 400 on validation error (1)",
			contentType: contentTypeMetricBinary,
			metrics: []*schema.MetricData{
				{
					OrgId:    1,
					Name:     "...", // Invalid
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     time.Now().Unix() - 100,
				},
			},
			reqValidation: func(request *mimirpb.WriteRequest) bool {
				return true
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureIncomingRequest", "fake").Return(nil)
				recorderMock.On("measureIncomingSamples", "fake", 1).Return(nil)
				recorderMock.On("measureConversionDuration", "fake", mock.Anything).Return(nil)
				recorderMock.On("measureReceivedRequest", "fake").Return(nil)
				recorderMock.On("measureReceivedSamples", "fake", 1).Return(nil)
				recorderMock.On("measureRejectedSamples", "fake", "name_cannot_be_empty").Return(nil)
				return recorderMock
			},
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "graphite_tagged"},
									{Name: "foo", Value: "bar"},
									{Name: "name", Value: "some.test.metric"},
									{Name: "tag", Value: "value"},
								},
								Samples: []mimirpb.Sample{
									{Value: 1, TimestampMs: (time.Now().Unix() - 100) * 1000},
								},
								Exemplars: []mimirpb.Exemplar{},
							},
						},
					},
					SkipLabelValidation: true,
				}).Return(nil)
				return remoteWriteMock
			},
			status: http.StatusBadRequest,
		},
		{
			name:        "handler return 400 on validation error (2)",
			contentType: contentTypeMetricBinary,
			metrics: []*schema.MetricData{
				{
					OrgId:    1,
					Name:     "some.test.name",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "invalid",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     time.Now().Unix() - 100,
				},
			},
			reqValidation: func(request *mimirpb.WriteRequest) bool {
				return true
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureIncomingRequest", "fake").Return(nil)
				recorderMock.On("measureIncomingSamples", "fake", 1).Return(nil)
				recorderMock.On("measureConversionDuration", "fake", mock.Anything).Return(nil)
				recorderMock.On("measureReceivedRequest", "fake").Return(nil)
				recorderMock.On("measureReceivedSamples", "fake", 1).Return(nil)
				recorderMock.On("measureRejectedSamples", "fake", "invalid_mtype").Return(nil)
				return recorderMock
			},
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "graphite_tagged"},
									{Name: "foo", Value: "bar"},
									{Name: "name", Value: "some.test.metric"},
									{Name: "tag", Value: "value"},
								},
								Samples: []mimirpb.Sample{
									{Value: 1, TimestampMs: (time.Now().Unix() - 100) * 1000},
								},
								Exemplars: []mimirpb.Exemplar{},
							},
						},
					},
					SkipLabelValidation: true,
				}).Return(nil)
				return remoteWriteMock
			},
			status: http.StatusBadRequest,
		},
		{
			name:            "unknown content type should fail with 415",
			contentType:     "unknown",
			metrics:         []*schema.MetricData{},
			reqValidation:   func(request *mimirpb.WriteRequest) bool { return true },
			recorderMock:    func() *MockRecorder { return &MockRecorder{} },
			remoteWriteMock: func() *remotewritemock.Client { return &remotewritemock.Client{} },
			status:          http.StatusUnsupportedMediaType,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewRemoteWriteProxy(tc.remoteWriteMock(), tc.recorderMock())

			mda := schema.MetricDataArray(tc.metrics)
			data, err := msg.CreateMsg(mda, 0, msg.FormatMetricDataArrayMsgp)
			require.NoError(t, err)

			// Create HTTP request
			req, err := http.NewRequest("POST", "http://example.com", bytes.NewReader(data))
			require.NoError(t, err)
			req.Header.Add("Content-Type", tc.contentType)
			req.Header.Set("X-Scope-OrgID", "1")

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			require.Equal(t, tc.status, recorder.Code)
		})
	}
}
