package ddprom

import (
	"sort"
	"strings"

	"github.com/prometheus/prometheus/prompb"
)

const allHostTagsCommaSeparator = ","

// AllHostTagsPrompbLabel just creates an AllHostTags prompb.Label value in the format that FromAllHostTagsLabelValue method understands
// It lowercases the input and replaces invalid characters by underscores before storing them, but it doesn't do any other kind of assumption on the underlying data.
func AllHostTagsPrompbLabel(in []string) prompb.Label {
	seen := make(map[string]bool)
	tags := make([]string, 0, len(in))
	for _, tag := range in {
		escaped := strings.ToLower(invalidTagCharsReplacer.Replace(tag))
		if !seen[escaped] {
			seen[escaped] = true
			tags = append(tags, escaped)
		}
	}
	sort.Strings(tags)
	allTags := strings.Join(tags, allHostTagsCommaSeparator)

	return prompb.Label{
		Name:  AllHostTagsLabelName,
		Value: allTags,
	}
}

// FromAllHostTagsLabelValue parses the value of the AllHostTagsLabel returning the tags originally stored in it.
func FromAllHostTagsLabelValue(value string) []string {
	if value == "" {
		// If we don't have any input then return an empty slice rather than a single empty string
		// Otherwise the empty string can be converted into a _dot_unnamed_dot_ tag
		return nil
	}
	return strings.Split(value, allHostTagsCommaSeparator)
}
