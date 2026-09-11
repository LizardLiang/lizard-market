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

	t.Run("slash command preserves unknown ledger keys", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"session_id": testLedgerSession, "cwd": "C:/repo", "started_at": 42, "custom_key": "keep me"})

		recordInlineGod(testLedgerSession, "C:/repo", "/kratos:iris fix the build")

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

	t.Run("a quoted launcher line never binds the god", func(t *testing.T) {
		// Measured dead as a payload shape (Claude Code 2.1.268 sends the
		// literal slash command) and live as an injection vector: any prompt
		// quoting `agent load <god> --resolve` — a review of this very file —
		// rebound the session's inline god.
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"session_id": testLedgerSession, "inline_god": "iris"})

		recordInlineGod(testLedgerSession, "C:/repo", "why does `agent load odysseus --resolve --part body` print nothing?")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "iris" {
			t.Errorf("inline_god = %q, want iris (a quoted launcher line must not rebind the god)", got)
		}
	})

	t.Run("launcher body never grants a bypass", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris", "gate_bypass": false,
			"inline_edited_files": []any{"a.ts", "b.ts"}, "inline_god_since": oldStamp})

		// The body contains "do it yourself" in its own prose — evaluating the
		// bypass pattern against it would grant a permanent stand-down.
		recordInlineGod(testLedgerSession, "C:/repo", irisBody)

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

		recordInlineGod(testLedgerSession, "C:/repo", "/kratos:odysseus plan the gate")

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

		recordInlineGod(testLedgerSession, "C:/repo", "/kratos:iris fix the failing build")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "iris" {
			t.Errorf("inline_god = %q, want iris", got)
		}
	})

	t.Run("plan command records odysseus", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"session_id": testLedgerSession})

		recordInlineGod(testLedgerSession, "C:/repo", "/kratos:plan move the sidebar")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "odysseus" {
			t.Errorf("inline_god = %q, want odysseus", got)
		}
	})

	t.Run("non-god command leaves the god alone", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris"})

		recordInlineGod(testLedgerSession, "C:/repo", "/kratos:status")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "iris" {
			t.Errorf("inline_god = %q, want iris (an unknown command must not switch gods)", got)
		}
	})

	t.Run("a plain user prompt refills the budget", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris", "inline_edited_files": []any{"a.ts", "b.ts"}, "gate_bypass": true})

		recordInlineGod(testLedgerSession, "C:/repo", "now fix the other thing too")

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

		recordInlineGod(testLedgerSession, "C:/repo", "you do the html part")

		if !ledgerBool(mustReadLedger(t), ledgerKeyGateBypass) {
			t.Error("gate_bypass not set from an explicit instruction")
		}
	})

	t.Run("prose about the gate does not set the bypass", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris"})

		recordInlineGod(testLedgerSession, "C:/repo", "the inline gate is broken, look into it")

		if ledgerBool(mustReadLedger(t), ledgerKeyGateBypass) {
			t.Error("a bare mention of 'inline' granted a bypass")
		}
	})

	t.Run("task notification is not a user turn", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris", "inline_edited_files": []any{"a.ts", "b.ts"}, "gate_bypass": true})

		recordInlineGod(testLedgerSession, "C:/repo", "<task-notification>\n<task-id>abc</task-id>\nyou do not touch this\n</task-notification>")

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

		recordInlineGod(testLedgerSession, "C:/repo", "/kratos:iris do the thing")

		if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "iris" {
			t.Errorf("inline_god = %q, want iris", got)
		}
	})

	t.Run("no session id is a no-op", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		recordInlineGod("", "C:/repo", "/kratos:iris do the thing")
		// Nothing to assert beyond not panicking and writing no file.
		if entries, err := os.ReadDir(filepath.Join(t.TempDir(), ".kratos", "sessions")); err == nil && len(entries) > 0 {
			t.Errorf("wrote %d ledger files without a session id", len(entries))
		}
	})

	t.Run("a fabricated ledger carries the caller's context", func(t *testing.T) {
		// A stub with only session_id dropped the cwd the gate's project-root
		// test falls back to, and the fields the session notice reads.
		setHomeEnv(t, t.TempDir())

		recordInlineGod(testLedgerSession, "C:/repo/sub", "/kratos:iris do the thing")

		m := mustReadLedger(t)
		if got := ledgerString(m, "cwd"); got != "C:/repo/sub" {
			t.Errorf("cwd = %q, want C:/repo/sub", got)
		}
		if got := ledgerString(m, "project"); got != "sub" {
			t.Errorf("project = %q, want sub", got)
		}
		if _, ok := m["started_at"]; !ok {
			t.Error("started_at missing from the fabricated ledger")
		}
		if got := ledgerString(m, "source"); got == "" {
			t.Error("source missing from the fabricated ledger")
		}
	})

	t.Run("an unchanged ledger is not rewritten", func(t *testing.T) {
		setHomeEnv(t, t.TempDir())
		path := sessionLedgerFile(testLedgerSession)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		// Compact on purpose: a rewrite would re-indent it.
		const compact = `{"inline_god":"iris","inline_edited_files":[],"gate_bypass":false}`
		if err := os.WriteFile(path, []byte(compact), 0o644); err != nil {
			t.Fatal(err)
		}

		recordInlineGod(testLedgerSession, "C:/repo", "keep going")

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != compact {
			t.Errorf("ledger rewritten with no change:\n%s", data)
		}
	})

	t.Run("a prompt opening with an angle bracket is a user turn", func(t *testing.T) {
		// Only <task-notification> is a harness event. Treating every "<" as
		// one let a user's own XML snippet keep the previous turn's spent
		// budget and bypass.
		setHomeEnv(t, t.TempDir())
		seedLedger(t, map[string]any{"inline_god": "iris", "inline_edited_files": []any{"a.ts", "b.ts"}, "gate_bypass": true})

		recordInlineGod(testLedgerSession, "C:/repo", "<br> renders wrong in the header, fix it")

		m := mustReadLedger(t)
		if got := ledgerStrings(m, ledgerKeyEditedFiles); len(got) != 0 {
			t.Errorf("inline_edited_files = %v, want empty on a user turn", got)
		}
		if ledgerBool(m, ledgerKeyGateBypass) {
			t.Error("gate_bypass survived a user turn that did not ask for it")
		}
	})
}

// TestGateBypassPhrases pins the stand-down regex. A bare \byou do\b matched
// ordinary questions and switched the gate off for the turn; every phrase below
// is either an instruction (bypass) or prose about one (no bypass).
func TestGateBypassPhrases(t *testing.T) {
	cases := []struct {
		prompt string
		want   bool
	}{
		{"you do the html part", true},
		{"You do it, faster than explaining", true},
		{"just do it yourself", true},
		{"do this inline please", true},
		{"inline it, it is two lines", true},
		{"can you do a quick review?", false},
		{"why did you do that?", false},
		{"how do you do the release here?", false},
		{"the way you do these reviews is fine, keep it", false},
		{"the inline gate is broken, look into it", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := gateBypassRE.MatchString(tc.prompt); got != tc.want {
			t.Errorf("gateBypassRE.MatchString(%q) = %v, want %v", tc.prompt, got, tc.want)
		}
	}
}

// TestInlineGodFromPrompt pins the prompt → god mapping on its own.
func TestInlineGodFromPrompt(t *testing.T) {
	cases := []struct {
		prompt string
		want   string
	}{
		// A launcher line, quoted or expanded, names no god: only the typed
		// slash command binds one.
		{"node launch.cjs agent load iris --resolve --part body", ""},
		{"node launch.cjs agent load odysseus --resolve --part extras", ""},
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
