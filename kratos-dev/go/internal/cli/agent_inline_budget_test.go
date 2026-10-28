package cli

import (
	"strings"
	"testing"
	"unicode/utf8"
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

// hookOutputCap is the 10,000 characters Claude Code accepts from one hook
// output: an additionalContext string or plain stdout
// (https://code.claude.com/docs/en/hooks.md, "JSON output"). Larger text is
// written to a file and the model sees a 2,000-character preview. Counted in
// characters (UTF-16 code units in Claude Code; runes here).
const hookOutputCap = 10000

// protocolBlockBudget leaves room under hookOutputCap for the binary, root and
// session lines, the lessons block and the budget-drop notice that
// hooks/path-inject.cjs joins to the protocol block. extrasBudget is the
// 500-character margin the SessionStart extras mode and the SubagentStart hook
// enforce at run time (hooks/hook-budget.cjs, HOOK_OUTPUT_BUDGET).
const (
	protocolBlockBudget = 8600
	extrasBudget        = hookOutputCap - 500
)

// The protocol block every god receives from the SubagentStart hook must fit
// the hook cap with a resolved root and binary of realistic length.
// `agent load --part extras` of the inline gods (SessionStart replay after a
// compaction) must fit it too.
func TestAgentProtocolFitsHookCap(t *testing.T) {
	entries, err := agentsFS.ReadDir("agents")
	if err != nil {
		t.Fatal(err)
	}
	root := "C:/Users/lizard_liang/.claude/plugins/cache/lizard-plugins/kratos/2.121.0"
	bin := "C:/Users/lizard_liang/.kratos/bin/kratos.exe"
	resolve := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "<KRATOS_ROOT>", root), "<kratos-bin>", bin)
	}

	for _, e := range entries {
		god := strings.TrimSuffix(e.Name(), ".md")
		block, err := composeProtocolFor(god)
		if err != nil {
			t.Errorf("%s: %v", god, err)
			continue
		}
		if n := utf8.RuneCountInString(resolve(block)); n > protocolBlockBudget {
			t.Errorf("%s: `agent protocol --resolve` is %d characters (budget %d). Claude Code caps a hook output at %d characters and injects a 2,000-character preview beyond it. Trim references/agent-protocol.md.",
				god, n, protocolBlockBudget, hookOutputCap)
		}
		if god != "iris" && god != "odysseus" {
			continue
		}
		parts, err := composeAgentLoad(god, "command", "")
		if err != nil {
			t.Errorf("%s: %v", god, err)
			continue
		}
		if n := utf8.RuneCountInString(resolve(parts.Extras)); n > extrasBudget {
			t.Errorf("%s: `agent load --part extras` is %d characters (budget %d). The SessionStart extras hook cannot replay it whole.",
				god, n, extrasBudget)
		}
	}
}
