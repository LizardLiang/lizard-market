package db

import (
	"testing"
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ghost rows — throwaway SessionStart ids and windows closed without a
// SessionEnd — are abandoned by age; live rows are untouched.
func TestAbandonStaleSessions(t *testing.T) {
	conn := NewTestDBWithSchema(t)
	now := time.Now().UnixMilli()
	mk := func(id string, ageHours int, steps int64) {
		s := &models.Session{SessionID: id, Project: "/p", StartedAt: now - int64(ageHours)*3600*1000, Status: "active", TotalSteps: steps}
		require.NoError(t, CreateSession(conn, s))
	}
	mk("fresh-empty", 1, 0)   // just started, no steps yet — keep
	mk("ghost-empty", 30, 0)  // throwaway startup id — abandon (idle rule)
	mk("busy-2d", 48, 12)     // window still open, or closed without SessionEnd — keep under idle rule
	mk("busy-10d", 240, 72)   // ten days old — abandon only under the stale rule
	ended := &models.Session{SessionID: "done", Project: "/p", StartedAt: now - 240*3600*1000, Status: "active"}
	require.NoError(t, CreateSession(conn, ended))
	require.NoError(t, EndSession(conn, "done", "finished"))

	n, err := CountStaleSessions(conn, 24*time.Hour, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "idle rule alone: only the zero-step ghost")

	n, err = AbandonStaleSessions(conn, 24*time.Hour, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	got, err := GetSession(conn, "ghost-empty")
	require.NoError(t, err)
	assert.Equal(t, "abandoned", got.Status)
	assert.NotNil(t, got.EndedAt)
	require.NotNil(t, got.Summary)
	assert.Contains(t, *got.Summary, "auto-closed")
	for _, id := range []string{"fresh-empty", "busy-2d", "busy-10d"} {
		s, err := GetSession(conn, id)
		require.NoError(t, err)
		assert.Equal(t, "active", s.Status, id)
	}

	n, err = AbandonStaleSessions(conn, 24*time.Hour, 7*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "stale rule: the ten-day-old busy row")
	s, err := GetSession(conn, "busy-10d")
	require.NoError(t, err)
	assert.Equal(t, "abandoned", s.Status)
	s, err = GetSession(conn, "busy-2d")
	require.NoError(t, err)
	assert.Equal(t, "active", s.Status)
	s, err = GetSession(conn, "done")
	require.NoError(t, err)
	assert.Equal(t, "completed", s.Status, "ended rows are never rewritten")

	n, err = AbandonStaleSessions(conn, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "both rules off → no-op")

	// An abandoned id resumes like an ended one.
	require.NoError(t, ReactivateSession(conn, "ghost-empty"))
	s, err = GetSession(conn, "ghost-empty")
	require.NoError(t, err)
	assert.Equal(t, "active", s.Status)
	assert.Nil(t, s.EndedAt)
}
