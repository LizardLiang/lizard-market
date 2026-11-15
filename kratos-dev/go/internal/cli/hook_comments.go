package cli

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// caveatCommentRE matches a comment that carries a design concern or a
// workaround instead of a question to the user: "deliberately NOT calling
// fetch_orders_and_info", "would overwrite Total", "for now", "known issue".
// The list is deliberately narrow — no bare "not", no "assumes" (Hermes's
// review rule covers it), no "TODO" (a tracked "TODO #123" is legitimate) — so
// an ordinary explanatory comment never trips the gate.
var caveatCommentRE = regexp.MustCompile(`(?i)deliberately (not|don't)|intentionally (not|don't)|work-?around|known (issue|bug|limitation)|caveat|would (overwrite|break|clobber)|\bfor now\b|\bhack\b|\bFIXME\b|\bXXX\b|\bWARN(ING)?:|(do not|don't) (call|change|remove|reorder)|must not be called`)

// commentLineRE matches a line that is a comment in the languages
// codeFileExts covers (plus a docstring delimiter opening a block).
var commentLineRE = regexp.MustCompile(`^\s*(#|//|/\*|\*|--|<!--|"""|''')`)

// docstringDelimRE counts the triple-quote delimiters on a line so an added
// Python docstring block is scanned as comment text until it closes.
var docstringDelimRE = regexp.MustCompile(`"""|'''`)

// caveatWaiverRE is the only accepted waiver: the report states
// CAVEAT-COMMENT-KEPT: <file:line — why it is an external fact, not a design
// concern> (a vendor bug the code must work around, a documented API quirk).
var caveatWaiverRE = regexp.MustCompile(`(?i)caveat-comment-kept:`)

// hunkHeaderRE captures the new-file start line of a unified diff hunk.
var hunkHeaderRE = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// maxCaveatDiffBytes bounds the diff the gate is willing to scan; a larger
// commit fails open rather than stall the hook.
const maxCaveatDiffBytes = 1 << 20

// maxCaveatHitsListed caps how many offending lines one failure names.
const maxCaveatHitsListed = 3

// diffLine is one added line of a commit, with its path and new-file line.
type diffLine struct {
	File string
	Line int
	Text string
}

// addedCommentLines returns every added line of diff (git show / git diff
// output) that is a comment or sits inside an added docstring block, in code
// files only (codeFileExts). Removed lines, hunk headers, and string literals
// on code lines are never returned.
func addedCommentLines(diff string) []diffLine {
	var (
		out    []diffLine
		file   string
		isCode bool
		lineNo int
		inDoc  bool
	)
	for _, raw := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(raw, "+++ "):
			file = strings.TrimPrefix(raw[4:], "b/")
			file = strings.TrimSpace(file)
			isCode = file != "/dev/null" && isCodeFile(file)
			inDoc = false
			continue
		case strings.HasPrefix(raw, "--- "), strings.HasPrefix(raw, "diff "), strings.HasPrefix(raw, "index "):
			continue
		}
		if m := hunkHeaderRE.FindStringSubmatch(raw); m != nil {
			lineNo, _ = strconv.Atoi(m[1])
			inDoc = false
			continue
		}
		if raw == "" || raw[0] == '-' || raw[0] == '\\' {
			continue
		}
		added := raw[0] == '+'
		text := raw[1:]
		cur := lineNo
		lineNo++
		if !isCode {
			continue
		}
		wasInDoc := inDoc
		if n := len(docstringDelimRE.FindAllString(text, -1)); n%2 == 1 {
			// A delimiter at line start opens or closes a docstring block. A
			// delimiter elsewhere on a code line (x = """) opens a string
			// literal, which is code — leave the state alone.
			if inDoc || commentLineRE.MatchString(text) {
				inDoc = !inDoc
			}
		}
		if !added {
			continue
		}
		if wasInDoc || commentLineRE.MatchString(text) {
			out = append(out, diffLine{File: file, Line: cur, Text: strings.TrimSpace(text)})
		}
	}
	return out
}

// aresCommentGateFailure enforces the "a concern is a question, not a
// comment" rule at Ares's hand-back: the commit named by the report's
// Landed: line may not add a comment or docstring line matching
// caveatCommentRE. The e0fbf49 docstring that explained why a job "deliberately
// NOT" refreshed its order count hid a false invariant (stale SelectState
// rows) that shipped 105 unwanted 安排出貨 calls to production.
//
// Fails OPEN on every infrastructure problem (no Landed hash, no cwd, not a
// work tree, git missing or erroring, oversized diff) — the Landed gate
// already reports a missing or bogus hash. The only waiver is
// CAVEAT-COMMENT-KEPT: <file:line — why> in the report.
func aresCommentGateFailure(report, cwd string) string {
	if caveatWaiverRE.MatchString(report) {
		return ""
	}
	m := landedLineRE.FindStringSubmatch(report)
	if m == nil {
		return ""
	}
	hash := m[2]
	if cwd == "" || !isGitWorkTree(cwd) {
		return ""
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	out, err := exec.Command(git, "-C", cwd, "show", "--format=", "--unified=0", "--no-color", hash).Output()
	if err != nil {
		debugLog("comment gate: git show %s in %s: %v", hash, cwd, err)
		return ""
	}
	if len(out) > maxCaveatDiffBytes {
		debugLog("comment gate: diff for %s is %d bytes, skipping", hash, len(out))
		return ""
	}
	var hits []string
	for _, l := range addedCommentLines(string(out)) {
		if caveatCommentRE.MatchString(l.Text) {
			hits = append(hits, fmt.Sprintf("%s:%d: %q", l.File, l.Line, l.Text))
		}
	}
	if len(hits) == 0 {
		return ""
	}
	extra := ""
	if len(hits) > maxCaveatHitsListed {
		extra = fmt.Sprintf(" (+%d more)", len(hits)-maxCaveatHitsListed)
		hits = hits[:maxCaveatHitsListed]
	}
	return fmt.Sprintf("commit %s adds a comment carrying a design caveat (%s%s) — a concern belongs to the user, not the code: remove the comment, commit, report the new hash, and put the concern in your report (ARES NEEDS DESIGN if it changes what should have been built); CAVEAT-COMMENT-KEPT: <file:line — why> only for an external fact", hash, strings.Join(hits, "; "), extra)
}
