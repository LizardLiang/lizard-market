package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// odysseusPayload builds a spawned-Odysseus PreToolUse payload — the shape the
// retired hooks/plan-mode-guard.cjs was built on.
func odysseusPayload(tool string, input map[string]any) string {
	return payloadJSON(map[string]any{
		"agent_type": "kratos:odysseus",
		"tool_name":  tool,
		"tool_input": input,
	})
}

func payloadJSON(m map[string]any) string {
	raw, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// irisLedger is a session ledger for an inline Iris with files already counted.
// The list is []any because that is what a JSON round trip produces.
func irisLedger(files ...string) map[string]any {
	list := make([]any, 0, len(files))
	for _, f := range files {
		list = append(list, normalizeLedgerPath(f))
	}
	return map[string]any{
		"session_id":          "sess-1",
		"cwd":                 "C:/repo",
		"inline_god":          "iris",
		"inline_edited_files": list,
	}
}

func irisWrite(file string) string {
	return payloadJSON(map[string]any{
		"session_id": "sess-1",
		"cwd":        "C:/repo",
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": file},
	})
}

// TestEditGateDecisions pins the whole gate policy: the 22 Odysseus cases
// ported from the retired plan-mode-guard.cjs (which only ever saw a *spawned*
// Odysseus), the five false denies that guard produced, Iris's per-turn file
// budget, and every fail-open path — including the regression that matters
// most, a spawned Ares under an exhausted Iris budget.
func TestEditGateDecisions(t *testing.T) {
	cases := []struct {
		name           string
		payload        string
		ledger         map[string]any
		want           string   // "allow", "deny", or "" for no decision (fail open)
		wantFiles      []string // expected inline_edited_files write, nil for no write
		wantWrite      bool
		reasonContains []string
	}{
		// ---- Odysseus writes (spawned — ported from hook_planguard_test.go) ----
		{
			name:    "draft plan write allowed",
			payload: odysseusPayload("Write", map[string]any{"file_path": ".claude/.Arena/tactical-plans/2026-07-28-thing.md"}),
			want:    "allow",
		},
		{
			name:    "per-answer edit of the draft allowed",
			payload: odysseusPayload("Edit", map[string]any{"file_path": "C:/repo/.claude/.Arena/tactical-plans/2026-07-28-thing.md"}),
			want:    "allow",
		},
		{
			name:    "spec delta write allowed",
			payload: odysseusPayload("Write", map[string]any{"file_path": ".claude/feature/2026-07-28-thing/spec-delta/planning.md"}),
			want:    "allow",
		},
		{
			// `kratos spec archive` moves promoted deltas into spec-delta/archived/.
			name:    "archived spec delta denied",
			payload: odysseusPayload("Write", map[string]any{"file_path": ".claude/feature/2026-07-28-thing/spec-delta/archived/planning.md"}),
			want:    "deny",
		},
		{
			name:    "other feature deliverables still denied",
			payload: odysseusPayload("Write", map[string]any{"file_path": ".claude/feature/2026-07-28-thing/prd.md"}),
			want:    "deny",
		},
		{
			name:           "spawned odysseus source write denied",
			payload:        odysseusPayload("Write", map[string]any{"file_path": "src/index.ts"}),
			want:           "deny",
			reasonContains: []string{"kratos:ares"},
		},
		// ---- Odysseus bash (ported) ----
		{
			// The protocol's own timestamp fallback shape must pass: the wrapper,
			// the stderr redirect and the `|| date` fallback are inert.
			name:    "protocol timestamp fallback allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `TS=$(kratos now 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)`}),
			want:    "allow",
		},
		{
			name:    "session active allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `"C:/Users/x/.kratos/bin/kratos.exe" session active "C:/repo"`}),
			want:    "allow",
		},
		{
			name:    "pipeline get allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "kratos pipeline get --compact --feature x"}),
			want:    "allow",
		},
		{
			name:    "memory list allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "~/.kratos/bin/kratos memory list --limit 40"}),
			want:    "allow",
		},
		{
			name:    "non-date fallback denied",
			payload: odysseusPayload("Bash", map[string]any{"command": `TS=$(kratos now || rm -rf build)`}),
			want:    "deny",
		},
		{
			name:    "stripped redirect still catches a chained mutation",
			payload: odysseusPayload("Bash", map[string]any{"command": "kratos now 2>/dev/null; rm -rf build"}),
			want:    "deny",
		},
		{
			// The mutation blacklist matches \bmove\b, so a task title containing
			// "move" would deny the slug mint if the kratos allowlist were checked
			// after it.
			name:    "slug mint with a mutating word in the title allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `kratos slug --dated "move the sidebar"`}),
			want:    "allow",
		},
		{
			name:    "quoted absolute binary path allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `"C:/Program Files/kratos/kratos.exe" spec validate my-slug`}),
			want:    "allow",
		},
		{
			name:    "template get allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "~/.kratos/bin/kratos template get spec-delta-template"}),
			want:    "allow",
		},
		{
			name:    "chained command after an allowed kratos prefix denied",
			payload: odysseusPayload("Bash", map[string]any{"command": `kratos slug -d "x" && rm -rf build`}),
			want:    "deny",
		},
		{
			name:    "command substitution after an allowed kratos prefix denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "kratos slug --dated \"$(rm -rf build)\""}),
			want:    "deny",
		},
		{
			name:    "timestamp subcommand allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "kratos now"}),
			want:    "allow",
		},
		{
			name:    "spec archive denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "kratos spec archive my-slug"}),
			want:    "deny",
		},
		{
			name:    "pipeline update denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "kratos pipeline update --feature x --stage 7 --status complete"}),
			want:    "deny",
		},
		{
			name:    "destructive shell denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "rm -rf build"}),
			want:    "deny",
		},
		{
			name:    "read-only git allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "git status"}),
			want:    "allow",
		},
		{
			name:    "non-odysseus agents unaffected",
			payload: payloadJSON(map[string]any{"agent_type": "kratos:ares", "tool_name": "Write", "tool_input": map[string]any{"file_path": "src/index.ts"}}),
			want:    "",
		},
		// ---- the five false denies the JS guard produced on 2026-09-11 ----
		{
			name:    "sed range read allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `sed -n '1,20p' main.go`}),
			want:    "allow",
		},
		{
			name:    "head allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "head -50 internal/cli/hook.go"}),
			want:    "allow",
		},
		{
			name:    "tail allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "tail -20 build.log"}),
			want:    "allow",
		},
		{
			name:    "wc allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "wc -l agents/iris.md"}),
			want:    "allow",
		},
		{
			name:    "git -C read-only allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "git -C C:/repo status"}),
			want:    "allow",
		},
		{
			// The agent protocol mandates these calls for every agent; the JS
			// guard denied Odysseus his own ledger write.
			name:    "step record-agent allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `kratos step record-agent "sess-1" odysseus sonnet "plan the gate" --project "C:/repo"`}),
			want:    "allow",
		},
		{
			// Widening the allowlist must not widen it to writes.
			name:    "sed in place denied",
			payload: odysseusPayload("Bash", map[string]any{"command": `sed -i 's/a/b/' main.go`}),
			want:    "deny",
		},
		{
			name:    "redirect after a read-only command denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "git status > out.txt"}),
			want:    "deny",
		},
		// ---- inline Odysseus (the ledger path the JS guard could never see) ----
		{
			name:    "inline odysseus source write denied",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Write", "tool_input": map[string]any{"file_path": "C:/repo/src/index.ts"}}),
			ledger:  map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:    "deny",
		},
		{
			name:    "inline odysseus plan write allowed",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Write", "tool_input": map[string]any{"file_path": "C:/repo/.claude/.Arena/tactical-plans/x.md"}}),
			ledger:  map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:    "allow",
		},
		// ---- Iris budget ----
		{
			name:      "first source file allowed and counted",
			payload:   irisWrite("C:/repo/src/a.ts"),
			ledger:    irisLedger(),
			want:      "",
			wantFiles: []string{"C:/repo/src/a.ts"},
			wantWrite: true,
		},
		{
			name:      "second source file allowed and counted",
			payload:   irisWrite("C:/repo/src/b.ts"),
			ledger:    irisLedger("C:/repo/src/a.ts"),
			want:      "",
			wantFiles: []string{"C:/repo/src/a.ts", "C:/repo/src/b.ts"},
			wantWrite: true,
		},
		{
			name:           "third distinct source file denied",
			payload:        irisWrite("C:/repo/src/c.ts"),
			ledger:         irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:           "deny",
			reasonContains: []string{`Task(subagent_type: "kratos:ares"`, "ORIGINAL_USER_REQUEST", "src/a.ts", "src/b.ts"},
		},
		{
			name:    "repeat edit of a counted file allowed",
			payload: irisWrite("C:/repo/src/a.ts"),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			name:    "document allowed and uncounted",
			payload: irisWrite("C:/repo/README.md"),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			name:    "claude bookkeeping allowed and uncounted",
			payload: irisWrite("C:/repo/.claude/feature/x/status.json"),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			name:    "scratchpad allowed and uncounted",
			payload: irisWrite("C:/repo/tmp/claude/scratch/probe.ts"),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			name:    "path outside cwd allowed and uncounted",
			payload: irisWrite("D:/elsewhere/src/d.ts"),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			name:    "bash never gated for iris",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Bash", "tool_input": map[string]any{"command": "rm -rf build"}}),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			name:    "cwd falls back to the ledger",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "tool_name": "Write", "tool_input": map[string]any{"file_path": "C:/repo/src/c.ts"}}),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "deny",
		},
		// ---- budget reset on dispatch ----
		{
			name:      "spawning ares clears the budget",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Agent", "tool_input": map[string]any{"subagent_type": "kratos:ares"}}),
			ledger:    irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:      "",
			wantFiles: []string{},
			wantWrite: true,
		},
		{
			name:      "spawning hades clears the budget",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Task", "tool_input": map[string]any{"subagent_type": "kratos:hades"}}),
			ledger:    irisLedger("C:/repo/src/a.ts"),
			want:      "",
			wantFiles: []string{},
			wantWrite: true,
		},
		{
			name:      "spawning hermes does not clear the budget",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Agent", "tool_input": map[string]any{"subagent_type": "kratos:hermes"}}),
			ledger:    irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:      "",
			wantWrite: false,
		},
		// ---- bypass ----
		{
			name:    "explicit user instruction stands the gate down",
			payload: irisWrite("C:/repo/src/c.ts"),
			ledger: map[string]any{
				"inline_god":          "iris",
				"cwd":                 "C:/repo",
				"gate_bypass":         true,
				"inline_edited_files": []any{"c:/repo/src/a.ts", "c:/repo/src/b.ts"},
			},
			want: "",
		},
		// ---- fail open ----
		{
			name:    "no ledger file",
			payload: irisWrite("C:/repo/src/c.ts"),
			ledger:  nil,
			want:    "",
		},
		{
			name:    "empty inline god",
			payload: irisWrite("C:/repo/src/c.ts"),
			ledger:  map[string]any{"session_id": "sess-1", "cwd": "C:/repo"},
			want:    "",
		},
		{
			name:    "god with no rule",
			payload: irisWrite("C:/repo/src/c.ts"),
			ledger:  map[string]any{"inline_god": "hermes", "cwd": "C:/repo", "inline_edited_files": []any{"a", "b"}},
			want:    "",
		},
		{
			name:    "missing session id",
			payload: payloadJSON(map[string]any{"cwd": "C:/repo", "tool_name": "Write", "tool_input": map[string]any{"file_path": "C:/repo/src/c.ts"}}),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			// The regression that matters most: Ares must never be denied, even
			// when the inline god's budget is spent. Without the agent_type
			// discriminator this gate would break the dispatch it exists to create.
			name: "spawned ares under an exhausted iris budget",
			payload: payloadJSON(map[string]any{
				"session_id": "sess-1", "cwd": "C:/repo", "agent_type": "kratos:ares",
				"tool_name": "Write", "tool_input": map[string]any{"file_path": "C:/repo/src/c.ts"},
			}),
			ledger: irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:   "",
		},
		{
			name: "spawned ares via subagent_type under an exhausted iris budget",
			payload: payloadJSON(map[string]any{
				"session_id": "sess-1", "cwd": "C:/repo", "subagent_type": "kratos:ares",
				"tool_name": "Write", "tool_input": map[string]any{"file_path": "C:/repo/src/c.ts"},
			}),
			ledger: irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var input preToolUseInput
			if err := json.Unmarshal([]byte(tc.payload), &input); err != nil {
				t.Fatalf("payload does not unmarshal: %v", err)
			}
			got := editGateDecision(input, tc.ledger)

			if got.Decision != tc.want {
				t.Fatalf("decision = %q, want %q (reason %q)", got.Decision, tc.want, got.Reason)
			}
			for _, want := range tc.reasonContains {
				if !strings.Contains(got.Reason, want) {
					t.Errorf("reason %q does not contain %q", got.Reason, want)
				}
			}
			if got.WriteFiles != tc.wantWrite {
				t.Fatalf("WriteFiles = %v, want %v", got.WriteFiles, tc.wantWrite)
			}
			if !tc.wantWrite {
				return
			}
			if len(got.Files) != len(tc.wantFiles) {
				t.Fatalf("files = %v, want %v", got.Files, tc.wantFiles)
			}
			for i, want := range tc.wantFiles {
				if got.Files[i] != normalizeLedgerPath(want) {
					t.Errorf("files[%d] = %q, want %q", i, got.Files[i], normalizeLedgerPath(want))
				}
			}
		})
	}
}

// TestEditGateWritesLedger covers the end-to-end hook: a counted edit appends
// to the ledger on disk, unknown keys survive, and a deny prints the reason.
func TestEditGateWritesLedger(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	sessionID := "sess-editgate"
	ledgerPath := filepath.Join(home, ".kratos", "sessions", sessionID+".json")
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := map[string]any{
		"session_id": sessionID,
		"project":    "repo",
		"cwd":        "C:/repo",
		"started_at": 1234567890,
		"source":     "startup",
		"inline_god": "iris",
	}
	raw, _ := json.Marshal(seed)
	if err := os.WriteFile(ledgerPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	write := func(file string) {
		handleEditGate([]byte(payloadJSON(map[string]any{
			"session_id": sessionID, "cwd": "C:/repo", "tool_name": "Write",
			"tool_input": map[string]any{"file_path": file},
		})))
	}

	write("C:/repo/src/a.ts")
	write("C:/repo/src/b.ts")
	write("C:/repo/src/a.ts") // repeat — still two files

	m, err := readInlineLedger(sessionID)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	files := ledgerStrings(m, ledgerKeyEditedFiles)
	if len(files) != 2 {
		t.Fatalf("inline_edited_files = %v, want 2 entries", files)
	}
	for _, key := range []string{"session_id", "project", "cwd", "started_at", "source", "inline_god"} {
		if _, ok := m[key]; !ok {
			t.Errorf("ledger lost key %q on write", key)
		}
	}

	// The third distinct file is denied, and the ledger is not touched.
	input := preToolUseInput{
		ToolName:  "Write",
		SessionID: sessionID,
		Cwd:       "C:/repo",
		ToolInput: preToolUseToolInput{FilePath: "C:/repo/src/c.ts"},
	}
	res := editGateDecision(input, m)
	if res.Decision != "deny" {
		t.Fatalf("third file decision = %q, want deny", res.Decision)
	}
	if res.WriteFiles {
		t.Error("a deny must not write the ledger")
	}
}

// TestHooksJSONRegistersEditGate pins the wiring: one gate, on the matcher that
// covers the writes, the shell and the dispatch reset — and no trace of the
// retired plan-mode guard, whose file this feature deletes.
func TestHooksJSONRegistersEditGate(t *testing.T) {
	hooksDir := filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks")
	data, err := os.ReadFile(filepath.Join(hooksDir, "hooks.json"))
	if err != nil {
		t.Fatalf("cannot read hooks.json: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "hook edit-gate") {
		t.Error("hooks.json does not register `hook edit-gate`")
	}
	if !strings.Contains(body, `"Write|Edit|MultiEdit|Bash|Agent|Task"`) {
		t.Error("edit-gate matcher must cover Write|Edit|MultiEdit|Bash|Agent|Task")
	}
	if strings.Contains(body, "plan-mode-guard") {
		t.Error("hooks.json still references the retired plan-mode-guard.cjs")
	}
	if _, err := os.Stat(filepath.Join(hooksDir, "plan-mode-guard.cjs")); err == nil {
		t.Error("plan-mode-guard.cjs still exists; the gate replaced it")
	}
}

// TestSessionStartPreservesLedgerKeys pins the compaction fix: registerSession
// rebuilt the state file from scratch, so a mid-session SessionStart ("compact"
// or "resume") erased inline_god and inline_edited_files and switched the gate
// off for the rest of the session.
func TestSessionStartPreservesLedgerKeys(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks", "session-start.cjs")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read session-start.cjs: %v", err)
	}
	if !strings.Contains(string(data), "...prev") {
		t.Error("registerSession must spread the previous state file, or it erases the edit gate's keys on compaction")
	}
}
