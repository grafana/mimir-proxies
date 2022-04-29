package ddprom

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus/prometheus/prompb"
)

func TestTagsToLabels(t *testing.T) {
	t.Run("happy case, duplicated values, escaping", func(t *testing.T) {
		tags := []string{
			"foo:bar",
			"FOO:BAR", // will be deduplicated
			"foo:baz", // extra value
			"bar:baz,boom",
		}

		expected := []prompb.Label{
			{Name: "bar", Value: "'baz_boom'"},
			{Name: "foo", Value: "'bar','baz'"},
		}
		got := TagsToLabels(tags).PrompbLabels()

		assert.Equal(t, expected, got)
	})
}
