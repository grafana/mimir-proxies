package ddprom

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestMetricToTags(t *testing.T) {
	metric := model.Metric{
		model.MetricNameLabel:                               "foobar",
		model.LabelName("tagwithvalue"):                     "'value'",
		model.LabelName("tagwithtwovalues"):                 "'value1','value2'",
		model.LabelName("_dot_unnamed_dot_tagwithoutvalue"): "'tagwithoutvalue'",
	}

	expectedTags := []Tag{
		NewTag("tagwithoutvalue"),
		NewTag("tagwithtwovalues:value1"),
		NewTag("tagwithtwovalues:value2"),
		NewTag("tagwithvalue:value"),
	}

	tags := ExtractMetricTags(metric)
	assert.Equal(t, expectedTags, tags)
}

func TestTag(t *testing.T) {
	t.Run("tag without value", func(t *testing.T) {
		tag := NewTag("tag")
		assert.True(t, tag.IsUnnamed())
		assert.Equal(t, "tag", tag.Key())
		assert.Equal(t, "tag", tag.Tag())
	})
	t.Run("tag with value", func(t *testing.T) {
		tag := NewTag("foo:bar")
		assert.False(t, tag.IsUnnamed())
		assert.Equal(t, "foo", tag.Key())
		assert.Equal(t, "foo:bar", tag.Tag())
	})
}
