package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Memory represents a durable fact about the user (preference, habit, weak spot)
type Memory struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	Category  string `json:"category"` // preference, habit, weak-spot, context
	CreatedAt int64  `json:"created_at"`
}

// AddMemory inserts a new user memory
func AddMemory(db *sql.DB, text, category string) (*Memory, error) {
	now := time.Now().UnixMilli()
	query := `
		INSERT INTO user_memories (text, category, created_at)
		VALUES (?, ?, ?)
	`
	result, err := db.Exec(query, text, category, now)
	if err != nil {
		return nil, fmt.Errorf("failed to add memory: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert ID: %w", err)
	}

	return &Memory{
		ID:        id,
		Text:      text,
		Category:  category,
		CreatedAt: now,
	}, nil
}

// ListMemories retrieves memories, optionally filtered by category ("all" or "" means no filter)
func ListMemories(db *sql.DB, category string) ([]*Memory, error) {
	query := `SELECT id, text, category, created_at FROM user_memories`
	var args []interface{}

	if category != "" && category != "all" {
		query += " WHERE category = ?"
		args = append(args, category)
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		m := &Memory{}
		if err := rows.Scan(&m.ID, &m.Text, &m.Category, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// RemoveMemory deletes a memory by ID
func RemoveMemory(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM user_memories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove memory: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memory %d not found", id)
	}
	return nil
}

// GetMemory retrieves a single memory by ID
func GetMemory(db *sql.DB, id int64) (*Memory, error) {
	m := &Memory{}
	err := db.QueryRow(
		`SELECT id, text, category, created_at FROM user_memories WHERE id = ?`, id,
	).Scan(&m.ID, &m.Text, &m.Category, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}
	return m, nil
}
