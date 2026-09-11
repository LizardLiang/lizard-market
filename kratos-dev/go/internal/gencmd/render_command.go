package gencmd

import (
	"fmt"
	"sort"
	"strings"
)

// standardNoteSeparator is the default clause appended after "Operate **in
// the main context**" for gods that don't override it via command_note.
const standardNoteSeparator = " — do NOT spawn a subagent via the Task tool."

// commandRefsSentences is the single source of truth for the command_refs
// enum: both the set of valid values (agent.go's LoadAgents validates
// against its keys) and the "additional references" hint sentence each one
// renders (refsParagraph looks it up here). A value added to only one of
// those two use sites is impossible by construction — there's only one map.
// "" (unset) behaves like "standard"; "none" renders no sentence at all.
// agent-protocol.md is deliberately absent from every sentence: its relevant
// sections arrive pre-composed in the loaded body (agent load injects the
// protocol_sections block), so a runtime Read of it would be pure waste.
var commandRefsSentences = map[string]string{
	"none":           "",
	"standard":       "If the agent definition above requires reading additional references, read them with the Read tool before acting.",
	"templates":      "If the agent definition above requires reading additional references (e.g., templates under `templates/`), read them with the Read tool before acting.",
	"arena-protocol": "If the agent definition above requires reading additional references (e.g., `references/arena-protocol.md`), read them with the Read tool before acting.",
	"rules":          "If the agent definition above requires reading additional references (e.g., `rules/` for review standards), read them with the Read tool before acting.",
}

// sortedCommandRefsKeys returns commandRefsSentences' keys sorted, for
// building a stable, human-readable list in validation error messages.
func sortedCommandRefsKeys() []string {
	keys := make([]string, 0, len(commandRefsSentences))
	for k := range commandRefsSentences {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// capitalize upper-cases the first byte of a lowercase god name ("ares" ->
// "Ares"). All god names are plain ASCII words, so byte indexing is safe.
func capitalize(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// deriveDescription computes the command frontmatter description text from an
// agent's frontmatter description, following the observed launcher
// convention: truncate at the first semicolon, normalize " - " to " — ",
// lowercase the first letter unless the description opens with an all-caps
// acronym (PM, QA, PRD, ...), then append the pipeline stage suffix if the
// agent declares one.
func deriveDescription(a *Agent) string {
	text := a.Description
	if idx := strings.Index(text, ";"); idx >= 0 {
		text = text[:idx]
	}
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, " - ", " — ")
	text = lowerFirstUnlessAcronym(text)

	desc := fmt.Sprintf("Run as %s (%s) inline in the main session", capitalize(a.Name), text)
	if a.Stage != "" {
		desc += fmt.Sprintf(" — pipeline Stage %s", a.Stage)
	}
	return desc
}

// lowerFirstUnlessAcronym lowercases only the first rune of text, unless the
// leading word is an all-caps acronym (e.g. "PM", "QA", "PRD"), in which case
// the text is returned unchanged so the acronym isn't mangled.
func lowerFirstUnlessAcronym(text string) string {
	if text == "" {
		return text
	}
	firstWord := text
	if idx := strings.IndexByte(text, ' '); idx >= 0 {
		firstWord = text[:idx]
	}
	if firstWord != "" && strings.ToUpper(firstWord) == firstWord && strings.ToLower(firstWord) != firstWord {
		return text
	}
	return strings.ToLower(text[:1]) + text[1:]
}

// refsParagraph returns the "additional references" hint sentence for a
// command_refs enum value ("" behaves like "standard").
func refsParagraph(kind string) string {
	if kind == "" {
		kind = "standard"
	}
	return commandRefsSentences[kind]
}

// launcherBashTools are the Bash commands every launcher's dynamic-injection
// lines need pre-approved. A failed permission check on a !`cmd` line aborts
// the whole command invocation, so these must always be in allowed-tools.
var launcherBashTools = []string{"Bash(echo:*)", "Bash(node:*)"}

// LauncherAllowedTools merges a partial's allowed-tools list (may be "") with
// the launcher's own Bash pre-approvals. A bare "Bash" entry already covers
// every Bash command, so the scoped entries are omitted in that case; the
// partial's own entries always come first and are never reordered.
func LauncherAllowedTools(partialTools string) string {
	var out []string
	seen := map[string]bool{}
	bareBash := false
	for _, t := range strings.Split(partialTools, ",") {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		if t == "Bash" {
			bareBash = true
		}
		seen[t] = true
		out = append(out, t)
	}
	if !bareBash {
		for _, t := range launcherBashTools {
			if !seen[t] {
				out = append(out, t)
			}
		}
	}
	return strings.Join(out, ", ")
}

// RenderCommand renders the full commands/<name>.md launcher content for the
// given agent. Every god uses the launch.cjs loader with --resolve (so the
// emitted body carries no <KRATOS_ROOT>/<kratos-bin> tokens), split over two
// lines — `--part body` and `--part extras` — because Claude Code inlines at
// most 30,000 characters per !`cmd` line and a whole god is 31–34 KB (one
// line reached the model as a 2 KB <persisted-output> preview in the 2026-09
// review). hasSuffixLoader appends --mode=command to the extras line when a
// command-mode-suffix/<name>.md file exists. partial may be nil.
func RenderCommand(a *Agent, partial *Partial, hasSuffixLoader bool) string {
	var fm strings.Builder
	fm.WriteString("---\n")
	fmt.Fprintf(&fm, "name: %s\n", a.Name)
	fmt.Fprintf(&fm, "description: %s\n", deriveDescription(a))
	fm.WriteString("generated: true\n")
	partialTools := ""
	if partial != nil {
		partialTools = partial.AllowedTools
	}
	fmt.Fprintf(&fm, "allowed-tools: %s\n", LauncherAllowedTools(partialTools))
	fm.WriteString("---")

	// Dynamic injection uses the documented inline form: !`command`. A bare
	// "!command" line is NOT recognized by Claude Code — it reaches the model as
	// literal text and the persona never loads (observed 26/26 times in the
	// 2026-09 transcript review). Keep all three lines in the backtick form.
	echoLine := "!`echo \"KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}\"`"

	baseCmd := fmt.Sprintf(`node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load %s --resolve`, a.Name)
	bodyCmd := baseCmd + " --part body"
	extrasCmd := baseCmd + " --part extras"
	if hasSuffixLoader {
		extrasCmd += " --mode=command"
	}
	bodyLine := "!`" + bodyCmd + "`"
	extrasLine := "!`" + extrasCmd + "`"

	note := standardNoteSeparator
	if a.CommandNote != "" {
		note = a.CommandNote
	}
	persona := fmt.Sprintf(
		"You ARE %s for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context**%s",
		capitalize(a.Name), note,
	)

	fallback := fmt.Sprintf(
		"If no `# %s -` agent definition appears above, the loader did not run: execute `%s` and then `%s` once each with the Bash tool, adopt their combined output as your definition, and only then act on the request. If the definition above is a `<persisted-output>` preview instead of the full text, Read the file it names in full before acting.",
		capitalize(a.Name), bodyCmd, extrasCmd,
	)

	blocks := []string{fm.String(), echoLine, bodyLine, extrasLine, "---", persona, fallback}

	if refs := refsParagraph(a.CommandRefs); refs != "" {
		blocks = append(blocks, refs)
	}

	if partial != nil && partial.Placement == "before-request" {
		blocks = append(blocks, "---", partial.Body)
	}

	blocks = append(blocks, "Request: $ARGUMENTS")

	if partial != nil && partial.Placement == "after-request" {
		blocks = append(blocks, "---", partial.Body)
	}

	return strings.Join(blocks, "\n\n") + "\n"
}
