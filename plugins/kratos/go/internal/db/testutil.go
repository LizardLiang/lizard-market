package db

import (
	"database/sql"
	"testing"
)

// NewTestDB creates an in-memory database for testing
func NewTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// NewTestDBWithSchema creates an initialized test database
func NewTestDBWithSchema(t *testing.T) *sql.DB {
	db := NewTestDB(t)
	if err := InitDB(db); err != nil {
		t.Fatal(err)
	}
	return db
}
