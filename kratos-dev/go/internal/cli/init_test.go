package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitCmd_Success tests successful execution of InitCmd
func TestInitCmd_Success(t *testing.T) {
	// Setup temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Create command
	cmd := InitCmd()

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Execute
	err := cmd.Execute()
	require.NoError(t, err)

	// Verify JSON output
	var result map[string]string
	err = json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "initialized", result["status"])

	// Verify database was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

// TestInitCmd_CreatesSchema tests that InitCmd creates database schema
func TestInitCmd_CreatesSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	cmd := InitCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := cmd.Execute()
	require.NoError(t, err)

	// Open database and verify tables exist
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 5, "Should have multiple tables")
}

// TestInitCmd_Idempotent tests that InitCmd can be run multiple times
func TestInitCmd_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Run init twice
	cmd1 := InitCmd()
	var output1 bytes.Buffer
	cmd1.SetOut(&output1)
	err := cmd1.Execute()
	require.NoError(t, err)

	cmd2 := InitCmd()
	var output2 bytes.Buffer
	cmd2.SetOut(&output2)
	err = cmd2.Execute()
	require.NoError(t, err) // Should not fail

	// Both should return success
	var result map[string]string
	json.Unmarshal(output2.Bytes(), &result)
	assert.Equal(t, "initialized", result["status"])
}
