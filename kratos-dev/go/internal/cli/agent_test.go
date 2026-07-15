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

// TestAgentLoadResolveCommandModeCoversProtocolAndSuffix verifies --resolve
// substitutes tokens inside the injected protocol block and command-mode
// suffix, not just the agent body. hermes carries protocol_sections and a
// suffix with <KRATOS_ROOT>/<kratos-bin> tokens.
func TestAgentLoadResolveCommandModeCoversProtocolAndSuffix(t *testing.T) {
	unresolved := runAgentLoad(t, "hermes", "--mode=command")
	if !strings.Contains(unresolved, "<KRATOS_ROOT>") && !strings.Contains(unresolved, "<kratos-bin>") {
		t.Fatalf("fixture assumption broken: expected hermes --mode=command body (incl. protocol+suffix) to contain KRATOS_ROOT/kratos-bin tokens before resolving, got:\n%s", truncate(unresolved))
	}
	if !strings.Contains(unresolved, "# Agent Protocol (injected)") {
		t.Fatalf("expected hermes --mode=command output to carry the composed protocol block, got:\n%s", truncate(unresolved))
	}

	out := runAgentLoad(t, "hermes", "--mode=command", "--resolve", "--root", "/resolved/root")

	if strings.Contains(out, "<KRATOS_ROOT>") {
		t.Errorf("expected <KRATOS_ROOT> resolved across body+protocol+suffix, got:\n%s", truncate(out))
	}
	if strings.Contains(out, "<kratos-bin>") {
		t.Errorf("expected <kratos-bin> resolved across body+protocol+suffix, got:\n%s", truncate(out))
	}
	if !strings.Contains(out, "/resolved/root") {
		t.Errorf("expected resolved root to appear in combined output, got:\n%s", truncate(out))
	}
}

// runAgentProtocol executes `agent protocol` with the given args and returns
// stdout plus any execution error.
func runAgentProtocol(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := AgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"protocol"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

// TestAgentProtocolComposes verifies the composed block carries exactly the
// sections athena's protocol_sections lists — with the do-not-read header,
// without orchestrator-only sections or anchor comments.
func TestAgentProtocolComposes(t *testing.T) {
	out, err := runAgentProtocol(t, "athena")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Agent Protocol (injected)",
		"Do NOT read that file",
		"## Auto-Discovery",
		"## Status Updates via Kratos CLI",
		"## Interactive Questions (AskUserQuestion)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in athena protocol block, got:\n%s", want, truncate(out))
		}
	}
	for _, reject := range []string{
		"## Spawn Prompt Fields",
		"## Spawning Athena",
		"## Path Resolution",
		"<!-- protocol:",
	} {
		if strings.Contains(out, reject) {
			t.Errorf("expected %q excluded from athena protocol block, got:\n%s", reject, truncate(out))
		}
	}
}

// TestAgentProtocolUnknownGod verifies an unknown agent name errors.
func TestAgentProtocolUnknownGod(t *testing.T) {
	if _, err := runAgentProtocol(t, "notagod"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

// TestAgentProtocolResolve verifies --resolve leaves no tokens in the block.
func TestAgentProtocolResolve(t *testing.T) {
	out, err := runAgentProtocol(t, "athena", "--resolve", "--root", "/r")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "<KRATOS_ROOT>") || strings.Contains(out, "<kratos-bin>") {
		t.Errorf("expected tokens resolved in protocol block, got:\n%s", truncate(out))
	}
}

// TestAgentLoadInjectsProtocol verifies the block is appended on every load,
// not just --mode=command — clio has no command-mode suffix.
func TestAgentLoadInjectsProtocol(t *testing.T) {
	out := runAgentLoad(t, "clio")
	if !strings.Contains(out, "# Agent Protocol (injected)") {
		t.Errorf("expected protocol block in plain `agent load clio`, got:\n%s", truncate(out))
	}
	if !strings.Contains(out, "## Boundaries") {
		t.Errorf("expected § Boundaries in clio protocol block, got:\n%s", truncate(out))
	}
}

// TestAllAgentsProtocolSectionsCompose is an in-test drift gate (alongside
// gencmd's gen-check validation): every embedded agent's protocol_sections
// must compose without error against the embedded agent-protocol.md.
func TestAllAgentsProtocolSectionsCompose(t *testing.T) {
	entries, err := agentsFS.ReadDir("agents")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".md")
		if _, err := composeProtocolFor(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func truncate(s string) string {
	const max = 2000
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
