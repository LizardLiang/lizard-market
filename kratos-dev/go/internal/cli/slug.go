package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// SlugCmd returns the 'slug' command that converts text to a filesystem-safe
// slug. Orchestrators use it instead of hand-slugging plan titles, so the
// same title always yields the same path.
func SlugCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "slug <text>...",
		Short: "Convert text to a kebab-case slug",
		Long:  "Lowercase, non-alphanumeric characters become '-', consecutive '-' collapse, leading/trailing '-' stripped. Multiple args are joined with spaces first.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s := slugify(strings.Join(args, " "))
			if s == "" {
				return fmt.Errorf("input produced an empty slug")
			}
			fmt.Println(s)
			return nil
		},
	}
}

// slugify implements the plan-title rule from commands/strategy.md: lowercase,
// any run of non-alphanumeric characters becomes a single '-', trimmed.
func slugify(text string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}
