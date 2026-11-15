package gencmd

import "strings"

// wordWrap greedily wraps text into lines no longer than width (indent
// included): a greedy word-wrap similar to Python's textwrap.fill (does not
// break long words — a single word longer than width is left on its own
// overflowing line), each line prefixed with indent. Used to reflow
// SKILL.md's folded frontmatter description when the god roster changes, so
// line breaks stay deterministic instead of depending on manual editing.
func wordWrap(text, indent string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	cur := indent + words[0]
	for _, w := range words[1:] {
		candidate := cur + " " + w
		if len(candidate) <= width {
			cur = candidate
			continue
		}
		lines = append(lines, cur)
		cur = indent + w
	}
	lines = append(lines, cur)
	return lines
}
