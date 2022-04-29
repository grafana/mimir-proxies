package ddprom

import (
	"sort"
	"strings"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/prometheus/prometheus/prompb"
)

// TagsToLabels maps a tag slice to Labels
func TagsToLabels(tags []string) Labels {
	lbls := make(Labels, len(tags))
	for _, tag := range tags {
		lbls.AddTag(tag)
	}
	return lbls
}

// Labels represents a set of prometheus labels
// Build one of these using make(Labels), add tags to it using the methods.
type Labels map[string]*label

func (ls Labels) AddTag(tagValue string) {
	tag := NewTag(tagValue)
	lblName := tag.LabelName()
	if l, ok := ls[lblName]; ok {
		l.addValue(tag.LabelValue())
		return
	}
	ls[lblName] = newLabel(lblName, tag.LabelValue(), tag.IsUnnamed())
}

// SetTagIfNotPresent adds a tag with desired name and value if there's no tag with that name present already
// This method accepts user-friendly unescaped name and value.
func (ls Labels) SetTagIfNotPresent(tag Tag) {
	lblName := tag.LabelName()
	if _, ok := ls[lblName]; !ok {
		ls[lblName] = newLabel(lblName, tag.LabelValue(), tag.IsUnnamed())
	}
}

func (ls Labels) PrompbLabels() []prompb.Label {
	pls := make([]prompb.Label, 0, len(ls))
	for _, l := range ls {
		pls = append(pls, l.prompbLabel())
	}
	sort.Slice(pls, func(i, j int) bool { return pls[i].Name < pls[j].Name })
	return pls
}

func (ls Labels) LabelAdapters() []mimirpb.LabelAdapter {
	labels := make(labels.Labels, 0, len(ls))
	for _, l := range ls {
		labels = append(labels, l.prometheusLabel())
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Name < labels[j].Name })
	return mimirpb.FromLabelsToLabelAdapters(labels)
}

// label represents a Prometheus label which can have multiple tag values
type label struct {
	name   string
	values []string

	isForUnnamedTag bool
}

// newLabel creates a new label
// It expects name and value params to be properly escaped already and being lowercase.
// Tag.LabelName(), Tag.LabelValue() and Tag.IsUnnamed() are good candidates for this constructor's params,
// but we don't require a Tag for performance reasons (otherwise we'd need to escape label name too many times)
func newLabel(name, value string, isForUnnamedTag bool) *label {
	return &label{name: name, values: []string{value}, isForUnnamedTag: isForUnnamedTag}
}

// addValue expects value to be escaped and in lowercase.
func (l *label) addValue(value string) {
	if l.isForUnnamedTag {
		// having same unnamed tag multiple times is idempotent
		return
	}
	if !containsString(l.values, value) {
		l.values = append(l.values, value)
	}
}

// prompbLabel will provide the prompb.Label value for this label
// As a side effect, it will sort the values slice
func (l *label) prompbLabel() prompb.Label {
	sb := strings.Builder{}
	sort.Strings(l.values)
	for i, v := range l.values {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('\'')
		sb.WriteString(v)
		sb.WriteByte('\'')
	}
	return prompb.Label{Name: l.name, Value: sb.String()}
}

// prometheusLabel will provide the labels.Label value for this label
// As a side effect, it will sort the values slice
func (l *label) prometheusLabel() labels.Label {
	sb := strings.Builder{}
	sort.Strings(l.values)
	for i, v := range l.values {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('\'')
		sb.WriteString(v)
		sb.WriteByte('\'')
	}
	return labels.Label{Name: l.name, Value: sb.String()}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
