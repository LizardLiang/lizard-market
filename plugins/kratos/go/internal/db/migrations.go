package db

import (
	"database/sql"
	_ "embed"
)

// schemaSQL embeds the schema.sql file at compile time
// This ensures the Go binary can initialize the database without external files
// Note: This is a copy of ../../../memory/schema.sql maintained for Go embedding
//go:embed schema.sql
var schemaSQL string

// InitDB initializes the database schema by executing the embedded schema.sql
// This function is idempotent - it can be safely called multiple times
func InitDB(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}
