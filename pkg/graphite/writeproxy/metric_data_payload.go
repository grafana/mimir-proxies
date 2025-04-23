package writeproxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/prometheus/prometheus/prompb"

	"github.com/grafana/metrictank/schema"
	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/grafana/mimir/pkg/util/spanlogger"
	"github.com/prometheus/prometheus/model/labels"
)

const (
	TaggedMetricName   = "graphite_tagged"
	UntaggedMetricName = "graphite_untagged"
)

type MetricDataPayload []*schema.MetricData

func (m MetricDataPayload) GeneratePromMetrics() ([]labels.Labels,
	[]mimirpb.Sample,
	error) {
	resLabels := make([]labels.Labels, 0, len(m))
	resSamples := make([]mimirpb.Sample, 0, len(m))

	labelsBuilder := labels.NewBuilder(nil)
	for _, md := range m {
		labels, sample, err := promMetricsFromMetricData(md, labelsBuilder)
		if err != nil {
			return nil, nil, err
		}

		resLabels = append(resLabels, labels)
		resSamples = append(resSamples, sample)
	}

	return resLabels, resSamples, nil
}

func (m MetricDataPayload) GeneratePromTimeSeries(ctx context.Context) ([]prompb.TimeSeries, error) {
	log, _ := spanlogger.New(ctx, "graphiteWriter.GeneratePromTimeSeries")
	defer log.Finish()

	series := make([]prompb.TimeSeries, 0, len(m))

	labelsBuilder := labels.NewBuilder(nil)
	for _, md := range m {
		labels, sample, err := promMetricsFromMetricData(md, labelsBuilder)
		if err != nil {
			return nil, err
		}
		serie := prompb.TimeSeries{
			Labels: labelsToLabelsProto(labels),
			Samples: []prompb.Sample{
				{Value: sample.Value, Timestamp: sample.TimestampMs},
			},
		}

		series = append(series, serie)
	}

	return series, nil
}

func (m MetricDataPayload) GeneratePreallocTimeseries(ctx context.Context) ([]mimirpb.PreallocTimeseries, error) {
	log, _ := spanlogger.New(ctx, "graphiteWriter.GeneratePreallocTimeseries")
	defer log.Finish()

	tsSlice := mimirpb.PreallocTimeseriesSliceFromPool()

	labelsBuilder := labels.NewBuilder(nil)
	for _, md := range m {
		labels, sample, err := promMetricsFromMetricData(md, labelsBuilder)
		if err != nil {
			return nil, err
		}
		ts := mimirpb.TimeseriesFromPool()
		ts.Labels = mimirpb.FromLabelsToLabelAdapters(labels)
		ts.Samples = []mimirpb.Sample{sample}

		tsSlice = append(tsSlice, mimirpb.PreallocTimeseries{
			TimeSeries: ts,
		})
	}

	return tsSlice, nil
}

func promMetricsFromMetricData(md *schema.MetricData, builder *labels.Builder) (labels.Labels,
	mimirpb.Sample,
	error) {
	if len(md.Tags) > 0 {
		return promMetricsFromMetricDataTagged(md, builder)
	}
	return promMetricsFromMetricDataUntagged(md, builder)
}

func promMetricsFromMetricDataTagged(
	md *schema.MetricData,
	builder *labels.Builder,
) (labels.Labels, mimirpb.Sample, error) {
	labels, err := LabelsFromTaggedName(md.Name, md.Tags, builder)
	return labels, mimirpb.Sample{Value: md.Value, TimestampMs: md.Time * 1000}, err
}

func LabelsFromTaggedName(name string, tags []string, builder *labels.Builder) (labels.Labels, error) {
	// 1 per tag, +1 for the graphite name, +1 for the prom name
	builder.Reset(make(labels.Labels, 0, len(tags)+2)) // nolint:gomnd

	for _, tag := range tags {
		equalIdx := strings.Index(tag, "=")
		if equalIdx <= 0 || equalIdx == len(tag)-1 {
			return nil, fmt.Errorf("encountered invalid tag %s", tag)
		}
		builder.Set(tag[:equalIdx], tag[equalIdx+1:])
	}

	builder.Set("name", name)
	builder.Set("__name__", TaggedMetricName)

	return builder.Labels(), nil
}

func promMetricsFromMetricDataUntagged(
	md *schema.MetricData,
	builder *labels.Builder,
) (labels.Labels, mimirpb.Sample, error) {
	return LabelsFromUntaggedName(md.Name, builder), mimirpb.Sample{
		Value:       md.Value,
		TimestampMs: md.Time * 1000,
	}, nil
}

func LabelsFromUntaggedName(name string, builder *labels.Builder) labels.Labels {
	// number of metric name nodes, +1 for the prom name
	builder.Reset(make(labels.Labels, 0, strings.Count(name, ".")+2)) // nolint:gomnd

	for i, node := range strings.Split(name, ".") {
		builder.Set(fmt.Sprintf("__n%03d__", i), node)
	}

	builder.Set("__name__", UntaggedMetricName)
	return builder.Labels()
}

// labelsToLabelsProto transforms labels into prompb labels.
func labelsToLabelsProto(labels labels.Labels) []prompb.Label {
	result := make([]prompb.Label, 0, len(labels))
	for _, l := range labels {
		result = append(result, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	return result
}
