package cli

import (
	"fmt"
	"regexp"
	"strings"
)

// launcherBodyRE recognizes an expanded /kratos:<command> body by what its
// opening line *starts with*: the KRATOS_ROOT echo, the hooks/launch.cjs load,
// or a bare `agent load <god> --resolve`, each optionally behind Claude Code's
// `!` + backtick command prefix.
//
// Anchored, not "contains". An unanchored pattern classified the comment's own
// counter-example — "what does hooks/launch.cjs do?" — as a launcher body, and
// a real user turn misread as a body keeps the previous turn's spent edit
// budget and its stand-down flag.
var launcherBodyRE = regexp.MustCompile("(?i)^[!`]*\\s*(?:echo\\s+\"?KRATOS_ROOT=|KRATOS_ROOT=|node\\s+\"?[^\"]*hooks/launch\\.cjs|agent load [a-z-]+ --resolve\\b)")

// isExpandedLauncherBody reports whether prompt is a slash-command expansion
// rather than something the user typed. The marker has to open the first
// non-empty line: a launcher body starts with the KRATOS_ROOT echo and the
// launch.cjs load lines, while a user sentence that merely mentions one of
// those strings ("what does hooks/launch.cjs do?") is a real user turn.
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

// harnessPseudoPromptRE matches a UserPromptSubmit payload that is a harness
// event rather than user text: Claude Code's own <task-notification> (posted
// when a spawned subagent finishes), and a subagent hand-back, which arrives
// as "Another Claude session sent a message" followed by an
// <agent-message from="..."> block carrying the report body. Anchored to the
// start of the (trimmed) prompt — a real user turn that merely mentions
// "another Claude session" mid-sentence must still count as a user turn.
var harnessPseudoPromptRE = regexp.MustCompile(`(?i)^(?:<task-notification|<agent-message|Another Claude session sent a message)`)

// isHarnessPseudoPrompt reports whether prompt is a harness event, not
// something the user typed. A hand-back's own report body is model output: it
// must not refill the inline edit-gate's file budget, grant or clear
// gate_bypass (gateBypassRE would otherwise run against a report line opening
// with "you do …"), or reach keyword detection (a report naming "ares,
// hermes" fired the Kratos keyword injection in the 2026-09-18 review).
func isHarnessPseudoPrompt(prompt string) bool {
	return harnessPseudoPromptRE.MatchString(strings.TrimSpace(prompt))
}

// slashGodRE matches the launcher invocation as the user types it.
var slashGodRE = regexp.MustCompile(`(?i)^\s*/kratos:([a-z-]+)\b`)

// inlineGodAliases maps a slash command that loads a god under another name.
// /kratos:plan is Odysseus running inline (commands/plan.md loads
// `agent load odysseus`), so plan mode must reach the Odysseus gate rules.
var inlineGodAliases = map[string]string{"plan": "odysseus"}

// gateBypassRE matches the user explicitly handing the work back to the model.
// Every alternative is an instruction, not a topic, and every one of them is
// anchored to the start of a line.
//
// Unanchored alternatives made the stand-down quotable: the gate's own deny
// message and README.md both contain "do it yourself" verbatim, so pasting
// either one into the prompt switched the gate off for the turn. A bare
// \binline\b matches prose about the gate itself ("the inline gate is broken")
// and a bare \byou do\b matched ordinary questions ("can you do a quick
// review?", "explain how you do it in the docs").
//
// Multi-line, because the instruction is often the last line of a longer
// prompt. The cost of the anchor is that a mid-sentence stand-down no longer
// counts ("just do it yourself" needs to be its own line) — an explicit
// instruction the user can repeat, traded against a bypass anyone can quote.
var gateBypassRE = regexp.MustCompile(`(?im)^\s*(?:you do\b|do it (?:yourself|inline)\b|do (?:this|that) inline\b|inline it\b)`)

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
// with an action verb: "pass it to ares", "send to ares", "have ares fix it",
// "ask hermes", "ares, look at this", "get odysseus to plan it", "plan it with
// odysseus" (the last form fell through to the full kratos:auto block in the
// 2026-09 review and cost four tool calls before reaching the same god).
func addressedGodRE(god string) *regexp.Regexp {
	g := regexp.QuoteMeta(god)
	return regexp.MustCompile(`(?i)(?:\b(?:pass|hand|send|give|forward)(?:\s+(?:it|this|that|these|those|the\s+\w+))?\s+to\s+` + g +
		`\b|\b(?:have|let|get|ask|tell|use|run|spawn|launch|call|summon)\s+` + g +
		`\b|\b(?:plan|design|debug|review|fix|implement|build|discuss|research)\b[^.!?]{0,40}\bwith\s+` + g +
		`\b|^\s*` + g + `\s*[,:]|\b` + g + `\s*[,:]\s)`)
}

// The complaint patterns match a prompt that questions earlier work: it opens with
// "why" ("why did you…", "why didn't…") or says the work ran "without having
// <god>". Such a prompt names a god but is not a request for one — the full
// "Do NOT respond" block made the model route instead of answering (2026-09
// review).
var (
	complaintWhyRE     = regexp.MustCompile(`(?i)^\s*why\b`)
	complaintWithoutRE = regexp.MustCompile(`(?i)\bwithout\s+(?:having|asking|using)\s+(\w+)\b`)
)

// isComplaint reports whether cleaned questions earlier work. The "without
// having <X>" form counts only when <X> is one of the matched keywords.
func isComplaint(matched []string, cleaned string) bool {
	if complaintWhyRE.MatchString(cleaned) {
		return true
	}
	for _, m := range complaintWithoutRE.FindAllStringSubmatch(cleaned, -1) {
		for _, kw := range matched {
			if strings.EqualFold(m[1], kw) {
				return true
			}
		}
	}
	return false
}

// modelOverrideRE captures a model the user pairs with the request: "using
// fable", "with opus", "on sonnet".
var modelOverrideRE = regexp.MustCompile(`(?i)\b(?:using|with|on)\s+(opus|sonnet|haiku|fable)\b`)

func titleCase(name string) string {
	return strings.ToUpper(name[:1]) + name[1:]
}

// modelRoute is the direct route for god when the user picked a model. The
// Skill route cannot take a model, so every god — Odysseus included — is
// spawned through the Agent tool.
func modelRoute(god, model string) string {
	if god == "odysseus" {
		return fmt.Sprintf("Agent(subagent_type: \"kratos:odysseus\", model: \"%s\") instead of the inline kratos:plan skill — tell Odysseus to return his blocking questions in his final message, then ask the user those questions", model)
	}
	return strings.Replace(directRouteGods[god], `")`, fmt.Sprintf(`", model: "%s")`, model), 1)
}

// buildKeywordContext decides what a keyword match injects:
//   - A complaint about earlier work gets a soft one-line note: answer first.
//   - Exactly one direct-route god addressed by name gets a one-line routing
//     hint — loading kratos:auto first only added a skill load and a banner
//     before routing to that same god (4 of 5 fires in the 2026-09 review).
//     A model named with it ("using fable") switches the route to an Agent
//     spawn with that model.
//   - Everything else keeps the full activation block.
func buildKeywordContext(matched []string, cleaned string) string {
	if isComplaint(matched, cleaned) {
		var gods []string
		for _, kw := range matched {
			gods = append(gods, titleCase(kw))
		}
		name := strings.Join(gods, "/")
		return fmt.Sprintf("[KRATOS NOTE] The user mentions %s while questioning earlier work — answer them first; route to %s only if they ask.", name, name)
	}
	if len(matched) == 1 {
		god := matched[0]
		if route, ok := directRouteGods[god]; ok && addressedGodRE(god).MatchString(cleaned) {
			if mm := modelOverrideRE.FindStringSubmatch(cleaned); mm != nil {
				route = modelRoute(god, strings.ToLower(mm[1]))
			}
			return fmt.Sprintf("[KRATOS ROUTE] The user addressed %s directly. Route straight to that god — %s — and skip the kratos:auto router.",
				titleCase(god), route)
		}
	}
	return buildInjectionContext(matched)
}
