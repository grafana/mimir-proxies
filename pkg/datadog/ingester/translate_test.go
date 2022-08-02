package ingester

import (
	"context"
	"testing"

	"github.com/grafana/mimir-proxies/pkg/datadog/htstorage"
	"github.com/grafana/mimir-proxies/pkg/datadog/htstorage/htstoragemock"
	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddprom"
	"github.com/grafana/mimir-proxies/pkg/datadog/ddstructs"

	"github.com/prometheus/prometheus/prompb"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestDDSamplesToPromSamples(t *testing.T) {
	tests := []struct {
		name         string
		ddPoints     []ddstructs.Point
		ddMetricType ddstructs.APIMetricType
		ddInterval   int64
		expected     []prompb.Sample
	}{
		{
			name: "counter type",
			ddPoints: []ddstructs.Point{
				{Ts: 1234, Value: 5},
				{Ts: 1235, Value: 3},
				{Ts: 1236, Value: 102},
			},
			ddMetricType: ddstructs.APICountType,
			expected: []prompb.Sample{
				{Timestamp: 1234000, Value: 5},
				{Timestamp: 1235000, Value: 3},
				{Timestamp: 1236000, Value: 102},
			},
		},
		{
			name: "rate type with 10s interval",
			ddPoints: []ddstructs.Point{
				{Ts: 1234, Value: 5},
				{Ts: 1235, Value: 3.2},
				{Ts: 1236, Value: 102},
			},
			ddMetricType: ddstructs.APIRateType,
			ddInterval:   10,
			expected: []prompb.Sample{
				{Timestamp: 1234000, Value: 50},
				{Timestamp: 1235000, Value: 32},
				{Timestamp: 1236000, Value: 1020},
			},
		},
		{
			name: "rate type with 5s interval",
			ddPoints: []ddstructs.Point{
				{Ts: 1234, Value: 5},
				{Ts: 1235, Value: 3},
				{Ts: 1236, Value: 102},
			},
			ddMetricType: ddstructs.APIRateType,
			ddInterval:   5,
			expected: []prompb.Sample{
				{Timestamp: 1234000, Value: 25},
				{Timestamp: 1235000, Value: 15},
				{Timestamp: 1236000, Value: 510},
			},
		},
		{
			name: "rate type with 0s interval",
			ddPoints: []ddstructs.Point{
				{Ts: 1234, Value: 5},
				{Ts: 1235, Value: 3.2},
				{Ts: 1236, Value: 102},
			},
			ddMetricType: ddstructs.APIRateType,
			ddInterval:   0,
			expected: []prompb.Sample{
				{Timestamp: 1234000, Value: 5},
				{Timestamp: 1235000, Value: 3.2},
				{Timestamp: 1236000, Value: 102},
			},
		},
		{
			name: "rate type with negative interval",
			ddPoints: []ddstructs.Point{
				{Ts: 1234, Value: 5},
				{Ts: 1235, Value: 3.2},
				{Ts: 1236, Value: 102},
			},
			ddMetricType: ddstructs.APIRateType,
			ddInterval:   -10,
			expected: []prompb.Sample{
				{Timestamp: 1234000, Value: 5},
				{Timestamp: 1235000, Value: 3.2},
				{Timestamp: 1236000, Value: 102},
			},
		},
		{
			name: "gauge type",
			ddPoints: []ddstructs.Point{
				{Ts: 1234, Value: 5},
				{Ts: 1235, Value: 3},
				{Ts: 1236, Value: 102},
			},
			ddMetricType: ddstructs.APIGaugeType,
			expected: []prompb.Sample{
				{Timestamp: 1234000, Value: 5},
				{Timestamp: 1235000, Value: 3},
				{Timestamp: 1236000, Value: 102},
			},
		},
		{
			name: "empty type",
			ddPoints: []ddstructs.Point{
				{Ts: 1234, Value: 5},
				{Ts: 1235, Value: 3},
				{Ts: 1236, Value: 102},
			},
			expected: []prompb.Sample{
				{Timestamp: 1234000, Value: 5},
				{Timestamp: 1235000, Value: 3},
				{Timestamp: 1236000, Value: 102},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ddSamplesToPromSamples(tt.ddPoints, tt.ddMetricType, tt.ddInterval)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestDDSeriesToPromSeries(t *testing.T) {
	tests := []struct {
		name         string
		ddSeries     ddstructs.Series
		storageMock  func() *htstoragemock.Storage
		expected     *mimirpb.WriteRequest
		expectsError bool
	}{
		{
			name: "empty series",
			storageMock: func() *htstoragemock.Storage {
				return &htstoragemock.Storage{}
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: make([]mimirpb.PreallocTimeseries, 0),
				Metadata:   make([]*mimirpb.MetricMetadata, 0),
			},
		},
		{
			name: "single series with no type have gauge type by default",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Name: "metric",
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 4321,
						},
					},
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "").Once().Return(nil, nil)
				return m1
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: []mimirpb.PreallocTimeseries{
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "__name__",
									Value: "metric",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: string(ddstructs.APIGaugeType),
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 1234000, // dd timestamps are in seconds
									Value:       4321,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
				},
				Metadata: []*mimirpb.MetricMetadata{
					{MetricFamilyName: "metric", Type: mimirpb.GAUGE},
				},
			},
		},
		{
			name: "single series with multiple samples unsorted",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Name: "metric",
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 3,
						},
						{
							Ts:    1231,
							Value: 1,
						},
						{
							Ts:    1233,
							Value: 2,
						},
					},
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "").Once().Return(nil, nil)
				return m1
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: []mimirpb.PreallocTimeseries{
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "__name__",
									Value: "metric",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: string(ddstructs.APIGaugeType),
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 1231000,
									Value:       1,
								},
								{
									TimestampMs: 1233000,
									Value:       2,
								},
								{
									TimestampMs: 1234000,
									Value:       3,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
				},
				Metadata: []*mimirpb.MetricMetadata{
					{MetricFamilyName: "metric", Type: mimirpb.GAUGE},
				},
			},
		},
		{
			name: "no metric name",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Host: "test",
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 4321,
						},
					},
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "test").Once().Return(nil, nil)
				return m1
			},
			expectsError: true,
		},
		{
			name: "multiple series with tags and types",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Host:  "test",
					Name:  "metric1",
					MType: ddstructs.APIGaugeType,
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 4321,
						},
					},
				},
				&ddstructs.Serie{
					Name: "metric2",
					Host: "host2",
					Points: []ddstructs.Point{
						{
							Ts:    5678,
							Value: 8765,
						},
					},
				},
				&ddstructs.Serie{
					Name:   "metric3",
					Device: "device",
					Points: []ddstructs.Point{
						{
							Ts:    910,
							Value: 19,
						},
					},
				},
				&ddstructs.Serie{
					Host:     "test4",
					Name:     "metric4",
					MType:    ddstructs.APIRateType,
					Interval: 6,
					Points: []ddstructs.Point{
						{
							Ts:    920,
							Value: 7,
						},
					},
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "").Once().Return(nil, nil)
				m1.On("Get", mock.Anything, "host2").Once().Return(nil, nil)
				m1.On("Get", mock.Anything, "test").Once().Return([]prompb.Label{
					{Name: "hosttag", Value: "value"},
					{Name: ddprom.AllHostTagsLabelName, Value: "hosttag:value"},
				}, nil)
				m1.On("Get", mock.Anything, "test4").Once().Return(nil, nil)
				return m1
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: []mimirpb.PreallocTimeseries{
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "host",
									Value: "'test'",
								},
								{
									Name:  "hosttag",
									Value: "'value'",
								},
								{
									Name:  "__name__",
									Value: "metric1",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: "gauge",
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 1234000, // dd timestamps are in seconds
									Value:       4321,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "host",
									Value: "'host2'",
								},
								{
									Name:  "__name__",
									Value: "metric2",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: string(ddstructs.APIGaugeType),
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 5678000, // dd timestamps are in seconds
									Value:       8765,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "device",
									Value: "'device'",
								},
								{
									Name:  "__name__",
									Value: "metric3",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: string(ddstructs.APIGaugeType),
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 910000, // dd timestamps are in seconds
									Value:       19,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "host",
									Value: "'test4'",
								},
								{
									Name:  "__name__",
									Value: "metric4",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: "rate",
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 920000,
									Value:       42,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
				},
				Metadata: []*mimirpb.MetricMetadata{
					{MetricFamilyName: "metric1", Type: mimirpb.GAUGE},
					{MetricFamilyName: "metric2", Type: mimirpb.GAUGE},
					{MetricFamilyName: "metric3", Type: mimirpb.GAUGE},
					{MetricFamilyName: "metric4", Type: mimirpb.GAUGE},
				},
			},
			expectsError: false,
		},
		{
			name: "host tags error",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Name: "metric",
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 4321,
						},
					},
					Host: "failing-host",
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "failing-host").Once().Return(nil, context.DeadlineExceeded)
				return m1
			},
			expectsError: true,
		},
		{
			name: "not found host tags",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Name: "metric",
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 4321,
						},
					},
					Host: "not-found-host",
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "not-found-host").Once().Return(nil, htstorage.NotFoundError{})
				return m1
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: []mimirpb.PreallocTimeseries{
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "host",
									Value: "'not-found-host'",
								},
								{
									Name:  "__name__",
									Value: "metric",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: string(ddstructs.APIGaugeType),
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 1234000, // dd timestamps are in seconds
									Value:       4321,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
				},
				Metadata: []*mimirpb.MetricMetadata{
					{MetricFamilyName: "metric", Type: mimirpb.GAUGE},
				},
			},
		},
		{
			name: "series multitags are comma separated",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Name: "metric",
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 4321,
						},
					},
					Tags: []string{
						"foo:bar",
						"foo:qux",
					},
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "").Once().Return(nil, nil)
				return m1
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: []mimirpb.PreallocTimeseries{
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "foo",
									Value: "'bar','qux'",
								},
								{
									Name:  "__name__",
									Value: "metric",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: string(ddstructs.APIGaugeType),
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 1234000, // dd timestamps are in seconds
									Value:       4321,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
				},
				Metadata: []*mimirpb.MetricMetadata{
					{MetricFamilyName: "metric", Type: mimirpb.GAUGE},
				},
			},
		},
		{
			name: "empty tags from a series are dropped",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Name: "metric",
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 4321,
						},
					},
					Tags: []string{
						"foo:bar",
						"",
					},
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "").Once().Return(nil, nil)
				return m1
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: []mimirpb.PreallocTimeseries{
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "foo",
									Value: "'bar'",
								},
								{
									Name:  "__name__",
									Value: "metric",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: string(ddstructs.APIGaugeType),
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 1234000, // dd timestamps are in seconds
									Value:       4321,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
				},
				Metadata: []*mimirpb.MetricMetadata{
					{MetricFamilyName: "metric", Type: mimirpb.GAUGE},
				},
			},
		},
		{
			name: "series that only have an empty tag get the tag dropped",
			ddSeries: ddstructs.Series{
				&ddstructs.Serie{
					Name: "metric",
					Points: []ddstructs.Point{
						{
							Ts:    1234,
							Value: 4321,
						},
					},
					Tags: []string{
						"",
					},
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "").Once().Return(nil, nil)
				return m1
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: []mimirpb.PreallocTimeseries{
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "__name__",
									Value: "metric",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: string(ddstructs.APIGaugeType),
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 1234000, // dd timestamps are in seconds
									Value:       4321,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
				},
				Metadata: []*mimirpb.MetricMetadata{
					{MetricFamilyName: "metric", Type: mimirpb.GAUGE},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			storeClient := tt.storageMock()
			actual, err := ddSeriesToPromWriteRequest(ctx, tt.ddSeries, storeClient)

			if tt.expectsError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)

			storeClient.AssertExpectations(t)
		})
	}
}

func TestDDCheckRunToPromSeries(t *testing.T) {
	tests := []struct {
		name         string
		checks       ddstructs.ServiceChecks
		storageMock  func() *htstoragemock.Storage
		expected     *mimirpb.WriteRequest
		expectsError bool
	}{
		{
			name: "host tags error",
			checks: ddstructs.ServiceChecks{
				&ddstructs.ServiceCheck{
					CheckName: "ok-check",
					Status:    ddstructs.ServiceCheckOK,
					Host:      "failing-host",
					TS:        1234,
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "failing-host").Once().Return(nil, context.DeadlineExceeded)
				return m1
			},
			expectsError: true,
		},
		{
			name: "not found host tags",
			checks: ddstructs.ServiceChecks{
				&ddstructs.ServiceCheck{
					CheckName: "warning-check",
					Status:    ddstructs.ServiceCheckWarning,
					Host:      "not-found-host",
					TS:        1234,
				},
			},
			storageMock: func() *htstoragemock.Storage {
				m1 := &htstoragemock.Storage{}
				m1.On("Get", mock.Anything, "not-found-host").Once().Return(nil, htstorage.NotFoundError{})
				return m1
			},
			expected: &mimirpb.WriteRequest{
				Timeseries: []mimirpb.PreallocTimeseries{
					{
						TimeSeries: &mimirpb.TimeSeries{
							Labels: []mimirpb.LabelAdapter{
								{
									Name:  "host",
									Value: "'not-found-host'",
								},
								{
									Name:  "__name__",
									Value: "warning__check",
								},
								{
									Name:  ddprom.DDTypeLabel,
									Value: "service_check",
								},
							},
							Samples: []mimirpb.Sample{
								{
									TimestampMs: 1234000, // dd timestamps are in seconds
									Value:       1,
								},
							},
							Exemplars: []mimirpb.Exemplar{},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			storeClient := tt.storageMock()
			actual, err := ddCheckRunToPromWriteRequest(ctx, tt.checks, storeClient)

			if tt.expectsError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)

			storeClient.AssertExpectations(t)
		})
	}
}
