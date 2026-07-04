package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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

	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Set SQLite pragmas for performance and reliability
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}

	return db, nil
}
