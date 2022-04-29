/*
Package ddprom offers the contract of Datadog tags and metrics storage in Prometheus, as well as special labels handling.
In this context, a "tag" is always a Datadog tag (there are no tags in Prometheus) and a "label" is always a Prometheus label,
as there are no labels in Datadog.
*/
package ddprom

import (
	"regexp"
	"strings"

	"github.com/grafana/mimir-proxies/pkg/util/bytereplacer"

	"github.com/prometheus/prometheus/model/labels"
)

const (
	internalLabelPrefix   = ".internal."
	unnamedTagLabelPrefix = ".unnamed."
)

var (
	escapedInternalPrefix        = escapeLabelName(internalLabelPrefix)
	escapedUnnamedTagLabelPrefix = escapeLabelName(unnamedTagLabelPrefix)
)

var (
	// invalidMetricNameCharsReplacer replaces the characters that are not valid for a metric name in DataDog specification
	// https://docs.datadoghq.com/developers/metrics/#naming-custom-metrics
	invalidMetricNameCharsReplacer = bytereplacer.New(regexp.MustCompile(`[^\w.]`), '_')
	metricEscaper                  = newEscaper([]escapedChar{
		{'_', "__"},
		{'.', "_dot_"},
	})

	// invalidTagNameCharsReplacer replaces the characters that are not valid for a tag name in DataDog specification
	// https://docs.datadoghq.com/getting_started/tagging/#defining-tags
	invalidTagNameCharsReplacer = bytereplacer.New(regexp.MustCompile(`[^\w\-/.]`), '_')
	tagEscaper                  = newEscaper([]escapedChar{
		{'_', "__"},
		{'.', "_dot_"},
		{'-', "_dsh_"},
		{'/', "_sls_"},
	})

	// invalidTagCharsReplacer replaces the characters that are not valid for a tag in DataDog specification
	// https://docs.datadoghq.com/getting_started/tagging/#defining-tags
	invalidTagCharsReplacer = bytereplacer.New(regexp.MustCompile(`[^\w\-/.:]`), '_')
	// invalidTagCharsForQueryReplacer is like invalidTagCharsReplacer but also allows wildcards
	invalidTagCharsForQueryReplacer = bytereplacer.New(regexp.MustCompile(`[^\w\-/.:*]`), '_')
)

func IsInternalLabel(name string) bool {
	return name == labels.MetricName ||
		strings.HasPrefix(name, escapedInternalPrefix) ||
		(strings.HasPrefix(name, "_dot_") && !strings.HasPrefix(name, escapedUnnamedTagLabelPrefix)) // TODO remove legacy matcher
}

// MakeInternalLabelName transforms a string into an internal label name that
// will not collide with customer-defined labels.
func MakeInternalLabelName(label string) string {
	return escapedInternalPrefix + escapeLabelName(label)
}

// MakeInternalMetricName transforms a string into an internal metric name that
// will not collide with customer-defined metrics.
func MakeInternalMetricName(name string) string {
	return escapedInternalPrefix + MetricToProm(name)
}

// IsInternalMetricName returns true for metric names created using MakeInternalMetricName
func IsInternalMetricName(name string) bool {
	return strings.HasPrefix(name, escapedInternalPrefix)
}

// MetricToProm translates DataDog metric names into valid Prometheus metrics,
// previously removing any invalid for DataDog characters.
func MetricToProm(name string) string {
	name = invalidMetricNameCharsReplacer.Replace(name)
	return metricEscaper.escape(name)
}

// IsUnnamedTagLabelName returns true if label name corresponds to an unnamed tag
func IsUnnamedTagLabelName(name string) bool {
	return strings.HasPrefix(name, escapedUnnamedTagLabelPrefix)
}

// escapeLabelName escapes a desired label name to be stored in Prometheus.
// All datadog-invalid keys are replaced with underscores before escaping.
func escapeLabelName(name string) string {
	name = strings.ToLower(name)
	name = invalidTagNameCharsReplacer.Replace(name)
	return tagEscaper.escape(name)
}

// UnescapeLabelName provides the original (except for invalid chars) name of the tag stored in a Prometheus label name.
func UnescapeLabelName(val string) string {
	return tagEscaper.unescape(val)
}
