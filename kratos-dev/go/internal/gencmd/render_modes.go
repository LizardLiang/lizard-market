package gencmd

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	modelMatrixRe        = regexp.MustCompile(`(?s)<!-- gen:model-matrix -->\n.*?\n<!-- /gen:model-matrix -->`)
	modelMatrixSummaryRe = regexp.MustCompile(`(?s)<!-- gen:model-matrix-summary -->\n.*?\n<!-- /gen:model-matrix-summary -->`)
)

// RenderModesModelMatrix rewrites the generated Model Matrix table and its
// derived summary line in modes/modes.md between HTML comment markers (same
// pattern as SKILL.md's gen:quick-gods region — see render_skill.go).
// Rows: pipeline gods (those with a non-empty stage: frontmatter field)
// first, ordered by stage; then the remaining gods alphabetically (ties
// within the same stage, e.g. Hermes/Cassandra both stage 9, preserve
// alphabetical order). Everything else in content is left untouched. Fails
// loud if the markers can't be located — model field validity is already
// enforced by LoadAgents.
func RenderModesModelMatrix(content string, agents map[string]*Agent) (string, error) {
	rows := orderedMatrixRows(agents)

	var table strings.Builder
	table.WriteString("<!-- gen:model-matrix -->\n")
	table.WriteString("| Agent | Stage | Normal | Eco | Power |\n")
	table.WriteString("|-------|-------|--------|-----|-------|\n")
	for _, a := range rows {
		fmt.Fprintf(&table, "| **%s** | %s | %s | %s | %s |\n", capitalize(a.Name), a.Stage, a.Model, a.ModelEco, a.ModelPower)
	}
	table.WriteString("<!-- /gen:model-matrix -->")

	if !modelMatrixRe.MatchString(content) {
		return "", fmt.Errorf("modes.md: could not find the gen:model-matrix region")
	}
	content = modelMatrixRe.ReplaceAllLiteralString(content, table.String())

	summary := "<!-- gen:model-matrix-summary -->\n" + renderModelMatrixSummary(rows) + "\n<!-- /gen:model-matrix-summary -->"
	if !modelMatrixSummaryRe.MatchString(content) {
		return "", fmt.Errorf("modes.md: could not find the gen:model-matrix-summary region")
	}
	content = modelMatrixSummaryRe.ReplaceAllLiteralString(content, summary)

	return content, nil
}

// orderedMatrixRows returns every agent, pipeline gods (stage != "") first
// ordered by stage's leading numeric prefix, then the rest alphabetically.
// SortedNames already yields alphabetical order, and the subsequent
// SliceStable sort-by-stage preserves that order for agents sharing a stage
// (e.g. Cassandra before Hermes, both stage "9").
func orderedMatrixRows(agents map[string]*Agent) []*Agent {
	var staged, rest []*Agent
	for _, name := range SortedNames(agents) {
		a := agents[name]
		if a.Stage != "" {
			staged = append(staged, a)
		} else {
			rest = append(rest, a)
		}
	}
	// Plain lexicographic comparison on the stage string already yields the
	// intended pipeline order for every stage value in use ("0" < "1" < "2"
	// < "2→3" < "4" < ... < "9") because "2" is a strict prefix of "2→3"
	// (shorter sorts first) and every other stage differs in its first
	// digit. SliceStable preserves the incoming alphabetical order (from
	// SortedNames) for agents sharing a stage, e.g. Cassandra before Hermes
	// (both stage "9").
	sort.SliceStable(staged, func(i, j int) bool {
		return staged[i].Stage < staged[j].Stage
	})
	return append(staged, rest...)
}

// modelCounts tallies how many agents route to each alias for one mode
// (normal/eco/power), used to derive the summary cost line.
type modelCounts struct{ opus, sonnet, haiku int }

func (c *modelCounts) add(alias string) {
	switch alias {
	case "opus":
		c.opus++
	case "sonnet":
		c.sonnet++
	case "haiku":
		c.haiku++
	}
}

func (c modelCounts) String() string {
	return fmt.Sprintf("%d Opus / %d Sonnet / %d Haiku", c.opus, c.sonnet, c.haiku)
}

// renderModelMatrixSummary derives the "Summary: Normal ... Eco ... Power
// ..." cost line from the same agent rows the table renders — so it can
// never drift from the matrix the way the old hand-maintained line could.
func renderModelMatrixSummary(rows []*Agent) string {
	var normal, eco, power modelCounts
	for _, a := range rows {
		normal.add(a.Model)
		eco.add(a.ModelEco)
		power.add(a.ModelPower)
	}
	return fmt.Sprintf(
		"Summary: Normal ≈ %s · Eco ≈ %s (~70-85%% cheaper) · Power ≈ %s (~2-7× cost).",
		normal.String(), eco.String(), power.String(),
	)
}
