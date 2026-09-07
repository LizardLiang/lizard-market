package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stage9Status(codeReview, risk string) map[string]interface{} {
	stage := map[string]interface{}{"status": "complete"}
	if codeReview != "" {
		stage["code_review_verdict"] = codeReview
	}
	if risk != "" {
		stage["risk_verdict"] = risk
	}
	return map[string]interface{}{"pipeline": map[string]interface{}{"9-review": stage}}
}

// Regression for the 2026-09 review: `pipeline get` showed
// code_review_verdict = approved while the gate, scanning the last 500 bytes
// of code-review.md, said "does not declare a passing verdict" — and the
// orchestrator then appended its own APPROVED to Hermes's file. The
// structured verdict must win.
func TestVerifyVerdict_StructuredWinsOverMarkdown(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	// Verdict heading early, then a long filtered-findings appendix that never
	// repeats the word "approved" — the tail scan alone would block.
	writeDeliverable(t, dir, "code-review.md",
		"## Verdict\nApproved\n\n## Filtered findings\n"+strings.Repeat("- [FILTERED] minor note\n", 60))
	failures := evaluateFinalGate(dir, stage9Status("approved", "clear"))
	assert.Empty(t, failures, "structured approved verdict must pass: %v", failures)

	// A structured failing verdict blocks even when the markdown says approved.
	writeDeliverable(t, dir, "code-review.md", "review\n\n## Verdict\nApproved")
	failures = evaluateFinalGate(dir, stage9Status("changes-required", "clear"))
	require.NotEmpty(t, failures)
	assert.Contains(t, failures[0], "code_review_verdict")

	// Synonyms normalize.
	failures = evaluateFinalGate(dir, stage9Status("changes requested", "clear"))
	require.NotEmpty(t, failures)
}

func TestVerifyVerdict_FallsBackToVerdictSection(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	// No structured verdict; the Verdict heading sits well above the last 500 bytes.
	writeDeliverable(t, dir, "code-review.md",
		"## Summary\nfindings\n\n## Verdict\nApproved\n\n## Appendix\n"+strings.Repeat("detail line\n", 80))
	failures := evaluateFinalGate(dir, stage9Status("", "clear"))
	assert.Empty(t, failures, "the whole Verdict section is read, not just the tail: %v", failures)

	// Failing text inside the Verdict section still wins over an earlier "approved".
	writeDeliverable(t, dir, "code-review.md", "Approved in spirit.\n\n## Verdict\nChanges Required — two blockers")
	failures = evaluateFinalGate(dir, stage9Status("", "clear"))
	require.NotEmpty(t, failures)
	assert.Contains(t, failures[0], "failing verdict")

	// No verdict at all: the message tells the operator what was found and
	// forbids editing the reviewer's file.
	writeDeliverable(t, dir, "code-review.md", "## Verdict\nTBD — pending")
	failures = evaluateFinalGate(dir, stage9Status("", "clear"))
	require.NotEmpty(t, failures)
	assert.Contains(t, failures[0], "last line found")
	assert.Contains(t, failures[0], "Do not edit the reviewer's file")
}

func TestVerifyVerdict_MissingFileBlocksEvenWithStructuredPass(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	require.NoError(t, os.Remove(filepath.Join(dir, "risk-analysis.md")))
	failures := evaluateFinalGate(dir, stage9Status("approved", "clear"))
	require.NotEmpty(t, failures, "a verdict without its deliverable is not shippable")
}

func TestVerdictRegion(t *testing.T) {
	assert.Equal(t, "## Verdict\nSound\n", verdictRegion([]byte("intro\n## Verdict\nSound\n")))
	assert.Equal(t, "### Verdict: Aligned\ntail", verdictRegion([]byte("# Doc\n### Verdict: Aligned\ntail")))
	long := strings.Repeat("x", 700)
	assert.Len(t, verdictRegion([]byte(long)), 500, "no heading → last 500 bytes")
}

// gitRepo creates a temp repository with one committed file and returns its
// path and the commit hash. Skips when git is unavailable.
func gitRepo(t *testing.T) (string, string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command(git, append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644))
	run("add", "a.txt")
	run("commit", "-q", "-m", "init")
	return dir, run("rev-parse", "--short", "HEAD")
}

func TestEvaluateLanded(t *testing.T) {
	dir, hash := gitRepo(t)

	assert.Empty(t, evaluateLanded(dir, hash), "clean tree + existing commit is landed")
	assert.Empty(t, evaluateLanded(dir, ""), "hash is optional")

	// Bookkeeping under .claude/ never counts.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude", "feature"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".claude", "feature", "status.json"), []byte("{}"), 0o644))
	assert.Empty(t, evaluateLanded(dir, hash))

	// A modified tracked source file blocks.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("two\n"), 0o644))
	failures := evaluateLanded(dir, hash)
	require.Len(t, failures, 1)
	assert.Contains(t, failures[0], "a.txt")

	// An unknown commit blocks.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644))
	failures = evaluateLanded(dir, "deadbeef")
	require.Len(t, failures, 1)
	assert.Contains(t, failures[0], "deadbeef")

	// Not a repository: a single explanatory reason.
	failures = evaluateLanded(t.TempDir(), "")
	require.Len(t, failures, 1)
	assert.Contains(t, failures[0], "not a git work tree")
}

func TestAresLandedGateFailure(t *testing.T) {
	dir, hash := gitRepo(t)

	// Message rule: no Landed line, cwd unknown → blocked on the message alone.
	f := aresLandedGateFailure(subagentStopInput{LastAssistantMessage: "Task list:\n1. [x] done\ncreated a.ts\nImplementation complete."})
	assert.Contains(t, f, "Landed:")

	// Waiver.
	assert.Equal(t, "", aresLandedGateFailure(subagentStopInput{LastAssistantMessage: "LANDED-NOT-APPLICABLE: User Mode task files only"}))

	// Real commit in the repo passes; a fake one is caught.
	ok := subagentStopInput{Cwd: dir, LastAssistantMessage: "Implementation complete.\nLanded: main@" + hash}
	assert.Equal(t, "", aresLandedGateFailure(ok))
	bad := subagentStopInput{Cwd: dir, LastAssistantMessage: "Implementation complete.\nLanded: main@deadbeef"}
	assert.Contains(t, aresLandedGateFailure(bad), "deadbeef")

	// Bold markdown around the label still parses.
	bold := subagentStopInput{Cwd: dir, LastAssistantMessage: "**Landed:** feat/x@" + hash}
	assert.Equal(t, "", aresLandedGateFailure(bold))

	// Outside any repository the message rule still applies (an explicit waiver
	// is required), but a Landed line is not verified against git.
	assert.Contains(t, aresLandedGateFailure(subagentStopInput{Cwd: t.TempDir(), LastAssistantMessage: "created a.ts. done"}), "Landed:")
	assert.Equal(t, "", aresLandedGateFailure(subagentStopInput{Cwd: t.TempDir(), LastAssistantMessage: "created a.ts. done\nLanded: main@abcdef1"}))
}
