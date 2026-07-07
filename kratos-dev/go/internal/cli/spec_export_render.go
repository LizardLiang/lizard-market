package cli

import (
	"embed"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
	"time"
)

// Hand-rolled markdown-to-HTML renderer for the constrained spec-template grammar
// (## Purpose/## Requirements, ### Requirement:, SHALL paragraphs, #### Scenario:,
// - **WHEN/THEN/AND** bullets, generic bullets, **bold**, `inline code`). No
// third-party markdown library: every line is escaped first (html.EscapeString)
// and only the recognized constructs above are then wrapped in markup. Anything
// outside the grammar — a stray markdown table, for instance — falls through the
// default case below and renders as an escaped plain-text paragraph: never
// dropped, never interpreted as markup.

//go:embed export/shell.html
var specExportShellFS embed.FS

var (
	scenarioLineRE = regexp.MustCompile(`^####\s+Scenario:\s*(.*)$`)
	whenThenLineRE = regexp.MustCompile(`^-\s+\*\*(WHEN|THEN|AND)\*\*\s*(.*)$`)
	bulletLineRE   = regexp.MustCompile(`^-\s+(.*)$`)
	boldRE         = regexp.MustCompile(`\*\*(.+?)\*\*`)
	inlineCodeRE   = regexp.MustCompile("`([^`]+)`")
)

// specExportPageData is the root template data for export/shell.html.
type specExportPageData struct {
	GeneratedAt  string
	Capabilities []specExportCapabilityView
}

// specExportCapabilityView is one capability's rendered view. PurposeHTML is
// already-safe HTML (escaped, then markup-wrapped by renderPurposeHTML) — the
// other fields are plain strings and rely on html/template's own contextual
// auto-escaping when interpolated.
type specExportCapabilityView struct {
	Slug         string
	Title        string
	Author       string
	Updated      string
	GitHash      string
	PurposeHTML  template.HTML
	Requirements []specExportRequirementView
}

// specExportRequirementView is one requirement's rendered view. BodyHTML is
// already-safe HTML, built the same way as PurposeHTML.
type specExportRequirementView struct {
	ID       string
	Name     string
	BodyHTML template.HTML
}

// renderInline escapes a line of spec-derived text first, then wraps the two
// recognized inline constructs (bold, inline code) in markup. Because escaping
// happens before markup is applied, a literal "&", "<", or ">" in the source
// always survives as its escaped entity rather than being interpreted as HTML.
func renderInline(s string) string {
	escaped := html.EscapeString(s)
	escaped = boldRE.ReplaceAllString(escaped, "<strong>$1</strong>")
	escaped = inlineCodeRE.ReplaceAllString(escaped, "<code>$1</code>")
	return escaped
}

// renderBlockHTML converts a block of spec markdown (already newline-normalized)
// into HTML: blank-line-separated paragraphs, #### Scenario: headings, and
// - **WHEN/THEN/AND**/generic bullet lists. Any line matching none of those
// shapes is treated as plain paragraph text — the escape-first fallback that
// keeps unrecognized constructs (e.g. a markdown table row) visible rather than
// silently dropped.
func renderBlockHTML(text string) string {
	lines := strings.Split(text, "\n")

	var out strings.Builder
	var paragraphBuf []string
	var listBuf []string

	flushParagraph := func() {
		if len(paragraphBuf) == 0 {
			return
		}
		out.WriteString("<p>")
		out.WriteString(renderInline(strings.Join(paragraphBuf, " ")))
		out.WriteString("</p>\n")
		paragraphBuf = nil
	}
	flushList := func() {
		if len(listBuf) == 0 {
			return
		}
		out.WriteString("<ul>\n")
		for _, item := range listBuf {
			out.WriteString("<li>" + item + "</li>\n")
		}
		out.WriteString("</ul>\n")
		listBuf = nil
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			flushParagraph()
			flushList()
		case scenarioLineRE.MatchString(line):
			flushParagraph()
			flushList()
			m := scenarioLineRE.FindStringSubmatch(line)
			out.WriteString("<h4 class=\"scenario\">Scenario: " + renderInline(m[1]) + "</h4>\n")
		case whenThenLineRE.MatchString(line):
			flushParagraph()
			m := whenThenLineRE.FindStringSubmatch(line)
			listBuf = append(listBuf, "<strong class=\"kw\">"+m[1]+"</strong> "+renderInline(m[2]))
		case bulletLineRE.MatchString(line):
			flushParagraph()
			m := bulletLineRE.FindStringSubmatch(line)
			listBuf = append(listBuf, renderInline(m[1]))
		default:
			flushList()
			paragraphBuf = append(paragraphBuf, line)
		}
	}
	flushParagraph()
	flushList()

	return out.String()
}

// renderPurposeHTML renders a shard's ## Purpose section.
func renderPurposeHTML(purpose string) string {
	return renderBlockHTML(normalizeNewlines(purpose))
}

// renderRequirementBodyHTML renders one requirement's body, skipping its own
// "### Requirement: <Name>" header line (the name is displayed separately by the
// caller, in the <summary> element).
func renderRequirementBodyHTML(body string) string {
	text := normalizeNewlines(body)
	if idx := strings.IndexByte(text, '\n'); idx != -1 {
		if reqHeaderRE.MatchString(strings.TrimSpace(text[:idx])) {
			text = text[idx+1:]
		}
	} else if reqHeaderRE.MatchString(strings.TrimSpace(text)) {
		text = ""
	}
	return renderBlockHTML(text)
}

// buildExportPageData converts parsed spec shards into template data, applying
// the hand-rolled renderer to each shard's Purpose and each requirement's Body.
func buildExportPageData(shards []specExportShard) specExportPageData {
	data := specExportPageData{
		GeneratedAt: time.Now().UTC().Format(exportTimestampFormat),
	}
	for _, s := range shards {
		title := shardTitle(s.Shard, s.Capability)
		capView := specExportCapabilityView{
			Slug:        s.Capability,
			Title:       title,
			Author:      s.Shard.FrontMatter["author"],
			Updated:     s.Shard.FrontMatter["updated"],
			GitHash:     s.Shard.FrontMatter["git_hash"],
			PurposeHTML: template.HTML(renderPurposeHTML(s.Shard.Purpose)),
		}
		for i, r := range s.Shard.Requirements {
			capView.Requirements = append(capView.Requirements, specExportRequirementView{
				ID:       fmt.Sprintf("req-%s-%d", s.Capability, i),
				Name:     r.Name,
				BodyHTML: template.HTML(renderRequirementBodyHTML(r.Body)),
			})
		}
		data.Capabilities = append(data.Capabilities, capView)
	}
	return data
}

// renderSpecExportHTML fills the embedded, self-contained HTML shell (inline
// CSS/JS, zero external resources) with the given shards' data.
func renderSpecExportHTML(shards []specExportShard) (string, error) {
	raw, err := specExportShellFS.ReadFile("export/shell.html")
	if err != nil {
		return "", fmt.Errorf("cannot read embedded export shell: %w", err)
	}
	tmpl, err := template.New("shell").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("cannot parse export shell template: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, buildExportPageData(shards)); err != nil {
		return "", fmt.Errorf("cannot render export html: %w", err)
	}
	return buf.String(), nil
}
