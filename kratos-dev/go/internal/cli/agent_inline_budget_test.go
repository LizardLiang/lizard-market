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

// protocolBlockBudget bounds the protocol block a god gets from the
// SubagentStart hook. The run-time budget is 9,500 characters
// (hooks/hook-budget.cjs, HOOK_OUTPUT_BUDGET). The head parts (binary, root and
// session lines) measure about 570 characters with a long Windows project path,
// since the path appears in two of them; lessons add at most about 350. So a
// block of 8,700 fits only without lessons (hephaestus, the largest god, gives
// 9,475 in total with no lessons), and with lessons the run-time fitter drops
// protocol sections and prints its notice. extrasBudget is the same 500-character
// margin for the SessionStart extras mode.
const (
	protocolBlockBudget = 8700
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
