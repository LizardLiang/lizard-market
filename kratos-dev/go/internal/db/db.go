// Package db provides the SQLite persistence layer for sessions, steps, and features.
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	// Pure-Go SQLite driver registration (no CGO).
	_ "modernc.org/sqlite"
)

// GetDBPath returns the path to the Kratos memory database
// Checks KRATOS_MEMORY_DB env var, defaults to ~/.kratos/memory.db
func GetDBPath() string {
	if path := os.Getenv("KRATOS_MEMORY_DB"); path != "" {
		return path
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kratos", "memory.db")
}

// GetConnection establishes a connection to the SQLite database
// Automatically creates the directory if it doesn't exist
func GetConnection() (*sql.DB, error) {
	dbPath := GetDBPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Open database connection. Pragmas travel in the DSN rather than a
	// post-Open db.Exec — see sqliteDSN.
	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return nil, err
	}

	return db, nil
}

// sqliteDSN appends the startup pragmas as DSN query parameters instead of
// running them once via db.Exec after sql.Open.
//
// database/sql pools multiple physical connections behind one *sql.DB, and
// modernc.org/sqlite applies query-string "_pragma" parameters inside its own
// driver.Open — once per physical connection the pool ever creates (see
// modernc.org/sqlite's conn.go newConn -> applyQueryParams). A one-off
// db.Exec("PRAGMA ...") right after sql.Open only reaches whichever single
// connection serviced that call; every other connection the pool later opens
// reverts to SQLite's defaults — no WAL, synchronous FULL, and critically no
// busy_timeout, so a second writer hitting a lock held by another connection
// fails immediately with SQLITE_BUSY instead of waiting. Concurrent
// `step record-agent` / `memory add` calls (16-24 parallel writers observed)
// hit exactly this: 30-60% failed with "database is locked".
//
// busy_timeout(5000) makes a connection wait up to 5000ms for a lock instead
// of failing immediately. Pushed first because modernc.org/sqlite applies
// _pragma values in an order where busy_timeout must be set before the
// others can block on it (see the driver's own sort in applyQueryParams).
func sqliteDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "foreign_keys(ON)")
	return path + "?" + q.Encode()
}
