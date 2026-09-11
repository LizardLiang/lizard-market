package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// `session gc` closes ghost rows and prunes old ledger files; --dry-run only
// counts. HOME is redirected so the ledger prune never touches ~/.kratos.
func TestSessionGc(t *testing.T) {
	useTempDB(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	ledger := filepath.Join(home, ".kratos", "sessions")
	require.NoError(t, os.MkdirAll(ledger, 0o755))
	oldFile := filepath.Join(ledger, "old.json")
	newFile := filepath.Join(ledger, "new.json")
	require.NoError(t, os.WriteFile(oldFile, []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(newFile, []byte("{}"), 0o644))
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldFile, tenDaysAgo, tenDaysAgo))

	conn, err := db.GetConnection()
	require.NoError(t, err)
	now := time.Now().UnixMilli()
	for _, s := range []*models.Session{
		{SessionID: "ghost", Project: "/p", StartedAt: now - 30*3600*1000, Status: "active"},
		{SessionID: "stale-busy", Project: "/p", StartedAt: now - 10*24*3600*1000, Status: "active", TotalSteps: 5},
		{SessionID: "live", Project: "/p", StartedAt: now - 3600*1000, Status: "active", TotalSteps: 2},
	} {
		require.NoError(t, db.CreateSession(conn, s))
	}
	conn.Close()

	dry, err := runCLI(t, "session", "gc", "--dry-run")
	require.NoError(t, err)
	assert.Equal(t, true, dry["dry_run"])
	assert.Equal(t, float64(2), dry["session_candidates"])
	assert.Equal(t, float64(0), dry["sessions_abandoned"])
	assert.Equal(t, float64(1), dry["ledger_candidates"])
	assert.Equal(t, float64(0), dry["ledger_removed"])
	_, statErr := os.Stat(oldFile)
	assert.NoError(t, statErr, "dry run deletes nothing")

	res, err := runCLI(t, "session", "gc")
	require.NoError(t, err)
	assert.Equal(t, float64(2), res["sessions_abandoned"])
	assert.Equal(t, float64(1), res["ledger_removed"])
	_, statErr = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(statErr), "old ledger file removed")
	_, statErr = os.Stat(newFile)
	assert.NoError(t, statErr, "fresh ledger file kept")

	conn, err = db.GetConnection()
	require.NoError(t, err)
	defer conn.Close()
	for id, want := range map[string]string{"ghost": "abandoned", "stale-busy": "abandoned", "live": "active"} {
		s, err := db.GetSession(conn, id)
		require.NoError(t, err)
		assert.Equal(t, want, s.Status, id)
	}

	_, err = runCLI(t, "session", "gc", "--days", "0")
	require.Error(t, err)
}

// `session start` closes zero-step ghosts on its own — the legacy path used
// to refuse ("active session already exists") because of them.
func TestSessionStart_AbandonsIdleGhosts(t *testing.T) {
	useTempDB(t)
	conn, err := db.GetConnection()
	require.NoError(t, err)
	now := time.Now().UnixMilli()
	require.NoError(t, db.CreateSession(conn, &models.Session{SessionID: "ghost", Project: "/proj/a", StartedAt: now - 30*3600*1000, Status: "active"}))
	require.NoError(t, db.CreateSession(conn, &models.Session{SessionID: "recent", Project: "/proj/b", StartedAt: now - 3600*1000, Status: "active"}))
	conn.Close()

	// Legacy path: the ghost no longer blocks a new session in the same project.
	started, err := runCLI(t, "session", "start", "/proj/a")
	require.NoError(t, err)
	assert.Equal(t, "active", started["status"])

	conn, err = db.GetConnection()
	require.NoError(t, err)
	defer conn.Close()
	ghost, err := db.GetSession(conn, "ghost")
	require.NoError(t, err)
	assert.Equal(t, "abandoned", ghost.Status)
	recent, err := db.GetSession(conn, "recent")
	require.NoError(t, err)
	assert.Equal(t, "active", recent.Status, "a fresh zero-step row is not a ghost yet")
}
