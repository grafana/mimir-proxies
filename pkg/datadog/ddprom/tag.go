package ddprom

import (
	"sort"
	"strings"

	"github.com/grafana/mimir-proxies/pkg/util/bytereplacer"
	"github.com/prometheus/common/model"
)

const singleQuote = '\''

// ExtractMetricTags extracts all the tags stored in a Prometheus model.Metric
func ExtractMetricTags(metric model.Metric) []Tag {
	values := make([]Tag, 0, len(metric)-1)
	for name, value := range metric {
		if IsInternalLabel(string(name)) {
			continue
		}
		values = append(values, ExtractTagsFromLabel(string(name), string(value))...)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Tag() < values[j].Tag()
	})
	return values
}

// ExtractTagsFromLabel extracts the tags that a label provides
// This method assumes that the label provides is not an internal one, i.e.,
// IsInternalLabel(name) == false
func ExtractTagsFromLabel(name, value string) []Tag {
	if IsUnnamedTagLabelName(name) {
		// we don't store multiple values for unnamed tags, so we can safely read just one value
		return []Tag{
			{
				tag:       unquoteOneTagValue(value),
				isUnnamed: true,
			},
		}
	}

	tagName := UnescapeLabelName(name)
	quotedValues := strings.Split(value, ",")
	tags := make([]Tag, len(quotedValues))
	for i, qv := range quotedValues {
		tags[i] = NewTagFromKeyValue(tagName, unquoteOneTagValue(qv))
	}
	return tags
}

// NewTag creates a new Tag model from a single Tag string,
// it provides access to information like the tag name and whether it's unnamed or not
func NewTag(tag string) Tag {
	idx := strings.Index(tag, ":")
	if idx == -1 {
		return Tag{
			tag:       tag,
			isUnnamed: true,
		}
	}
	return Tag{
		tag:       tag,
		key:       tag[:idx],
		value:     tag[idx+1:],
		isUnnamed: false,
	}
}

// NewTagFromKeyValue builds a tag from a know key:value pair
func NewTagFromKeyValue(key, value string) Tag {
	return Tag{
		tag:       key + ":" + value,
		key:       key,
		value:     value,
		isUnnamed: false,
	}
}

// Tag represents a Datadog Tag and it's used to avoid splitting the tag to understand its underlying nature again and again.
type Tag struct {
	tag, key, value string
	isUnnamed       bool
}

// Tag is the entire tag indentifier
func (t Tag) Tag() string {
	return t.tag
}

// Key refers to the entire tag identifier when tag doesn't have any colon,
// when tag has a colon, the key is the part until the first colon, like `key:value`
func (t Tag) Key() string {
	if t.isUnnamed {
		return t.tag
	}
	return t.key
}

// IsUnnamed returns true if the tag is unnamed,
// i.e., when it doesn't have any colons,
// i.e., it is not a `key:value` tag
func (t Tag) IsUnnamed() bool {
	return t.isUnnamed
}

// LabelName provides the prometheus label name for this tag
// It calculates the value on the fly and doesn't cache it, so try to avoid calling it multiple times when possible.
func (t Tag) LabelName() string {
	if t.isUnnamed {
		return escapeLabelName(unnamedTagLabelPrefix + t.tag)
	}
	return escapeLabelName(t.key)
}

// LabelValue provides the prometheus label value for this tag,
// This doesn't include the harness for multi-value encoding within a single label.
// It calculates the value on the fly and doesn't cache it, so try to avoid calling it multiple times when possible.
func (t Tag) LabelValue() string {
	return t.labelValue(invalidTagCharsReplacer)
}

// LabelValueForQuery is like LabelValue but it also allows the wildcard character
func (t Tag) LabelValueForQuery() string {
	return t.labelValue(invalidTagCharsForQueryReplacer)
}

func (t Tag) labelValue(replacer bytereplacer.Replacer) string {
	var val string
	if t.isUnnamed {
		val = t.tag
	} else {
		val = t.value
	}

	val = strings.ToLower(val)
	val = replacer.Replace(val)
	return val
}

// unquoteOneTagValue removes the surrounding single from one tag value
func unquoteOneTagValue(v string) string {
	if len(v) < 2 || v[0] != singleQuote || v[len(v)-1] != singleQuote {
		// This is something that shouldn't happen except for the migration from single-valued tags
		return v
	}
	return v[1 : len(v)-1]
}
