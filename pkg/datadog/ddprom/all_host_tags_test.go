package ddprom

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromAllHostTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple splitting of single host tag",
			input:    "foo",
			expected: []string{"foo"},
		},
		{
			name:     "simple splitting of multiple host tags",
			input:    "foo,bar,baz",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "blank host tag input",
			input:    "",
			expected: []string(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := FromAllHostTagsLabelValue(tt.input)
			assert.Equal(t, actual, tt.expected)
		})
	}
}

func TestAllHostTags(t *testing.T) {
	t.Run("replaces invalid characters with underscore, especially the comma", func(t *testing.T) {
		tags := []string{"foo", "bar", "foo,bar", "foo/baz"}

		lbl := AllHostTagsPrompbLabel(tags)
		assert.Equal(t, AllHostTagsLabelName, lbl.Name)
		assert.Equal(t, "bar,foo,foo/baz,foo_bar", lbl.Value)
	})

	t.Run("produces same output regardless the order, case or repeatendess", func(t *testing.T) {
		first := []string{"foo:bar", "foo:boo", "baz/boom"}
		second := make([]string, len(first))
		copy(second, first)
		second[0], second[1] = second[1], strings.ToUpper(second[0])
		second = append(second, second[0])

		firstLabel := AllHostTagsPrompbLabel(first)
		secondLabel := AllHostTagsPrompbLabel(second)

		assert.Equal(t, firstLabel, secondLabel)
	})

	t.Run("inverse function FromAllHostTagsLabelValue(AllHostTagsPrompbLabel.Value)", func(t *testing.T) {
		tags := []string{"baz/boom", "foo:bar", "foo:boo"}

		lbl := AllHostTagsPrompbLabel(tags)
		assert.Equal(t, AllHostTagsLabelName, lbl.Name)

		assert.Equal(t, tags, FromAllHostTagsLabelValue(lbl.Value))
	})
}
