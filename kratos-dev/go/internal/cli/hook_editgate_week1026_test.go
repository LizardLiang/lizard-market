package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Gate rules from the 2026-10-20 → 10-31 transcript review.

func aresBash(command string) editGateResult {
	return editGateDecision(preToolUseInput{
		ToolName: "Bash", SessionID: "sess-1", Cwd: "C:/repo",
		AgentType: "kratos:ares", AgentID: "a1",
		ToolInput: preToolUseToolInput{Command: command},
	}, nil)
}

func TestAresBashGate(t *testing.T) {
	deny := []struct{ name, cmd, reason string }{
		{"stash push with pathspec (2026-10-23)", `git stash push -q -m "pre-change" -- frontend/src/x.ts`, aresGitDenyReason},
		{"stash pop", "git stash pop", aresGitDenyReason},
		{"checkout pathspec", "git checkout -- package.json", aresGitDenyReason},
		{"checkout branch", "git checkout main", aresGitDenyReason},
		{"switch branch", "git switch main", aresGitDenyReason},
		{"restore", "git restore src/a.ts", aresGitDenyReason},
		{"clean", "git clean -fd", aresGitDenyReason},
		{"reset hard", "git reset --hard HEAD~1", aresGitDenyReason},
		{"git -C worktree stash", `git -C "D:/wt/feature" stash`, aresGitDenyReason},
		{"stash after another command", "npm test && git stash", aresGitDenyReason},
		{"SKIP_LOCAL_CI (2026-10-24)", `SKIP_LOCAL_CI=1 git commit -m "feat: x"`, aresHookBypassDenyReason},
		{"no-verify", `git commit --no-verify -m "feat: x"`, aresHookBypassDenyReason},
		{"HUSKY=0", `HUSKY=0 git commit -m x`, aresHookBypassDenyReason},
	}
	for _, tc := range deny {
		t.Run("deny/"+tc.name, func(t *testing.T) {
			got := aresBash(tc.cmd)
			if got.Decision != "deny" || got.Reason != tc.reason {
				t.Fatalf("%q: got %q %q", tc.cmd, got.Decision, got.Reason)
			}
		})
	}
	allow := []struct{ name, cmd string }{
		{"new branch", "git checkout -b feat/x"},
		{"switch -c", "git switch -c feat/x"},
		{"stash list", "git stash list"},
		{"mixed reset", "git reset HEAD~1"},
		{"unstage", "git reset -- src/a.ts"},
		{"worktree baseline", "git worktree add ../baseline HEAD"},
		{"commit message mentions stash", `git commit -m "fix: drop the git stash step from the deploy script"`},
		{"commit message mentions no-verify", `git commit -m "docs: explain why --no-verify is forbidden"`},
		{"tests", "npm test -- --run src/a.test.ts"},
		{"status", "git status --porcelain"},
	}
	for _, tc := range allow {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if got := aresBash(tc.cmd); got.Decision != "" {
				t.Fatalf("%q: decision = %q (%s), want none", tc.cmd, got.Decision, got.Reason)
			}
		})
	}
	t.Run("edits by Ares are never gated here", func(t *testing.T) {
		got := editGateDecision(preToolUseInput{ToolName: "Edit", SessionID: "sess-1", AgentType: "kratos:ares", AgentID: "a1",
			ToolInput: preToolUseToolInput{FilePath: "C:/repo/src/a.ts"}}, nil)
		if got.Decision != "" {
			t.Fatalf("decision = %q, want none", got.Decision)
		}
	})
}

func writeMarkedChildTranscript(t *testing.T, dir, session, agent, firstLine string) string {
	t.Helper()
	sub := filepath.Join(dir, session, "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	body := firstLine + "\n" + `{"type":"assistant"}` + "\n"
	if err := os.WriteFile(filepath.Join(sub, "agent-"+agent+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, session+".jsonl")
}

func TestReadOnlyChildGitGate(t *testing.T) {
	dir := t.TempDir()
	marked := writeMarkedChildTranscript(t, dir, "sess-1", "child1", `{"type":"user","message":{"role":"user","content":"MISSION: Focused Code Review\nWORKING TREE IS READ-ONLY. Never run git checkout/restore/stash/reset/clean."}}`)
	plain := writeMarkedChildTranscript(t, dir, "sess-1", "child2", `{"type":"user","message":{"role":"user","content":"MISSION: Explore the repo"}}`)
	child := func(transcript, agent, cmd string) editGateResult {
		return editGateDecision(preToolUseInput{
			ToolName: "Bash", SessionID: "sess-1", Cwd: "C:/repo", TranscriptPath: transcript,
			AgentType: "general-purpose", AgentID: agent,
			ToolInput: preToolUseToolInput{Command: cmd},
		}, nil)
	}
	t.Run("detach in a read-only child denied (2026-10-21)", func(t *testing.T) {
		got := child(marked, "child1", "git checkout -q --detach")
		if got.Decision != "deny" || got.Reason != readOnlyChildDenyReason {
			t.Fatalf("got %q %q", got.Decision, got.Reason)
		}
	})
	t.Run("stash in a read-only child denied", func(t *testing.T) {
		if got := child(marked, "child1", "git stash && npm test"); got.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", got.Decision)
		}
	})
	t.Run("read-only git in a read-only child allowed", func(t *testing.T) {
		if got := child(marked, "child1", "git show HEAD~1:src/a.ts | head -40"); got.Decision != "" {
			t.Fatalf("decision = %q, want none", got.Decision)
		}
	})
	t.Run("same command in an unmarked child allowed", func(t *testing.T) {
		if got := child(plain, "child2", "git checkout -q --detach"); got.Decision != "" {
			t.Fatalf("decision = %q, want none", got.Decision)
		}
	})
	t.Run("missing transcript fails open", func(t *testing.T) {
		if got := child(marked, "nobody", "git checkout -q --detach"); got.Decision != "" {
			t.Fatalf("decision = %q, want none", got.Decision)
		}
	})
	t.Run("payload naming the subagent file directly", func(t *testing.T) {
		direct := filepath.Join(dir, "sess-1", "subagents", "agent-child1.jsonl")
		if got := child(direct, "child1", "git reset --hard"); got.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", got.Decision)
		}
	})
}

func TestIrisSpawnCapScoped(t *testing.T) {
	scoped := func(target, scope string, spawns map[string]any) editGateResult {
		ledger := irisLedgerWithSpawns(spawns)
		ledger["inline_god"] = "iris"
		return editGateDecisionWith(spawnInput(target), gateToolExtra{Prompt: "MISSION: Dedicated Code Review\n" + scope + "\nTARGET: git diff main...HEAD"}, ledger)
	}
	t.Run("second hermes on another worktree allowed (2026-10-25 16:28)", func(t *testing.T) {
		res := scoped("kratos:hermes", "WORKTREE: D:/wt/buttons", map[string]any{"hermes": float64(1), "hermes@d:/wt/whmap": float64(1)})
		if res.Decision != "" {
			t.Fatalf("decision = %q (%s), want none", res.Decision, res.Reason)
		}
		if res.TurnSpawns["hermes"] != 2 || res.TurnSpawns["hermes@d:/wt/buttons"] != 1 {
			t.Fatalf("turn_spawns = %v", res.TurnSpawns)
		}
	})
	t.Run("second hermes on the same worktree denied", func(t *testing.T) {
		res := scoped("kratos:hermes", "WORKTREE: D:\\wt\\whmap\\", map[string]any{"hermes": float64(1), "hermes@d:/wt/whmap": float64(1)})
		if res.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", res.Decision)
		}
	})
	t.Run("hermes without a scope line keeps the per-turn rule", func(t *testing.T) {
		res := scoped("kratos:hermes", "FOCUS: all", map[string]any{"hermes": float64(1), "hermes@d:/wt/whmap": float64(1)})
		if res.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", res.Decision)
		}
	})
	t.Run("fix ares per worktree", func(t *testing.T) {
		res := scoped("kratos:ares", "BRANCH: feat/buttons", map[string]any{"hermes": float64(2), "ares": float64(1), "ares@feat/whmap": float64(1)})
		if res.Decision != "" {
			t.Fatalf("decision = %q (%s), want none", res.Decision, res.Reason)
		}
		res = scoped("kratos:ares", "BRANCH: feat/whmap", map[string]any{"hermes": float64(2), "ares": float64(1), "ares@feat/whmap": float64(1)})
		if res.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", res.Decision)
		}
	})
	t.Run("five module scopes in one worktree all pass, a repeat is denied", func(t *testing.T) {
		spawns := map[string]any{}
		for i, mod := range []string{"auth", "billing", "orders", "search", "users"} {
			res := scoped("kratos:hermes", "WORKTREE: D:/wt/app\nSCOPE: ONLY files under src/modules/"+mod+"/", spawns)
			if res.Decision != "" {
				t.Fatalf("module %d (%s): decision = %q (%s), want none", i, mod, res.Decision, res.Reason)
			}
			spawns = map[string]any{}
			for k, v := range res.TurnSpawns {
				spawns[k] = float64(v)
			}
		}
		res := scoped("kratos:hermes", "WORKTREE: D:/wt/app\nSCOPE: ONLY files under src/modules/auth/", spawns)
		if res.Decision != "deny" {
			t.Fatalf("repeat module: decision = %q, want deny", res.Decision)
		}
	})
	t.Run("second ares fix round with free-text SCOPE labels is denied", func(t *testing.T) {
		first := scoped("kratos:hermes", "FOCUS: all", map[string]any{})
		spawns := map[string]any{}
		for k, v := range first.TurnSpawns {
			spawns[k] = float64(v)
		}
		r1 := scoped("kratos:ares", "SCOPE: exactly the 11 findings below", spawns)
		if r1.Decision != "" {
			t.Fatalf("first fix round: decision = %q (%s), want none", r1.Decision, r1.Reason)
		}
		spawns = map[string]any{}
		for k, v := range r1.TurnSpawns {
			spawns[k] = float64(v)
		}
		r2 := scoped("kratos:ares", "SCOPE: exactly the 2 findings below", spawns)
		if r2.Decision != "deny" {
			t.Fatalf("second fix round: decision = %q, want deny", r2.Decision)
		}
	})
	t.Run("an extra scope line on the same worktree does not split the cap", func(t *testing.T) {
		res := scoped("kratos:hermes", "WORKTREE: D:\\wt\\whmap\nBRANCH: x", map[string]any{"hermes": float64(1), "hermes@d:/wt/whmap": float64(1)})
		if res.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", res.Decision)
		}
	})
	t.Run("reason names the two ways out", func(t *testing.T) {
		if !strings.Contains(irisSpawnCapReason, "AskUserQuestion answer") || !strings.Contains(irisSpawnCapReason, "WORKTREE:") || !strings.Contains(irisSpawnCapReason, "SCOPE:") {
			t.Fatalf("reason = %q", irisSpawnCapReason)
		}
	})
}

func TestGateSpawnScope(t *testing.T) {
	cases := map[string]string{
		"MISSION: x\nWORKTREE: D:\\wt\\A\\\nTARGET: y": "d:/wt/a",
		"WORKDIR: `/home/u/wt/b`":                      "/home/u/wt/b",
		"branch: Feat/X":                               "feat/x",
		"SCOPE: ONLY files under src/modules/Auth/":    "only files under src/modules/auth",
		"TARGET: src/a.ts":                             "",
		"":                                             "",
	}
	for prompt, want := range cases {
		if got := gateSpawnScope(prompt, "kratos:hermes"); got != want {
			t.Errorf("gateSpawnScope(%q) = %q, want %q", prompt, got, want)
		}
	}
	if got := gateSpawnScope("WORKTREE: D:/wt/a\nSCOPE: exactly the 11 findings below", "kratos:ares"); got != "d:/wt/a" {
		t.Errorf("ares scope = %q, want the worktree alone", got)
	}
	if got := gateSpawnScope("SCOPE: exactly the 11 findings below", "kratos:ares"); got != "" {
		t.Errorf("ares free-text scope = %q, want empty", got)
	}
}

func TestSendMessageControlMessageExempt(t *testing.T) {
	dir := t.TempDir()
	tp := writeAgentTranscript(t, dir, "sess-1", "abc", 76)
	send := func(message string) editGateResult {
		in := preToolUseInput{ToolName: "SendMessage", SessionID: "sess-1", Cwd: "C:/repo", TranscriptPath: tp}
		return editGateDecisionWith(in, gateToolExtra{To: "abc", Message: message}, irisLedger())
	}
	t.Run("stop message allowed at 76 turns (2026-10-28 15:02)", func(t *testing.T) {
		if res := send("Stop after the current task, commit what is green and hand back."); res.Decision != "" {
			t.Fatalf("decision = %q (%s), want none", res.Decision, res.Reason)
		}
	})
	t.Run("status question allowed", func(t *testing.T) {
		if res := send("How long do you still need? Report progress, do not start the e2e suite."); res.Decision != "" {
			t.Fatalf("decision = %q, want none", res.Decision)
		}
	})
	t.Run("new scope still denied", func(t *testing.T) {
		if res := send("Also add the B1 finding: re-開帳 on every slip. Fix it in the same round."); res.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", res.Decision)
		}
	})
	t.Run("long message starting with stop still denied", func(t *testing.T) {
		if res := send("stop " + strings.Repeat("and then do this other thing ", 20)); res.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", res.Decision)
		}
	})
	e := parseGateToolExtra([]byte(`{"tool_input":{"to":"abc","message":"stop"}}`))
	if e.Message != "stop" {
		t.Fatalf("Message = %q", e.Message)
	}
}

// The five commands the Odysseus gate denied on 2026-10-26/27 (binary
// v2.120.0), plus the memory save it blocked, all classify read-only now.
func TestOdysseusReadOnlyAdditions(t *testing.T) {
	legit := []string{
		`ls "$S/ecprobe" | head; ls "$S" | grep -i walk`,
		`git status; git log --oneline -5; cd D:/wt/whmap && wc -l frontend/src/pages/Sync.tsx`,
		`grep -o '"op":"[a-z_]*"' calls.jsonl | sort | uniq -c`,
		`grep -n "def main" backend/app/main.py | head; echo ----; grep -n "Depends" backend/app/api/v1/sync.py | head`,
		`gh auth status | head; ls D:/wt | grep -i erp`,
		`gh pr view 42 --json title,body`,
		`kratos memory add "State a limitation in one plain sentence first" --category rule`,
		`C:/Users/u/.kratos/bin/kratos.exe memory add "x" --category preference`,
	}
	for _, cmd := range legit {
		t.Run(cmd[:20], func(t *testing.T) {
			if got := odysseusBashDecision(cmd); got.Decision != "" {
				t.Errorf("%q: decision = %q (%s), want none", cmd, got.Decision, got.Reason)
			}
		})
	}
	for _, cmd := range []string{`gh pr merge 42`, `gh repo delete x`, `kratos memory rm 12`, `gh auth login`} {
		t.Run("deny/"+cmd, func(t *testing.T) {
			if got := odysseusBashDecision(cmd); got.Decision != "deny" {
				t.Errorf("%q: decision = %q, want deny", cmd, got.Decision)
			}
		})
	}
	t.Run("write to the Claude memory dir allowed", func(t *testing.T) {
		for _, p := range []string{
			`C:\Users\u\.claude\projects\D--repo\memory\say-limits-plainly.md`,
			"/home/u/.claude/projects/-home-u-repo/memory/MEMORY.md",
		} {
			var input preToolUseInput
			if err := json.Unmarshal([]byte(odysseusPayload("Write", map[string]any{"file_path": p, "content": "x"})), &input); err != nil {
				t.Fatal(err)
			}
			if got := editGateDecision(input, nil); got.Decision != "" {
				t.Errorf("%q: decision = %q, want none", p, got.Decision)
			}
		}
	})
	t.Run("write to a source file still denied", func(t *testing.T) {
		var input preToolUseInput
		if err := json.Unmarshal([]byte(odysseusPayload("Write", map[string]any{"file_path": "C:/repo/src/memory/index.ts", "content": "x"})), &input); err != nil {
			t.Fatal(err)
		}
		if got := editGateDecision(input, nil); got.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", got.Decision)
		}
	})
}

func TestResetTurnLedger(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	seed := func() {
		seedLedger(t, map[string]any{
			"session_id": testLedgerSession, "cwd": "C:/repo", "inline_god": "iris",
			"inline_edited_files": []any{"C:/repo/src/a.ts"},
			"turn_spawns":         map[string]any{"hermes": float64(1), "ares": float64(1)},
		})
	}
	t.Run("answered question resets the round (2026-10-20 23:25)", func(t *testing.T) {
		seed()
		resetTurnLedger(turnResetInput{ToolName: "AskUserQuestion", SessionID: testLedgerSession})
		m := mustReadLedger(t)
		if len(ledgerTurnSpawns(m)) != 0 || len(ledgerStrings(m)) != 0 {
			t.Fatalf("ledger not reset: %v", m)
		}
		if m["inline_god"] != "iris" {
			t.Error("reset must keep inline_god")
		}
	})
	t.Run("other tools ignored", func(t *testing.T) {
		seed()
		resetTurnLedger(turnResetInput{ToolName: "Edit", SessionID: testLedgerSession})
		if len(ledgerTurnSpawns(mustReadLedger(t))) != 2 {
			t.Fatal("a non-question PostToolUse must not reset the round")
		}
	})
	t.Run("a spawned agent's question ignored", func(t *testing.T) {
		seed()
		resetTurnLedger(turnResetInput{ToolName: "AskUserQuestion", SessionID: testLedgerSession, AgentType: "kratos:odysseus", AgentID: "x"})
		if len(ledgerTurnSpawns(mustReadLedger(t))) != 2 {
			t.Fatal("a subagent's question must not reset the main context's round")
		}
	})
}

func TestHooksJSONRegistersTurnReset(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, m := range doc.Hooks["PostToolUse"] {
		if m.Matcher == "AskUserQuestion" {
			for _, h := range m.Hooks {
				if strings.Contains(h.Command, "hook turn-reset") {
					return
				}
			}
		}
	}
	t.Fatal("hooks.json has no PostToolUse AskUserQuestion → hook turn-reset entry")
}

func TestHandbackSizeFailure(t *testing.T) {
	if _, ok := handbackSizeFailure(strings.Repeat("x", 5000), aresHandbackMaxChars, "ares"); ok {
		t.Fatal("a report at the cap must pass")
	}
	f, ok := handbackSizeFailure(strings.Repeat("字", 5001), aresHandbackMaxChars, "ares")
	if !ok || !strings.Contains(f, "5001 chars") || !strings.Contains(f, "implementation-notes.md") {
		t.Fatalf("got %q %v", f, ok)
	}
	f, ok = handbackSizeFailure(strings.Repeat("x", 9000), hermesHandbackMaxChars, "hermes")
	if !ok || !strings.Contains(f, "code-review.md") {
		t.Fatalf("got %q %v", f, ok)
	}
	t.Run("oversized Hermes hand-back denied, then released after the cap", func(t *testing.T) {
		cwd := t.TempDir()
		in := preToolUseInput{ToolName: "SubagentHandback", AgentType: "kratos:hermes", AgentID: "h1", Cwd: cwd,
			ToolInput: preToolUseToolInput{Message: strings.Repeat("finding\n", 2000)}}
		for i := 0; i < gateMaxBlocks; i++ {
			if res := handbackGateDecision(in); res.Decision != "deny" || !strings.Contains(res.Reason, "chars (cap") {
				t.Fatalf("attempt %d: got %q %q", i+1, res.Decision, res.Reason)
			}
		}
		if res := handbackGateDecision(in); res.Decision != "" {
			t.Fatalf("after %d denials the hand-back must pass, got %q", gateMaxBlocks, res.Decision)
		}
	})
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestLandedIgnoresDirtyBaseline(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "a.txt")
	gitRun(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")

	// The user's own untracked file, present before Ares starts.
	if err := os.WriteFile(filepath.Join(repo, "vite.walkthrough.config.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseDir := t.TempDir()
	old := dirtyBaselineDirFn
	dirtyBaselineDirFn = func() string { return baseDir }
	t.Cleanup(func() { dirtyBaselineDirFn = old })

	if f := evaluateLanded(repo, ""); len(f) == 0 {
		t.Fatal("without a baseline the untracked file must count — the control for this test")
	}
	recordDirtyBaseline(repo)
	if f := evaluateLanded(repo, ""); len(f) != 0 {
		t.Fatalf("pre-existing untracked file counted against landing: %v", f)
	}
	// Work Ares leaves uncommitted is still unlanded.
	if err := os.WriteFile(filepath.Join(repo, "ares-left-this.ts"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := evaluateLanded(repo, "")
	if len(f) != 1 || !strings.Contains(f[0], "ares-left-this.ts") || strings.Contains(f[0], "vite.walkthrough") {
		t.Fatalf("got %v", f)
	}
}
