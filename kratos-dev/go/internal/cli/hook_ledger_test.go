package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testLedgerSession = "sess-ledger"

// seedLedger writes a session ledger and returns a reader for it.
func seedLedger(t *testing.T, m map[string]any) {
	t.Helper()
	path := sessionLedgerFile(testLedgerSession)
	if path == "" {
		t.Fatal("sessionLedgerFile returned empty path — HOME not redirected")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadLedger(t *testing.T) map[string]any {
	t.Helper()
	m, err := readInlineLedger(testLedgerSession)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	return m
}

// TestPromptSubmitRecordsInlineGod pins the only writer of inline_god and
// gate_bypass.
//
// Claude Code 2.1.268 delivers the literal `/kratos:iris …` text to
// UserPromptSubmit, not the expanded launcher body (verified with a payload
// probe on 2026-09-11), so both shapes have to name the god.
func TestPromptSubmitRecordsInlineGod(t *testing.T) {
	const oldStamp = "2020-01-01T00:00:00Z"
	irisBody := "KRATOS_ROOT=/plugins/kratos\n" +
		`node "/plugins/kratos/hooks/launch.cjs" agent load iris --resolve --part body` + "\n" +
		"Running **inline** is what lets AskUserQuestion reach the user. " +
		"If you want, do it yourself."

	t.Run("launcher body records the god", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"session_id": testLedgerSession, "cwd": "C:/repo", "started_at": 42, "custom_key": "keep me"})

		recordInlineGod(testLedgerSession, irisBody)

		m := mustReadLedger(t)
		if got := ledgerString(m, ledgerKeyInlineGod); got != "iris" {
			t.Errorf("inline_god = %q, want iris", got)
		}
		if ledgerString(m, ledgerKeyInlineSince) == "" {
			t.Error("inline_god_since not set")
		}
		for _, key := range []string{"session_id", "cwd", "started_at", "custom_key"} {
			if _, ok := m[key]; !ok {
				t.Errorf("ledger lost key %q", key)
			}
		}
	})

	t.Run("launcher body never grants a bypass", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris", "gate_bypass": false,
			"inline_edited_files": []any{"a.ts", "b.ts"}, "inline_god_since": oldStamp})

		// The body contains "do it yourself" in its own prose — evaluating the
		// bypass pattern against it would grant a permanent stand-down.
		recordInlineGod(testLedgerSession, irisBody)

		m := mustReadLedger(t)
		if ledgerBool(m, ledgerKeyGateBypass) {
			t.Error("gate_bypass set from a launcher body")
		}
		if got := ledgerString(m, ledgerKeyInlineSince); got != oldStamp {
			t.Errorf("inline_god_since = %q, want it unchanged for the same god", got)
		}
		if got := ledgerStrings(m, ledgerKeyEditedFiles); len(got) != 2 {
			t.Errorf("relaunching the same god refilled the budget: %v", got)
		}
	})

	t.Run("a different god resets the budget", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris", "inline_god_since": oldStamp,
			"inline_edited_files": []any{"a.ts", "b.ts"}})

		recordInlineGod(testLedgerSession, "node launch.cjs agent load odysseus --resolve --part body")

		m := mustReadLedger(t)
		if got := ledgerString(m, ledgerKeyInlineGod); got != "odysseus" {
			t.Errorf("inline_god = %q, want odysseus", got)
		}
		if got := ledgerString(m, ledgerKeyInlineSince); got == oldStamp {
			t.Error("inline_god_since not refreshed on a god change")
		}
		if got := ledgerStrings(m, ledgerKeyEditedFiles); len(got) != 0 {
			t.Errorf("inline_edited_files = %v, want empty on a god change", got)
		}
	})

	t.Run("slash command records the god", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"session_id": testLedgerSession})

		recordInlineGod(testLedgerSession, "/kratos:iris fix the failing build")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "iris" {
			t.Errorf("inline_god = %q, want iris", got)
		}
	})

	t.Run("plan command records odysseus", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"session_id": testLedgerSession})

		recordInlineGod(testLedgerSession, "/kratos:plan move the sidebar")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "odysseus" {
			t.Errorf("inline_god = %q, want odysseus", got)
		}
	})

	t.Run("non-god command leaves the god alone", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris"})

		recordInlineGod(testLedgerSession, "/kratos:status")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "iris" {
			t.Errorf("inline_god = %q, want iris (an unknown command must not switch gods)", got)
		}
	})

	t.Run("a plain user prompt refills the budget", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris", "inline_edited_files": []any{"a.ts", "b.ts"}, "gate_bypass": true})

		recordInlineGod(testLedgerSession, "now fix the other thing too")

		m := mustReadLedger(t)
		if got := ledgerStrings(m, ledgerKeyEditedFiles); len(got) != 0 {
			t.Errorf("inline_edited_files = %v, want empty on a new turn", got)
		}
		if ledgerBool(m, ledgerKeyGateBypass) {
			t.Error("gate_bypass survived a turn that did not ask for it")
		}
	})

	t.Run("explicit instruction sets the bypass", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris"})

		recordInlineGod(testLedgerSession, "you do the html part")

		if !ledgerBool(mustReadLedger(t), ledgerKeyGateBypass) {
			t.Error("gate_bypass not set from an explicit instruction")
		}
	})

	t.Run("prose about the gate does not set the bypass", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris"})

		recordInlineGod(testLedgerSession, "the inline gate is broken, look into it")

		if ledgerBool(mustReadLedger(t), ledgerKeyGateBypass) {
			t.Error("a bare mention of 'inline' granted a bypass")
		}
	})

	t.Run("task notification is not a user turn", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris", "inline_edited_files": []any{"a.ts", "b.ts"}, "gate_bypass": true})

		recordInlineGod(testLedgerSession, "<task-notification>\n<task-id>abc</task-id>\nyou do not touch this\n</task-notification>")

		m := mustReadLedger(t)
		if got := ledgerStrings(m, ledgerKeyEditedFiles); len(got) != 2 {
			t.Errorf("inline_edited_files = %v, want unchanged by a harness notification", got)
		}
		if !ledgerBool(m, ledgerKeyGateBypass) {
			t.Error("a harness notification cleared the user's bypass")
		}
	})

	t.Run("missing ledger file is created", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())

		recordInlineGod(testLedgerSession, "/kratos:iris do the thing")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "iris" {
			t.Errorf("inline_god = %q, want iris", got)
		}
	})

	t.Run("no session id is a no-op", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		recordInlineGod("", "/kratos:iris do the thing")
		// Nothing to assert beyond not panicking and writing no file.
		if entries, err := os.ReadDir(filepath.Join(t.TempDir(), ".kratos", "sessions")); err == nil && len(entries) > 0 {
			t.Errorf("wrote %d ledger files without a session id", len(entries))
		}
	})
}

// TestInlineGodFromPrompt pins the prompt → god mapping on its own.
func TestInlineGodFromPrompt(t *testing.T) {
	cases := []struct {
		prompt string
		want   string
	}{
		{"node launch.cjs agent load iris --resolve --part body", "iris"},
		{"node launch.cjs agent load odysseus --resolve --part extras", "odysseus"},
		{"/kratos:iris check the build", "iris"},
		{"/kratos:odysseus plan the gate", "odysseus"},
		{"/kratos:plan the gate", "odysseus"},
		{"/kratos:status", ""},
		{"/kratos:main build a thing", ""},
		{"just fix the bug", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := inlineGodFromPrompt(tc.prompt); got != tc.want {
			t.Errorf("inlineGodFromPrompt(%q) = %q, want %q", tc.prompt, got, tc.want)
		}
	}
}
