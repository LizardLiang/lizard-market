package cli

import (
	"regexp"
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

func TestDatedSlug(t *testing.T) {
	// Assert the shape (YYYY-MM-DD-<slug>), not a hardcoded date — the test
	// must keep passing regardless of what day it runs.
	datedShape := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-add-auth$`)

	got := datedSlug("Add Auth")
	assert.True(t, datedShape.MatchString(got), "datedSlug output %q does not match dated shape", got)

	// Without dating, behavior is byte-identical to plain slugify.
	assert.Equal(t, "add-auth", slugify("Add Auth"))
}
