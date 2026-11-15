package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// runSessionStartIn is runSessionStart with a caller-owned home and cwd, so a
// test can seed handoff.md and read the state file back.
func runSessionStartIn(t *testing.T, home, cwd, source string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	root, err := filepath.Abs(filepath.Join(hooksDirPath(), ".."))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin", "kratos")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skip("plugin binary not built")
	}
	payload, _ := json.Marshal(map[string]any{"session_id": "sess-c", "source": source, "cwd": cwd})
	cmd := exec.Command(node, filepath.Join(root, "hooks", "session-start.cjs"))
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "CLAUDE_PLUGIN_ROOT="+root)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("session-start.cjs (%s): %v", source, err)
	}
	return string(out)
}

// Compact SessionStart: environment facts from handoff.md are re-injected,
// the preference list is not, and the spec-delta reminder prints once.
func TestSessionStartCompactEnvironmentAndNagOnce(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	arena := filepath.Join(cwd, ".claude", ".Arena")
	if err := os.MkdirAll(arena, 0o755); err != nil {
		t.Fatal(err)
	}
	handoff := "# Session Handoff\n\n## Shipped\n- x\n\n## Environment\n<!-- template comment -->\n- fake Mallbic — :8790 — python mbfake/fake_mallbic.py --test-only\n- worktree leweigen-erp-v2-whmap — feat/whmap\n- [server/worktree/script] — [port/branch/path] — [start or restart command]\n\n## Next steps\n1. y\n"
	if err := os.WriteFile(filepath.Join(arena, "handoff.md"), []byte(handoff), 0o644); err != nil {
		t.Fatal(err)
	}
	delta := filepath.Join(cwd, ".claude", "feature", "whmap", "spec-delta")
	if err := os.MkdirAll(delta, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(delta, "stock.md"), []byte("# delta\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := runSessionStartIn(t, home, cwd, "startup")
	if !strings.Contains(first, "un-archived spec delta") {
		t.Fatalf("startup must print the spec-delta reminder:\n%s", first)
	}
	if strings.Contains(first, "environment still standing") {
		t.Fatalf("startup must not print the environment block (the handoff notice covers it):\n%s", first)
	}

	compact := runSessionStartIn(t, home, cwd, "compact")
	if !strings.Contains(compact, "fake Mallbic — :8790") || !strings.Contains(compact, "feat/whmap") {
		t.Fatalf("compact must re-inject the ## Environment lines:\n%s", compact)
	}
	if strings.Contains(compact, "template comment") || strings.Contains(compact, "[server/worktree/script]") {
		t.Fatalf("compact must skip comments and template placeholders:\n%s", compact)
	}
	if strings.Contains(compact, "## Stored user preferences") {
		t.Fatalf("compact must not re-list the preferences:\n%s", compact)
	}
	if strings.Contains(compact, "un-archived spec delta") {
		t.Fatalf("the spec-delta reminder was already shown at startup and must not repeat at compaction:\n%s", compact)
	}
	if !strings.Contains(compact, "Output constraint") {
		t.Fatalf("compact still carries the output constraint:\n%s", compact)
	}

	state, err := os.ReadFile(filepath.Join(home, ".kratos", "sessions", "sess-c.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), `"spec_delta_nag_shown": true`) {
		t.Fatalf("state file must record the shown reminder:\n%s", state)
	}
}
