package cli

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPipelineUpdateConcurrentVerdictsDoNotClobber exercises the exact race
// item A closes: Hermes and Cassandra both call `pipeline update --stage 9`
// concurrently, one filing code_review_verdict and the other risk_verdict.
// Before withStatusLock serialized the read-modify-write, both callers read
// the same on-disk snapshot and whichever wrote last clobbered the other's
// verdict — the reported repro lost one field in 30 of 30 runs. 30 rounds
// here reproduces that reliably without the fix and must be clean with it.
// This test uses only pre-existing symbols (pipelineUpdate, readStatusJSON,
// writeStatusJSON, now) so it compiles against the pre-fix baseline too —
// see pipeline_lock_internals_test.go for tests of the lock helper itself.
func TestPipelineUpdateConcurrentVerdictsDoNotClobber(t *testing.T) {
	tmpDir := t.TempDir()
	featureDir := filepath.Join(tmpDir, ".claude", "feature", "concurrent-feat")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(oldWd) }()

	path := filepath.Join(featureDir, "status.json")

	const rounds = 30
	for i := 0; i < rounds; i++ {
		// Reset stage 9 between rounds so every round starts from the same
		// "neither verdict filed yet" state as the reported repro.
		require.NoError(t, writeStatusJSON(path, map[string]interface{}{
			"feature":  "concurrent-feat",
			"stage":    "9-review",
			"updated":  now(),
			"pipeline": map[string]interface{}{},
			"history":  []interface{}{},
		}))

		var wg sync.WaitGroup
		wg.Add(2)
		errs := make(chan error, 2)
		go func() {
			defer wg.Done()
			errs <- pipelineUpdate("concurrent-feat", "9", "complete", "", "approved", "code-review.md", "")
		}()
		go func() {
			defer wg.Done()
			errs <- pipelineUpdate("concurrent-feat", "9", "complete", "", "clear", "risk-analysis.md", "")
		}()
		wg.Wait()
		close(errs)
		for e := range errs {
			require.NoError(t, e)
		}

		result, rerr := readStatusJSON(path)
		require.NoError(t, rerr)
		stage, _ := result["pipeline"].(map[string]interface{})["9-review"].(map[string]interface{})
		require.NotNil(t, stage, "round %d: stage 9-review missing", i)
		assert.Equal(t, "approved", stage["code_review_verdict"], "round %d: Hermes verdict lost", i)
		assert.Equal(t, "clear", stage["risk_verdict"], "round %d: Cassandra verdict lost", i)
	}
}
