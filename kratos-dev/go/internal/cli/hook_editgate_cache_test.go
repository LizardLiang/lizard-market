package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countRootLookups swaps gateRootLookup for a counter that returns want.
func countRootLookups(t *testing.T, want string) *int {
	t.Helper()
	n := 0
	orig := gateRootLookup
	gateRootLookup = func(string) string { n++; return want }
	t.Cleanup(func() { gateRootLookup = orig })
	return &n
}

func TestGateCachedRoot(t *testing.T) {
	t.Run("same cwd computes once", func(t *testing.T) {
		n := countRootLookups(t, "C:/repo")
		ledger := map[string]any{}
		for i := 0; i < 3; i++ {
			if got := gateCachedRoot(ledger, "C:/repo/src"); got != "C:/repo" {
				t.Fatalf("root = %q", got)
			}
		}
		if *n != 1 {
			t.Fatalf("lookups = %d, want 1", *n)
		}
		if ledger[ledgerKeyRootCwd] != "C:/repo/src" {
			t.Fatalf("cached cwd = %v", ledger[ledgerKeyRootCwd])
		}
	})
	t.Run("different cwd recomputes", func(t *testing.T) {
		n := countRootLookups(t, "C:/repo")
		ledger := map[string]any{}
		gateCachedRoot(ledger, "C:/repo/a")
		gateCachedRoot(ledger, "C:/repo/b")
		if *n != 2 {
			t.Fatalf("lookups = %d, want 2", *n)
		}
		if ledger[ledgerKeyRootCwd] != "C:/repo/b" {
			t.Fatalf("cached cwd = %v, want the newest", ledger[ledgerKeyRootCwd])
		}
	})
	t.Run("empty result is never cached", func(t *testing.T) {
		n := countRootLookups(t, "")
		ledger := map[string]any{}
		gateCachedRoot(ledger, "C:/repo")
		gateCachedRoot(ledger, "C:/repo")
		if *n != 2 {
			t.Fatalf("lookups = %d, want 2", *n)
		}
		if _, ok := ledger[ledgerKeyRoot]; ok {
			t.Fatal("an empty root was stored")
		}
	})
	t.Run("no ledger still answers", func(t *testing.T) {
		n := countRootLookups(t, "C:/repo")
		if got := gateCachedRoot(nil, "C:/repo/x"); got != "C:/repo" || *n != 1 {
			t.Fatalf("root = %q lookups = %d", got, *n)
		}
	})
	t.Run("empty cwd does no lookup", func(t *testing.T) {
		n := countRootLookups(t, "C:/repo")
		if got := gateCachedRoot(map[string]any{}, ""); got != "" || *n != 0 {
			t.Fatalf("root = %q lookups = %d", got, *n)
		}
	})
}

// TestEditGateRootCachedAcrossCalls runs the whole hook twice for an inline
// Iris editing a project file and checks the ledger on disk carries the root
// after the first call, so the second call runs no lookup.
func TestEditGateRootCachedAcrossCalls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".kratos", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	proj := filepath.ToSlash(t.TempDir())
	ledger := map[string]any{"session_id": "sess-c", "cwd": proj, "inline_god": "iris"}
	data, _ := json.Marshal(ledger)
	if err := os.WriteFile(filepath.Join(dir, "sess-c.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	n := countRootLookups(t, proj)
	payload := payloadJSON(map[string]any{"session_id": "sess-c", "cwd": proj, "tool_name": "Write", "tool_input": map[string]any{"file_path": proj + "/src/a.go"}})
	handleEditGate([]byte(payload))
	handleEditGate([]byte(strings.Replace(payload, "a.go", "b.go", 1)))
	if *n != 1 {
		t.Fatalf("lookups across two hook runs = %d, want 1", *n)
	}
	got, err := readInlineLedger("sess-c")
	if err != nil {
		t.Fatal(err)
	}
	if got[ledgerKeyRoot] != proj || got[ledgerKeyRootCwd] != proj {
		t.Fatalf("ledger root pair = %v / %v", got[ledgerKeyRoot], got[ledgerKeyRootCwd])
	}
}

// TestEditGateEmptyLedgerRunsNoLookup pins R5: with no inline god, no tool
// except the Agent/Task/SendMessage/Skill ledger paths reaches the containment
// lookup (a `git rev-parse`), and the credential guard still answers.
func TestEditGateEmptyLedgerRunsNoLookup(t *testing.T) {
	n := countRootLookups(t, "C:/repo")
	empty := map[string]any{"session_id": "sess-1", "cwd": "C:/repo"}
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit", "Bash", "PowerShell"} {
		in := preToolUseInput{SessionID: "sess-1", Cwd: "C:/repo", ToolName: tool, ToolInput: preToolUseToolInput{FilePath: "C:/repo/src/a.go", Command: "ls"}}
		if res := editGateDecision(in, empty); res.Decision != "" {
			t.Fatalf("%s: decision = %q", tool, res.Decision)
		}
	}
	if *n != 0 {
		t.Fatalf("lookups with an empty ledger = %d, want 0", *n)
	}

	cred := preToolUseInput{SessionID: "sess-1", Cwd: "C:/repo", ToolName: "Bash", ToolInput: preToolUseToolInput{Command: `sqlcmd -S prod -U sa -P hunter2 -Q "select 1"`}}
	if res := editGateDecision(cred, empty); res.Decision != "ask" {
		t.Fatalf("credential-shaped command with an empty ledger: decision = %q, want ask", res.Decision)
	}
	if *n != 0 {
		t.Fatalf("lookups after the credential case = %d, want 0", *n)
	}

	spawn := preToolUseInput{SessionID: "sess-1", Cwd: "C:/repo", ToolName: "Agent", ToolInput: preToolUseToolInput{SubagentType: "kratos:ares"}}
	if res := editGateDecision(spawn, empty); res.Decision != "" || res.Files != nil {
		t.Fatalf("Agent with an empty ledger: %+v", res)
	}
	if res := editGateDecision(preToolUseInput{SessionID: "sess-1", ToolName: "Skill", ToolInput: preToolUseToolInput{Skill: "kratos:iris"}}, empty); res.SetGod != "iris" {
		t.Fatalf("Skill with an empty ledger: SetGod = %q, want iris", res.SetGod)
	}
}

// TestHooksJSONPreToolUseBudget pins R4: a 3 s PreToolUse budget timed out 31
// times in one week under memory pressure, and a timed-out hook fails open.
func TestHooksJSONPreToolUseBudget(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string  `json:"command"`
				Timeout float64 `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, event := range []string{"PreToolUse", "PermissionRequest"} {
		for _, g := range cfg.Hooks[event] {
			for _, h := range g.Hooks {
				seen++
				if h.Timeout < 10 {
					t.Errorf("%s hook %q has timeout %v, want at least 10", event, h.Command, h.Timeout)
				}
			}
		}
	}
	if seen < 3 {
		t.Fatalf("found %d gate hooks, want at least 3", seen)
	}
}
