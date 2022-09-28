package ingester

import (
	"context"
	"testing"

	"github.com/grafana/mimir/pkg/mimirpb"

	remotewritemock2 "github.com/grafana/mimir-proxies/pkg/remotewrite/remotewritemock"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddprom"
	"github.com/grafana/mimir-proxies/pkg/datadog/ddstructs"
	"github.com/grafana/mimir-proxies/pkg/datadog/htstorage/htstoragemock"

	"github.com/stretchr/testify/require"

	"github.com/grafana/mimir-proxies/pkg/errorx"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIngester_StoreMetrics(t *testing.T) {
	t.Run("returns errors from remote write client", func(t *testing.T) {
		assert := assert.New(t)

		recorderMock := &MockRecorder{}
		recorderMock.On("measureMetricsParsed", 0).Once().Return(nil)
		defer recorderMock.AssertExpectations(t)

		remoteWriteClientMock := &remotewritemock2.Client{}
		remoteWriteClientMock.On("Write", mock.Anything,
			&mimirpb.WriteRequest{Timeseries: make([]mimirpb.PreallocTimeseries, 0), Metadata: make([]*mimirpb.MetricMetadata, 0)}).
			Return(errorx.BadRequest{})

		ingesterUnderTest := New(&htstoragemock.Storage{}, recorderMock, remoteWriteClientMock)

		storeMetricsErr := ingesterUnderTest.StoreMetrics(context.Background(), ddstructs.Series{})
		assert.ErrorAs(storeMetricsErr, &errorx.BadRequest{})
	})
}

func TestIngester_StoreCheckRun(t *testing.T) {
	t.Run("service check series have datadog metric type label appended", func(t *testing.T) {
		assert := assert.New(t)

		recorderMock := &MockRecorder{}

		htstorageMock := &htstoragemock.Storage{}
		htstorageMock.On("Get", mock.Anything, "host1").Once().Return([]prompb.Label{
			{
				Name:  hostLabel,
				Value: "host1",
			},
			{
				Name:  ddprom.AllHostTagsLabelName,
				Value: "labelfromhost:valuefromhost",
			},
		}, nil)

		serviceChecks := ddstructs.ServiceChecks{
			{
				CheckName: "my.check.ok",
				Status:    ddstructs.ServiceCheckOK,
				Host:      "host1",
			},
		}

		expectedWriteTimeSeries := []mimirpb.PreallocTimeseries{
			{
				TimeSeries: &mimirpb.TimeSeries{
					Labels: []mimirpb.LabelAdapter{
						{
							Name:  hostLabel,
							Value: "'host1'",
						},
						{
							Name:  "labelfromhost",
							Value: "'valuefromhost'",
						},
						{
							Name:  "__name__",
							Value: "my_dot_check_dot_ok",
						},
						{
							Name:  ddprom.DDTypeLabel,
							Value: "service_check",
						},
					},
					Samples: []mimirpb.Sample{
						{
							Value:       0,
							TimestampMs: 0,
						},
					},
					Exemplars: []mimirpb.Exemplar{},
				},
			},
		}

		expectedWriteRequest := &mimirpb.WriteRequest{
			Timeseries: expectedWriteTimeSeries,
		}

		remoteWriteClientMock := &remotewritemock2.Client{}
		remoteWriteClientMock.On("Write", mock.Anything,
			expectedWriteRequest).Return(errorx.BadRequest{})

		ingesterUnderTest := New(htstorageMock, recorderMock, remoteWriteClientMock)

		storeMetricsErr := ingesterUnderTest.StoreCheckRun(context.Background(), serviceChecks)
		assert.ErrorAs(storeMetricsErr, &errorx.BadRequest{})
	})
}

func TestIngester_StoreHostTags(t *testing.T) {
	const hostName = "expected.hostname"

	t.Run("with duplicated tags", func(t *testing.T) {
		tags := []string{
			"foo:bar",
			"bar:baz",
			"foo:boom",
			"bee:boo",
		}

		expectedLabels := []prompb.Label{
			{Name: "bar", Value: "'baz'"},
			{Name: "bee", Value: "'boo'"},
			{Name: "foo", Value: "'bar','boom'"},
			{Name: ddprom.AllHostTagsLabelName, Value: "bar:baz,bee:boo,foo:bar,foo:boom"},
		}

		htStorageMock := &htstoragemock.Storage{}
		htStorageMock.On("Set", mock.Anything, hostName, expectedLabels).Return(nil)
		defer htStorageMock.AssertExpectations(t)

		ingester := New(htStorageMock, &MockRecorder{}, &remotewritemock2.Client{})
		err := ingester.StoreHostTags(context.Background(), hostName, tags)
		require.NoError(t, err)
	})
}
