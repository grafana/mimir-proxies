package htstorage

import (
	"context"
	"fmt"
	"testing"
	"time"

	apimock2 "github.com/grafana/mimir-proxies/pkg/remoteread/apimock"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddprom"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCortex_Get(t *testing.T) {
	ctx := context.Background()
	hostname := "host1"
	now := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	timeNowMock := func() time.Time { return now }
	query := fmt.Sprintf(`%s{%s=%q}[1h]`, ddprom.HostTagsMetricName, ddprom.HostLabelName, hostname)

	t.Run("query range result must be a matrix", func(t *testing.T) {
		expectedErr := fmt.Errorf("failed to cast query result to model.Matrix for query %q", query)
		apiMock := &apimock2.API{}
		apiMock.On("Query", ctx, query, now).
			Return(model.Vector{}, nil, nil)

		testedCortex := NewCortexGetter(apiMock, timeNowMock)
		_, err := testedCortex.Get(ctx, hostname)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("no labels found", func(t *testing.T) {
		expectedErr := NotFoundError{msg: `labels not found for host "host1"`}
		apiMock := &apimock2.API{}
		apiMock.On("Query", ctx, query, now).
			Return(model.Matrix{}, nil, nil)

		testedCortex := NewCortexGetter(apiMock, timeNowMock)
		_, err := testedCortex.Get(ctx, hostname)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("labels are properly returned", func(t *testing.T) {
		expectedLabels := []prompb.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
		}
		mockedMatrix := model.Matrix{
			{
				Metric: model.Metric{
					"foo":                                 "bar",
					model.LabelName(ddprom.HostLabelName): "host1",
					labels.MetricName:                     model.LabelValue(ddprom.HostTagsMetricName),
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(now.UnixNano() / int64(time.Millisecond)),
						Value:     1,
					},
				},
			},
		}

		apiMock := &apimock2.API{}
		apiMock.On("Query", ctx, query, now).
			Return(mockedMatrix, nil, nil)

		testedCortex := NewCortexGetter(apiMock, timeNowMock)
		lbls, err := testedCortex.Get(ctx, hostname)
		require.NoError(t, err)
		assert.Equal(t, expectedLabels, lbls)
	})

	t.Run("given two sets of samples, take the newest", func(t *testing.T) {
		expectedLabels := []prompb.Label{
			{
				Name:  "baz",
				Value: "qux",
			},
			{
				Name:  "foo",
				Value: "bar",
			},
		}
		mockedMatrix := model.Matrix{
			{
				Metric: model.Metric{
					"foo":                                 "bar",
					model.LabelName(ddprom.HostLabelName): "host1",
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(now.Add(-1*time.Minute).UnixNano() / int64(time.Millisecond)),
						Value:     1,
					},
				},
			},
			{
				Metric: model.Metric{
					"foo":                                 "bar",
					"baz":                                 "qux",
					model.LabelName(ddprom.HostLabelName): "host1",
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(now.UnixNano() / int64(time.Millisecond)),
						Value:     1,
					},
				},
			},
		}

		apiMock := &apimock2.API{}
		apiMock.On("Query", ctx, query, now).
			Return(mockedMatrix, nil, nil)

		testedCortex := NewCortexGetter(apiMock, timeNowMock)
		lbls, err := testedCortex.Get(ctx, hostname)
		require.NoError(t, err)
		assert.Equal(t, expectedLabels, lbls)
	})

	t.Run("series at time zero is correctly processed", func(t *testing.T) {
		expectedLabels := []prompb.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
		}
		mockedMatrix := model.Matrix{
			{
				Metric: model.Metric{
					"foo":                                 "bar",
					model.LabelName(ddprom.HostLabelName): "host1",
					labels.MetricName:                     model.LabelValue(ddprom.HostTagsMetricName),
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(0),
						Value:     1,
					},
				},
			},
		}

		apiMock := &apimock2.API{}
		apiMock.On("Query", ctx, query, now).
			Return(mockedMatrix, nil, nil)

		testedCortex := NewCortexGetter(apiMock, timeNowMock)
		lbls, err := testedCortex.Get(ctx, hostname)
		require.NoError(t, err)
		assert.Equal(t, expectedLabels, lbls)
	})
}
func TestCortex_GetAll(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	timeNowMock := func() time.Time { return now }
	query := ddprom.HostTagsMetricName + "[1h]"

	t.Run("get all host labels", func(t *testing.T) {
		expectedHostLabels := map[string]Host{
			"host1": {
				Labels: []prompb.Label{{
					Name:  "foo",
					Value: "bar",
				}},
				LastReportedTime: now,
			},
			"host2": {
				Labels: []prompb.Label{{
					Name:  "baz",
					Value: "qux",
				}},
				LastReportedTime: now,
			},
		}
		mockedMatrix := model.Matrix{
			{
				Metric: model.Metric{
					"foo":                                 "bar",
					model.LabelName(ddprom.HostLabelName): "host1",
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(now.UnixNano() / int64(time.Millisecond)),
						Value:     1,
					},
				},
			},
			{
				Metric: model.Metric{
					"baz":                                 "qux",
					model.LabelName(ddprom.HostLabelName): "host2",
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(now.UnixNano() / int64(time.Millisecond)),
						Value:     1,
					},
				},
			},
		}

		apiMock := &apimock2.API{}
		apiMock.On("Query", ctx, query, now).
			Return(mockedMatrix, nil, nil)

		testedCortex := NewCortexGetter(apiMock, timeNowMock)
		hostLabels, err := testedCortex.GetAll(ctx, now.Add(-time.Hour))
		require.NoError(t, err)
		assert.Equal(t, expectedHostLabels, hostLabels)
	})

	t.Run("no host labels", func(t *testing.T) {
		expectedHostLabels := map[string]Host{}
		mockedMatrix := model.Matrix{}

		apiMock := &apimock2.API{}
		apiMock.On("Query", ctx, query, now).
			Return(mockedMatrix, nil, nil)

		testedCortex := NewCortexGetter(apiMock, timeNowMock)
		hostLabels, err := testedCortex.GetAll(ctx, now.Add(-time.Hour))
		require.NoError(t, err)
		assert.Equal(t, expectedHostLabels, hostLabels)
	})
}
