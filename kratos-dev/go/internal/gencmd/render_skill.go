package gencmd

import (
	"fmt"
	"regexp"
	"strings"
)

// Fixed prose surrounding the god-name list in SKILL.md's frontmatter
// description. Only the list itself (CanonicalOrder) is generated; this
// prose is constant and must be edited by hand if it ever needs to change.
const (
	skillDescPrefix = `Kratos orchestrator activated by: "Kratos" keyword, any god-agent name (`
	skillDescSuffix = `), "continue"/"next stage" during active pipelines, or queries about features, PRDs, specs, code reviews, and implementation. When unsure, activate.`
)

var (
	skillDescBlockRe = regexp.MustCompile(`(?m)^description: >-\n(?:  .*\n)+`)
	quickBulletRe    = regexp.MustCompile(`- Quick-mode gods \([^)]*\): read`)
	skillBulletRe    = regexp.MustCompile(`- All other gods \([^)]*\): invoke`)
)

// RenderSkillGodRegions rewrites the god-derived regions of
// skills/auto/SKILL.md: the frontmatter description's god-name list and the
// two Activation bullets (Quick-mode gods / All other gods). Everything else
// in content is left untouched. Fails loud if the roster in agents/*.md
// doesn't reconcile with the hand-curated orders in roster.go, or if the
// expected regions can't be located (file structure changed underneath it).
func RenderSkillGodRegions(content string, agents map[string]*Agent) (string, error) {
	if err := validateRoster(agents); err != nil {
		return "", err
	}

	descText := skillDescPrefix + strings.Join(CanonicalOrder, ", ") + skillDescSuffix
	wrapped := wordWrap(descText, "  ", 78)
	newDescBlock := "description: >-\n" + strings.Join(wrapped, "\n") + "\n"

	if !skillDescBlockRe.MatchString(content) {
		return "", fmt.Errorf("SKILL.md: could not find 'description: >-' folded block")
	}
	content = skillDescBlockRe.ReplaceAllLiteralString(content, newDescBlock)

	quickList := "<!-- gen:quick-gods -->" + strings.Join(QuickGodOrder, ", ") + "<!-- /gen:quick-gods -->"
	if !quickBulletRe.MatchString(content) {
		return "", fmt.Errorf("SKILL.md: could not find the Quick-mode gods Activation bullet")
	}
	content = quickBulletRe.ReplaceAllLiteralString(content, "- Quick-mode gods ("+quickList+"): read")

	skillList := "<!-- gen:skill-gods -->" + strings.Join(OwnCommandGodOrder, ", ") + "<!-- /gen:skill-gods -->"
	if !skillBulletRe.MatchString(content) {
		return "", fmt.Errorf("SKILL.md: could not find the All-other-gods Activation bullet")
	}
	content = skillBulletRe.ReplaceAllLiteralString(content, "- All other gods ("+skillList+"): invoke")

	return content, nil
}

// validateRoster cross-checks the hand-curated display orders in roster.go
// against the live agents/*.md roster: every agent must appear in exactly one
// of QuickGodOrder/OwnCommandGodOrder, consistent with its quick_route field,
// and CanonicalOrder must contain exactly the same set of gods.
func validateRoster(agents map[string]*Agent) error {
	canonical := map[string]bool{}
	for _, n := range CanonicalOrder {
		canonical[n] = true
	}
	agentDisplay := map[string]bool{}
	for name := range agents {
		agentDisplay[capitalize(name)] = true
	}

	for _, n := range CanonicalOrder {
		if !agentDisplay[n] {
			return fmt.Errorf("gencmd.CanonicalOrder lists %q but no agents/%s.md exists", n, strings.ToLower(n))
		}
	}
	for name := range agents {
		disp := capitalize(name)
		if !canonical[disp] {
			return fmt.Errorf("agents/%s.md has no entry in gencmd.CanonicalOrder — add it (see kratos-dev/codegen/README.md)", name)
		}
	}

	quick := map[string]bool{}
	for _, n := range QuickGodOrder {
		quick[n] = true
	}
	own := map[string]bool{}
	for _, n := range OwnCommandGodOrder {
		own[n] = true
	}

	for name, a := range agents {
		disp := capitalize(name)
		if quick[disp] && own[disp] {
			return fmt.Errorf("%s appears in both gencmd.QuickGodOrder and gencmd.OwnCommandGodOrder", disp)
		}
		if a.QuickRoute {
			if !quick[disp] {
				return fmt.Errorf("agents/%s.md has quick_route: true but is missing from gencmd.QuickGodOrder", name)
			}
		} else {
			if !own[disp] {
				return fmt.Errorf("agents/%s.md is not quick_route but is missing from gencmd.OwnCommandGodOrder", name)
			}
		}
	}

	if len(QuickGodOrder)+len(OwnCommandGodOrder) != len(CanonicalOrder) {
		return fmt.Errorf(
			"gencmd.QuickGodOrder (%d) + gencmd.OwnCommandGodOrder (%d) != gencmd.CanonicalOrder (%d)",
			len(QuickGodOrder), len(OwnCommandGodOrder), len(CanonicalOrder),
		)
	}

	return nil
}
