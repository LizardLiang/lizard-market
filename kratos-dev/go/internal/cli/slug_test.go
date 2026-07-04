package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"My Plan", "my-plan"},
		{"My Plan: Q3!", "my-plan-q3"},
		{"  spaced  out  ", "spaced-out"},
		{"already-a-slug", "already-a-slug"},
		{"UPPER_case mixed", "upper-case-mixed"},
		{"---dashes---everywhere---", "dashes-everywhere"},
		{"v2.84.3 release", "v2-84-3-release"},
		{"中文 title 系統", "title"},
		{"!!!", ""},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, slugify(c.in), "slugify(%q)", c.in)
	}
}
