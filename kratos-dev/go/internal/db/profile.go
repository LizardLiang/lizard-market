package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ProfileEntry represents a structured fact about the user (key/value)
type ProfileEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updated_at"`
}

// SetProfile inserts or updates a profile entry (upsert by key)
func SetProfile(db *sql.DB, key, value string) (*ProfileEntry, error) {
	now := time.Now().UnixMilli()
	query := `
		INSERT INTO user_profile (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`
	if _, err := db.Exec(query, key, value, now); err != nil {
		return nil, fmt.Errorf("failed to set profile key: %w", err)
	}

	return &ProfileEntry{
		Key:       key,
		Value:     value,
		UpdatedAt: now,
	}, nil
}

// GetProfile retrieves a single profile entry by key
func GetProfile(db *sql.DB, key string) (*ProfileEntry, error) {
	p := &ProfileEntry{}
	err := db.QueryRow(
		`SELECT key, value, updated_at FROM user_profile WHERE key = ?`, key,
	).Scan(&p.Key, &p.Value, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("profile key %q not found", key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile key: %w", err)
	}
	return p, nil
}

// ListProfile retrieves all profile entries ordered by key
func ListProfile(db *sql.DB) ([]*ProfileEntry, error) {
	rows, err := db.Query(`SELECT key, value, updated_at FROM user_profile ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("failed to list profile: %w", err)
	}
	defer rows.Close()

	var entries []*ProfileEntry
	for rows.Next() {
		p := &ProfileEntry{}
		if err := rows.Scan(&p.Key, &p.Value, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan profile entry: %w", err)
		}
		entries = append(entries, p)
	}
	return entries, rows.Err()
}

// RemoveProfile deletes a profile entry by key
func RemoveProfile(db *sql.DB, key string) error {
	result, err := db.Exec("DELETE FROM user_profile WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to remove profile key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("profile key %q not found", key)
	}
	return nil
}
