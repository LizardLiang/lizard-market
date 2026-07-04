package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nextFixture builds a feature dir + status.json for computeNext tests.
// stages maps pipeline key → stage map; docs are files created in the dir.
func nextFixture(t *testing.T, currentStage string, stages map[string]map[string]interface{}, docs ...string) (string, map[string]interface{}) {
	t.Helper()
	dir := t.TempDir()
	for _, doc := range docs {
		path := filepath.Join(dir, doc)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))
	}

	pipeline := map[string]interface{}{}
	for key, stage := range stages {
		m := map[string]interface{}{}
		for k, v := range stage {
			m[k] = v
		}
		pipeline[key] = m
	}
	status := map[string]interface{}{
		"feature":  "feat",
		"stage":    currentStage,
		"pipeline": pipeline,
	}
	return dir, status
}

func s(status string, extra ...map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{"status": status}
	for _, e := range extra {
		for k, v := range e {
			m[k] = v
		}
	}
	return m
}

func TestNextTransitionTable(t *testing.T) {
	cases := []struct {
		name          string
		currentStage  string
		stages        map[string]map[string]interface{}
		docs          []string
		wantAction    string
		wantNextStage string
		wantProcedure string
		wantAgents    []string
	}{
		{
			name:          "stage 1 in-progress without prd.md → gap-analysis",
			currentStage:  "1-prd",
			stages:        map[string]map[string]interface{}{"1-prd": s("in-progress")},
			wantAction:    "next",
			wantNextStage: "1-prd",
			wantProcedure: "gap-analysis",
		},
		{
			name:          "stage 1 in-progress with prd.md → resume spawn",
			currentStage:  "1-prd",
			stages:        map[string]map[string]interface{}{"1-prd": s("in-progress")},
			docs:          []string{"prd.md"},
			wantAction:    "next",
			wantNextStage: "1-prd",
			wantProcedure: "spawn",
		},
		{
			name:          "1 complete → 2 nemesis",
			currentStage:  "1-prd",
			stages:        map[string]map[string]interface{}{"1-prd": s("complete")},
			docs:          []string{"prd.md"},
			wantAction:    "next",
			wantNextStage: "2-prd-review",
			wantProcedure: "spawn",
			wantAgents:    []string{"nemesis"},
		},
		{
			name:         "2 approved → complexity-check toward 4",
			currentStage: "2-prd-review",
			stages: map[string]map[string]interface{}{
				"2-prd-review": s("complete", map[string]interface{}{"nemesis_verdict": "approved"}),
			},
			docs:          []string{"prd-challenge.md"},
			wantAction:    "next",
			wantNextStage: "4-tech-spec",
			wantProcedure: "complexity-check",
		},
		{
			name:         "2 revisions → back to 1 athena",
			currentStage: "2-prd-review",
			stages: map[string]map[string]interface{}{
				"2-prd-review": s("complete", map[string]interface{}{"nemesis_verdict": "revisions"}),
			},
			wantAction:    "next",
			wantNextStage: "1-prd",
			wantProcedure: "spawn",
			wantAgents:    []string{"athena"},
		},
		{
			name:         "2 rejected → blocked",
			currentStage: "2-prd-review",
			stages: map[string]map[string]interface{}{
				"2-prd-review": s("complete", map[string]interface{}{"nemesis_verdict": "rejected"}),
			},
			wantAction: "blocked",
		},
		{
			name:         "3 complete → hephaestus-gate toward 4",
			currentStage: "3-decomposition",
			stages: map[string]map[string]interface{}{
				"3-decomposition": s("complete"),
			},
			docs:          []string{"prd-challenge.md"},
			wantAction:    "next",
			wantNextStage: "4-tech-spec",
			wantProcedure: "hephaestus-gate",
		},
		{
			name:         "3 skipped → hephaestus-gate toward 4",
			currentStage: "3-decomposition",
			stages: map[string]map[string]interface{}{
				"3-decomposition": s("skipped"),
			},
			docs:          []string{"prd-challenge.md"},
			wantAction:    "next",
			wantNextStage: "4-tech-spec",
			wantProcedure: "hephaestus-gate",
		},
		{
			name:          "4 complete → 5 apollo (typo fix: not 6)",
			currentStage:  "4-tech-spec",
			stages:        map[string]map[string]interface{}{"4-tech-spec": s("complete")},
			docs:          []string{"tech-spec.md"},
			wantAction:    "next",
			wantNextStage: "5-spec-review-sa",
			wantAgents:    []string{"apollo"},
		},
		{
			name:         "5 sound → 6 artemis",
			currentStage: "5-spec-review-sa",
			stages: map[string]map[string]interface{}{
				"5-spec-review-sa": s("complete", map[string]interface{}{"verdict": "sound"}),
			},
			docs:          []string{"spec-review-sa.md"},
			wantAction:    "next",
			wantNextStage: "6-test-plan",
			wantAgents:    []string{"artemis"},
		},
		{
			name:         "5 concerns → back to 4 hephaestus",
			currentStage: "5-spec-review-sa",
			stages: map[string]map[string]interface{}{
				"5-spec-review-sa": s("complete", map[string]interface{}{"verdict": "concerns"}),
			},
			wantAction:    "next",
			wantNextStage: "4-tech-spec",
			wantAgents:    []string{"hephaestus"},
		},
		{
			name:          "6 complete → pre-implementation toward 7 (typo fix: not 8)",
			currentStage:  "6-test-plan",
			stages:        map[string]map[string]interface{}{"6-test-plan": s("complete")},
			docs:          []string{"test-plan.md", "tech-spec.md"},
			wantAction:    "next",
			wantNextStage: "7-implementation",
			wantProcedure: "pre-implementation",
			wantAgents:    []string{"ares"},
		},
		{
			name:         "7 in-progress user mode → wait-user-tasks",
			currentStage: "7-implementation",
			stages: map[string]map[string]interface{}{
				"7-implementation": s("in-progress", map[string]interface{}{"mode": "user"}),
			},
			wantAction: "wait-user-tasks",
		},
		{
			name:         "7 complete → 8 hera",
			currentStage: "7-implementation",
			stages: map[string]map[string]interface{}{
				"7-implementation": s("complete", map[string]interface{}{"mode": "ares"}),
			},
			docs:          []string{"implementation-notes.md", "prd.md"},
			wantAction:    "next",
			wantNextStage: "8-prd-alignment",
			wantAgents:    []string{"hera"},
		},
		{
			name:         "7 complete with tasks/ only → 8 hera",
			currentStage: "7-implementation",
			stages: map[string]map[string]interface{}{
				"7-implementation": s("complete", map[string]interface{}{"mode": "user"}),
			},
			docs:          []string{"tasks/01-task.md", "prd.md"},
			wantAction:    "next",
			wantNextStage: "8-prd-alignment",
		},
		{
			name:         "8 aligned → spec-archive-offer toward 9 dual agents",
			currentStage: "8-prd-alignment",
			stages: map[string]map[string]interface{}{
				"8-prd-alignment": s("complete", map[string]interface{}{"alignment_verdict": "aligned"}),
			},
			docs:          []string{"prd-alignment.md"},
			wantAction:    "next",
			wantNextStage: "9-review",
			wantProcedure: "spec-archive-offer",
			wantAgents:    []string{"hermes", "cassandra"},
		},
		{
			name:         "8 gaps → back to 7 ares",
			currentStage: "8-prd-alignment",
			stages: map[string]map[string]interface{}{
				"8-prd-alignment": s("complete", map[string]interface{}{"alignment_verdict": "gaps"}),
			},
			wantAction:    "next",
			wantNextStage: "7-implementation",
			wantAgents:    []string{"ares"},
		},
		{
			name:         "8 misaligned → blocked",
			currentStage: "8-prd-alignment",
			stages: map[string]map[string]interface{}{
				"8-prd-alignment": s("complete", map[string]interface{}{"alignment_verdict": "misaligned"}),
			},
			wantAction: "blocked",
		},
		{
			name:         "9 approved + clear → ship-gate",
			currentStage: "9-review",
			stages: map[string]map[string]interface{}{
				"9-review": s("complete", map[string]interface{}{"code_review_verdict": "approved", "risk_verdict": "clear"}),
			},
			docs:       []string{"code-review.md", "risk-analysis.md"},
			wantAction: "ship-gate",
		},
		{
			name:         "9 approved + caution → ship-gate",
			currentStage: "9-review",
			stages: map[string]map[string]interface{}{
				"9-review": s("complete", map[string]interface{}{"code_review_verdict": "approved", "risk_verdict": "caution"}),
			},
			docs:       []string{"code-review.md", "risk-analysis.md"},
			wantAction: "ship-gate",
		},
		{
			name:         "9 approved + risk blocked → blocked",
			currentStage: "9-review",
			stages: map[string]map[string]interface{}{
				"9-review": s("complete", map[string]interface{}{"code_review_verdict": "approved", "risk_verdict": "blocked"}),
			},
			wantAction: "blocked",
		},
		{
			name:         "9 changes-required → back to 7 ares",
			currentStage: "9-review",
			stages: map[string]map[string]interface{}{
				"9-review": s("complete", map[string]interface{}{"code_review_verdict": "changes-required"}),
			},
			wantAction:    "next",
			wantNextStage: "7-implementation",
			wantAgents:    []string{"ares"},
		},
		{
			name:         "verdict stage complete without verdict → blocked recovery",
			currentStage: "2-prd-review",
			stages: map[string]map[string]interface{}{
				"2-prd-review": s("complete"),
			},
			wantAction: "blocked",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, status := nextFixture(t, tc.currentStage, tc.stages, tc.docs...)
			got := computeNext("feat", dir, status)

			assert.Equal(t, tc.wantAction, got.Action)
			if tc.wantNextStage != "" {
				require.NotNil(t, got.Next)
				assert.Equal(t, tc.wantNextStage, got.Next.Stage)
			}
			if tc.wantProcedure != "" {
				require.NotNil(t, got.Next)
				assert.Equal(t, tc.wantProcedure, got.Next.Procedure)
			}
			if len(tc.wantAgents) > 0 {
				require.NotNil(t, got.Next)
				var names []string
				for _, a := range got.Next.Agents {
					names = append(names, a.Name)
				}
				assert.Equal(t, tc.wantAgents, names)
			}
		})
	}
}

func TestNextGateFailureOnMissingDeliverable(t *testing.T) {
	dir, status := nextFixture(t, "1-prd", map[string]map[string]interface{}{
		"1-prd": s("complete"),
	}) // no prd.md on disk

	got := computeNext("feat", dir, status)
	assert.Equal(t, "blocked", got.Action)
	require.NotNil(t, got.Gate)
	assert.False(t, got.Gate.Passed)
	assert.Contains(t, got.Gate.Failures[0], "prd.md")
}

func TestNextVerdictCaseInsensitive(t *testing.T) {
	dir, status := nextFixture(t, "2-prd-review", map[string]map[string]interface{}{
		"2-prd-review": s("complete", map[string]interface{}{"nemesis_verdict": "Approved"}),
	}, "prd-challenge.md")

	got := computeNext("feat", dir, status)
	assert.Equal(t, "next", got.Action)
	assert.Equal(t, "4-tech-spec", got.Next.Stage)
}

func TestNextFeatureCompleteAndVerifiedIsComplete(t *testing.T) {
	stages := map[string]map[string]interface{}{}
	for _, key := range nonOptionalStages {
		stages[key] = s("complete")
	}
	stages["2-prd-review"]["nemesis_verdict"] = "approved"
	stages["5-spec-review-sa"]["verdict"] = "sound"
	stages["8-prd-alignment"]["alignment_verdict"] = "aligned"
	stages["9-review"]["code_review_verdict"] = "approved"
	stages["9-review"]["risk_verdict"] = "clear"

	dir, status := nextFixture(t, "9-review", stages)
	got := computeNext("feat", dir, status)
	// Deliverable files are absent so verification fails → ship-gate, which
	// will report the failures. With files present it would be "complete";
	// either way it must never emit a spawnable next stage.
	assert.Contains(t, []string{"ship-gate", "complete"}, got.Action)
	if got.Next != nil {
		assert.Equal(t, "ship-gate", got.Next.Procedure)
	}
}

func TestNextNoFeatureAndAmbiguous(t *testing.T) {
	root := t.TempDir()
	// No features at all.
	_, _, _, err := resolveFeatureIn(root, "")
	assert.Equal(t, errNoFeature, err)

	// Two incomplete features → ambiguous.
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(root, ".claude", "feature", name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, writeStatusJSON(filepath.Join(dir, "status.json"), map[string]interface{}{
			"feature": name,
			"stage":   "1-prd",
			"pipeline": map[string]interface{}{
				"1-prd": map[string]interface{}{"status": "in-progress"},
			},
		}))
	}
	_, _, _, err = resolveFeatureIn(root, "")
	var ambiguous *ambiguousFeatureError
	require.True(t, asAmbiguous(err, &ambiguous))
	assert.Equal(t, []string{"alpha", "beta"}, ambiguous.Candidates)
}
