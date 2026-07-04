package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetDBPath_WithEnvVar tests GetDBPath with environment variable set
func TestGetDBPath_WithEnvVar(t *testing.T) {
	// Set environment variable
	customPath := "/custom/path/memory.db"
	os.Setenv("KRATOS_MEMORY_DB", customPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Test
	result := GetDBPath()
	assert.Equal(t, customPath, result)
}

// TestGetDBPath_Default tests GetDBPath default behavior
func TestGetDBPath_Default(t *testing.T) {
	// Ensure no env var is set
	os.Unsetenv("KRATOS_MEMORY_DB")

	// Test
	result := GetDBPath()

	// Verify contains ~/.kratos/memory.db
	assert.Contains(t, result, ".kratos")
	assert.Contains(t, result, "memory.db")
	assert.NotEmpty(t, result)
}

// TestGetConnection_Success tests successful database connection
func TestGetConnection_Success(t *testing.T) {
	// Use temporary directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Test
	db, err := GetConnection()
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Verify pragmas were set
	var walMode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&walMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", strings.ToLower(walMode))

	// Verify foreign keys enabled
	var fkEnabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	require.NoError(t, err)
	assert.Equal(t, 1, fkEnabled)
}

// TestGetConnection_CreatesDirectory tests that GetConnection creates parent directories
func TestGetConnection_CreatesDirectory(t *testing.T) {
	// Use temporary directory with nested path
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nested", "dir", "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Test
	db, err := GetConnection()
	require.NoError(t, err)
	defer db.Close()

	// Verify directory was created
	_, err = os.Stat(filepath.Dir(dbPath))
	assert.NoError(t, err)
}

// TestGetDBPath_EdgeCases tests edge cases for GetDBPath
func TestGetDBPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		contains []string
	}{
		{
			name:     "empty env var uses default",
			envValue: "",
			contains: []string{".kratos", "memory.db"},
		},
		{
			name:     "absolute path from env",
			envValue: "/tmp/custom.db",
			contains: []string{"/tmp/custom.db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("KRATOS_MEMORY_DB", tt.envValue)
				defer os.Unsetenv("KRATOS_MEMORY_DB")
			} else {
				os.Unsetenv("KRATOS_MEMORY_DB")
			}

			result := GetDBPath()
			for _, substr := range tt.contains {
				assert.Contains(t, result, substr)
			}
		})
	}
}
