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
// rather than something the user typed. The marker has to be on the opening
// line: a launcher body starts with the KRATOS_ROOT echo and the launch.cjs
// load lines, while a user sentence that merely quotes one of those strings
// ("what does hooks/launch.cjs do?") is a real user turn — and a real turn
// misread as a body kept the previous turn's spent edit budget.
func isExpandedLauncherBody(prompt string) bool {
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return launcherBodyRE.MatchString(line)
	}
	return false
}

// slashGodRE matches the launcher invocation as the user types it.
var slashGodRE = regexp.MustCompile(`(?i)^\s*/kratos:([a-z-]+)\b`)

// inlineGodAliases maps a slash command that loads a god under another name.
// /kratos:plan is Odysseus running inline (commands/plan.md loads
// `agent load odysseus`), so plan mode must reach the Odysseus gate rules.
var inlineGodAliases = map[string]string{"plan": "odysseus"}

// gateBypassRE matches the user explicitly handing the work back to the model.
// Every alternative is an instruction, not a topic: a bare \binline\b matches
// prose about the gate itself ("the inline gate is broken"), and a bare
// \byou do\b matched ordinary questions ("can you do a quick review?", "why did
// you do that?") and switched the gate off for the turn. The instruction shape
// actually observed — "you do the html part" — opens the prompt, so that one is
// anchored.
var gateBypassRE = regexp.MustCompile(`(?i)^\s*you do\b|\byou do it\b|\bdo it yourself\b|\bdo (?:it|this|that) inline\b|\binline it\b`)

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
