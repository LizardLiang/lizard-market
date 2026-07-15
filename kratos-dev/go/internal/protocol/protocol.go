// Package protocol parses references/agent-protocol.md into anchored sections
// and composes per-agent protocol blocks from a protocol_sections slug list.
// Used by `kratos agent protocol <god>` / `kratos agent load` (injection) and
// by gencmd (slug validation at make gen / gen-check time).
package protocol

import (
	"fmt"
	"regexp"
	"strings"
)

// Header prefixes every composed block and forbids the runtime re-read the
// injection exists to eliminate.
const Header = "# Agent Protocol (injected)\n\n" +
	"The sections below are your complete copy of the shared agent protocol\n" +
	"(`references/agent-protocol.md`). Do NOT read that file — everything\n" +
	"relevant to you is already here."

var (
	anchorRe = regexp.MustCompile(`^<!-- protocol: ([a-z][a-z0-9-]+) -->$`)
	slugRe   = regexp.MustCompile(`^[a-z][a-z0-9-]+$`)
)

// Doc is a parsed agent-protocol.md: section slugs in document order and
// each slug's content (heading line through the last line before the next
// `## ` heading, anchor line and trailing `---` separators stripped).
type Doc struct {
	Order    []string
	Sections map[string]string
}

// Parse splits md on `## ` headings. Every heading must be followed
// immediately (blank lines allowed) by an anchor line
// `<!-- protocol: <slug> -->`; a missing or duplicate anchor is an error so
// heading/anchor drift fails loud instead of silently dropping a section.
// CRLF input parses identically to LF (go:embed + autocrlf working trees).
func Parse(md string) (*Doc, error) {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	md = strings.ReplaceAll(md, "\r", "\n")

	lines := strings.Split(md, "\n")
	doc := &Doc{Sections: map[string]string{}}

	var slug string
	var body []string
	flush := func() error {
		if slug == "" {
			return nil
		}
		if _, dup := doc.Sections[slug]; dup {
			return fmt.Errorf("duplicate protocol anchor %q", slug)
		}
		doc.Order = append(doc.Order, slug)
		doc.Sections[slug] = trimSection(body)
		slug, body = "", nil
		return nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "## ") {
			if slug != "" {
				body = append(body, line)
			}
			continue
		}
		if err := flush(); err != nil {
			return nil, err
		}
		// Find the anchor: next non-blank line after the heading.
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		m := anchorRe.FindStringSubmatch(strings.TrimSpace(lineAt(lines, j)))
		if m == nil {
			return nil, fmt.Errorf("heading %q has no <!-- protocol: <slug> --> anchor", strings.TrimSpace(line))
		}
		slug = m[1]
		body = []string{line}
		i = j // skip past the anchor line
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(doc.Order) == 0 {
		return nil, fmt.Errorf("no protocol sections found")
	}
	return doc, nil
}

func lineAt(lines []string, i int) string {
	if i >= len(lines) {
		return ""
	}
	return lines[i]
}

// trimSection drops leading/trailing blank lines and a trailing `---`
// separator (which belongs to the document layout, not the section).
func trimSection(body []string) string {
	s := strings.TrimSpace(strings.Join(body, "\n"))
	s = strings.TrimSuffix(s, "\n---")
	return strings.TrimSpace(s)
}

// Compose renders the sections named by slugs under Header, in document
// order regardless of list order. Unknown or duplicate slugs are errors.
func Compose(doc *Doc, slugs []string) (string, error) {
	want := map[string]bool{}
	for _, s := range slugs {
		if !slugRe.MatchString(s) {
			return "", fmt.Errorf("invalid protocol section slug %q", s)
		}
		if _, ok := doc.Sections[s]; !ok {
			return "", fmt.Errorf("unknown protocol section %q (valid: %s)", s, strings.Join(doc.Order, ", "))
		}
		if want[s] {
			return "", fmt.Errorf("duplicate protocol section %q", s)
		}
		want[s] = true
	}
	if len(want) == 0 {
		return "", nil
	}
	parts := []string{Header}
	for _, s := range doc.Order {
		if want[s] {
			parts = append(parts, doc.Sections[s])
		}
	}
	return strings.Join(parts, "\n\n---\n\n"), nil
}

// ParseList splits a flat comma-separated protocol_sections frontmatter
// value into trimmed lowercase slugs, dropping empties.
func ParseList(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
