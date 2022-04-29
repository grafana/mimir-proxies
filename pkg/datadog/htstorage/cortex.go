package htstorage

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/grafana/mimir-proxies/pkg/remoteread"
	"github.com/grafana/mimir-proxies/pkg/remotewrite"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddprom"

	"github.com/prometheus/prometheus/promql/parser"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"

	labelsutil "github.com/grafana/mimir-proxies/pkg/util/labels"
)

type Cortex struct {
	write   remotewrite.Client
	read    remoteread.API
	timeNow func() time.Time
}

func NewCortexStorage(write remotewrite.Client, read remoteread.API, timeNow func() time.Time) Storage {
	return &Cortex{write: write, read: read, timeNow: timeNow}
}

func NewCortexGetter(read remoteread.API, timeNow func() time.Time) Getter {
	return &Cortex{read: read, timeNow: timeNow}
}

func (c *Cortex) Set(ctx context.Context, hostName string, lbls []prompb.Label) error {
	req := request(lbls, hostName)
	defer mimirpb.ReuseSlice(req.Timeseries)
	return c.write.Write(ctx, req)
}

// Get fetches the labels for a given host, or returns a NotFoundError if no labels were defined for that host.
// Labels are sorted lexicographically by label name.
func (c *Cortex) Get(ctx context.Context, hostName string) ([]prompb.Label, error) {
	hostMatcher, err := labels.NewMatcher(labels.MatchEqual, ddprom.HostLabelName, hostName)
	if err != nil {
		return nil, err
	}
	matchers := []*labels.Matcher{hostMatcher}

	m, err := c.get(ctx, c.timeNow().Add(-time.Hour), matchers)
	if err != nil {
		return nil, err
	}

	h, ok := m[hostName]
	if !ok {
		return nil, NotFoundError{msg: fmt.Sprintf("labels not found for host %q", hostName)}
	}

	return h.Labels, nil
}

// GetAll fetches all host tags in the specified time window and returns a map where the key elements are host names
// and the values are prometheus labels.
// For each host, labels are sorted lexicographically by label name.
// The actual LastReportedField value may be up to 5 minutes earlier compared to the actual value due to Cortex staleness handling
func (c *Cortex) GetAll(ctx context.Context, from time.Time) (map[string]Host, error) {
	return c.get(ctx, from, nil)
}

func (c *Cortex) get(ctx context.Context, from time.Time, matchers []*labels.Matcher) (map[string]Host, error) {
	hostMatcher, err := labels.NewMatcher(labels.MatchEqual, model.MetricNameLabel, ddprom.HostTagsMetricName)
	if err != nil {
		return nil, err
	}
	m := []*labels.Matcher{hostMatcher}

	now := c.timeNow()

	metric := &parser.MatrixSelector{
		VectorSelector: &parser.VectorSelector{
			Name:          ddprom.HostTagsMetricName,
			LabelMatchers: append(m, matchers...),
		},
		Range: now.Sub(from),
	}

	query := metric.String()

	result, _, err := c.read.Query(ctx, query, now)
	if err != nil {
		return nil, err
	}

	matrix, ok := result.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("failed to cast query result to model.Matrix for query %q", query)
	}

	if len(matrix) == 0 {
		return map[string]Host{}, nil
	}

	// The matrix may contain multiple series per host so we store the matrix position for the newest series per
	// host
	type pos struct {
		ts  int64
		pos int
	}

	latestHostSeries := make(map[string]pos)
	for i, s := range matrix {
		var host string
		for labelName, labelValue := range s.Metric {
			if string(labelName) == ddprom.HostLabelName {
				host = string(labelValue)
				break
			}
		}

		_, found := latestHostSeries[host]
		ts := newestSampleTime(s)

		if !found || latestHostSeries[host].ts < ts {
			latestHostSeries[host] = pos{ts: ts, pos: i}
		}
	}

	hostLabels := make(map[string]Host)
	for host, p := range latestHostSeries {
		var lbls []prompb.Label
		for labelName, labelValue := range matrix[p.pos].Metric {
			if labelName == labels.MetricName || string(labelName) == ddprom.HostLabelName {
				continue
			}
			lbls = append(lbls, prompb.Label{Name: string(labelName), Value: string(labelValue)})
		}
		sort.Slice(lbls, func(i, j int) bool { return lbls[i].Name < lbls[j].Name })

		hostLabels[host] = Host{
			Labels:           lbls,
			LastReportedTime: time.Unix(0, p.ts*int64(time.Millisecond)).UTC(),
		}
	}

	return hostLabels, nil
}

func request(lbls []prompb.Label, hostName string) *mimirpb.WriteRequest {
	lbls = append(
		lbls,
		prompb.Label{Name: ddprom.HostLabelName, Value: hostName},
		prompb.Label{Name: labels.MetricName, Value: ddprom.HostTagsMetricName},
	)
	labels := labelsutil.LabelProtosToLabels(lbls)
	labelAdapter := mimirpb.FromLabelsToLabelAdapters(labels)
	samples := []mimirpb.Sample{
		{
			Value:       1,
			TimestampMs: time.Now().UnixNano() / int64(time.Millisecond),
		},
	}

	ts := mimirpb.TimeseriesFromPool()
	ts.Labels = labelAdapter
	ts.Samples = samples

	tsSlice := mimirpb.PreallocTimeseriesSliceFromPool()
	tsSlice = append(tsSlice, mimirpb.PreallocTimeseries{
		TimeSeries: ts,
	})

	return &mimirpb.WriteRequest{
		Timeseries: tsSlice,
		Metadata: []*mimirpb.MetricMetadata{
			{
				MetricFamilyName: ddprom.HostTagsMetricName,
				Type:             mimirpb.GAUGE,
			},
		},
	}
}

func newestSampleTime(stream *model.SampleStream) int64 {
	if len(stream.Values) == 0 {
		return 0
	}
	return int64(stream.Values[len(stream.Values)-1].Timestamp)
}
