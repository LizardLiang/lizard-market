package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitDB_CreatesUserProfileTable verifies the schema upgrade: InitDB run
// against a database created before this change gains the user_profile table.
func TestInitDB_CreatesUserProfileTable(t *testing.T) {
	db := NewTestDB(t)

	err := InitDB(db)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_profile'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "user_profile table should exist")
}

func TestSetProfile(t *testing.T) {
	db := NewTestDBWithSchema(t)

	p, err := SetProfile(db, "timezone", "Asia/Taipei")
	require.NoError(t, err)
	assert.Equal(t, "timezone", p.Key)
	assert.Equal(t, "Asia/Taipei", p.Value)
	assert.NotZero(t, p.UpdatedAt)
}

func TestSetProfile_Overwrites(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := SetProfile(db, "current_focus", "payments launch")
	require.NoError(t, err)
	_, err = SetProfile(db, "current_focus", "kratos v3")
	require.NoError(t, err)

	got, err := GetProfile(db, "current_focus")
	require.NoError(t, err)
	assert.Equal(t, "kratos v3", got.Value)

	entries, err := ListProfile(db)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "upsert should not create a second row")
}

func TestGetProfile_NotFound(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := GetProfile(db, "missing")
	assert.Error(t, err)
}

func TestListProfile_OrderedByKey(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := SetProfile(db, "timezone", "Asia/Taipei")
	require.NoError(t, err)
	_, err = SetProfile(db, "name", "Lizard")
	require.NoError(t, err)

	entries, err := ListProfile(db)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "name", entries[0].Key)
	assert.Equal(t, "timezone", entries[1].Key)
}

func TestRemoveProfile(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := SetProfile(db, "timezone", "Asia/Taipei")
	require.NoError(t, err)

	err = RemoveProfile(db, "timezone")
	require.NoError(t, err)

	_, err = GetProfile(db, "timezone")
	assert.Error(t, err, "profile key should no longer exist after removal")
}

func TestRemoveProfile_NotFound(t *testing.T) {
	db := NewTestDBWithSchema(t)

	err := RemoveProfile(db, "missing")
	assert.Error(t, err)
}
