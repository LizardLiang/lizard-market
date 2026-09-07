package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression for the 2026-09 review: hooks record steps against the Claude
// Code session id, which had no row, and RecordAgentSpawn failed with
// "FOREIGN KEY constraint failed". EnsureSession creates the row on demand.
func TestEnsureSession_CreatesThenReuses(t *testing.T) {
	db := NewTestDBWithSchema(t)

	s, created, err := EnsureSession(db, "claude-sess-1", "/proj/a")
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "claude-sess-1", s.SessionID)
	assert.Equal(t, "/proj/a", s.Project)
	assert.Equal(t, "active", s.Status)

	again, created, err := EnsureSession(db, "claude-sess-1", "/proj/other")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, s.ID, again.ID)
	assert.Equal(t, "/proj/a", again.Project, "existing row wins over a later project hint")

	// Recording against the ensured id must not hit the foreign key.
	require.NoError(t, RecordAgentSpawn(db, "claude-sess-1", "odysseus", "sonnet", "plan"))
	got, err := GetSession(db, "claude-sess-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), got.TotalAgentsSpawned)
}

func TestEnsureSession_RequiresID(t *testing.T) {
	db := NewTestDBWithSchema(t)
	_, _, err := EnsureSession(db, "", "/proj")
	assert.Error(t, err)
}

func TestEnsureSession_DefaultsProject(t *testing.T) {
	db := NewTestDBWithSchema(t)
	s, _, err := EnsureSession(db, "sess-x", "")
	require.NoError(t, err)
	assert.Equal(t, "unknown", s.Project)
}

func TestReactivateSession(t *testing.T) {
	db := NewTestDBWithSchema(t)
	_, _, err := EnsureSession(db, "sess-r", "/proj")
	require.NoError(t, err)
	require.NoError(t, EndSession(db, "sess-r", "done"))

	ended, err := GetSession(db, "sess-r")
	require.NoError(t, err)
	assert.Equal(t, "completed", ended.Status)
	assert.NotNil(t, ended.EndedAt)

	require.NoError(t, ReactivateSession(db, "sess-r"))
	back, err := GetSession(db, "sess-r")
	require.NoError(t, err)
	assert.Equal(t, "active", back.Status)
	assert.Nil(t, back.EndedAt)
}

func TestSetInitialRequestIfEmpty(t *testing.T) {
	db := NewTestDBWithSchema(t)
	_, _, err := EnsureSession(db, "sess-i", "/proj")
	require.NoError(t, err)

	require.NoError(t, SetInitialRequestIfEmpty(db, "sess-i", "fix the login bug"))
	require.NoError(t, SetInitialRequestIfEmpty(db, "sess-i", "second prompt must not overwrite"))

	var got string
	require.NoError(t, db.QueryRow(`SELECT initial_request FROM sessions WHERE session_id = ?`, "sess-i").Scan(&got))
	assert.Equal(t, "fix the login bug", got)
}

func TestSetInitialRequestIfEmpty_Caps(t *testing.T) {
	db := NewTestDBWithSchema(t)
	_, _, err := EnsureSession(db, "sess-c", "/proj")
	require.NoError(t, err)
	long := make([]byte, 2000)
	for i := range long {
		long[i] = 'x'
	}
	require.NoError(t, SetInitialRequestIfEmpty(db, "sess-c", string(long)))
	var got string
	require.NoError(t, db.QueryRow(`SELECT initial_request FROM sessions WHERE session_id = ?`, "sess-c").Scan(&got))
	assert.Len(t, got, 500)
}
