package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// setHomeEnv redirects os.UserHomeDir() to dir for the duration of the test, covering
// both the unix (HOME) and windows (USERPROFILE) code paths.
func setHomeEnv(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

// writeHandoff creates .claude/.Arena/handoff.md under root with the given content and
// mtime offset (negative = in the past).
func writeHandoff(t *testing.T, root, content string, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(root, ".claude", ".Arena")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "handoff.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	mtime := time.Now().Add(-age)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	return path
}

func TestMatchesResumePhrase(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"continue", "let's continue where we left off", true},
		{"resume", "resume the task", true},
		{"keep going", "please keep going", true},
		{"where were we", "where were we?", true},
		{"where did we stop", "where did we stop yesterday", true},
		{"pick up", "pick up the linter changes", true},
		{"case insensitive", "CONTINUE", true},
		{"negative: continuec not a word", "the continuec token", false},
		{"negative: unrelated text", "please fix the login bug", false},
		{"negative: resumeable not a word", "the job is resumeable", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesResumePhrase(tt.text)
			if got != tt.want {
				t.Errorf("matchesResumePhrase(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestMatchesResumePhraseSanitizedInput(t *testing.T) {
	// Code-fenced "continue" must not match once run through sanitizePrompt, the same
	// cleaning handlePromptSubmit applies before resume-phrase matching.
	prompt := "hello ```\ncontinue\n``` world"
	cleaned := sanitizePrompt(prompt)
	if matchesResumePhrase(cleaned) {
		t.Errorf("matchesResumePhrase should not match code-fenced continue after sanitization: %q", cleaned)
	}
}

func TestHandoffInjectionContext(t *testing.T) {
	t.Run("fresh handoff injects on resume phrase", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		setHomeEnv(t, home)
		writeHandoff(t, root, "# Session Handoff\n\nShipped: the thing.", 1*time.Hour)

		ctx := handoffInjectionContext("continue", hookInput{Cwd: root, SessionID: "s1"})
		if ctx == "" {
			t.Fatal("expected non-empty handoff context")
		}
		if !strings.Contains(ctx, "Shipped: the thing.") {
			t.Errorf("context missing handoff content: %q", ctx)
		}
	})

	t.Run("no resume phrase means no injection", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		setHomeEnv(t, home)
		writeHandoff(t, root, "# Session Handoff\n\nShipped: the thing.", 1*time.Hour)

		ctx := handoffInjectionContext("please fix the bug", hookInput{Cwd: root, SessionID: "s1"})
		if ctx != "" {
			t.Errorf("expected no injection without a resume phrase, got %q", ctx)
		}
	})

	t.Run("missing handoff file", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		setHomeEnv(t, home)

		ctx := handoffInjectionContext("continue", hookInput{Cwd: root, SessionID: "s1"})
		if ctx != "" {
			t.Errorf("expected no injection when handoff.md is missing, got %q", ctx)
		}
	})

	t.Run("stale handoff (>=7 days) skipped", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		setHomeEnv(t, home)
		writeHandoff(t, root, "# Session Handoff\n\nold stuff", 8*24*time.Hour)

		ctx := handoffInjectionContext("continue", hookInput{Cwd: root, SessionID: "s1"})
		if ctx != "" {
			t.Errorf("expected stale handoff to be skipped, got %q", ctx)
		}
	})

	t.Run("once-per-session marker honored", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		setHomeEnv(t, home)
		writeHandoff(t, root, "# Session Handoff\n\nShipped: the thing.", 1*time.Hour)

		input := hookInput{Cwd: root, SessionID: "s-repeat"}
		first := handoffInjectionContext("continue", input)
		if first == "" {
			t.Fatal("expected injection on first call")
		}
		second := handoffInjectionContext("continue", input)
		if second != "" {
			t.Errorf("expected no injection on second call (marker should suppress it), got %q", second)
		}
	})

	t.Run("missing session_id skips the guard (always injects)", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		setHomeEnv(t, home)
		writeHandoff(t, root, "# Session Handoff\n\nShipped: the thing.", 1*time.Hour)

		input := hookInput{Cwd: root, SessionID: ""}
		first := handoffInjectionContext("continue", input)
		if first == "" {
			t.Fatal("expected injection on first call")
		}
		second := handoffInjectionContext("continue", input)
		if second == "" {
			t.Error("expected injection on second call too — no session_id means guard is skipped")
		}
	})
}

func TestMergeContexts(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"both empty", []string{"", ""}, ""},
		{"only first", []string{"a", ""}, "a"},
		{"only second", []string{"", "b"}, "b"},
		{"both present", []string{"a", "b"}, "a\n\nb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeContexts(tt.parts...)
			if got != tt.want {
				t.Errorf("mergeContexts(%v) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestPromptSubmitInMergedKeywordAndHandoff(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setHomeEnv(t, home)
	writeHandoff(t, root, "# Session Handoff\n\nShipped: the auth fix.", 1*time.Hour)

	payload, _ := json.Marshal(map[string]string{
		"prompt":     "kratos, continue",
		"session_id": "s-merged",
		"cwd":        root,
	})

	out := promptSubmitIn(payload)
	if out.HookSpecificOutput == nil {
		t.Fatal("expected hookSpecificOutput, got passthrough")
	}
	ctx := out.HookSpecificOutput.AdditionalContext
	if !strings.Contains(ctx, "KRATOS KEYWORD DETECTED") {
		t.Errorf("merged context missing skill activation block: %q", ctx)
	}
	if !strings.Contains(ctx, "Shipped: the auth fix.") {
		t.Errorf("merged context missing handoff content: %q", ctx)
	}
}

func TestPromptSubmitInBareContinueNoKeyword(t *testing.T) {
	// Regression guard for the early-return trap: a bare "continue" with no god
	// keyword must still inject the handoff, not passthrough.
	root := t.TempDir()
	home := t.TempDir()
	setHomeEnv(t, home)
	writeHandoff(t, root, "# Session Handoff\n\nShipped: the auth fix.", 1*time.Hour)

	payload, _ := json.Marshal(map[string]string{
		"prompt":     "continue",
		"session_id": "s-bare",
		"cwd":        root,
	})

	out := promptSubmitIn(payload)
	if out.HookSpecificOutput == nil {
		t.Fatal("expected handoff injection on bare 'continue', got passthrough")
	}
	if strings.Contains(out.HookSpecificOutput.AdditionalContext, "KRATOS KEYWORD DETECTED") {
		t.Error("bare 'continue' should not trigger keyword injection")
	}
	if !strings.Contains(out.HookSpecificOutput.AdditionalContext, "Shipped: the auth fix.") {
		t.Errorf("expected handoff content, got %q", out.HookSpecificOutput.AdditionalContext)
	}
}

func TestPromptSubmitInPassthroughWhenNeitherMatches(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setHomeEnv(t, home)
	// No handoff file at all.

	payload, _ := json.Marshal(map[string]string{
		"prompt":     "please fix the login bug",
		"session_id": "s-none",
		"cwd":        root,
	})

	out := promptSubmitIn(payload)
	if out.HookSpecificOutput != nil {
		t.Errorf("expected passthrough, got additionalContext %q", out.HookSpecificOutput.AdditionalContext)
	}
	if !out.Continue {
		t.Error("passthrough output must still set continue: true")
	}
}

func TestPromptSubmitInKratosPrefixSuppressesHandoff(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setHomeEnv(t, home)
	writeHandoff(t, root, "# Session Handoff\n\nShipped: the auth fix.", 1*time.Hour)

	payload, _ := json.Marshal(map[string]string{
		"prompt":     "/kratos:recall continue",
		"session_id": "s-prefix",
		"cwd":        root,
	})

	out := promptSubmitIn(payload)
	if out.HookSpecificOutput != nil {
		t.Errorf("/kratos: prefix should pass through untouched, got %q", out.HookSpecificOutput.AdditionalContext)
	}
}

func TestCapUTF8Bytes(t *testing.T) {
	t.Run("under limit returns unchanged", func(t *testing.T) {
		s := "hello world"
		got := capUTF8Bytes(s, 1024)
		if got != s {
			t.Errorf("expected unchanged string, got %q", got)
		}
	})

	t.Run("truncates without splitting a multi-byte rune", func(t *testing.T) {
		// Build a string whose byte 10 boundary lands mid-rune for a 3-byte char (e.g. "世" = E4 B8 96).
		s := strings.Repeat("a", 8) + "世界" + strings.Repeat("b", 20)
		got := capUTF8Bytes(s, 10) // cuts right inside the first 3-byte rune
		if !utf8.ValidString(got) {
			t.Fatalf("result is not valid UTF-8: %q (bytes: %v)", got, []byte(got))
		}
		// The partial rune must have been dropped, not left as invalid bytes.
		if strings.Contains(got, "�") {
			t.Errorf("result contains a replacement character, rune was split: %q", got)
		}
	})

	t.Run("adds truncation marker only when truncated", func(t *testing.T) {
		s := strings.Repeat("a", 20)
		got := capUTF8Bytes(s, 10)
		if !strings.HasSuffix(got, "... (truncated)") {
			t.Errorf("expected truncation marker, got %q", got)
		}
		untouched := capUTF8Bytes("short", 10)
		if strings.Contains(untouched, "truncated") {
			t.Errorf("should not add truncation marker when under limit, got %q", untouched)
		}
	})

	t.Run("8KB cap on a large multi-byte handoff", func(t *testing.T) {
		// Repeat a 3-byte rune enough times to exceed 8KB and force a mid-rune cut.
		s := strings.Repeat("世", 5000) // 15000 bytes
		got := capUTF8Bytes(s, handoffMaxBytes)
		if !utf8.ValidString(got) {
			t.Fatalf("result is not valid UTF-8")
		}
		if len(got) > handoffMaxBytes+len("\n... (truncated)") {
			t.Errorf("result exceeds cap + marker: %d bytes", len(got))
		}
	})
}

func TestHandoffMarkerPruning(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	dir := handoffMarkerDir()
	if dir == "" {
		t.Fatal("handoffMarkerDir returned empty with HOME set")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	oldPath := filepath.Join(dir, "old-session")
	freshPath := filepath.Join(dir, "fresh-session")
	if err := os.WriteFile(oldPath, []byte("123"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(freshPath, []byte("456"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldPath, old, old); err != nil {
		t.Fatal(err)
	}

	pruneHandoffMarkers(dir)

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("expected old marker to be pruned, err = %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Errorf("expected fresh marker to survive, err = %v", err)
	}
}
