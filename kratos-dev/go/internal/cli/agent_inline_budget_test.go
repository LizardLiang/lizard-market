package cli

import (
	"strings"
	"testing"
)

// inlineInjectionBudget is 1,000 bytes under the 30,000 characters Claude Code
// inlines for a !`cmd` line in a slash command (the Bash tool's limit,
// https://code.claude.com/docs/en/tools-reference.md). Larger output is
// written to a file and the model sees a 2 KB preview — which is how every
// inline god ran on a truncated definition in the 2026-09 review. Bytes are
// measured, not runes: bytes ≥ characters, so the lint errs on the safe side
// and a CRLF checkout counts its extra byte per line.
const inlineInjectionBudget = 29000

// Both launcher lines of every god must stay under the budget at the worst
// case: command mode, a full lessons block (five lessons at the length cap,
// plus the retro nudge), and a long resolved root. A growing agents/<god>.md
// fails here instead of silently truncating in production.
func TestAgentLoadPartsFitInlineBudget(t *testing.T) {
	entries, err := agentsFS.ReadDir("agents")
	if err != nil {
		t.Fatal(err)
	}
	root := "C:/Users/" + strings.Repeat("x", 100) + "/.claude/plugins/cache/lizard-plugins/kratos/2.108.0"
	bin := root + "/bin/kratos-windows-amd64.exe"
	worst := make([]string, lessonsInjectMax)
	for i := range worst {
		worst[i] = strings.Repeat("字", feedbackLessonMaxLen)
	}

	for _, e := range entries {
		god := strings.TrimSuffix(e.Name(), ".md")
		parts, err := composeAgentLoad(god, "command", renderLessonsBlock(god, worst))
		if err != nil {
			t.Errorf("%s: %v", god, err)
			continue
		}
		if len(parts.Body) == 0 {
			t.Errorf("%s: empty embedded body — run `make sync-assets`", god)
		}
		for _, p := range []struct{ name, text string }{{"body", parts.Body}, {"extras", parts.Extras}} {
			resolved := strings.ReplaceAll(strings.ReplaceAll(p.text, "<KRATOS_ROOT>", root), "<kratos-bin>", bin)
			if n := len(resolved); n > inlineInjectionBudget {
				t.Errorf("%s: `agent load --part %s` would emit %d bytes (budget %d). Claude Code inlines at most 30,000 chars of a !-line; larger output reaches the model as a <persisted-output> preview. Trim agents/%s.md (body) or its protocol_sections / command-mode suffix (extras).",
					god, p.name, n, inlineInjectionBudget, god)
			}
		}
	}
}
