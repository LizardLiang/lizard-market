package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlanCommandUsesAgentLoader locks the regression class that made the
// /kratos:plan durability bug invisible.
//
// commands/plan.md used to load Odysseus with a raw `!cat` of agents/odysseus.md.
// That silently skips protocol composition and token resolution, so on the
// primary plan entry point the agent received no injected protocol sections and
// literal `<KRATOS_ROOT>` / `<kratos-bin>` strings. The generated launcher
// (commands/odysseus.md) always did it correctly, which is why the divergence
// went unnoticed. Hand-written command files that load an agent must use the
// loader too.
func TestPlanCommandUsesAgentLoader(t *testing.T) {
	planPath := filepath.Join("..", "..", "..", "..", "plugins", "kratos", "commands", "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("cannot read commands/plan.md at %s: %v", planPath, err)
	}
	body := string(data)

	if !strings.Contains(body, "agent load odysseus --resolve") {
		t.Error("commands/plan.md must load Odysseus via `agent load odysseus --resolve` so protocol sections compose and <KRATOS_ROOT>/<kratos-bin> resolve")
	}
	if strings.Contains(body, "!cat ") {
		t.Error("commands/plan.md uses a raw `!cat` loader; that skips protocol injection and token resolution")
	}
	// Two-part load: a single `agent load odysseus --resolve` line emits 32 KB,
	// over Claude Code's 30,000-char inline limit, and reaches the model as a
	// 2 KB <persisted-output> preview (2026-09 review).
	for _, want := range []string{
		"agent load odysseus --resolve --part body`",
		"agent load odysseus --resolve --part extras`",
		"<persisted-output>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("commands/plan.md missing %q — the loader must be split into --part body / --part extras lines", want)
		}
	}
	if strings.Contains(body, "agent load odysseus --resolve`") {
		t.Error("commands/plan.md still has the single-line loader; its 32 KB output is persisted, not inlined")
	}
}
