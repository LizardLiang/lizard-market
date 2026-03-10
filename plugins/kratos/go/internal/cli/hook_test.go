package cli

import (
	"strings"
	"testing"
)

func TestSanitizePrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // should NOT contain after sanitization
	}{
		{
			name:     "strips fenced code blocks",
			input:    "hello ```go\nfunc kratos() {}\n``` world",
			contains: "kratos",
		},
		{
			name:     "strips inline code",
			input:    "check `kratos.Config` please",
			contains: "kratos.Config",
		},
		{
			name:     "strips URLs",
			input:    "see https://github.com/kratos/example for details",
			contains: "kratos/example",
		},
		{
			name:     "strips system reminders",
			input:    "hello <system-reminder>kratos is great</system-reminder> world",
			contains: "kratos is great",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePrompt(tt.input)
			if strings.Contains(result, tt.contains) {
				t.Errorf("sanitizePrompt() still contains %q: got %q", tt.contains, result)
			}
		})
	}
}

func TestMatchKeywords(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "matches kratos",
			text: "hey Kratos build this",
			want: []string{"kratos"},
		},
		{
			name: "matches god name",
			text: "Athena, write a PRD",
			want: []string{"athena"},
		},
		{
			name: "matches multiple",
			text: "Kratos, have Ares implement and Hermes review",
			want: []string{"kratos", "ares", "hermes"},
		},
		{
			name: "no match on normal text",
			text: "please fix the login bug",
			want: nil,
		},
		{
			name: "case insensitive",
			text: "KRATOS do this",
			want: []string{"kratos"},
		},
		{
			name: "no partial match",
			text: "the kratosConfig variable",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchKeywords(tt.text)
			if len(got) != len(tt.want) {
				t.Errorf("matchKeywords() returned %d matches, want %d: %v", len(got), len(tt.want), got)
				return
			}
			for _, w := range tt.want {
				found := false
				for _, g := range got {
					if g == w {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("matchKeywords() missing expected match %q in %v", w, got)
				}
			}
		})
	}
}

func TestBuildInjectionContext(t *testing.T) {
	t.Run("kratos keyword", func(t *testing.T) {
		ctx := buildInjectionContext([]string{"kratos"})
		if !strings.Contains(ctx, "invoked Kratos by name") {
			t.Error("should mention Kratos invocation")
		}
		if !strings.Contains(ctx, "kratos:auto") {
			t.Error("should reference kratos:auto skill")
		}
	})

	t.Run("god name only", func(t *testing.T) {
		ctx := buildInjectionContext([]string{"athena"})
		if strings.Contains(ctx, "invoked Kratos by name") {
			t.Error("should NOT mention Kratos invocation for god-only match")
		}
		if !strings.Contains(ctx, "athena") {
			t.Error("should mention the god name")
		}
	})

	t.Run("mixed", func(t *testing.T) {
		ctx := buildInjectionContext([]string{"kratos", "ares", "hermes"})
		if !strings.Contains(ctx, "invoked Kratos by name") {
			t.Error("should mention Kratos")
		}
		if !strings.Contains(ctx, "ares") {
			t.Error("should mention ares")
		}
		if !strings.Contains(ctx, "hermes") {
			t.Error("should mention hermes")
		}
	})
}
