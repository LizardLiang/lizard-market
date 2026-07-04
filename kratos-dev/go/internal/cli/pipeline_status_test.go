package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var statusNow = time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

func statusFixture(currentStage, updated string, stages map[string]map[string]interface{}) map[string]interface{} {
	pipeline := map[string]interface{}{}
	for key, stage := range stages {
		m := map[string]interface{}{}
		for k, v := range stage {
			m[k] = v
		}
		pipeline[key] = m
	}
	return map[string]interface{}{
		"feature":  "feat",
		"priority": "P1",
		"stage":    currentStage,
		"updated":  updated,
		"pipeline": pipeline,
	}
}

func TestBuildFeatureReportProgress(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prd.md"), []byte("x"), 0o644))
	status := statusFixture("2-prd-review", "2026-07-05T00:00:00Z", map[string]map[string]interface{}{
		"1-prd":        s("complete"),
		"2-prd-review": s("in-progress"),
	})

	r := buildFeatureReport("feat", dir, status, 7, statusNow)
	assert.Equal(t, 2, r.StageNumber)
	assert.Equal(t, 9, r.TotalStages)
	assert.Equal(t, 1, r.Completed)
	assert.Equal(t, 8, r.Total)
	assert.Equal(t, 12, r.ProgressPct) // 1/8
	assert.Equal(t, "healthy", r.Health)
	assert.Equal(t, "next", r.Next.Action)
	assert.Len(t, r.Stages, 9)
}

func TestBuildFeatureReportHealthBlocked(t *testing.T) {
	dir := t.TempDir()
	status := statusFixture("2-prd-review", "2026-07-05T00:00:00Z", map[string]map[string]interface{}{
		"1-prd":        s("complete"),
		"2-prd-review": s("complete", map[string]interface{}{"nemesis_verdict": "rejected"}),
	})

	r := buildFeatureReport("feat", dir, status, 7, statusNow)
	assert.Equal(t, "blocked", r.Health)
	assert.Equal(t, "blocked", r.Next.Action)
}

func TestBuildFeatureReportConflictVersion(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prd.md"), []byte("x"), 0o644))
	status := statusFixture("5-spec-review-sa", "2026-07-05T00:00:00Z", map[string]map[string]interface{}{
		"1-prd":            s("complete", map[string]interface{}{"completed": "2026-07-03T00:00:00Z"}),
		"4-tech-spec":      s("complete", map[string]interface{}{"based_on_prd_version": "2026-07-01T00:00:00Z"}),
		"5-spec-review-sa": s("in-progress"),
	})

	r := buildFeatureReport("feat", dir, status, 7, statusNow)
	assert.Equal(t, "conflict", r.Health)
	require.NotEmpty(t, r.Conflicts)
	assert.Contains(t, r.Conflicts[0], "outdated")
}

func TestBuildFeatureReportConflictMtime(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "tech-spec.md")
	prdPath := filepath.Join(dir, "prd.md")
	require.NoError(t, os.WriteFile(specPath, []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(prdPath, []byte("x"), 0o644))
	old := statusNow.Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(specPath, old, old))
	require.NoError(t, os.Chtimes(prdPath, statusNow, statusNow))

	status := statusFixture("5-spec-review-sa", "2026-07-05T00:00:00Z", map[string]map[string]interface{}{
		"1-prd":            s("complete"),
		"4-tech-spec":      s("complete"),
		"5-spec-review-sa": s("in-progress"),
	})

	r := buildFeatureReport("feat", dir, status, 7, statusNow)
	assert.Equal(t, "conflict", r.Health)
	assert.Contains(t, r.Conflicts[0], "modified after")
}

func TestBuildFeatureReportStale(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prd.md"), []byte("x"), 0o644))
	status := statusFixture("1-prd", "2026-06-01T00:00:00Z", map[string]map[string]interface{}{
		"1-prd": s("in-progress"),
	})

	r := buildFeatureReport("feat", dir, status, 7, statusNow)
	assert.Equal(t, "stale", r.Health)

	// Within the window → healthy.
	status["updated"] = "2026-07-04T00:00:00Z"
	r = buildFeatureReport("feat", dir, status, 7, statusNow)
	assert.Equal(t, "healthy", r.Health)
}

func TestPipelineStatusPlanOnlyDetection(t *testing.T) {
	root := t.TempDir()
	// One real feature.
	dir := filepath.Join(root, ".claude", "feature", "real")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, writeStatusJSON(filepath.Join(dir, "status.json"),
		statusFixture("1-prd", "2026-07-05T00:00:00Z", map[string]map[string]interface{}{
			"1-prd": s("in-progress"),
		})))
	// One plan-only folder (spec-delta only, no status.json).
	planDir := filepath.Join(root, ".claude", "feature", "plan-only", "spec-delta")
	require.NoError(t, os.MkdirAll(planDir, 0o755))

	require.NoError(t, pipelineStatusRun(root, "", 7, true, statusNow))
}

func TestIsStale(t *testing.T) {
	assert.False(t, isStale("", 7, statusNow))
	assert.False(t, isStale("not-a-timestamp", 7, statusNow))
	assert.True(t, isStale("2026-06-01T00:00:00Z", 7, statusNow))
	assert.False(t, isStale("2026-07-04T00:00:00Z", 7, statusNow))
}
