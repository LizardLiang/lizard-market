package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// TestGetConnection_SetsBusyTimeout verifies the DSN carries busy_timeout, not
// just journal_mode/foreign_keys — the pragma the review's repro depends on.
func TestGetConnection_SetsBusyTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "busy.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	db, err := GetConnection()
	require.NoError(t, err)
	defer db.Close()

	var busyTimeoutMs int
	require.NoError(t, db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeoutMs))
	assert.Equal(t, 5000, busyTimeoutMs)
}

// TestGetConnection_ConcurrentWritesNoBusyError reproduces the review's
// repro: 16-24 parallel writers, each shaped like a separate `kratos step
// record-agent` / `memory add` process invocation — its own *sql.DB from its
// own GetConnection() call, not a second borrow from one already-configured
// pool — hitting the same on-disk database at once.
//
// Before the busy_timeout DSN fix this failed 30-60% of writes with
// "database is locked (5) (SQLITE_BUSY)": database/sql pools several
// physical connections per *sql.DB, and modernc.org/sqlite applies
// query-string "_pragma" values once per physical connection inside its own
// driver.Open. A one-off db.Exec("PRAGMA busy_timeout=...") after sql.Open
// only ever reached the single connection that served that call — every
// other connection any of these pools opened kept SQLite's default 0ms
// timeout and failed immediately on a lock instead of waiting for it.
func TestGetConnection_ConcurrentWritesNoBusyError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "concurrent.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// One connection creates a scratch table every writer below inserts
	// into — deliberately not the full sessions/steps schema, since the
	// pragma fix under test is orthogonal to table shape.
	setup, err := GetConnection()
	require.NoError(t, err)
	_, err = setup.Exec(`CREATE TABLE IF NOT EXISTS busy_test (id INTEGER PRIMARY KEY, worker INTEGER)`)
	require.NoError(t, err)
	require.NoError(t, setup.Close())

	const writers = 20
	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			conn, err := GetConnection()
			if err != nil {
				errs[n] = fmt.Errorf("worker %d: GetConnection: %w", n, err)
				return
			}
			defer conn.Close()
			if _, err := conn.Exec(`INSERT INTO busy_test (worker) VALUES (?)`, n); err != nil {
				errs[n] = fmt.Errorf("worker %d: insert: %w", n, err)
			}
		}(i)
	}
	wg.Wait()

	var failures []string
	for i, err := range errs {
		if err != nil {
			failures = append(failures, fmt.Sprintf("worker %d: %v", i, err))
		}
	}
	if len(failures) > 0 {
		t.Errorf("%d/%d concurrent writers failed with an error (want 0):\n%s", len(failures), writers, strings.Join(failures, "\n"))
	}

	final, err := GetConnection()
	require.NoError(t, err)
	defer final.Close()
	var count int
	require.NoError(t, final.QueryRow(`SELECT COUNT(*) FROM busy_test`).Scan(&count))
	assert.Equal(t, writers, count, "every concurrent writer's row must have committed")
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
