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

// evidenceWaiverRE matches the two accepted waivers of the RED/GREEN evidence.
var evidenceWaiverRE = regexp.MustCompile(`(?i)(evidence-skipped|tests-not-applicable):`)

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
	var failures []string

	m := reachLineRE.FindStringSubmatch(report)
	if m == nil {
		failures = append(failures, "no `Reach: <entry point> — reached by <callers/inputs/states the request did not name> → <test | fix | NEEDS DESIGN>` line (or `Reach: none beyond the request`, or `Reach: not applicable — no code changed`) — run the Reach Review on `git diff <start>..HEAD` and report one line per changed entry point")
	}

	if !evidenceWaiverRE.MatchString(report) && (!redWordRE.MatchString(report) || !greenWordRE.MatchString(report)) {
		failures = append(failures, "no fail-then-pass evidence — report RED (the check failing before your change) and GREEN (passing after) for each testable item, or `EVIDENCE-SKIPPED: <reason>` / `TESTS-NOT-APPLICABLE: <reason>`")
	}

	if m != nil && reachNotApplicableRE.MatchString(strings.TrimSpace(strings.Trim(m[1], "*"))) {
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
