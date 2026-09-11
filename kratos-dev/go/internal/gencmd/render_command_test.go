package gencmd

import (
	"strings"
	"testing"
)

// TestRenderCommandUniversalResolveLoader guards the KRATOS_ROOT deterministic
// resolution change: every god's generated launcher must load its agent body
// via `launch.cjs agent load <name> --resolve` (never the bare !cat loader),
// so the emitted body is pre-resolved and carries no <KRATOS_ROOT> tokens at
// spawn/inline time — in two `--part` lines, each under Claude Code's
// 30,000-char inline limit. Suffix gods append --mode=command to extras.
func TestRenderCommandUniversalResolveLoader(t *testing.T) {
	plain := &Agent{Name: "ares", Description: "Implementation specialist for writing code"}
	out := RenderCommand(plain, nil, false)

	wantBody := "!`node \"${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs\" agent load ares --resolve --part body`"
	wantExtras := "!`node \"${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs\" agent load ares --resolve --part extras`"
	for _, want := range []string{wantBody, wantExtras} {
		if !strings.Contains(out, want) {
			t.Errorf("expected loader line %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "agent load ares --resolve`") {
		t.Errorf("single-line loader must be gone (31-34 KB output is persisted, not inlined), got:\n%s", out)
	}
	if got := strings.Count(out, "\n!`"); got != 3 {
		t.Errorf("expected exactly 3 dynamic-injection lines (echo, body, extras), got %d:\n%s", got, out)
	}
	if strings.Contains(out, `!cat "${CLAUDE_PLUGIN_ROOT}/agents/ares.md"`) {
		t.Errorf("expected no plain !cat loader, got:\n%s", out)
	}
	if strings.Contains(out, "--mode=command") {
		t.Errorf("expected no --mode=command for a god with no suffix loader, got:\n%s", out)
	}
	// The echo line stays — launcher-static partial text still carries tokens.
	if !strings.Contains(out, "!`echo \"KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}\"`") {
		t.Errorf("expected !echo KRATOS_ROOT line to be retained, got:\n%s", out)
	}
	if !strings.Contains(out, "If the definition above is a `<persisted-output>` preview") {
		t.Errorf("expected the persisted-output fallback clause, got:\n%s", out)
	}
}

// TestRenderCommandSuffixLoaderAppendsModeCommand verifies suffix gods (e.g.
// hermes, which carries a command-mode-suffix/*.md) get the same --resolve
// loaders, with --mode=command on the extras line only.
func TestRenderCommandSuffixLoaderAppendsModeCommand(t *testing.T) {
	suffixed := &Agent{Name: "hermes", Description: "Code reviewer for quality and correctness"}
	out := RenderCommand(suffixed, nil, true)

	wantExtras := "!`node \"${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs\" agent load hermes --resolve --part extras --mode=command`"
	if !strings.Contains(out, wantExtras) {
		t.Errorf("expected loader line %q in output, got:\n%s", wantExtras, out)
	}
	wantBody := "!`node \"${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs\" agent load hermes --resolve --part body`"
	if !strings.Contains(out, wantBody) {
		t.Errorf("expected body loader line %q without --mode=command, got:\n%s", wantBody, out)
	}
	// extras line + the fallback sentence that repeats the command.
	if got := strings.Count(out, "--mode=command"); got != 2 {
		t.Errorf("expected --mode=command exactly twice (extras line, fallback), got %d:\n%s", got, out)
	}
}
