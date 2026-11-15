package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func spawnInput(target string) preToolUseInput {
	return preToolUseInput{
		ToolName:  "Agent",
		SessionID: "sess-1",
		Cwd:       "C:/repo",
		ToolInput: preToolUseToolInput{SubagentType: target},
	}
}

func irisLedgerWithSpawns(spawns map[string]any) map[string]any {
	l := irisLedger()
	l["turn_spawns"] = spawns
	return l
}

func TestIrisSpawnCap(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		prompt   string
		spawns   map[string]any
		god      string
		wantDeny bool
	}{
		{"first hermes allowed", "kratos:hermes", "", map[string]any{}, "iris", false},
		{"second hermes denied", "kratos:hermes", "", map[string]any{"hermes": float64(1)}, "iris", true},
		{"first ares allowed", "kratos:ares", "fix round 1", map[string]any{}, "iris", false},
		{"fix ares after hermes allowed", "kratos:ares", "fix the BLOCKER", map[string]any{"hermes": float64(1)}, "iris", false},
		{"second fix ares after hermes denied", "kratos:ares", "fix the BLOCKER", map[string]any{"hermes": float64(1), "ares": float64(1)}, "iris", true},
		{"ares after ares without hermes allowed", "kratos:ares", "fix round", map[string]any{"ares": float64(1)}, "iris", false},
		{"hermes word in an ares prompt is not a signal", "kratos:ares", "DONE: fix(hermes): x", map[string]any{"ares": float64(1)}, "iris", false},
		{"no inline god untouched", "kratos:hermes", "", map[string]any{"hermes": float64(3)}, "", false},
		{"other god untouched", "kratos:hermes", "", map[string]any{"hermes": float64(3)}, "odysseus", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ledger := irisLedgerWithSpawns(tc.spawns)
			ledger["inline_god"] = tc.god
			res := editGateDecisionWith(spawnInput(tc.target), gateToolExtra{Prompt: tc.prompt}, ledger)
			if tc.wantDeny {
				if res.Decision != "deny" || res.Reason != irisSpawnCapReason {
					t.Fatalf("decision = %q (%q), want deny with the cap reason", res.Decision, res.Reason)
				}
				if res.TurnSpawns != nil {
					t.Error("a denied spawn must not be counted")
				}
				return
			}
			if res.Decision != "" {
				t.Fatalf("decision = %q, want none", res.Decision)
			}
		})
	}
}

// The ledger records only hermes and the post-hermes ares spawns, in the order
// the hook runs: impl Ares, Hermes, one fix Ares pass; a second fix Ares denies.
func TestIrisSpawnCapSequence(t *testing.T) {
	ledger := irisLedger()
	ledger["inline_god"] = "iris"
	spawn := func(god string) bool {
		res := editGateDecisionWith(spawnInput(god), gateToolExtra{Prompt: "x"}, ledger)
		if res.Decision == "deny" {
			return true
		}
		if res.TurnSpawns != nil {
			ledger["turn_spawns"] = res.TurnSpawns
		}
		return false
	}
	for i, step := range []struct {
		god  string
		deny bool
	}{
		{"kratos:ares", false}, {"kratos:hermes", false}, {"kratos:ares", false}, {"kratos:ares", true}, {"kratos:hermes", true},
	} {
		if got := spawn(step.god); got != step.deny {
			t.Fatalf("step %d (%s): deny = %v, want %v", i, step.god, got, step.deny)
		}
	}
	spawn("kratos:metis")
	if _, ok := ledgerTurnSpawns(ledger)["metis"]; ok {
		t.Error("an unrelated subagent_type must not be written into the ledger")
	}
}

func TestIrisSpawnCapCountsAndPersists(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	seedLedgerFor(t, "sess-1", map[string]any{"session_id": "sess-1", "cwd": "C:/repo", "inline_god": "iris", "custom_key": "keep"})

	spawn := `{"session_id":"sess-1","cwd":"C:/repo","tool_name":"Agent","tool_input":{"subagent_type":"kratos:hermes","prompt":"review"}}`
	handleEditGate([]byte(spawn))
	m, err := readInlineLedger("sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if got := ledgerTurnSpawns(m)["hermes"]; got != 1 {
		t.Fatalf("turn_spawns[hermes] = %d, want 1", got)
	}
	if m["custom_key"] != "keep" {
		t.Error("unknown ledger key lost")
	}

	handleEditGate([]byte(spawn))
	m, _ = readInlineLedger("sess-1")
	if got := ledgerTurnSpawns(m)["hermes"]; got != 1 {
		t.Fatalf("a denied spawn changed the count to %d", got)
	}
}

func seedLedgerFor(t *testing.T, session string, m map[string]any) {
	t.Helper()
	if err := os.MkdirAll(sessionLedgerDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeInlineLedger(session, m); err != nil {
		t.Fatal(err)
	}
}

func TestUserTurnResetsTurnSpawns(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	seedLedger(t, map[string]any{
		"session_id": testLedgerSession, "cwd": "C:/repo", "inline_god": "iris",
		"turn_spawns": map[string]any{"hermes": float64(1), "ares": float64(1)},
	})
	recordInlineGod(testLedgerSession, "C:/repo", "run another review please")
	m := mustReadLedger(t)
	if n := len(ledgerTurnSpawns(m)); n != 0 {
		t.Fatalf("turn_spawns = %v after a user turn, want empty", m["turn_spawns"])
	}
	if m["inline_god"] != "iris" {
		t.Error("a user turn must not drop inline_god")
	}
}

func writeAgentTranscript(t *testing.T, dir, session, agent string, assistantLines int) string {
	t.Helper()
	sub := filepath.Join(dir, session, "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < assistantLines; i++ {
		b.WriteString(`{"type":"assistant"}` + "\n")
		b.WriteString(`{"type":"user"}` + "\n")
	}
	if err := os.WriteFile(filepath.Join(sub, "agent-"+agent+".jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, session+".jsonl")
}

func TestSendMessageBackstop(t *testing.T) {
	dir := t.TempDir()
	send := func(transcript, to string) preToolUseInput {
		return preToolUseInput{ToolName: "SendMessage", SessionID: "sess-1", Cwd: "C:/repo", TranscriptPath: transcript}
	}
	tp61 := writeAgentTranscript(t, dir, "sess-1", "abc", 61)
	tp59 := writeAgentTranscript(t, dir, "sess-1", "def", 59)

	t.Run("61 assistant lines denied", func(t *testing.T) {
		res := editGateDecisionWith(send(tp61, "abc"), gateToolExtra{To: "abc"}, irisLedger())
		if res.Decision != "deny" || res.Reason != "This agent holds 61 turns of context. Spawn a fresh agent with the results file." {
			t.Fatalf("got %q %q", res.Decision, res.Reason)
		}
	})
	t.Run("59 assistant lines allowed", func(t *testing.T) {
		if res := editGateDecisionWith(send(tp59, "def"), gateToolExtra{To: "def"}, irisLedger()); res.Decision != "" {
			t.Fatalf("decision = %q, want none", res.Decision)
		}
	})
	t.Run("missing transcript fails open", func(t *testing.T) {
		if res := editGateDecisionWith(send(tp61, "nope"), gateToolExtra{To: "nope"}, irisLedger()); res.Decision != "" {
			t.Fatalf("decision = %q, want none", res.Decision)
		}
	})
	t.Run("traversal in recipient fails open", func(t *testing.T) {
		if res := editGateDecisionWith(send(tp61, "../x"), gateToolExtra{To: "../x"}, irisLedger()); res.Decision != "" {
			t.Fatalf("decision = %q, want none", res.Decision)
		}
	})
	t.Run("no inline god", func(t *testing.T) {
		l := irisLedger()
		l["inline_god"] = ""
		if res := editGateDecisionWith(send(tp61, "abc"), gateToolExtra{To: "abc"}, l); res.Decision != "" {
			t.Fatalf("decision = %q, want none", res.Decision)
		}
	})
	t.Run("spawned caller untouched", func(t *testing.T) {
		in := send(tp61, "abc")
		in.AgentType = "general-purpose"
		in.AgentID = "x"
		if res := editGateDecisionWith(in, gateToolExtra{To: "abc"}, irisLedger()); res.Decision != "" {
			t.Fatalf("decision = %q, want none", res.Decision)
		}
	})
}

func TestParseGateToolExtra(t *testing.T) {
	e := parseGateToolExtra([]byte(`{"tool_input":{"prompt":"p","to":"abc"}}`))
	if e.Prompt != "p" || e.To != "abc" {
		t.Fatalf("got %+v", e)
	}
	if e := parseGateToolExtra([]byte(`not json`)); e != (gateToolExtra{}) {
		t.Fatalf("got %+v", e)
	}
}

// subdirRepo builds <root>/.claude and <root>/src/apps/web and returns both.
func subdirRepo(t *testing.T) (root, sub string) {
	t.Helper()
	root = filepath.ToSlash(t.TempDir())
	sub = root + "/src/apps/web"
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root+"/.claude/.Arena", 0o755); err != nil {
		t.Fatal(err)
	}
	return root, sub
}

func odysseusWriteAt(cwd, file string) preToolUseInput {
	return preToolUseInput{
		ToolName:  "Write",
		AgentType: "kratos:odysseus",
		AgentID:   "p1",
		Cwd:       cwd,
		ToolInput: preToolUseToolInput{FilePath: file},
	}
}

func TestGateContainmentRootFromSubdirectory(t *testing.T) {
	root, sub := subdirRepo(t)

	t.Run("plan write from a subdirectory passes", func(t *testing.T) {
		res := editGateDecision(odysseusWriteAt(sub, root+"/.claude/.Arena/tactical-plans/x.md"), nil)
		if res.Decision != "" {
			t.Fatalf("decision = %q (%s), want none", res.Decision, res.Reason)
		}
	})
	t.Run("spec delta from a subdirectory passes", func(t *testing.T) {
		res := editGateDecision(odysseusWriteAt(sub, root+"/.claude/feature/f/spec-delta/cap.md"), nil)
		if res.Decision != "" {
			t.Fatalf("decision = %q (%s), want none", res.Decision, res.Reason)
		}
	})
	t.Run("traversal out of the plan root still denies", func(t *testing.T) {
		for _, f := range []string{
			root + "/.claude/.Arena/tactical-plans/../../etc/passwd.md",
			root + "/../outside/.claude/.Arena/tactical-plans/x.md",
		} {
			if res := editGateDecision(odysseusWriteAt(sub, f), nil); res.Decision != "deny" {
				t.Errorf("%s: decision = %q, want deny", f, res.Decision)
			}
		}
	})
	t.Run("non plan file still denies", func(t *testing.T) {
		if res := editGateDecision(odysseusWriteAt(sub, root+"/src/apps/web/a.ts"), nil); res.Decision != "deny" {
			t.Fatalf("decision = %q, want deny", res.Decision)
		}
	})
}

func TestGateContainmentRootFallbacks(t *testing.T) {
	t.Run("no claude directory falls back to cwd", func(t *testing.T) {
		dir := filepath.ToSlash(t.TempDir())
		sub := dir + "/a/b"
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if got := gateContainmentRoot(sub); got != sub {
			t.Fatalf("root = %q, want %q", got, sub)
		}
		res := editGateDecision(odysseusWriteAt(sub, dir+"/.claude/.Arena/tactical-plans/x.md"), nil)
		if res.Decision != "deny" {
			t.Fatalf("a plan above the cwd with no .claude anywhere: decision = %q, want deny", res.Decision)
		}
	})
	t.Run("nonexistent cwd is returned unchanged", func(t *testing.T) {
		if got := gateContainmentRoot("C:/no/such/dir"); got != "C:/no/such/dir" {
			t.Fatalf("root = %q", got)
		}
		if got := gateContainmentRoot(""); got != "" {
			t.Fatalf("root = %q", got)
		}
	})
	t.Run("iris counts project files above the launch directory", func(t *testing.T) {
		root, sub := subdirRepo(t)
		ledger := irisLedger()
		ledger["cwd"] = sub
		in := preToolUseInput{ToolName: "Write", SessionID: "sess-1", Cwd: sub, ToolInput: preToolUseToolInput{FilePath: root + "/lib/a.ts"}}
		res := editGateDecision(in, ledger)
		if len(res.Files) != 1 {
			t.Fatalf("files = %v, want the file above the launch directory counted", res.Files)
		}
	})
}

func writeIDTranscript(t *testing.T, dir, session, agent string, lines, ids int) string {
	t.Helper()
	sub := filepath.Join(dir, session, "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < lines; i++ {
		b.WriteString(`{"type":"assistant","message":{"id":"msg_` + strconv.Itoa(i%ids) + `"}}` + "\n")
	}
	b.WriteString("unparsable line with \"type\":\"assistant\"\n")
	if err := os.WriteFile(filepath.Join(sub, "agent-"+agent+".jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, session+".jsonl")
}

func TestSendMessageBackstopCountsDistinctMessageIDs(t *testing.T) {
	dir := t.TempDir()
	send := func(tp string) preToolUseInput {
		return preToolUseInput{ToolName: "SendMessage", SessionID: "sess-1", TranscriptPath: tp}
	}
	tp := writeIDTranscript(t, dir, "sess-1", "a", 120, 60)
	if res := editGateDecisionWith(send(tp), gateToolExtra{To: "a"}, irisLedger()); res.Decision != "" {
		t.Fatalf("120 lines / 60 ids: decision = %q, want none", res.Decision)
	}
	tp = writeIDTranscript(t, dir, "sess-1", "b", 122, 61)
	res := editGateDecisionWith(send(tp), gateToolExtra{To: "b"}, irisLedger())
	if res.Decision != "deny" || !strings.Contains(res.Reason, "61 turns") {
		t.Fatalf("61 ids: got %q %q", res.Decision, res.Reason)
	}
}
