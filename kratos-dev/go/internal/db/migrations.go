package db

import (
	"database/sql"
	_ "embed"
)

// schemaSQL embeds the schema.sql file at compile time
// This ensures the Go binary can initialize the database without external files
// Note: This is a copy of ../../../memory/schema.sql maintained for Go embedding
//
//go:embed schema.sql
var schemaSQL string

// InitDB initializes the database schema by executing the embedded schema.sql
// This function is idempotent - it can be safely called multiple times
func InitDB(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return err
	}
	// Additive migrations for databases created before a column existed.
	// CREATE TABLE IF NOT EXISTS never alters an existing table.
	return EnsureMemoryProjectColumn(db)
}
