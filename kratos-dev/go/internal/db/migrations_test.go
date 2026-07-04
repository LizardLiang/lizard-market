package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitDB_CreatesAllTables tests that InitDB creates all expected tables
func TestInitDB_CreatesAllTables(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Test
	err = InitDB(db)
	require.NoError(t, err)

	// Verify expected tables exist
	expectedTables := []string{
		"schema_version",
		"sessions",
		"steps",
		"features",
		"decisions",
		"file_changes",
		"steps_fts",
		"decisions_fts",
	}

	for _, tableName := range expectedTables {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			tableName,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Table %s should exist", tableName)
	}
}

// TestInitDB_Idempotent tests that InitDB can be run multiple times safely
func TestInitDB_Idempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Run InitDB twice
	err = InitDB(db)
	require.NoError(t, err)

	err = InitDB(db)
	require.NoError(t, err) // Should not fail
}

// TestInitDB_CreatesIndexes tests that InitDB creates indexes
func TestInitDB_CreatesIndexes(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = InitDB(db)
	require.NoError(t, err)

	// Verify indexes exist
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "Should have at least one index")
}

// TestInitDB_EnablesFTS5 tests that InitDB creates FTS5 virtual tables
func TestInitDB_EnablesFTS5(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = InitDB(db)
	require.NoError(t, err)

	// Verify FTS5 tables are virtual tables
	var sql string
	err = db.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type='table' AND name='steps_fts'",
	).Scan(&sql)
	require.NoError(t, err)
	assert.Contains(t, sql, "fts5", "steps_fts should be FTS5 virtual table")
}

// TestInitDB_CreatesSchemaVersion tests that schema_version table is initialized
func TestInitDB_CreatesSchemaVersion(t *testing.T) {
	db := NewTestDB(t)

	err := InitDB(db)
	require.NoError(t, err)

	// Verify schema_version table exists and has correct value
	var version int
	err = db.QueryRow("SELECT version FROM schema_version WHERE id = 1").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version, "Schema version should be 1")
}

// TestInitDB_CreatesTriggers tests that FTS triggers are created
func TestInitDB_CreatesTriggers(t *testing.T) {
	db := NewTestDB(t)

	err := InitDB(db)
	require.NoError(t, err)

	// Verify triggers exist
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='trigger'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "Should have at least one trigger for FTS")
}
