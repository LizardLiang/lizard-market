// Package gencmd generates plugins/kratos/commands/<god>.md launchers and the
// god-derived regions of plugins/kratos/skills/auto/SKILL.md from
// plugins/kratos/agents/*.md frontmatter. See kratos-dev/codegen/README.md for
// the add-a-god workflow.
package gencmd

import (
	"fmt"
	"strings"
)

// Frontmatter is a flat key -> raw value map parsed from a leading
// "---\n...\n---" block, plus whatever body content follows it.
type Frontmatter struct {
	Fields map[string]string
	Body   string
}

// ParseFrontmatter parses a leading frontmatter block of flat "key: value"
// lines (one field per line, no nesting). It fails loud — returns a non-nil
// error naming path and line — on any non-blank line inside the block that
// isn't a simple "key: value" pair, or if the block is never closed.
func ParseFrontmatter(path, content string) (*Frontmatter, error) {
	content = strings.TrimPrefix(content, "\uFEFF") // strip a leading UTF-8 BOM, if present
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return nil, fmt.Errorf("%s: frontmatter must start with '---'", path)
	}

	fm := &Frontmatter{Fields: map[string]string{}}
	i := 1
	closed := false
	for ; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if line == "---" {
			closed = true
			i++
			break
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			return nil, fmt.Errorf("%s: unparseable frontmatter line %d: %q", path, i+1, line)
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			return nil, fmt.Errorf("%s: unparseable frontmatter line %d: %q", path, i+1, line)
		}
		fm.Fields[key] = unquote(val)
	}
	if !closed {
		return nil, fmt.Errorf("%s: frontmatter never closed with '---'", path)
	}
	fm.Body = strings.Join(lines[i:], "\n")
	return fm, nil
}

func unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// normalizeLF converts CRLF/CR line endings to LF so content read from a
// Windows (autocrlf) working tree compares equal to the LF content this
// package always writes.
func normalizeLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
