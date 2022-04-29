package ddprom

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataDogMetricToProm(t *testing.T) {
	for _, tc := range []struct {
		dd   string
		prom string
	}{
		{
			dd:   "foo.bar",
			prom: "foo_dot_bar",
		},
		{
			dd:   "foo.bar",
			prom: "foo_dot_bar",
		},
		{
			dd:   "foo$bar",
			prom: "foo__bar",
		},
		{
			dd:   "foo$dot$bar",
			prom: "foo__dot__bar",
		},
	} {
		t.Run(tc.prom, func(t *testing.T) {
			got := MetricToProm(tc.dd)
			assert.Equal(t, tc.prom, got)
		})
	}
}

func TestEscapeTagPromLabelName(t *testing.T) {
	tests := []struct {
		in   string
		name string
		val  string
	}{
		{
			in:   "a:b",
			name: "a",
			val:  "b",
		},
		{
			in:   "b_c.d/e-f:a",
			name: "b__c_dot_d_sls_e_dsh_f",
			val:  "a",
		},
		{
			in:   "foo",
			name: "_dot_unnamed_dot_foo",
			val:  "foo",
		},
		{
			in:   "foo/bar",
			name: "_dot_unnamed_dot_foo_sls_bar",
			val:  "foo/bar",
		},
		{
			in:   "foo$:b",
			name: "foo__",
			val:  "b",
		},
		{
			in:   "foo$dsh$foo:bar",
			name: "foo__dsh__foo",
			val:  "bar",
		},
		{
			// "Tags are converted to lowercase."
			// https://docs.datadoghq.com/getting_started/tagging/#defining-tags
			in:   "tagsAreAlways:LOWERCASE",
			name: "tagsarealways",
			val:  "lowercase",
		},
	}

	for _, tt := range tests {
		tag := NewTag(tt.in)

		assert.Equal(t, tt.name, tag.LabelName())
		assert.Equal(t, tt.val, tag.LabelValue())
	}
}

func TestRoundtripTag(t *testing.T) {
	for _, rawTag := range []string{
		"foo:bar",
		"foo.bar:baz",
		"foo:bar:baz",
		"foo_dot_bar:alreadyescaped",
		"foo._.bar:baz",
		"foo_._bar:baz",
		"foo/bar:baz",
		"foo:bar/baz",
		"foo:bar_sls_baz",
		"foo:bar__sls__baz",
		"foo",
	} {
		t.Run(rawTag, func(t *testing.T) {
			tag := NewTag(rawTag)
			lbl := newLabel(tag.LabelName(), tag.LabelValue(), tag.IsUnnamed())
			pl := lbl.prompbLabel()
			assert.Equal(t, []Tag{tag}, ExtractTagsFromLabel(pl.Name, pl.Value))
		})
	}
}

func TestIsInternalLabel(t *testing.T) {
	for name, expected := range map[string]bool{
		"_dot_internal_dot_foobar":       true,
		NewTag("unnamedtag").LabelName(): false, // unnamed tags are not internal labels
		"__foobar":                       false,
		HostLabelName:                    true,
		"_dot_legacyinternal":            true, // TODO: remove once we forget about `_dot_` prefixed internal labels
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, expected, IsInternalLabel(name))
		})
	}
}

func TestMakeInternalLabelName(t *testing.T) {
	for name, expected := range map[string]string{
		"foo":    "_dot_internal_dot_foo",
		"e_cho.": "_dot_internal_dot_e__cho_dot_",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, expected, MakeInternalLabelName(name))
		})
	}
}

func TestMakeInternalMetricName(t *testing.T) {
	for name, expected := range map[string]string{
		"foo":    "_dot_internal_dot_foo",
		"e_cho.": "_dot_internal_dot_e__cho_dot_",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, expected, MakeInternalMetricName(name))
		})
	}
}
