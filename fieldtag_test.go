package data

import (
	"testing"

	"github.com/huandu/go-assert"
)

func TestParseFieldTag(t *testing.T) {
	cases := []struct {
		Tag      string
		FieldTag *FieldTag
	}{
		{ // 空 tag
			"",
			&FieldTag{},
		},
		{ // 只有 alias
			"abc",
			&FieldTag{
				Alias: "abc",
			},
		},
		{ // 各种选项
			",omitempty,not-valid,squash,",
			&FieldTag{
				OmitEmpty: true,
				Squash:    true,
			},
		},
		{ // 忽略 -
			"-",
			&FieldTag{
				Skipped: true,
			},
		},
		{ // 所有都包含
			"a1_b2,squash,omitempty",
			&FieldTag{
				Alias:     "a1_b2",
				OmitEmpty: true,
				Squash:    true,
			},
		},
	}

	for _, c := range cases {
		expected := c.FieldTag
		actual := ParseFieldTag(c.Tag)
		assert.AssertEqual(t, expected, actual)
	}
}
