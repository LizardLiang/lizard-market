package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// SlugCmd returns the 'slug' command that converts text to a filesystem-safe
// slug. Orchestrators use it instead of hand-slugging plan titles, so the
// same title always yields the same path.
func SlugCmd() *cobra.Command {
	var dated bool
	cmd := &cobra.Command{
		Use:   "slug <text>...",
		Short: "Convert text to a kebab-case slug",
		Long:  "Lowercase, non-alphanumeric characters become '-', consecutive '-' collapse, leading/trailing '-' stripped. Multiple args are joined with spaces first. --dated prepends today's local date (YYYY-MM-DD-) for chronologically sortable artifact names (feature folders, tactical/strategic plans); omit it when the same title must always yield the same path.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			s := slugify(text)
			if s == "" {
				return fmt.Errorf("input produced an empty slug")
			}
			if dated {
				s = datedSlug(text)
			}
			fmt.Println(s)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dated, "dated", false, "prepend today's local date (YYYY-MM-DD-) to the slug")
	return cmd
}

// datedSlug prepends today's local date (YYYY-MM-DD-) to the slugified text,
// for chronologically sortable artifact names (feature folders, tactical and
// strategic plans). Local time is used so the date matches when the user
// created the artifact, not UTC.
func datedSlug(text string) string {
	return time.Now().Format("2006-01-02") + "-" + slugify(text)
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
