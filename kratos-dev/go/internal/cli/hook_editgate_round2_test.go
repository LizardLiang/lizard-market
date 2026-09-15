package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gateShellCase is one command and the verdict the Odysseus Bash rule must
// reach for it.
type gateShellCase struct {
	name string
	cmd  string
}

// gateDenyCommands is the bypass table from the 2026-09-11 round-2 review:
// every shape that reached the shell while the classifier called the line a
// read. Three of them were live holes in the shipped gate (process
// substitution, backslash-escaped quotes, unspaced redirects); the rest are the
// neighbors of those holes, kept here so narrowing the classifier again cannot
// quietly re-open one.
var gateDenyCommands = []gateShellCase{
	// ---- B1: process substitution, invisible to a `$(`-only pattern ----
	{"process substitution into cat", `cat <(rm -rf build)`},
	{"process substitution into grep", `grep foo <(rm -rf build)`},
	{"process substitution into sed", `sed -n 1p <(rm -rf build)`},
	{"process substitution into git log", `git log --format=%h <(rm -rf b)`},
	{"output process substitution", `cat notes.md > >(rm -rf build)`},
	// ---- B2: a backslash-escaped quote is not a delimiter ----
	{"escaped quotes hide a semicolon", `ls \"a ; rm -rf build \"z`},
	{"escaped quotes hide a pipe", `ls \"a | rm -rf build \"z`},
	{"escaped quotes then a real separator", `cat \"x\" ; rm -rf build`},
	{"unbalanced quote before a separator", `ls "unbalanced ; rm -rf build`},
	// ---- B3: the shell needs no space around a redirect ----
	{"unspaced redirect", `ls>out.txt`},
	{"unspaced append", `cat foo>>bar`},
	{"unspaced redirect after git", `git status>out.txt`},
	{"explicit stdout descriptor", `ls 1>out.txt`},
	{"unspaced powershell redirect", `Get-Content x>y`},
	{"spaced redirect", `git status > out.txt`},
	// ---- substitution and separators ----
	{"dollar substitution", `cat $(rm -rf build)`},
	{"quoted dollar substitution", `cat "$(rm -rf build)"`},
	{"backtick substitution", "cat `rm -rf build`"},
	{"here-doc body", "cat <<EOF\nrm -rf build\nEOF"},
	{"pipe both streams into a mutation", `grep -rn x . |& rm -rf build`},
	{"subshell parentheses", `(rm -rf build)`},
	{"subshell after a reader", `cat f; (rm -rf build)`},
	{"newline after a kratos call", "kratos slug x\nrm -rf build"},
	{"build chained after a reader", `git status && go build -o out ./...`},
	{"nested quotes then a separator", `cat "nested 'quotes' ok" ; rm -rf build`},
	// ---- W1: most of the git branch surface mutates ----
	{"git branch delete", `git branch -D main`},
	{"git branch lowercase delete", `git branch -d feature/old`},
	{"git branch rename", `git branch -m old new`},
	{"git branch force", `git branch -f main HEAD~3`},
	{"git branch create", `git branch new-feature`},
	// ---- W6: find writes files without a shell redirect ----
	{"find -fprintf", `find . -name "*.go" -fprintf out.txt "%p"`},
	{"find -fls", `find . -fls listing.txt`},
	{"find -fprint", `find . -fprint listing.txt`},
	{"find -delete", `find . -name "*.tmp" -delete`},
	{"sed in place", `sed -i 's/a/b/' main.go`},
	{"sed -n combined with -i", `sed -n -i 's/a/b/p' f`},
	{"git -C mutating verb", `git -C /repo checkout main`},
	{"command without -v runs the program", `command rm -rf build`},
	{"powershell remove", `Remove-Item -Recurse build`},
	{"echo into a file", `echo "a" > x`},
}

// gateLegitCommands is the other half of the round-2 table: commands Odysseus
// actually runs while planning. A classifier narrowed to close the table above
// must still leave every one of these alone — a gate that denies the agent's
// own inspection commands teaches the agent to route around it.
var gateLegitCommands = []gateShellCase{
	{"git status", `git status`},
	{"git status short", `git status --short`},
	{"git diff", `git diff`},
	{"git diff a revision", `git diff HEAD~1`},
	{"git show", `git show HEAD`},
	{"git log oneline", `git log --oneline -20`},
	{"git log grep", `git log --grep "rm the old path"`},
	{"git rev-parse", `git rev-parse HEAD`},
	{"git ls-files", `git ls-files`},
	{"git -C status", `git -C C:/repo status`},
	{"git branch bare", `git branch`},
	{"git branch all", `git branch -a`},
	{"git branch show-current", `git branch --show-current`},
	{"git branch remotes", `git branch --remotes`},
	{"ls", `ls`},
	{"ls long", `ls -la`},
	{"pwd", `pwd`},
	{"cat a source file", `cat main.go`},
	{"cat a plan whose name says move", `cat .claude/.Arena/tactical-plans/2026-09-11-move-sidebar.md`},
	{"cat a file named mv.ts", `cat src/mv.ts`},
	{"head", `head -50 internal/cli/hook.go`},
	{"tail", `tail -20 build.log`},
	{"tail -n", `tail -n 20 build.log`},
	{"wc", `wc -l agents/iris.md`},
	{"which", `which codex`},
	{"command -v", `command -v codex`},
	{"stat", `stat go.mod`},
	{"file", `file bin/kratos`},
	{"sed range read", `sed -n '1,20p' main.go`},
	{"sed quiet read", `sed --quiet '1,20p' main.go`},
	{"grep recursive", `grep -rn "gate" .`},
	{"grep a pattern with a pipe", `grep -rn "a|b" src/`},
	{"grep for a mutation word", `grep -rn "move" src/`},
	{"grep with escaped quotes in the pattern", `grep -rn "he said \"hi\"" src/`},
	{"rg files", `rg --files`},
	{"find by name", `find . -name "*.go"`},
	{"find by type", `find . -type f -name "*.md"`},
	{"powershell list", `Get-ChildItem -Recurse`},
	{"powershell read", `Get-Content agents/iris.md`},
	{"powershell location", `Get-Location`},
	{"powershell search", `Select-String -Pattern gate -Path *.go`},
	{"powershell test path", `Test-Path C:/repo/go.mod`},
	{"two readers chained", `git status && ls -la`},
	{"reader piped into a reader", `grep -rn "gate" . | head -20`},
	{"single-quoted separator is data", `cat 'single quoted ; not a separator'`},
	{"two readers on two lines", "git status\ngit diff"},
	// ---- W7: stderr plumbing writes nothing ----
	{"merge stderr", `ls 2>&1`},
	{"merge stderr then pipe", `git status 2>&1 | head -20`},
	{"stdout to stderr", `cat f 1>&2`},
	{"discard stderr", `cat notes.md 2>/dev/null`},
	{"discard stderr then pipe", `grep -rn gate . 2>/dev/null | head -5`},
	// ---- read-only kratos subcommands ----
	{"kratos now", `kratos now`},
	{"protocol timestamp fallback", `TS=$(kratos now 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)`},
	{"kratos slug with a mutating title", `kratos slug --dated "move the sidebar"`},
	{"kratos pipeline get", `kratos pipeline get --compact --feature x`},
	{"kratos template get", `kratos template get spec-delta-template`},
	{"kratos record-agent with an ampersand", `kratos step record-agent "sess-1" odysseus sonnet "auth & billing split" --project "C:/repo"`},
	{"quoted absolute binary path", `"C:/Program Files/kratos/kratos.exe" spec validate my-slug`},
}

// TestOdysseusBashClassification runs the round-2 bypass table against the
// production rule. It is the regression the review's own rule proposal asks
// for: narrowing a classifier must carry a test per previously-denied input,
// and widening one must carry the reads it was widened for.
func TestOdysseusBashClassification(t *testing.T) {
	t.Run("denied", func(t *testing.T) {
		for _, tc := range gateDenyCommands {
			t.Run(tc.name, func(t *testing.T) {
				if got := odysseusBashDecision(tc.cmd); got.Decision != "deny" {
					t.Errorf("command %q: decision = %q, want deny", tc.cmd, got.Decision)
				}
			})
		}
	})
	t.Run("legitimate", func(t *testing.T) {
		for _, tc := range gateLegitCommands {
			t.Run(tc.name, func(t *testing.T) {
				got := odysseusBashDecision(tc.cmd)
				if got.Decision != "" {
					t.Errorf("command %q: decision = %q, want none (%s)", tc.cmd, got.Decision, got.Reason)
				}
				// A permitted command must be permitted *because the
				// classifier read it*, not because some earlier branch fell
				// open. "No decision" alone cannot tell the two apart, now
				// that a permitted command no longer answers "allow".
				if !isReadOnlyKratosCommand(tc.cmd) && !isReadOnlyShellCommand(tc.cmd) {
					t.Errorf("command %q is not classified read-only by either rule", tc.cmd)
				}
			})
		}
	})
}

// odysseusBashDecision runs one Bash command through the spawned-Odysseus path.
func odysseusBashDecision(command string) editGateResult {
	var input preToolUseInput
	if err := json.Unmarshal([]byte(odysseusPayload("Bash", map[string]any{"command": command})), &input); err != nil {
		panic(err)
	}
	return editGateDecision(input, nil)
}

// TestGateNeverAllows is the contract, as a property over every input the gate
// is tested on: the decision is "deny" or nothing, never "allow".
//
// This is the whole safety margin of a shell classifier built from regular
// expressions. Under an explicit "allow" each of the three bypasses found in
// round 2 executed `rm -rf` with the user's permission prompt skipped; with no
// "allow" the next miss — and there will be one — degrades to the prompt the
// user would have seen anyway.
func TestGateNeverAllows(t *testing.T) {
	for _, tc := range append(append([]gateShellCase{}, gateDenyCommands...), gateLegitCommands...) {
		if got := odysseusBashDecision(tc.cmd); got.Decision == "allow" {
			t.Errorf("Bash %q returned an explicit allow", tc.cmd)
		}
	}

	// The write side, over both gated gods and both payload shapes.
	paths := []string{
		".claude/.Arena/tactical-plans/2026-07-28-thing.md",
		"C:/repo/.claude/.Arena/tactical-plans/x.md",
		".claude/feature/2026-07-28-thing/spec-delta/planning.md",
		"C:/repo/src/index.ts",
		"C:/repo/README.md",
		"C:/repo/.claude/feature/x/status.json",
	}
	ledgers := []map[string]any{
		nil,
		{"inline_god": "odysseus", "cwd": "C:/repo"},
		irisLedger(),
		irisLedger("C:/repo/src/a.ts", "C:/repo/src/b.ts"),
	}
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit", "Bash", "Agent", "Task"} {
		for _, p := range paths {
			raws := []string{
				odysseusPayload(tool, map[string]any{"file_path": p}),
				payloadJSON(map[string]any{
					"session_id": "sess-1", "cwd": "C:/repo", "tool_name": tool,
					"tool_input": map[string]any{"file_path": p},
				}),
			}
			for _, ledger := range ledgers {
				for _, raw := range raws {
					var input preToolUseInput
					if err := json.Unmarshal([]byte(raw), &input); err != nil {
						t.Fatal(err)
					}
					if got := editGateDecision(input, ledger); got.Decision == "allow" {
						t.Errorf("%s on %q returned an explicit allow", tool, p)
					}
				}
			}
		}
	}
}

// TestOdysseusGateFallsBackToLedgerCwd pins W5: with an empty payload cwd the
// containment test had no root, so any absolute path whose text contained
// ".claude/.Arena/tactical-plans/" passed the plan rule. The ledger's own cwd
// is the fallback, exactly as irisGate already did.
func TestOdysseusGateFallsBackToLedgerCwd(t *testing.T) {
	ledger := map[string]any{"inline_god": "odysseus", "cwd": "C:/repo"}
	payload := payloadJSON(map[string]any{
		"session_id": "sess-1", "tool_name": "Write",
		"tool_input": map[string]any{"file_path": "D:/elsewhere/.claude/.Arena/tactical-plans/x.md"},
	})
	var input preToolUseInput
	if err := json.Unmarshal([]byte(payload), &input); err != nil {
		t.Fatal(err)
	}
	if got := editGateDecision(input, ledger); got.Decision != "deny" {
		t.Errorf("a plan path outside the ledger cwd = %q, want deny", got.Decision)
	}

	// The same path under the ledger's cwd is the planner's own work.
	under := payloadJSON(map[string]any{
		"session_id": "sess-1", "tool_name": "Write",
		"tool_input": map[string]any{"file_path": "C:/repo/.claude/.Arena/tactical-plans/x.md"},
	})
	if err := json.Unmarshal([]byte(under), &input); err != nil {
		t.Fatal(err)
	}
	if got := editGateDecision(input, ledger); got.Decision != "" {
		t.Errorf("a plan path under the ledger cwd = %q, want no decision", got.Decision)
	}
}

// TestGateNeedsLedger pins the hot-path shortcut: a spawned subagent that is
// not Odysseus is decided by the payload alone, and every Ares edit passes
// through this hook, so it must not pay for a ledger read it cannot use.
func TestGateNeedsLedger(t *testing.T) {
	cases := []struct {
		name  string
		input preToolUseInput
		want  bool
	}{
		{"main context", preToolUseInput{SessionID: "sess-1"}, true},
		{"no session id", preToolUseInput{}, false},
		{"spawned ares", preToolUseInput{SessionID: "sess-1", AgentType: "kratos:ares"}, false},
		{"spawned general-purpose by id", preToolUseInput{SessionID: "sess-1", AgentID: "agent-7"}, false},
		{"spawned odysseus", preToolUseInput{SessionID: "sess-1", AgentType: "kratos:odysseus"}, true},
	}
	for _, tc := range cases {
		if got := gateNeedsLedger(tc.input); got != tc.want {
			t.Errorf("%s: gateNeedsLedger = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestAresSpawnTemplateMatchesQuickMd pins the duplicate. irisDenyReason
// carries the spawn template verbatim so the model can act on the deny without
// opening commands/quick.md — which means the two copies can drift, and a deny
// message teaching a template the orchestrator no longer uses is worse than no
// template at all. Same shape as TestDocsPinIrisFileBudget.
func TestAresSpawnTemplateMatchesQuickMd(t *testing.T) {
	quickPath := filepath.Join("..", "..", "..", "..", "plugins", "kratos", "commands", "quick.md")
	data, err := os.ReadFile(quickPath)
	if err != nil {
		t.Fatalf("cannot read quick.md: %v", err)
	}
	quick := string(data)
	reason := irisDenyReason([]string{"C:/repo/src/a.ts"}, "C:/repo")

	for _, key := range []string{
		"subagent_type:",
		`mode: "acceptEdits"`,
		"MISSION:",
		"TARGET:",
		"REQUIREMENTS:",
		"ORIGINAL_USER_REQUEST:",
		"TICKET:",
		"INTENTION protocol",
		"No PRD or tech spec needed",
	} {
		if !strings.Contains(reason, key) {
			t.Errorf("the deny template no longer carries %q", key)
		}
		if !strings.Contains(quick, key) {
			t.Errorf("commands/quick.md no longer carries %q; the deny template still teaches it", key)
		}
	}
	if !strings.Contains(reason, `"kratos:ares"`) {
		t.Error("the deny template must name kratos:ares as the spawn target")
	}
}

// TestDenyReasonCannotStandTheGateDown pins the loop the anchored bypass regex
// closes: the deny message tells the user how to stand the gate down, so its
// own wording must not *be* a stand-down phrase when it is pasted back.
func TestDenyReasonCannotStandTheGateDown(t *testing.T) {
	reason := irisDenyReason([]string{"C:/repo/src/a.ts", "C:/repo/src/b.ts"}, "C:/repo")
	if gateBypassRE.MatchString(reason) {
		t.Error("pasting the deny message into the prompt stands the gate down; re-wrap the closing sentence so no stand-down phrase opens a line")
	}
	if gateBypassRE.MatchString(odysseusWriteDenyReason) || gateBypassRE.MatchString(odysseusBashDenyReason) {
		t.Error("an Odysseus deny message stands the gate down when pasted back")
	}
}

// TestDocsCannotStandTheGateDown extends the same rule to the shipped prose:
// README.md and hooks/README.md both quote the stand-down phrases, and a user
// pasting a paragraph of documentation must not disable the gate.
func TestDocsCannotStandTheGateDown(t *testing.T) {
	pluginDir := filepath.Join("..", "..", "..", "..", "plugins", "kratos")
	for _, rel := range []string{
		"README.md",
		filepath.Join("hooks", "README.md"),
		filepath.Join("agents", "iris.md"),
		filepath.Join("agents", "odysseus.md"),
	} {
		path := filepath.Join(pluginDir, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("cannot read %s: %v", rel, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if gateBypassRE.MatchString(line) {
				t.Errorf("%s:%d opens with a stand-down phrase, so pasting it disables the gate: %q", rel, i+1, line)
			}
		}
	}
}

// TestSessionLedgerFileRejectsUnsafeIDs pins W10: session_id arrives from a
// hook payload and used to reach filepath.Join unchecked, so "../../evil" read
// and rewrote a file outside ~/.kratos/sessions. A rejected id yields no path,
// which every caller treats as fail-open.
func TestSessionLedgerFileRejectsUnsafeIDs(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	for _, id := range []string{
		"../../evil",
		`..\..\evil`,
		"../evil",
		"..",
		"sub/dir",
		`sub\dir`,
		"C:/abs/path",
		"/etc/passwd",
		"has space",
		"semi;colon",
		"dot.dot",
		"",
	} {
		if got := sessionLedgerFile(id); got != "" {
			t.Errorf("sessionLedgerFile(%q) = %q, want no path", id, got)
		}
		if _, err := readInlineLedger(id); err == nil {
			t.Errorf("readInlineLedger(%q) succeeded", id)
		}
		if err := writeInlineLedger(id, map[string]any{"a": 1}); err == nil {
			t.Errorf("writeInlineLedger(%q) succeeded", id)
		}
	}
	for _, id := range []string{"sess-1", "route-test", "a1b2c3", "A_B-1", "01234567-89ab-cdef-0123-456789abcdef"} {
		if got := sessionLedgerFile(id); got == "" {
			t.Errorf("sessionLedgerFile(%q) rejected a real session id", id)
		}
	}
}

// TestAtomicWriteRemovesStaleTempSiblings pins W11: a killed writer leaves a
// randomly named ".<name>-*.tmp" beside the target, and nothing ever reused or
// replaced it, so .claude/feature/<name>/ collected orphans without bound.
func TestAtomicWriteRemovesStaleTempSiblings(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "status.json")

	stale := filepath.Join(dir, ".status.json-123456.tmp")
	fresh := filepath.Join(dir, ".status.json-999999.tmp")
	otherTarget := filepath.Join(dir, ".prd.md-123456.tmp")
	notTemp := filepath.Join(dir, "status.json.bak")
	for _, p := range []string{stale, fresh, otherTarget, notTemp} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * time.Hour)
	for _, p := range []string{stale, otherTarget} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}

	if err := atomicWriteFile(dest, []byte("{}\n")); err != nil {
		t.Fatalf("atomicWriteFile: %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("the stale temp sibling survived the write")
	}
	for _, p := range []string{fresh, otherTarget, notTemp} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed; only a stale sibling of this target may go", filepath.Base(p))
		}
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("the write did not land: %v", err)
	}
}

// TestSessionStartWritesLedgerAtomically pins W9: registerSession replaced the
// ledger with a plain writeFileSync, which truncates first — a Go edit-gate
// process reading in that window saw invalid JSON and lost the gate for that
// call.
func TestSessionStartWritesLedgerAtomically(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(hooksDirPath(), "session-start.cjs"))
	if err != nil {
		t.Fatalf("cannot read session-start.cjs: %v", err)
	}
	body := string(data)
	for _, want := range []string{"function writeStateFile", "renameSync"} {
		if !strings.Contains(body, want) {
			t.Errorf("session-start.cjs must replace the ledger through a temp file and a rename (missing %q)", want)
		}
	}
	if strings.Contains(body, "fs.writeFileSync(stateFile") {
		t.Error("registerSession still writes the ledger in place")
	}
}

// TestLaunchCjsReportsSpawnError pins W12: spawnSync reports a failure to start
// the binary in res.error, which the generic branch never inspected — a missing
// or unrunnable binary produced silence on every gated call.
func TestLaunchCjsReportsSpawnError(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(hooksDirPath(), "launch.cjs"))
	if err != nil {
		t.Fatalf("cannot read launch.cjs: %v", err)
	}
	if !strings.Contains(string(data), "res.error") {
		t.Error("the generic spawnSync branch must inspect res.error and report it on stderr")
	}
}
