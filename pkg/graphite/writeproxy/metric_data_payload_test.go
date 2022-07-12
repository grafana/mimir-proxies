package writeproxy

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/metrictank/schema"
	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
)

func TestGeneratePromMetrics(t *testing.T) {

	now := time.Now().Unix()
	var tests = map[string]struct {
		metricData MetricDataPayload
		expLabels  []labels.Labels
		expSamples []mimirpb.Sample
		expErr     bool
	}{
		"happy path: prom metric is properly generated": {
			metricData: MetricDataPayload{
				&schema.MetricData{
					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     now - 100,
				},
			},
			expLabels: []labels.Labels{
				{
					labels.Label{Name: "__name__", Value: "graphite_tagged"},
					labels.Label{Name: "foo", Value: "bar"},
					labels.Label{Name: "name", Value: "some.test.metric"},
					labels.Label{Name: "tag", Value: "value"},
				},
			},
			expSamples: []mimirpb.Sample{
				{Value: 0, TimestampMs: (now - 100) * 1000},
			},
		},
		"happy path: two metrics are properly translated": {
			metricData: MetricDataPayload{
				&schema.MetricData{
					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     now - 100,
				},
				&schema.MetricData{
					OrgId:    1,
					Name:     "some.test.metric2",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     now - 100,
				},
			},
			expLabels: []labels.Labels{
				{
					labels.Label{Name: "__name__", Value: "graphite_tagged"},
					labels.Label{Name: "foo", Value: "bar"},
					labels.Label{Name: "name", Value: "some.test.metric"},
					labels.Label{Name: "tag", Value: "value"},
				},
				{
					labels.Label{Name: "__name__", Value: "graphite_tagged"},
					labels.Label{Name: "foo", Value: "bar"},
					labels.Label{Name: "name", Value: "some.test.metric2"},
					labels.Label{Name: "tag", Value: "value"},
				},
			},
			expSamples: []mimirpb.Sample{
				{Value: 0, TimestampMs: (now - 100) * 1000},
				{Value: 0, TimestampMs: (now - 100) * 1000},
			},
		},
		"if there is an invalid tag method should fail": {
			metricData: MetricDataPayload{
				&schema.MetricData{
					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag value"}, // should be tag=value
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     now - 100,
				},
			},
			expErr: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			labels, samples, err := test.metricData.GeneratePromMetrics()
			if test.expErr {
				assert.Error(err)
			} else {
				assert.Equal(test.expLabels, labels)
				assert.Equal(test.expSamples, samples)
				assert.NoError(err)
			}
		})
	}
}

func TestGeneratePromTimeSeries(t *testing.T) {
	now := time.Now().Unix()
	ctx := context.Background()
	var tests = map[string]struct {
		metricData    MetricDataPayload
		expTimeSeries []prompb.TimeSeries
		expErr        bool
	}{
		"happy path: prom time series is properly generated": {
			metricData: MetricDataPayload{
				&schema.MetricData{
					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     now - 100,
				},
			},
			expTimeSeries: []prompb.TimeSeries{
				{
					Labels: []prompb.Label{
						{Name: "__name__", Value: "graphite_tagged"},
						{Name: "foo", Value: "bar"},
						{Name: "name", Value: "some.test.metric"},
						{Name: "tag", Value: "value"},
					},
					Samples: []prompb.Sample{
						{Value: 0, Timestamp: (now - 100) * 1000},
					},
				},
			},
			expErr: false,
		},
		"happy path: two metrics are properly translated": {
			metricData: MetricDataPayload{
				&schema.MetricData{
					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     now - 100,
				},
				&schema.MetricData{
					OrgId:    1,
					Name:     "some.test.metric2",
					Tags:     []string{"tag=value", "foo=bar"},
					Mtype:    "invalid",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     now - 100,
				},
			},
			expTimeSeries: []prompb.TimeSeries{
				{
					Labels: []prompb.Label{
						{Name: "__name__", Value: "graphite_tagged"},
						{Name: "foo", Value: "bar"},
						{Name: "name", Value: "some.test.metric"},
						{Name: "tag", Value: "value"},
					},
					Samples: []prompb.Sample{
						{Value: 0, Timestamp: (now - 100) * 1000},
					},
				},
				{
					Labels: []prompb.Label{
						{Name: "__name__", Value: "graphite_tagged"},
						{Name: "foo", Value: "bar"},
						{Name: "name", Value: "some.test.metric2"},
						{Name: "tag", Value: "value"},
					},
					Samples: []prompb.Sample{
						{Value: 0, Timestamp: (now - 100) * 1000},
					},
				},
			},
			expErr: false,
		},
		"tags with bad format should fail": {
			metricData: MetricDataPayload{
				&schema.MetricData{
					OrgId:    1,
					Name:     "some.test.metric",
					Tags:     []string{"tag value"}, // should be tag=value
					Mtype:    "gauge",
					Interval: 1, // property must be set, but cortex-graphite ignores it
					Time:     now - 100,
				},
			},
			expErr: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			series, err := test.metricData.GeneratePromTimeSeries(ctx)
			if test.expErr {
				assert.Error(err)
			} else {
				assert.Equal(test.expTimeSeries, series)
				assert.NoError(err)
			}
		})
	}
}

func TestLabelsFromTaggedName(t *testing.T) {
	labelsBuilder := labels.NewBuilder(nil)
	tests := map[string]struct {
		name      string
		tags      []string
		builder   *labels.Builder
		expLabels labels.Labels
		expErr    bool
	}{
		"happy path: label is properly extracted from tagged name": {
			name: "some.test.metric",
			tags: []string{
				"foo=bar",
			},
			builder: labelsBuilder,
			expLabels: labels.Labels{
				labels.Label{Name: "__name__", Value: "graphite_tagged"},
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "name", Value: "some.test.metric"},
			},
			expErr: false,
		},
		"tags with bad format fail": {
			name: "some.test.metric",
			tags: []string{
				"foo bar",
			},
			builder: labelsBuilder,
			expErr:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			labels, err := LabelsFromTaggedName(test.name, test.tags, test.builder)
			if test.expErr {
				assert.Error(err)
			} else {
				assert.Equal(test.expLabels, labels)
				assert.NoError(err)
			}

		})
	}
}

func TestLabelsFromUntaggedName(t *testing.T) {
	labelsBuilder := labels.NewBuilder(nil)
	tests := map[string]struct {
		name      string
		tags      []string
		builder   *labels.Builder
		expLabels labels.Labels
	}{
		"happy path: label is properly extracted from untagged name": {
			name:    "query.1.metric1",
			builder: labelsBuilder,
			expLabels: labels.Labels{
				labels.Label{Name: "__n000__", Value: "query"},
				labels.Label{Name: "__n001__", Value: "1"},
				labels.Label{Name: "__n002__", Value: "metric1"},
				labels.Label{Name: "__name__", Value: "graphite_untagged"},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			labels := LabelsFromUntaggedName(test.name, test.builder)

			assert.Equal(test.expLabels, labels)
		})
	}
}
