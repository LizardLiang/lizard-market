package cli

import (
	"regexp"
	"strings"
	"testing"
)

// reachBase is a valid Ares completion report from before the Reach rule,
// with the RED/GREEN evidence line and the Reach line left out.
const reachBase = `ARES COMPLETE

Task list:
1. [x] fix handler — done
Files created/modified: handler.go, handler_test.go
Landed: LANDED-NOT-APPLICABLE: test fixture
Not run: none
`

const reachEvidence = "Evidence: handler RED: TestHandler failed -> GREEN: TestHandler passed\n"
const reachLine = "Reach: handler.Serve - reached by the batch pipeline with empty input -> test\n"

func reachFailureText(failures []string) string { return strings.Join(failures, " | ") }

// reachTranscript writes an Ares transcript where the agent spawned one child
// with the given prompt (or none when prompt is empty) and returns the input
// that points the gate at it.
func reachInput(t *testing.T, agentID string, lines ...string) subagentStopInput {
	t.Helper()
	main := writeHandbackTranscript(t, "sessR", agentID, lines...)
	return subagentStopInput{TranscriptPath: main, SessionID: "sessR", AgentID: agentID, Cwd: t.TempDir()}
}

func TestAresReachLineRequired(t *testing.T) {
	report := reachBase + reachEvidence
	got := reachFailureText(aresReportFailures(report, subagentStopInput{}))
	if !strings.Contains(got, "Reach:") {
		t.Fatalf("report without a Reach: line must fail naming it, got %q", got)
	}
	ok := reachFailureText(aresReportFailures(report+reachLine, subagentStopInput{}))
	if strings.Contains(ok, "Reach") {
		t.Fatalf("report with a Reach: line must not fail on it, got %q", ok)
	}
}

func TestAresEvidenceRequired(t *testing.T) {
	got := reachFailureText(aresReportFailures(reachBase+reachLine, subagentStopInput{}))
	if !strings.Contains(got, "RED") || !strings.Contains(got, "GREEN") {
		t.Fatalf("report with no RED/GREEN and no waiver must fail naming both, got %q", got)
	}
	for _, waiver := range []string{"EVIDENCE-SKIPPED: docs only\n", "TESTS-NOT-APPLICABLE: docs only\n"} {
		f := reachFailureText(aresReportFailures(reachBase+reachLine+waiver, subagentStopInput{}))
		if strings.Contains(f, "RED") {
			t.Errorf("waiver %q must satisfy the evidence check, got %q", waiver, f)
		}
	}
	onlyRed := reachFailureText(aresReportFailures(reachBase+reachLine+"Evidence: RED: it failed\n", subagentStopInput{}))
	if !strings.Contains(onlyRed, "GREEN") {
		t.Errorf("RED without GREEN must fail, got %q", onlyRed)
	}
}

func TestAresReachCheckChildRequired(t *testing.T) {
	full := reachBase + reachEvidence + reachLine

	t.Run("no child spawned: fail naming the reach check", func(t *testing.T) {
		in := reachInput(t, "aresR1", userPromptLine("implement"), toolUseLine(false, "Bash", map[string]any{"command": "go test ./..."}))
		got := reachFailureText(aresReportFailures(full, in))
		if !strings.Contains(got, "child agent") {
			t.Fatalf("expected a failure naming the missing child agent, got %q", got)
		}
	})
	t.Run("child spawned without a diff range: fail", func(t *testing.T) {
		in := reachInput(t, "aresR2", userPromptLine("implement"), toolUseLine(false, "Agent", map[string]any{"prompt": "find the helper"}))
		got := reachFailureText(aresReportFailures(full, in))
		if !strings.Contains(got, "child agent") {
			t.Fatalf("a child whose prompt carries no git diff range is not a reach check, got %q", got)
		}
	})
	t.Run("child spawned with the diff range: pass", func(t *testing.T) {
		in := reachInput(t, "aresR3", userPromptLine("implement"), toolUseLine(false, "Agent", map[string]any{"prompt": "Run git diff abc1234..HEAD and answer the four questions"}))
		if f := aresReportFailures(full, in); len(f) != 0 {
			t.Fatalf("expected no failures, got %v", f)
		}
	})
	t.Run("legacy Task tool name also counts", func(t *testing.T) {
		in := reachInput(t, "aresR4", userPromptLine("implement"), toolUseLine(false, "Task", map[string]any{"prompt": "git diff abc1234..HEAD"}))
		if f := aresReportFailures(full, in); len(f) != 0 {
			t.Fatalf("expected no failures, got %v", f)
		}
	})
	t.Run("no readable transcript: fail open", func(t *testing.T) {
		if f := aresReportFailures(full, subagentStopInput{}); len(f) != 0 {
			t.Fatalf("an unknown transcript must not block, got %v", f)
		}
	})
	t.Run("no code changed: child not required", func(t *testing.T) {
		report := reachBase + "TESTS-NOT-APPLICABLE: report only\nReach: not applicable - no code changed\n"
		in := reachInput(t, "aresR5", userPromptLine("report"))
		if f := aresReportFailures(report, in); len(f) != 0 {
			t.Fatalf("expected no failures, got %v", f)
		}
	})
}

func TestAresReachChecksExemptions(t *testing.T) {
	for name, report := range map[string]string{
		"needs design":        "ARES NEEDS DESIGN\n\nFlow: 1. a\n",
		"needs plan mode":     "ARES NEEDS PLAN MODE\n\nReason: x\n",
		"needs clarification": "ARES NEEDS CLARIFICATION\n\nQuestion: x\n",
		"user mode": `ARES COMPLETE (User Mode)

Task list:
1. [x] Write task files — done
Files created/modified: tasks/00-overview.md
LANDED-NOT-APPLICABLE: User Mode — the user commits
Not run: none`,
	} {
		in := reachInput(t, "aresEx", userPromptLine("x"))
		if f := aresReportFailures(report, in); len(f) != 0 {
			t.Errorf("%s must stay exempt from the Reach checks, got %v", name, f)
		}
	}
}

// reachChildLine is a transcript line for the fresh-context reach-check child.
func reachChildLine() string {
	return toolUseLine(false, "Agent", map[string]any{"prompt": "Run git diff abc1234..HEAD and answer the four Reach questions"})
}

// TestAresReachTextPinned guards the prompt text the gate depends on: the
// Final Block and the report templates carry the Reach line, the Reach Review
// section names no bug category, and the child prompt template exists.
func TestAresReachTextPinned(t *testing.T) {
	raw, err := agentsFS.ReadFile("agents/ares.md")
	if err != nil {
		t.Fatal(err)
	}
	ares := strings.ReplaceAll(string(raw), "\r\n", "\n")

	if n := strings.Count(ares, "Reach: <entry point>"); n < 2 {
		t.Errorf("Final Block and the ARES COMPLETE template must carry the Reach line, found %d", n)
	}
	if n := strings.Count(ares, "[Final Block: Task list, Files created/modified, Landed, Not run, Reach]"); n != 2 {
		t.Errorf("wave and phase checkpoint templates must each reference the Final Block with Reach, found %d", n)
	}
	start := strings.Index(ares, "## Reach Review")
	end := strings.Index(ares, "## Output Format")
	if start < 0 || end < start {
		t.Fatal("ares.md has no Reach Review section before Output Format")
	}
	section := ares[start:end]
	for _, banned := range []string{"auth", "token", "role", "cache", "tenant", "user switch"} {
		if regexp.MustCompile(`(?i)\b` + banned).MatchString(section) {
			t.Errorf("Reach Review must not name the bug category %q", banned)
		}
	}
	if !strings.Contains(section, "template get reach-check-prompt") {
		t.Error("Reach Review must point at the reach-check-prompt template")
	}

	prompt, err := templatesFS.ReadFile("templates/reach-check-prompt.md")
	if err != nil {
		t.Fatalf("reach-check-prompt template missing: %v", err)
	}
	for _, want := range []string{"git diff <start>..HEAD", "Callers", "Inputs", "States", "Lifetime", "file:line"} {
		if !strings.Contains(string(prompt), want) {
			t.Errorf("reach-check-prompt must contain %q", want)
		}
	}

	um, err := templatesFS.ReadFile("templates/ares-user-mode-template.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(um), "Reach: not applicable") {
		t.Error("User Mode template must carry the no-code Reach line")
	}
}

const foldBase = `ARES COMPLETE

Task list:
1. [x] fix handler - done
`

const foldTail = "Landed: LANDED-NOT-APPLICABLE: test fixture\nNot run: none\n"

// TestAresReportFoldedHeaderShapes pins the report shapes where a label sits
// on its own line and the value follows on the next lines.
func TestAresReportFoldedHeaderShapes(t *testing.T) {
	cases := map[string]string{
		"files header then bullets": foldBase + "Files created/modified:\n- src/handler.go\n- src/handler_test.go\n" + foldTail +
			"Reach: not applicable - no code changed\nTESTS-NOT-APPLICABLE: fixture\n",
		"reach header then value lines": foldBase + "Files created/modified: src/handler.go\n" + foldTail +
			"Reach:\n- handler.Serve - reached by empty input -> test\nEVIDENCE-SKIPPED for the docs item, RED and GREEN described in prose\n",
		"waiver without colon": foldBase + "Files created/modified: src/handler.go\n" + foldTail +
			"Reach: not applicable - no code changed\nTESTS-NOT-APPLICABLE for this mission, nothing to run\n",
		"two reach lines, last is not applicable": foldBase + "Files created/modified: src/handler.go\n" + foldTail +
			"Reach: not run\nReach: not applicable - no code changed\nTESTS-NOT-APPLICABLE: fixture\n",
	}
	for name, report := range cases {
		in := reachInput(t, "aresFold", reachChildLine())
		if name == "two reach lines, last is not applicable" {
			in = reachInput(t, "aresFold", userPromptLine("x")) // no child: the last Reach line skips it
		}
		if f := aresReportFailures(report, in); len(f) != 0 {
			t.Errorf("%s must pass, got %v", name, f)
		}
	}
}

func TestAresReportFoldedHeaderStillDenies(t *testing.T) {
	const files = "Files created/modified:\n- src/handler.go\n"

	noGreen := foldBase + files + foldTail + "Reach: not applicable - no code changed\nEvidence: RED: it failed\n"
	if got := reachFailureText(aresReportFailures(noGreen, subagentStopInput{})); !strings.Contains(got, "GREEN") {
		t.Errorf("RED without GREEN and no waiver must fail, got %q", got)
	}

	onlyNotRun := foldBase + files + foldTail + "Reach: not run\nTESTS-NOT-APPLICABLE: fixture\n"
	in := reachInput(t, "aresFold2", userPromptLine("x"))
	if got := reachFailureText(aresReportFailures(onlyNotRun, in)); !strings.Contains(got, "child agent") {
		t.Errorf("a lone `Reach: not run` must still require the child, got %q", got)
	}

	noFiles := foldBase + "Files created/modified:\n- nothing\n" + foldTail + "Reach: not applicable - no code changed\nTESTS-NOT-APPLICABLE: fixture\n"
	if got := reachFailureText(aresReportFailures(noFiles, subagentStopInput{})); !strings.Contains(got, "no specific files") {
		t.Errorf("a file list without files must fail, got %q", got)
	}
}

func TestFoldHeaderListsStopsAtNextField(t *testing.T) {
	got := foldHeaderLists("Reach:\n- a\nLanded: x\n- b")
	if got != "Reach: a\nLanded: x\n- b" {
		t.Errorf("unexpected fold: %q", got)
	}
}

func TestFoldHeaderListsStopsAtBoldField(t *testing.T) {
	got := foldHeaderLists("Reach:\n- a\n**Landed:** main@abc1234\nVerify (cmd): go test")
	if got != "Reach: a\n**Landed:** main@abc1234\nVerify (cmd): go test" {
		t.Errorf("unexpected fold: %q", got)
	}
}
