package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runAgentLoad executes `agent load` with the given args and returns stdout.
func runAgentLoad(t *testing.T, args ...string) string {
	t.Helper()
	cmd := AgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"load"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent load %v: %v", args, err)
	}
	return out.String()
}

// TestAgentLoadDefaultUnresolved verifies --resolve is opt-in: without the
// flag, tokens pass through byte-identical to today's behavior.
func TestAgentLoadDefaultUnresolved(t *testing.T) {
	out := runAgentLoad(t, "ares")
	if !strings.Contains(out, "<KRATOS_ROOT>") {
		t.Fatalf("expected <KRATOS_ROOT> token to survive without --resolve, got:\n%s", truncate(out))
	}
}

// TestAgentLoadResolveRootFlag verifies --root wins and produces a
// token-free output with forward slashes even from a Windows-style path.
func TestAgentLoadResolveRootFlag(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", "") // ensure --root, not env, is exercised

	out := runAgentLoad(t, "ares", "--resolve", "--root", `C:\Users\foo\plugin`)

	if strings.Contains(out, "<KRATOS_ROOT>") {
		t.Errorf("expected <KRATOS_ROOT> to be fully substituted, got:\n%s", truncate(out))
	}
	if !strings.Contains(out, "C:/Users/foo/plugin") {
		t.Errorf("expected forward-slash root C:/Users/foo/plugin in output, got:\n%s", truncate(out))
	}
	if strings.Contains(out, `C:\Users\foo\plugin`) {
		t.Errorf("expected no backslash-form root in output, got:\n%s", truncate(out))
	}
}

// TestAgentLoadResolveRootFlagBeatsEnv verifies --root takes precedence over
// CLAUDE_PLUGIN_ROOT when both are present.
func TestAgentLoadResolveRootFlagBeatsEnv(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", "/env/root")

	out := runAgentLoad(t, "ares", "--resolve", "--root", "/flag/root")

	if !strings.Contains(out, "/flag/root") {
		t.Errorf("expected --root value /flag/root to win, got:\n%s", truncate(out))
	}
	if strings.Contains(out, "/env/root") {
		t.Errorf("expected env root /env/root to be overridden by --root, got:\n%s", truncate(out))
	}
}

// TestAgentLoadResolveEnvBeatsExeRelative verifies CLAUDE_PLUGIN_ROOT is used
// (over the exe-relative probe) when no --root flag is given.
func TestAgentLoadResolveEnvBeatsExeRelative(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", "/env/only/root")

	out := runAgentLoad(t, "ares", "--resolve")

	if !strings.Contains(out, "/env/only/root") {
		t.Errorf("expected env root /env/only/root to be used, got:\n%s", truncate(out))
	}
	if strings.Contains(out, "<KRATOS_ROOT>") {
		t.Errorf("expected <KRATOS_ROOT> fully substituted from env, got:\n%s", truncate(out))
	}
}

// TestAgentLoadResolveUnresolvableRootLeavesTokens verifies that when no
// --root is given, CLAUDE_PLUGIN_ROOT is unset, and the test binary's
// grandparent directory does not contain an agents/ subdirectory (true for
// `go test` binaries, which run from a temp build dir), <KRATOS_ROOT> is
// left unmodified rather than guessed.
func TestAgentLoadResolveUnresolvableRootLeavesTokens(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Dir(filepath.Dir(exe))
		if info, statErr := os.Stat(filepath.Join(candidate, "agents")); statErr == nil && info.IsDir() {
			t.Skip("test binary happens to sit under a directory with an agents/ sibling; exe-relative probe would resolve, invalidating this case")
		}
	}

	out := runAgentLoad(t, "ares", "--resolve")

	if !strings.Contains(out, "<KRATOS_ROOT>") {
		t.Errorf("expected <KRATOS_ROOT> to remain unresolved when no root is discoverable, got:\n%s", truncate(out))
	}
}

// TestAgentLoadResolveCommandModeCoversSliceAndSuffix verifies --resolve
// substitutes tokens inside the injected protocol slice and command-mode
// suffix, not just the agent body. hermes carries both a slice and a suffix
// with <KRATOS_ROOT>/<kratos-bin> tokens.
func TestAgentLoadResolveCommandModeCoversSliceAndSuffix(t *testing.T) {
	unresolved := runAgentLoad(t, "hermes", "--mode=command")
	if !strings.Contains(unresolved, "<KRATOS_ROOT>") && !strings.Contains(unresolved, "<kratos-bin>") {
		t.Fatalf("fixture assumption broken: expected hermes --mode=command body (incl. slice+suffix) to contain KRATOS_ROOT/kratos-bin tokens before resolving, got:\n%s", truncate(unresolved))
	}

	out := runAgentLoad(t, "hermes", "--mode=command", "--resolve", "--root", "/resolved/root")

	if strings.Contains(out, "<KRATOS_ROOT>") {
		t.Errorf("expected <KRATOS_ROOT> resolved across body+slice+suffix, got:\n%s", truncate(out))
	}
	if strings.Contains(out, "<kratos-bin>") {
		t.Errorf("expected <kratos-bin> resolved across body+slice+suffix, got:\n%s", truncate(out))
	}
	if !strings.Contains(out, "/resolved/root") {
		t.Errorf("expected resolved root to appear in combined output, got:\n%s", truncate(out))
	}
}

func truncate(s string) string {
	const max = 2000
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
