package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeDeliverable creates a deliverable file in featureDir with the given body.
func writeDeliverable(t *testing.T, featureDir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, name), []byte(body), 0o644))
}

// writeAllPassing creates every deliverable a fully-green feature needs.
func writeAllPassing(t *testing.T, dir string) {
	t.Helper()
	writeDeliverable(t, dir, "prd.md", "# PRD\nbody")
	writeDeliverable(t, dir, "decisions.md", "# Decisions\nbody")
	writeDeliverable(t, dir, "prd-challenge.md", "review\n\n## Verdict\nApproved")
	writeDeliverable(t, dir, "tech-spec.md", "# Spec\nbody")
	writeDeliverable(t, dir, "spec-review-sa.md", "review\n\n## Verdict\nSound")
	writeDeliverable(t, dir, "test-plan.md", "# Tests\nbody")
	writeDeliverable(t, dir, "implementation-notes.md", "# Notes\nbody")
	writeDeliverable(t, dir, "prd-alignment.md", "review\n\n## Verdict\nAligned")
	writeDeliverable(t, dir, "code-review.md", "review\n\n## Verdict\nApproved")
	writeDeliverable(t, dir, "risk-analysis.md", "review\n\n## Verdict\nClear")
}

func TestVerifyPassingVerdict_WordBoundary(t *testing.T) {
	dir := t.TempDir()

	// "unsound" must NOT satisfy a "sound" passing check — the classic substring trap.
	writeDeliverable(t, dir, "spec-review-sa.md", "The design is complex.\n\n## Verdict\nUnsound")
	err := verifyPassingVerdict(dir, finalVerdictReq{
		file: "spec-review-sa.md", passing: []string{"sound"}, failing: []string{"concerns", "unsound"},
	})
	require.Error(t, err, "unsound must be detected as a failing verdict, not a passing 'sound'")
	assert.Contains(t, err.Error(), "failing verdict")

	// "misaligned" must NOT satisfy an "aligned" passing check.
	writeDeliverable(t, dir, "prd-alignment.md", "coverage notes\n\n## Verdict\nMisaligned")
	err = verifyPassingVerdict(dir, finalVerdictReq{
		file: "prd-alignment.md", passing: []string{"aligned"}, failing: []string{"gaps", "misaligned"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failing verdict")

	// A genuine passing verdict passes.
	writeDeliverable(t, dir, "prd-alignment.md", "coverage notes\n\n## Verdict\nAligned")
	require.NoError(t, verifyPassingVerdict(dir, finalVerdictReq{
		file: "prd-alignment.md", passing: []string{"aligned"}, failing: []string{"gaps", "misaligned"},
	}))
}

func TestVerifyPassingVerdict_FailingWins(t *testing.T) {
	dir := t.TempDir()
	// Both a passing keyword and a failing keyword present → conservative fail.
	writeDeliverable(t, dir, "code-review.md",
		"The approach looks approved in spirit, but ...\n\n## Verdict\nChanges Required")
	err := verifyPassingVerdict(dir, finalVerdictReq{
		file:    "code-review.md",
		passing: []string{"approved"},
		failing: []string{"changes-required", "changes required", "changes-requested"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failing verdict")
}

func TestVerifyPassingVerdict_MissingFile(t *testing.T) {
	dir := t.TempDir()
	err := verifyPassingVerdict(dir, finalVerdictReq{
		file: "risk-analysis.md", passing: []string{"clear", "caution"}, failing: []string{"blocked"},
	})
	require.Error(t, err, "a missing deliverable must fail the gate, not pass it")
}

func TestEvaluateFinalGate_AllPassing(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	failures := evaluateFinalGate(dir, nil)
	assert.Empty(t, failures, "a fully-green feature must pass the gate: %v", failures)
}

func TestEvaluateFinalGate_FailingReviewBlocks(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	// Flip the code review to a failing verdict.
	writeDeliverable(t, dir, "code-review.md", "findings\n\n## Verdict\nChanges Required")
	failures := evaluateFinalGate(dir, nil)
	require.NotEmpty(t, failures, "a Changes Required code review must block the ship gate")
	assert.Contains(t, failures[0], "9-review")
}

func TestEvaluateFinalGate_RiskBlockedBlocks(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	writeDeliverable(t, dir, "risk-analysis.md", "findings\n\n## Verdict\nBlocked")
	failures := evaluateFinalGate(dir, nil)
	require.NotEmpty(t, failures, "a Blocked risk verdict must block the ship gate")
}

func TestEvaluateFinalGate_MissingDeliverableBlocks(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	require.NoError(t, os.Remove(filepath.Join(dir, "implementation-notes.md")))
	failures := evaluateFinalGate(dir, nil)
	require.NotEmpty(t, failures, "a missing implementation-notes.md must block the ship gate")
	assert.Contains(t, failures[0], "7-implementation")
}

func TestEvaluateFinalGate_SkippedStageIgnored(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	// Remove tech-spec and mark stage 4 skipped — the gate must not complain.
	require.NoError(t, os.Remove(filepath.Join(dir, "tech-spec.md")))
	status := map[string]interface{}{
		"pipeline": map[string]interface{}{
			"4-tech-spec": map[string]interface{}{"status": "skipped"},
		},
	}
	failures := evaluateFinalGate(dir, status)
	assert.Empty(t, failures, "an explicitly skipped stage must not block the gate: %v", failures)
}

func TestCompactStatus(t *testing.T) {
	status := map[string]interface{}{
		"feature": "demo",
		"stage":   "9-review",
		"history": []interface{}{
			map[string]interface{}{"action": "a"},
			map[string]interface{}{"action": "b"},
		},
		"pipeline": map[string]interface{}{
			"9-review": map[string]interface{}{
				"status":         "complete",
				"verdict":        "approved",
				"summary":        "keep me",
				"check_failures": []interface{}{map[string]interface{}{"tier": 1}},
			},
		},
	}

	out := compactStatus(status)

	_, hasHistory := out["history"]
	assert.False(t, hasHistory, "compact output must drop history[]")
	assert.Equal(t, "demo", out["feature"], "non-audit fields are preserved")

	stage := out["pipeline"].(map[string]interface{})["9-review"].(map[string]interface{})
	_, hasCF := stage["check_failures"]
	assert.False(t, hasCF, "compact output must drop per-stage check_failures[]")
	assert.Equal(t, "approved", stage["verdict"], "decision-relevant stage fields are preserved")
	assert.Equal(t, "complete", stage["status"])

	// The original must not be mutated.
	_, origHasHistory := status["history"]
	assert.True(t, origHasHistory, "compactStatus must not mutate the input status.json")
	origStage := status["pipeline"].(map[string]interface{})["9-review"].(map[string]interface{})
	_, origHasCF := origStage["check_failures"]
	assert.True(t, origHasCF, "compactStatus must not mutate the input stage map")
}

func TestIsFeatureVerified(t *testing.T) {
	dir := t.TempDir()
	writeAllPassing(t, dir)
	assert.True(t, isFeatureVerified(dir, nil), "green feature → verified")

	writeDeliverable(t, dir, "spec-review-sa.md", "notes\n\n## Verdict\nConcerns")
	assert.False(t, isFeatureVerified(dir, nil), "a Concerns spec review → not verified")
}
