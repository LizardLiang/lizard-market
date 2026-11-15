package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFeature(t *testing.T, root, name, statusJSON string, mtime time.Time) string {
	t.Helper()
	dir := filepath.Join(root, ".claude", "feature", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	if statusJSON != "" {
		p := filepath.Join(dir, "status.json")
		require.NoError(t, os.WriteFile(p, []byte(statusJSON), 0o644))
		require.NoError(t, os.Chtimes(p, mtime, mtime))
	}
	require.NoError(t, os.Chtimes(dir, mtime, mtime))
	return dir
}

func TestPipelineGC(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	old := now.Add(-60 * 24 * time.Hour)

	writeFeature(t, root, "legacy-keys", `{"pipeline":{"1-prd":{"status":"complete"},"10-prd-alignment":{"status":"complete"},"11-review":{"status":"pending"}}}`, now)
	writeFeature(t, root, "stale-feature", `{"pipeline":{"7-implementation":{"status":"complete"}}}`, old)
	writeFeature(t, root, "fresh-feature", `{"pipeline":{"7-implementation":{"status":"in-progress"}}}`, now)
	pendingDir := writeFeature(t, root, "pending-delta", `{"pipeline":{"9-review":{"status":"complete"}}}`, old)
	require.NoError(t, os.MkdirAll(filepath.Join(pendingDir, "spec-delta"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pendingDir, "spec-delta", "cap.md"), []byte("## ADDED Requirements\n"), 0o644))
	writeFeature(t, root, "plan-only-old", "", old)
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude", "feature", "_archive", "already"), 0o755))

	dry, err := pipelineGCIn(root, 30*24*time.Hour, true, now)
	require.NoError(t, err)
	assert.Len(t, dry.Archived, 3, "%+v", dry)
	_, statErr := os.Stat(filepath.Join(root, ".claude", "feature", "legacy-keys"))
	assert.NoError(t, statErr, "dry run must not move anything")

	res, err := pipelineGCIn(root, 30*24*time.Hour, false, now)
	require.NoError(t, err)
	names := map[string]string{}
	for _, a := range res.Archived {
		names[a.Feature] = a.Reason
	}
	assert.Contains(t, names["legacy-keys"], "legacy stage keys")
	assert.Contains(t, names["stale-feature"], "no activity")
	assert.Contains(t, names["plan-only-old"], "plan-only")
	assert.Equal(t, []string{"fresh-feature"}, res.Kept)
	require.Len(t, res.Skipped, 1)
	assert.Equal(t, "pending-delta", res.Skipped[0].Feature)

	_, err = os.Stat(filepath.Join(root, ".claude", "feature", "_archive", "legacy-keys", "status.json"))
	assert.NoError(t, err, "archived folder moved under _archive/")
	_, err = os.Stat(filepath.Join(root, ".claude", "feature", "legacy-keys"))
	assert.True(t, os.IsNotExist(err))

	// Idempotent: a second pass archives nothing new and never descends into _archive/.
	again, err := pipelineGCIn(root, 30*24*time.Hour, false, now)
	require.NoError(t, err)
	assert.Empty(t, again.Archived)
}

func TestLegacyStageKeys(t *testing.T) {
	p := map[string]interface{}{"1-prd": 1, "9-review": 1, "8-code-review": 1, "notes": 1, "0-research": 1}
	assert.Equal(t, []string{"8-code-review"}, legacyStageKeys(p))
}
