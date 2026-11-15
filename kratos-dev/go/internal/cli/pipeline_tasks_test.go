package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTaskFixture creates root/.claude/feature/<name>/status.json with a
// stage-7 tasks container. shape "object" uses {total, completed, items[]},
// shape "array" uses the legacy bare array.
func writeTaskFixture(t *testing.T, root, name, mode, s7status, shape string, statuses map[string]string) string {
	t.Helper()

	items := []interface{}{
		map[string]interface{}{"id": "01", "name": "Task one", "file": "01-task-one.md", "status": statuses["01"]},
		map[string]interface{}{"id": "02", "name": "Task two", "file": "02-task-two.md", "status": statuses["02"]},
		map[string]interface{}{"id": "03", "name": "Task three", "file": "03-task-three.md", "status": statuses["03"]},
	}

	var tasks interface{}
	if shape == "array" {
		tasks = items
	} else {
		completed := 0
		for _, s := range statuses {
			if s == "complete" {
				completed++
			}
		}
		tasks = map[string]interface{}{"total": len(items), "completed": completed, "items": items}
	}

	status := map[string]interface{}{
		"feature": name,
		"stage":   "7-implementation",
		"updated": "2026-01-01T00:00:00Z",
		"pipeline": map[string]interface{}{
			"7-implementation": map[string]interface{}{
				"status": s7status,
				"mode":   mode,
				"tasks":  tasks,
			},
			"8-prd-alignment": map[string]interface{}{
				"status": "blocked",
			},
		},
		"history": []interface{}{},
	}

	dir := filepath.Join(root, ".claude", "feature", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "status.json")
	require.NoError(t, writeStatusJSON(path, status))
	return path
}

func readFixture(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := readStatusJSON(path)
	require.NoError(t, err)
	return data
}

func fixtureStage7(t *testing.T, path string) (map[string]interface{}, map[string]interface{}) {
	t.Helper()
	data := readFixture(t, path)
	pipeline := data["pipeline"].(map[string]interface{})
	return data, pipeline["7-implementation"].(map[string]interface{})
}

func allPending() map[string]string {
	return map[string]string{"01": "pending", "02": "pending", "03": "pending"}
}

func TestTasksCompleteSingle(t *testing.T) {
	root := t.TempDir()
	path := writeTaskFixture(t, root, "feat", "user", "in-progress", "object", allPending())

	require.NoError(t, pipelineTasksComplete(root, "feat", []string{"01"}, false, false, true))

	_, s7 := fixtureStage7(t, path)
	tasks := s7["tasks"].(map[string]interface{})
	assert.EqualValues(t, 1, tasks["completed"])
	assert.EqualValues(t, 3, tasks["total"])
	items := tasks["items"].([]interface{})
	first := items[0].(map[string]interface{})
	assert.Equal(t, "complete", first["status"])
	assert.NotEmpty(t, first["completed_at"])
	second := items[1].(map[string]interface{})
	assert.Equal(t, "pending", second["status"])
}

func TestTasksCompleteAllAdvances(t *testing.T) {
	root := t.TempDir()
	path := writeTaskFixture(t, root, "feat", "user", "in-progress", "object", allPending())

	require.NoError(t, pipelineTasksComplete(root, "feat", nil, true, false, true))

	data, s7 := fixtureStage7(t, path)
	assert.Equal(t, "complete", s7["status"])
	assert.NotEmpty(t, s7["completed"])
	pipeline := data["pipeline"].(map[string]interface{})
	s8 := pipeline["8-prd-alignment"].(map[string]interface{})
	assert.Equal(t, "ready", s8["status"])
}

func TestTasksCompleteNoAdvance(t *testing.T) {
	root := t.TempDir()
	path := writeTaskFixture(t, root, "feat", "user", "in-progress", "object", allPending())

	require.NoError(t, pipelineTasksComplete(root, "feat", nil, true, true, true))

	data, s7 := fixtureStage7(t, path)
	assert.Equal(t, "in-progress", s7["status"])
	pipeline := data["pipeline"].(map[string]interface{})
	s8 := pipeline["8-prd-alignment"].(map[string]interface{})
	assert.Equal(t, "blocked", s8["status"])
}

func TestTasksCompleteIdempotent(t *testing.T) {
	root := t.TempDir()
	statuses := allPending()
	statuses["01"] = "complete"
	path := writeTaskFixture(t, root, "feat", "user", "in-progress", "object", statuses)

	require.NoError(t, pipelineTasksComplete(root, "feat", []string{"01", "02"}, false, false, true))

	_, s7 := fixtureStage7(t, path)
	tasks := s7["tasks"].(map[string]interface{})
	assert.EqualValues(t, 2, tasks["completed"])
	items := tasks["items"].([]interface{})
	// 01 was already complete — must not gain a completed_at stamp.
	first := items[0].(map[string]interface{})
	assert.Nil(t, first["completed_at"])
}

func TestTasksCompleteUnknownIDAtomic(t *testing.T) {
	root := t.TempDir()
	path := writeTaskFixture(t, root, "feat", "user", "in-progress", "object", allPending())
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	err = pipelineTasksComplete(root, "feat", []string{"01", "99"}, false, false, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "99")
	assert.Contains(t, err.Error(), "01: Task one")

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, before, after, "file must be untouched on validation failure")
}

func TestTasksCompleteLegacyArrayShape(t *testing.T) {
	root := t.TempDir()
	path := writeTaskFixture(t, root, "feat", "user", "in-progress", "array", allPending())

	require.NoError(t, pipelineTasksComplete(root, "feat", []string{"02"}, false, false, true))

	_, s7 := fixtureStage7(t, path)
	// Bare array must be normalized to the object container on write.
	tasks, ok := s7["tasks"].(map[string]interface{})
	require.True(t, ok, "tasks must be written back as object shape")
	assert.EqualValues(t, 1, tasks["completed"])
	assert.EqualValues(t, 3, tasks["total"])
}

func TestTasksCompleteWrongMode(t *testing.T) {
	root := t.TempDir()
	writeTaskFixture(t, root, "feat", "ares", "in-progress", "object", allPending())

	err := pipelineTasksComplete(root, "feat", []string{"01"}, false, false, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in user mode")
	assert.Contains(t, err.Error(), `"ares"`)
}

func TestTasksCompleteWrongStage(t *testing.T) {
	root := t.TempDir()
	writeTaskFixture(t, root, "feat", "user", "blocked", "object", allPending())

	err := pipelineTasksComplete(root, "feat", []string{"01"}, false, false, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrong stage")
}

func TestTasksAutoResolveFeature(t *testing.T) {
	root := t.TempDir()
	path := writeTaskFixture(t, root, "active", "user", "in-progress", "object", allPending())
	// Second feature not at stage 7 — must not interfere with auto-resolution.
	other := map[string]interface{}{
		"feature": "other",
		"stage":   "1-prd",
		"pipeline": map[string]interface{}{
			"1-prd": map[string]interface{}{"status": "in-progress"},
		},
	}
	otherDir := filepath.Join(root, ".claude", "feature", "other")
	require.NoError(t, os.MkdirAll(otherDir, 0o755))
	require.NoError(t, writeStatusJSON(filepath.Join(otherDir, "status.json"), other))

	require.NoError(t, pipelineTasksComplete(root, "", []string{"03"}, false, false, true))

	_, s7 := fixtureStage7(t, path)
	tasks := s7["tasks"].(map[string]interface{})
	assert.EqualValues(t, 1, tasks["completed"])
}

func TestProgressBar(t *testing.T) {
	pct, bar := progressBar(3, 10)
	assert.Equal(t, 30, pct)
	assert.Equal(t, "[██████░░░░░░░░░░░░░░] 30% (3/10 tasks)", bar)

	pct, bar = progressBar(10, 10)
	assert.Equal(t, 100, pct)
	assert.Equal(t, "[████████████████████] 100% (10/10 tasks)", bar)

	pct, bar = progressBar(0, 0)
	assert.Equal(t, 0, pct)
	assert.Equal(t, "[░░░░░░░░░░░░░░░░░░░░] 0% (0/0 tasks)", bar)
}

func TestTasksListJSONShape(t *testing.T) {
	root := t.TempDir()
	statuses := allPending()
	statuses["01"] = "complete"
	writeTaskFixture(t, root, "feat", "user", "in-progress", "object", statuses)

	// Exercise the list path end-to-end; output shape is covered by the
	// struct's json tags, here we just assert it runs and marshals.
	require.NoError(t, pipelineTasksList(root, "feat", true))

	var result tasksListResult
	result.Pct, result.Bar = progressBar(1, 3)
	out, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"pct":33`)
}
