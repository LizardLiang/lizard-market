package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitDB_CreatesUserMemoriesTable verifies the schema upgrade: InitDB run
// against a database created before this change gains the user_memories table.
func TestInitDB_CreatesUserMemoriesTable(t *testing.T) {
	db := NewTestDB(t)

	err := InitDB(db)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_memories'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "user_memories table should exist")
}

func TestAddMemory(t *testing.T) {
	db := NewTestDBWithSchema(t)

	m, err := AddMemory(db, "prefers terse output", "preference")
	require.NoError(t, err)
	assert.NotZero(t, m.ID)
	assert.Equal(t, "prefers terse output", m.Text)
	assert.Equal(t, "preference", m.Category)
	assert.NotZero(t, m.CreatedAt)
}

func TestListMemories_All(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := AddMemory(db, "prefers terse output", "preference")
	require.NoError(t, err)
	_, err = AddMemory(db, "often forgets to run tests", "weak-spot")
	require.NoError(t, err)

	memories, err := ListMemories(db, "all")
	require.NoError(t, err)
	assert.Len(t, memories, 2)
}

func TestListMemories_FilteredByCategory(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := AddMemory(db, "prefers terse output", "preference")
	require.NoError(t, err)
	_, err = AddMemory(db, "often forgets to run tests", "weak-spot")
	require.NoError(t, err)

	memories, err := ListMemories(db, "preference")
	require.NoError(t, err)
	require.Len(t, memories, 1)
	assert.Equal(t, "prefers terse output", memories[0].Text)
}

func TestRemoveMemory(t *testing.T) {
	db := NewTestDBWithSchema(t)

	m, err := AddMemory(db, "prefers terse output", "preference")
	require.NoError(t, err)

	err = RemoveMemory(db, m.ID)
	require.NoError(t, err)

	_, err = GetMemory(db, m.ID)
	assert.Error(t, err, "memory should no longer exist after removal")
}

func TestRemoveMemory_NotFound(t *testing.T) {
	db := NewTestDBWithSchema(t)

	err := RemoveMemory(db, 999)
	assert.Error(t, err)
}

func TestGetMemory(t *testing.T) {
	db := NewTestDBWithSchema(t)

	m, err := AddMemory(db, "prefers terse output", "preference")
	require.NoError(t, err)

	got, err := GetMemory(db, m.ID)
	require.NoError(t, err)
	assert.Equal(t, m.Text, got.Text)
	assert.Equal(t, m.Category, got.Category)
}
