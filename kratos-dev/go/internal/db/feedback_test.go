package db

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitDB_CreatesAgentFeedbackTable verifies the schema upgrade: InitDB run
// against a database created before this change gains the agent_feedback table.
func TestInitDB_CreatesAgentFeedbackTable(t *testing.T) {
	db := NewTestDB(t)

	err := InitDB(db)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='agent_feedback'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "agent_feedback table should exist")
}

func TestAddFeedback(t *testing.T) {
	db := NewTestDBWithSchema(t)

	f, err := AddFeedback(db, "ares", "no magic strings — use exported consts", "lizard-market")
	require.NoError(t, err)
	assert.NotZero(t, f.ID)
	assert.Equal(t, "ares", f.Agent)
	assert.Equal(t, "no magic strings — use exported consts", f.Lesson)
	assert.Equal(t, "lizard-market", f.Project)
	assert.NotZero(t, f.CreatedAt)
}

func TestListFeedback_All(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := AddFeedback(db, "ares", "no magic strings", "proj-a")
	require.NoError(t, err)
	_, err = AddFeedback(db, "hermes", "check for magic strings", "proj-b")
	require.NoError(t, err)

	feedback, err := ListFeedback(db, "", "", 0)
	require.NoError(t, err)
	assert.Len(t, feedback, 2)
}

func TestListFeedback_FilteredByAgent(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := AddFeedback(db, "ares", "no magic strings", "proj-a")
	require.NoError(t, err)
	_, err = AddFeedback(db, "hermes", "check for magic strings", "proj-a")
	require.NoError(t, err)

	feedback, err := ListFeedback(db, "ares", "", 0)
	require.NoError(t, err)
	require.Len(t, feedback, 1)
	assert.Equal(t, "no magic strings", feedback[0].Lesson)
}

func TestListFeedback_Limit(t *testing.T) {
	db := NewTestDBWithSchema(t)

	for i := 0; i < 7; i++ {
		f, err := AddFeedback(db, "ares", fmt.Sprintf("lesson %d", i), "proj-a")
		require.NoError(t, err)
		// Spread timestamps so recency ordering is deterministic even when
		// inserts land in the same millisecond.
		_, err = db.Exec("UPDATE agent_feedback SET created_at = ? WHERE id = ?", 1000+i, f.ID)
		require.NoError(t, err)
	}

	feedback, err := ListFeedback(db, "ares", "", 5)
	require.NoError(t, err)
	require.Len(t, feedback, 5)
	assert.Equal(t, "lesson 6", feedback[0].Lesson, "newest lesson should come first")
	assert.Equal(t, "lesson 2", feedback[4].Lesson, "oldest two lessons should be cut")
}

func TestListFeedback_PreferProject(t *testing.T) {
	db := NewTestDBWithSchema(t)

	older, err := AddFeedback(db, "ares", "current-project lesson", "proj-a")
	require.NoError(t, err)
	newer, err := AddFeedback(db, "ares", "other-project lesson", "proj-b")
	require.NoError(t, err)
	_, err = db.Exec("UPDATE agent_feedback SET created_at = 1000 WHERE id = ?", older.ID)
	require.NoError(t, err)
	_, err = db.Exec("UPDATE agent_feedback SET created_at = 2000 WHERE id = ?", newer.ID)
	require.NoError(t, err)

	feedback, err := ListFeedback(db, "ares", "proj-a", 0)
	require.NoError(t, err)
	require.Len(t, feedback, 2)
	assert.Equal(t, "current-project lesson", feedback[0].Lesson,
		"current-project lesson should sort before a newer other-project lesson")

	feedback, err = ListFeedback(db, "ares", "", 0)
	require.NoError(t, err)
	assert.Equal(t, "other-project lesson", feedback[0].Lesson,
		"without preferProject, pure recency ordering applies")
}

func TestRemoveFeedback(t *testing.T) {
	db := NewTestDBWithSchema(t)

	f, err := AddFeedback(db, "ares", "no magic strings", "proj-a")
	require.NoError(t, err)

	err = RemoveFeedback(db, f.ID)
	require.NoError(t, err)

	_, err = GetFeedback(db, f.ID)
	assert.Error(t, err, "feedback should no longer exist after removal")
}

func TestRemoveFeedback_NotFound(t *testing.T) {
	db := NewTestDBWithSchema(t)

	err := RemoveFeedback(db, 999)
	assert.Error(t, err)
}
