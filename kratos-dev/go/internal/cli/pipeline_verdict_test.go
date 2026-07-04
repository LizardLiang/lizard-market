package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerdictField(t *testing.T) {
	cases := []struct {
		stage, verdict, wantField, wantValue string
		wantErr                              bool
	}{
		{"2-prd-review", "approved", "nemesis_verdict", "approved", false},
		{"5-spec-review-sa", "concerns", "verdict", "concerns", false},
		{"8-prd-alignment", "gaps", "alignment_verdict", "gaps", false},
		{"9-review", "changes-required", "code_review_verdict", "changes-required", false},
		{"9-review", "changes-requested", "code_review_verdict", "changes-required", false}, // synonym normalized
		{"9-review", "Approved", "code_review_verdict", "approved", false},                  // case-insensitive
		{"9-review", "caution", "risk_verdict", "caution", false},
		{"9-review", "bogus", "", "", true},   // unknown value
		{"1-prd", "approved", "", "", true},   // non-review stage
		{"6-test-plan", "clear", "", "", true}, // non-review stage
	}
	for _, c := range cases {
		field, value, err := verdictField(c.stage, c.verdict)
		if c.wantErr {
			assert.Error(t, err, "%s/%s", c.stage, c.verdict)
			continue
		}
		require.NoError(t, err, "%s/%s", c.stage, c.verdict)
		assert.Equal(t, c.wantField, field, "%s/%s", c.stage, c.verdict)
		assert.Equal(t, c.wantValue, value, "%s/%s", c.stage, c.verdict)
	}
}

// TestPipelineUpdateUpsertsStageAndRoutesVerdicts exercises the real update
// path against a status.json whose pipeline map is empty (quick-path features,
// harness fixtures, recovered pipelines). Before the upsert fix this errored
// "unknown stage" and pushed agents into hand-editing status.json.
func TestPipelineUpdateUpsertsStageAndRoutesVerdicts(t *testing.T) {
	tmpDir := t.TempDir()
	featureDir := filepath.Join(tmpDir, ".claude", "feature", "test-feat")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(oldWd) }()

	path := filepath.Join(featureDir, "status.json")
	require.NoError(t, writeStatusJSON(path, map[string]interface{}{
		"feature":  "test-feat",
		"stage":    "9-review",
		"updated":  now(),
		"pipeline": map[string]interface{}{},
		"history":  []interface{}{},
	}))

	// Hermes writes its verdict on the missing stage key (upsert), Cassandra
	// writes hers afterwards — the two must land in different fields.
	require.NoError(t, pipelineUpdate("test-feat", "9", "complete", "", "changes-requested", "code-review.md", ""))
	require.NoError(t, pipelineUpdate("test-feat", "9", "complete", "", "caution", "risk-analysis.md", ""))

	result, err := readStatusJSON(path)
	require.NoError(t, err)
	stage := result["pipeline"].(map[string]interface{})["9-review"].(map[string]interface{})

	assert.Equal(t, "complete", stage["status"])
	assert.Equal(t, "changes-required", stage["code_review_verdict"]) // normalized synonym
	assert.Equal(t, "caution", stage["risk_verdict"])                 // no clobbering
	assert.Nil(t, stage["verdict"], "generic verdict field is not part of the 9-review schema")
	assert.NotEmpty(t, stage["started"])
	assert.NotEmpty(t, stage["completed"])

	// Invalid verdict is a hard error, not a silent misfile.
	err = pipelineUpdate("test-feat", "9", "complete", "", "lgtm", "", "")
	assert.Error(t, err)
}
