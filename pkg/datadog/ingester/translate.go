package ingester

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/grafana/mimir-proxies/pkg/datadog/htstorage"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddprom"
	"github.com/grafana/mimir-proxies/pkg/datadog/ddstructs"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
)

const (
	hostLabel               = "host"
	deviceLabel             = "device"
	datadogTypeServiceCheck = "service_check"
)

func ddSeriesToPromWriteRequest(ctx context.Context, series ddstructs.Series, htClient htstorage.Getter) (*mimirpb.WriteRequest, error) {
	tsSlice := mimirpb.PreallocTimeseriesSliceFromPool()
	metadata := make([]*mimirpb.MetricMetadata, 0, len(series))

	cachedAllHostTags := cachedAllHostTagsFn(htClient)
	for _, serie := range series {
		if serie.Name == "" {
			return nil, fmt.Errorf("serie name should not be empty")
		}

		lbls := make(ddprom.Labels)
		for _, tag := range serie.Tags {
			if tag == "" {
				continue
			}
			lbls.AddTag(tag)
		}

		hostTags, err := cachedAllHostTags(ctx, serie.Host)
		if err != nil {
			return nil, err
		}
		for _, tag := range hostTags {
			lbls.AddTag(tag)
		}

		if serie.Host != "" {
			lbls.SetTagIfNotPresent(ddprom.NewTagFromKeyValue(hostLabel, serie.Host))
		}

		if serie.Device != "" {
			lbls.SetTagIfNotPresent(ddprom.NewTagFromKeyValue(deviceLabel, serie.Device))
		}

		mtyp := ddstructs.APIGaugeType
		if serie.MType != "" {
			mtyp = serie.MType
		}

		labelAdapters := lbls.LabelAdapters()
		labelAdapters = append(
			labelAdapters,
			mimirpb.LabelAdapter{Name: labels.MetricName, Value: ddprom.MetricToProm(serie.Name)},
			mimirpb.LabelAdapter{Name: ddprom.DDTypeLabel, Value: string(mtyp)},
		)
		samples := ddSamplesToMimirSamples(serie.Points, serie.MType, serie.Interval)

		ts := mimirpb.TimeseriesFromPool()
		ts.Labels = labelAdapters
		ts.Samples = samples
		tsSlice = append(tsSlice, mimirpb.PreallocTimeseries{
			TimeSeries: ts,
		})

		metadata = append(metadata, &mimirpb.MetricMetadata{
			MetricFamilyName: ddprom.MetricToProm(serie.Name),
			Type:             mimirpb.GAUGE,
		})
	}

	return &mimirpb.WriteRequest{
		Timeseries: tsSlice,
		Metadata:   metadata,
	}, nil
}

func ddSamplesToPromSamples(points []ddstructs.Point, mType ddstructs.APIMetricType, intervalInSeconds int64) []prompb.Sample {
	promSamples := make([]prompb.Sample, 0, len(points))

	for _, pt := range points {
		value := float64(pt.Value)

		/*
						DD negative interval value behavior (via manual testing on 2021-02-25):
							- interval = 0 -> Rates are treated like gauges when querying (as_count() and as_rate() have no effect)
							- interval < 0 -> If negative interval is submitted with samples via /api/v1/series, it is dropped and the
							  rate is treated like a gauge during querying. If interval is set to negative by modifying metric
							  metadata via /api/v1/metrics/{metric_name}, DD will lets set it to a negative value.
						      It looks like the negative interval is applied with as_rate() function but the interval is ignored
			                                  when using as_count().
						    We'll treat interval <= 0 consistently by storing the raw value.
		*/
		// TODO: make script so tests against DD API are reproducible
		if mType == ddstructs.APIRateType && intervalInSeconds > 0 {
			value *= float64(intervalInSeconds)
		}

		promSamples = append(promSamples, prompb.Sample{
			Timestamp: int64(pt.Ts) * 1000,
			Value:     value,
		})
	}

	// Prometheus samples have to be in order, but DD samples don't have to be
	sort.Sort(byTimestamp(promSamples))

	return promSamples
}

type byTimestamp []prompb.Sample

func (a byTimestamp) Len() int           { return len(a) }
func (a byTimestamp) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTimestamp) Less(i, j int) bool { return a[i].Timestamp < a[j].Timestamp }

func ddSamplesToMimirSamples(points []ddstructs.Point, mType ddstructs.APIMetricType, intervalInSeconds int64) []mimirpb.Sample {
	mimirSamples := make([]mimirpb.Sample, 0, len(points))

	for _, pt := range points {
		value := float64(pt.Value)

		/*
			DD negative interval value behavior (via manual testing on 2021-02-25):
				- interval = 0 -> Rates are treated like gauges when querying (as_count() and as_rate() have no effect)
				- interval < 0 -> If negative interval is submitted with samples via /api/v1/series, it is dropped and the
				  rate is treated like a gauge during querying. If interval is set to negative by modifying metric
				  metadata via /api/v1/metrics/{metric_name}, DD will lets set it to a negative value.
			      It looks like the negative interval is applied with as_rate()
			      function but the interval is ignored when using as_count().
			    We'll treat interval <= 0 consistently by storing the raw value.
		*/
		// TODO: make script so tests against DD API are reproducible
		if mType == ddstructs.APIRateType && intervalInSeconds > 0 {
			value *= float64(intervalInSeconds)
		}

		mimirSamples = append(mimirSamples, mimirpb.Sample{
			TimestampMs: int64(pt.Ts) * 1000,
			Value:       value,
		})
	}

	// Prometheus samples have to be in order, but DD samples don't have to be
	sort.Sort(byTimestampMs(mimirSamples))

	return mimirSamples
}

type byTimestampMs []mimirpb.Sample

func (a byTimestampMs) Len() int           { return len(a) }
func (a byTimestampMs) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTimestampMs) Less(i, j int) bool { return a[i].TimestampMs < a[j].TimestampMs }

// ddCheckRunToPromWriteRequest converts datadog service checks metrics into prometheus series.
// It's important to mention that in DataDog the checks don't live in the same namespace as metrics, and you can't
// graph them unless you use special widgets. However we've decided to keep them in the same metrics namespace to
// reduce the amount of work in the Datadog Datasource while increasing feature parity.
// Every service check metric will be appended with a label that identifies the timeseries type as service checks.
func ddCheckRunToPromWriteRequest(ctx context.Context, checks ddstructs.ServiceChecks, htClient htstorage.Getter) (*mimirpb.WriteRequest, error) {
	tsSlice := mimirpb.PreallocTimeseriesSliceFromPool()

	cachedAllHostTags := cachedAllHostTagsFn(htClient)
	// iterate through checks and create a series for each check
	for _, check := range checks {
		if check.CheckName == "" {
			return nil, fmt.Errorf("check name should not be empty")
		}

		lbls := make(ddprom.Labels)
		for _, tag := range check.Tags {
			lbls.AddTag(tag)
		}

		hostTags, err := cachedAllHostTags(ctx, check.Host)
		if err != nil {
			return nil, err
		}
		for _, tag := range hostTags {
			lbls.AddTag(tag)
		}

		if check.Host != "" {
			lbls.SetTagIfNotPresent(ddprom.NewTagFromKeyValue(hostLabel, check.Host))
		}

		labelAdapters := lbls.LabelAdapters()
		labelAdapters = append(
			labelAdapters,
			mimirpb.LabelAdapter{Name: labels.MetricName, Value: ddprom.MetricToProm(check.CheckName)},
			mimirpb.LabelAdapter{Name: ddprom.DDTypeLabel, Value: datadogTypeServiceCheck},
		)
		samples := []mimirpb.Sample{
			{
				// TODO: maybe check the timestamp even before sending it, as it might be already in the future
				// TODO: it could be a good idea to fix them if they're just _slightly_ in the future
				TimestampMs: check.TS * 1000,
				Value:       float64(check.Status),
			},
		}

		ts := mimirpb.TimeseriesFromPool()
		ts.Labels = labelAdapters
		ts.Samples = samples

		tsSlice = append(tsSlice, mimirpb.PreallocTimeseries{TimeSeries: ts})
	}
	return &mimirpb.WriteRequest{Timeseries: tsSlice}, nil
}

func cachedAllHostTagsFn(htClient htstorage.Getter) func(ctx context.Context, host string) ([]string, error) {
	hostTagsCache := make(map[string][]string)

	return func(ctx context.Context, host string) ([]string, error) {
		if _, ok := hostTagsCache[host]; !ok {
			hostLabels, err := htClient.Get(ctx, host)
			if errors.As(err, &htstorage.NotFoundError{}) {
				hostTagsCache[host] = nil
			} else if err != nil {
				return nil, err
			}
			for _, lbl := range hostLabels {
				if lbl.Name == ddprom.AllHostTagsLabelName {
					hostTagsCache[host] = ddprom.FromAllHostTagsLabelValue(lbl.Value)
					break
				}
			}
		}
		return hostTagsCache[host], nil
	}
}
