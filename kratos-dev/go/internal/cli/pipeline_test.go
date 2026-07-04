package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPipelineTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create a fake git root by overriding gitRoot behavior
	featureDir := filepath.Join(tmpDir, ".claude", "feature", "test-feat")
	err := os.MkdirAll(featureDir, 0o755)
	require.NoError(t, err)

	return tmpDir, func() {}
}

func TestPipelineInit(t *testing.T) {
	tmpDir, cleanup := setupPipelineTest(t)
	defer cleanup()

	path := filepath.Join(tmpDir, ".claude", "feature", "test-feat", "status.json")

	// Directly call init with a known path (bypass gitRoot)
	ts := now()
	status := map[string]interface{}{
		"feature":     "test-feat",
		"description": "A test feature",
		"priority":    "P2",
		"created":     ts,
		"updated":     ts,
		"stage":       "1-prd",
		"pipeline": map[string]interface{}{
			"1-prd": map[string]interface{}{
				"status":  "in-progress",
				"started": ts,
			},
		},
		"documents": map[string]interface{}{},
		"history":   []interface{}{},
	}

	err := writeStatusJSON(path, status)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Verify content
	read, err := readStatusJSON(path)
	require.NoError(t, err)
	assert.Equal(t, "test-feat", read["feature"])
	assert.Equal(t, "A test feature", read["description"])
	assert.Equal(t, "P2", read["priority"])
	assert.Equal(t, "1-prd", read["stage"])
	assert.NotEmpty(t, read["created"])
}

func TestPipelineUpdate(t *testing.T) {
	tmpDir, cleanup := setupPipelineTest(t)
	defer cleanup()

	path := filepath.Join(tmpDir, ".claude", "feature", "test-feat", "status.json")

	// Create initial status
	initial := map[string]interface{}{
		"feature": "test-feat",
		"stage":   "1-prd",
		"updated": now(),
		"pipeline": map[string]interface{}{
			"1-prd": map[string]interface{}{
				"status":    "in-progress",
				"started":   now(),
				"completed": nil,
			},
		},
		"documents": map[string]interface{}{},
		"history":   []interface{}{},
	}
	err := writeStatusJSON(path, initial)
	require.NoError(t, err)

	// Read, update, write (simulating pipelineUpdate logic)
	statusJSON, err := readStatusJSON(path)
	require.NoError(t, err)

	ts := now()
	pipeline := statusJSON["pipeline"].(map[string]interface{})
	stageMap := pipeline["1-prd"].(map[string]interface{})

	stageMap["status"] = "complete"
	stageMap["completed"] = ts
	statusJSON["updated"] = ts
	statusJSON["stage"] = "1-prd"

	history := statusJSON["history"].([]interface{})
	statusJSON["history"] = append(history, map[string]interface{}{
		"timestamp": ts,
		"stage":     "1-prd",
		"action":    "status changed from 'in-progress' to 'complete'",
	})

	err = writeStatusJSON(path, statusJSON)
	require.NoError(t, err)

	// Verify
	result, err := readStatusJSON(path)
	require.NoError(t, err)

	pipelineResult := result["pipeline"].(map[string]interface{})
	prdResult := pipelineResult["1-prd"].(map[string]interface{})
	assert.Equal(t, "complete", prdResult["status"])
	assert.NotNil(t, prdResult["completed"])

	historyResult := result["history"].([]interface{})
	assert.Len(t, historyResult, 1)
}

func TestPipelineGet(t *testing.T) {
	tmpDir, cleanup := setupPipelineTest(t)
	defer cleanup()

	path := filepath.Join(tmpDir, ".claude", "feature", "test-feat", "status.json")

	status := map[string]interface{}{
		"feature": "test-feat",
		"stage":   "1-prd",
	}
	err := writeStatusJSON(path, status)
	require.NoError(t, err)

	result, err := readStatusJSON(path)
	require.NoError(t, err)
	assert.Equal(t, "test-feat", result["feature"])
}

func TestWriteStatusJSONAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "status.json")

	status := map[string]interface{}{"key": "value"}
	err := writeStatusJSON(path, status)
	require.NoError(t, err)

	// Verify no temp file remains
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))

	// Verify content
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestNowReturnsRFC3339(t *testing.T) {
	ts := now()
	assert.NotEmpty(t, ts)
	// RFC3339 contains 'T' separator and timezone
	assert.Contains(t, ts, "T")
}

func TestIsFeatureComplete(t *testing.T) {
	makeStage := func(status string) map[string]interface{} {
		return map[string]interface{}{"status": status}
	}
	allComplete := map[string]interface{}{
		"pipeline": map[string]interface{}{
			"1-prd":            makeStage("complete"),
			"2-prd-review":     makeStage("complete"),
			"3-decomposition":  makeStage("skipped"),
			"4-tech-spec":      makeStage("complete"),
			"5-spec-review-sa": makeStage("complete"),
			"6-test-plan":      makeStage("complete"),
			"7-implementation": makeStage("complete"),
			"8-prd-alignment":  makeStage("complete"),
			"9-review":         makeStage("complete"),
		},
	}
	assert.True(t, isFeatureComplete(allComplete), "all non-optional complete → true")

	// One non-optional stage in-progress
	inProgress := map[string]interface{}{
		"pipeline": map[string]interface{}{
			"1-prd":            makeStage("complete"),
			"2-prd-review":     makeStage("complete"),
			"3-decomposition":  makeStage("skipped"),
			"4-tech-spec":      makeStage("in-progress"),
			"5-spec-review-sa": makeStage("blocked"),
			"6-test-plan":      makeStage("blocked"),
			"7-implementation": makeStage("blocked"),
			"8-prd-alignment":  makeStage("blocked"),
			"9-review":         makeStage("blocked"),
		},
	}
	assert.False(t, isFeatureComplete(inProgress), "in-progress non-optional stage → false")

	// Optional stage skipped does not prevent completion
	withSkippedOptional := allComplete // same as allComplete: 3-decomposition is skipped
	assert.True(t, isFeatureComplete(withSkippedOptional), "skipped optional stage does not block completion")
}

func TestFeatureProgress(t *testing.T) {
	makeStage := func(status string) map[string]interface{} {
		return map[string]interface{}{"status": status}
	}

	noneComplete := map[string]interface{}{
		"pipeline": map[string]interface{}{
			"1-prd":            makeStage("in-progress"),
			"2-prd-review":     makeStage("blocked"),
			"3-decomposition":  makeStage("skipped"),
			"4-tech-spec":      makeStage("blocked"),
			"5-spec-review-sa": makeStage("blocked"),
			"6-test-plan":      makeStage("blocked"),
			"7-implementation": makeStage("blocked"),
			"8-prd-alignment":  makeStage("blocked"),
			"9-review":         makeStage("blocked"),
		},
	}
	done, total := featureProgress(noneComplete)
	assert.Equal(t, 0, done)
	assert.Equal(t, 8, total)

	twoComplete := map[string]interface{}{
		"pipeline": map[string]interface{}{
			"1-prd":            makeStage("complete"),
			"2-prd-review":     makeStage("complete"),
			"3-decomposition":  makeStage("skipped"),
			"4-tech-spec":      makeStage("in-progress"),
			"5-spec-review-sa": makeStage("blocked"),
			"6-test-plan":      makeStage("blocked"),
			"7-implementation": makeStage("blocked"),
			"8-prd-alignment":  makeStage("blocked"),
			"9-review":         makeStage("blocked"),
		},
	}
	done, total = featureProgress(twoComplete)
	assert.Equal(t, 2, done)
	assert.Equal(t, 8, total)

	allComplete := map[string]interface{}{
		"pipeline": map[string]interface{}{
			"1-prd":            makeStage("complete"),
			"2-prd-review":     makeStage("complete"),
			"3-decomposition":  makeStage("skipped"),
			"4-tech-spec":      makeStage("complete"),
			"5-spec-review-sa": makeStage("complete"),
			"6-test-plan":      makeStage("complete"),
			"7-implementation": makeStage("complete"),
			"8-prd-alignment":  makeStage("complete"),
			"9-review":         makeStage("complete"),
		},
	}
	done, total = featureProgress(allComplete)
	assert.Equal(t, 8, done)
	assert.Equal(t, 8, total)
}

func TestDiscoverStatusSymbol(t *testing.T) {
	assert.Equal(t, "✓", discoverStatusSymbol("complete"))
	assert.Equal(t, "⋯", discoverStatusSymbol("in-progress"))
	assert.Equal(t, "-", discoverStatusSymbol("skipped"))
	assert.Equal(t, "✗", discoverStatusSymbol("blocked"))
	assert.Equal(t, "✗", discoverStatusSymbol(""))
	assert.Equal(t, "✗", discoverStatusSymbol("ready"))
}
