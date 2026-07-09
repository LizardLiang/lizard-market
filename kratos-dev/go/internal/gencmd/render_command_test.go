package gencmd

import (
	"strings"
	"testing"
)

// TestRenderCommandUniversalResolveLoader guards the KRATOS_ROOT deterministic
// resolution change: every god's generated launcher must load its agent body
// via `launch.cjs agent load <name> --resolve` (never the bare !cat loader),
// so the emitted body is pre-resolved and carries no <KRATOS_ROOT> tokens at
// spawn/inline time. Suffix gods additionally append --mode=command.
func TestRenderCommandUniversalResolveLoader(t *testing.T) {
	plain := &Agent{Name: "ares", Description: "Implementation specialist for writing code"}
	out := RenderCommand(plain, nil, false)

	wantLoader := `!node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load ares --resolve`
	if !strings.Contains(out, wantLoader) {
		t.Errorf("expected loader line %q in output, got:\n%s", wantLoader, out)
	}
	if strings.Contains(out, `!cat "${CLAUDE_PLUGIN_ROOT}/agents/ares.md"`) {
		t.Errorf("expected no plain !cat loader, got:\n%s", out)
	}
	if strings.Contains(out, "--mode=command") {
		t.Errorf("expected no --mode=command for a god with no suffix loader, got:\n%s", out)
	}
	// The echo line stays — launcher-static partial text still carries tokens.
	if !strings.Contains(out, `!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"`) {
		t.Errorf("expected !echo KRATOS_ROOT line to be retained, got:\n%s", out)
	}
}

// TestRenderCommandSuffixLoaderAppendsModeCommand verifies suffix gods (e.g.
// hermes, which carries a command-mode-suffix/*.md) get the same --resolve
// loader plus --mode=command.
func TestRenderCommandSuffixLoaderAppendsModeCommand(t *testing.T) {
	suffixed := &Agent{Name: "hermes", Description: "Code reviewer for quality and correctness"}
	out := RenderCommand(suffixed, nil, true)

	wantLoader := `!node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load hermes --resolve --mode=command`
	if !strings.Contains(out, wantLoader) {
		t.Errorf("expected loader line %q in output, got:\n%s", wantLoader, out)
	}
}
