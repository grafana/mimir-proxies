package convert

import (
	"fmt"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
)

const (
	TaggedMetricName   = "graphite_tagged"
	UntaggedMetricName = "graphite_untagged"
)

func LabelsFromUntaggedName(name string, builder *labels.Builder) labels.Labels {
	// number of metric name nodes, +1 for the prom name
	builder.Reset(make(labels.Labels, 0, strings.Count(name, ".")+2)) //nolint:gomnd

	for i, node := range strings.Split(name, ".") {
		builder.Set(fmt.Sprintf("__n%03d__", i), node)
	}

	builder.Set("__name__", UntaggedMetricName)
	return builder.Labels()
}
