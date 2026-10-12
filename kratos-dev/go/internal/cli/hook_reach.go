package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// The Ares Reach Review gates. agents/ares.md asks every Ares completion
// report to carry a `Reach:` line, RED/GREEN evidence (or a waiver), and to
// show that a fresh-context child agent ran the reach check on the diff.
// aresReportFailures runs these three checks next to the Landed and comment
// checks, so the SubagentStop gate and the PreToolUse hand-back gate share them.
// Only Ares reports reach this code: Hephaestus and Hermes use other functions.

// reachLineRE matches the report line `Reach: <value>` and captures the value.
var reachLineRE = regexp.MustCompile(`(?im)^[ \t]*[-*]?[ \t]*\**reach\**[ \t]*:\**[ \t]*(\S.*)$`)

// reachNotApplicableRE matches the value Ares writes when it changed no code.
var reachNotApplicableRE = regexp.MustCompile(`(?i)^not applicable\b`)

var (
	redWordRE   = regexp.MustCompile(`\bRED\b`)
	greenWordRE = regexp.MustCompile(`\bGREEN\b`)
)

// evidenceWaiverRE matches the two accepted waivers of the RED/GREEN evidence,
// with the colon anywhere, or without it when the keyword starts a line, a
// list item or a sentence.
var evidenceWaiverRE = waiverRE(`evidence-skipped|tests-not-applicable`)

// testsNotApplicableRE is the same rule for the TESTS-NOT-APPLICABLE waiver
// alone, which the verify gate accepts in place of a test run.
var testsNotApplicableRE = waiverRE(`tests-not-applicable`)

func waiverRE(words string) *regexp.Regexp {
	return regexp.MustCompile(`(?im)(?:(?:` + words + `)\b[ \t]*:|(?:^[ \t]*[-*]?[ \t]*\**|[.!?][ \t]+)(?:` + words + `)\b)`)
}

// foldedHeaderRE matches a report line that holds only the label of a field
// whose value follows on the next lines: `Files created/modified:` or `Reach:`.
var foldedHeaderRE = regexp.MustCompile(`(?i)^[ \t]*[-*]?[ \t]*\**(files created/modified|reach)\**[ \t]*:\**[ \t]*$`)

// fieldLineRE matches the start of the next `<Label>:` field line of the Final
// Block. A bullet or numbered line is never a field line.
var fieldLineRE = regexp.MustCompile(`^[ \t]*\**[A-Za-z][A-Za-z0-9()/-]*( [A-Za-z0-9()/-]+){0,3}\**[ \t]*:`)

// waiverLineRE matches a line that opens with a waiver word, with or without
// the colon. A waiver is its own field and ends a folded list.
var waiverLineRE = regexp.MustCompile(`(?i)^[ \t]*[-*]?[ \t]*\**(?:evidence-skipped|tests-not-applicable)\b`)

// maxFoldedLines bounds how many lines follow a header into its logical line.
const maxFoldedLines = 8

// foldHeaderLists rewrites a label-only `Files created/modified:` or `Reach:`
// line together with the non-blank lines after it, up to maxFoldedLines and up
// to the next field line or waiver line. `Files created/modified:` and its
// list become one logical line. `Reach:` and its list become one
// `Reach: <item>` line per list item, so the checks that run on every Reach
// line run on every item. Reports written as a header plus a bullet list then
// match the single-line patterns the gate checks.
func foldHeaderLists(report string) string {
	lines := strings.Split(report, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		m := foldedHeaderRE.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			out = append(out, line)
			continue
		}
		header := strings.TrimRight(line, " \t\r")
		var items []string
		for len(items) < maxFoldedLines && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next == "" || fieldLineRE.MatchString(lines[i+1]) || waiverLineRE.MatchString(lines[i+1]) {
				break
			}
			items = append(items, strings.TrimLeft(next, "-* \t"))
			i++
		}
		switch {
		case len(items) == 0:
			out = append(out, header)
		case strings.EqualFold(m[1], "reach"):
			for _, item := range items {
				out = append(out, "Reach: "+item)
			}
		default:
			out = append(out, header+" "+strings.Join(items, " "))
		}
	}
	return strings.Join(out, "\n")
}

// isUserModeReport reports whether report is the User Mode report, which
// writes task files and no code, so it has no diff to review.
func isUserModeReport(report string) bool {
	for _, line := range strings.Split(report, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return strings.Contains(strings.ToLower(trimmed), "(user mode)")
	}
	return false
}

// aresReachGateFailures returns one entry per unmet Reach Review check.
// A brief is free text, so the gate cannot count its items reliably. The
// evidence check enforces presence, not one RED/GREEN pair per item.
func aresReachGateFailures(report string, input subagentStopInput) []string {
	if isUserModeReport(report) {
		return nil
	}
	report = foldHeaderLists(report)
	var failures []string

	reachLines := reachLineRE.FindAllStringSubmatch(report, -1)
	allNotApplicable := len(reachLines) > 0
	for _, rl := range reachLines {
		if !reachNotApplicableRE.MatchString(strings.TrimSpace(strings.Trim(rl[1], "*"))) {
			allNotApplicable = false
		}
	}
	if len(reachLines) == 0 {
		failures = append(failures, "no `Reach: <entry point> — reached by <callers/inputs/states the request did not name> → <test | fix | NEEDS DESIGN>` line (or `Reach: none beyond the request`, or `Reach: not applicable — no code changed`) — run the Reach Review on `git diff <start>..HEAD` and report one line per changed entry point")
	}

	if !evidenceWaiverRE.MatchString(report) && (!redWordRE.MatchString(report) || !greenWordRE.MatchString(report)) {
		failures = append(failures, "no fail-then-pass evidence — report RED (the check failing before your change) and GREEN (passing after) for each testable item, or `EVIDENCE-SKIPPED: <reason>` / `TESTS-NOT-APPLICABLE: <reason>`")
	}

	if allNotApplicable {
		return failures
	}
	if f := aresReachChildFailure(input); f != "" {
		failures = append(failures, f)
	}
	return failures
}

// aresReachChildFailure checks that Ares's own transcript shows a child agent
// spawned with a `git diff` range in its prompt — the fresh-context reach
// check. An unknown or unreadable transcript fails open: the gate must not
// block on missing data.
func aresReachChildFailure(input subagentStopInput) string {
	path := subagentHandbackTranscriptPath(input)
	if path == "" {
		return ""
	}
	spawned, err := transcriptSpawnedDiffChild(path)
	if err != nil {
		debugLog("ares reach gate: transcript scan failed (fail-open): %v", err)
		return ""
	}
	if spawned {
		return ""
	}
	return "no reach-check child agent — before the hand-back, spawn ONE child agent whose prompt carries `git diff <start>..HEAD` and the four Reach questions (`<kratos-bin> template get reach-check-prompt`), then fix or test every hit"
}

// transcriptSpawnedDiffChild reports whether the agent transcript at path
// holds an Agent or Task tool_use whose prompt names a `git diff` range.
func transcriptSpawnedDiffChild(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var entry handbackLine
		if json.Unmarshal(line, &entry) != nil || entry.Type != "assistant" || entry.Message == nil {
			continue
		}
		for _, b := range handbackParseBlocks(entry.Message.Content) {
			if b.Type != "tool_use" || (b.Name != "Agent" && b.Name != "Task") {
				continue
			}
			var in struct {
				Prompt string `json:"prompt"`
			}
			if json.Unmarshal(b.Input, &in) == nil && strings.Contains(in.Prompt, "git diff") {
				return true, nil
			}
		}
	}
	return false, sc.Err()
}
