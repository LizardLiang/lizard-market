package cli

import (
	"fmt"
	"regexp"
	"strings"
)

// launcherBodyRE recognizes an expanded /kratos:<command> body: the generated
// launchers echo KRATOS_ROOT and load the agent via hooks/launch.cjs.
var launcherBodyRE = regexp.MustCompile(`(?i)KRATOS_ROOT=|hooks/launch\.cjs|\bagent load [a-z-]+ --resolve\b`)

// isExpandedLauncherBody reports whether prompt is a slash-command expansion
// rather than something the user typed.
func isExpandedLauncherBody(prompt string) bool {
	return launcherBodyRE.MatchString(prompt)
}

// inlineGodRE pulls the god out of an expanded launcher body. Claude Code
// 2.1.268 delivers the literal `/kratos:<command>` text to UserPromptSubmit
// instead of the expansion (verified 2026-09-11 with a payload probe), so
// slashGodRE below is the path that fires in practice — this one is kept
// because isExpandedLauncherBody shows other harness versions do send bodies.
var inlineGodRE = regexp.MustCompile(`(?i)\bagent load ([a-z-]+) --resolve\b`)

// slashGodRE matches the launcher invocation as the user types it.
var slashGodRE = regexp.MustCompile(`(?i)^\s*/kratos:([a-z-]+)\b`)

// inlineGodAliases maps a slash command that loads a god under another name.
// /kratos:plan is Odysseus running inline (commands/plan.md loads
// `agent load odysseus`), so plan mode must reach the Odysseus gate rules.
var inlineGodAliases = map[string]string{"plan": "odysseus"}

// gateBypassRE matches the user explicitly handing the work back to the model.
// Deliberately narrower than a bare \binline\b, which matches ordinary prose
// about the gate itself ("the inline gate is broken") and would switch it off.
var gateBypassRE = regexp.MustCompile(`(?i)\byou do\b|\bdo it yourself\b|\bdo (?:it|this|that) inline\b|\binline it\b`)

// directRouteGods are the gods a user can address by name and get directly,
// without the kratos:auto router in between. Odysseus runs inline (plan
// mode); the others are spawned with the quick.md template.
var directRouteGods = map[string]string{
	"ares":     "Agent(subagent_type: \"kratos:ares\") with the spawn template in commands/quick.md (ORIGINAL_USER_REQUEST verbatim, mode acceptEdits), then the quick.md post-task",
	"hermes":   "Agent(subagent_type: \"kratos:hermes\") with the quick.md review template",
	"artemis":  "Agent(subagent_type: \"kratos:artemis\") with the quick.md test-plan template",
	"metis":    "Agent(subagent_type: \"kratos:metis\") with the quick.md research template",
	"daedalus": "Agent(subagent_type: \"kratos:daedalus\") with the quick.md decomposition template",
	"hades":    "Agent(subagent_type: \"kratos:hades\") with the quick.md debug template",
	"odysseus": "Skill(skill: \"kratos:plan\") — Odysseus runs inline so his questions reach the user",
}

// addressedGodRE returns a pattern matching the user addressing god by name
// with an action verb: "pass it to ares", "have ares fix it", "ask hermes",
// "ares, look at this", "get odysseus to plan it", "plan it with odysseus"
// (the last form fell through to the full kratos:auto block in the 2026-09
// review and cost four tool calls before reaching the same god).
func addressedGodRE(god string) *regexp.Regexp {
	g := regexp.QuoteMeta(god)
	return regexp.MustCompile(`(?i)(?:\b(?:pass|hand|send|give|forward)\s+(?:it|this|that|these|those|the\s+\w+)\s+to\s+` + g +
		`\b|\b(?:have|let|get|ask|tell|use|run|spawn|launch|call|summon)\s+` + g +
		`\b|\b(?:plan|design|debug|review|fix|implement|build|discuss|research)\b[^.!?]{0,40}\bwith\s+` + g +
		`\b|^\s*` + g + `\s*[,:]|\b` + g + `\s*[,:]\s)`)
}

// buildKeywordContext decides what a keyword match injects. When the user
// addressed exactly one direct-route god by name, a one-line routing hint is
// enough — loading kratos:auto first only added a skill load and a banner
// before routing to that same god (4 of 5 fires in the 2026-09 review).
// Everything else keeps the full activation block.
func buildKeywordContext(matched []string, cleaned string) string {
	if len(matched) == 1 {
		god := matched[0]
		if route, ok := directRouteGods[god]; ok && addressedGodRE(god).MatchString(cleaned) {
			return fmt.Sprintf("[KRATOS ROUTE] The user addressed %s directly. Route straight to that god — %s — and skip the kratos:auto router.",
				strings.ToUpper(god[:1])+god[1:], route)
		}
	}
	return buildInjectionContext(matched)
}
