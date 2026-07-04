package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Todo represents a personal task list item
type Todo struct {
	ID          int64   `json:"id"`
	Text        string  `json:"text"`
	Status      string  `json:"status"`       // open, done
	Source      string  `json:"source"`       // user, jira, ananke
	SourceRef   *string `json:"source_ref"`   // Jira ticket ID, if source=jira
	Project     string  `json:"project"`
	CreatedAt   int64   `json:"created_at"`
	CompletedAt *int64  `json:"completed_at"` // null if open
}

// AddTodo inserts a new todo item
func AddTodo(db *sql.DB, text, project, source string, sourceRef *string) (*Todo, error) {
	now := time.Now().UnixMilli()
	query := `
		INSERT INTO todos (text, status, source, source_ref, project, created_at)
		VALUES (?, 'open', ?, ?, ?, ?)
	`
	result, err := db.Exec(query, text, source, sourceRef, project, now)
	if err != nil {
		return nil, fmt.Errorf("failed to add todo: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert ID: %w", err)
	}

	return &Todo{
		ID:        id,
		Text:      text,
		Status:    "open",
		Source:    source,
		SourceRef: sourceRef,
		Project:   project,
		CreatedAt: now,
	}, nil
}

// ListTodos retrieves todos filtered by project, status, and source
func ListTodos(db *sql.DB, project, status, source string) ([]*Todo, error) {
	query := `
		SELECT id, text, status, source, source_ref, project, created_at, completed_at
		FROM todos
		WHERE project = ?
	`
	args := []interface{}{project}

	if status != "all" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if source != "all" {
		query += " AND source = ?"
		args = append(args, source)
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list todos: %w", err)
	}
	defer rows.Close()

	var todos []*Todo
	for rows.Next() {
		t := &Todo{}
		if err := rows.Scan(
			&t.ID, &t.Text, &t.Status, &t.Source,
			&t.SourceRef, &t.Project, &t.CreatedAt, &t.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan todo: %w", err)
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

// DoneTodo marks a todo as complete
func DoneTodo(db *sql.DB, id int64) (*Todo, error) {
	now := time.Now().UnixMilli()
	_, err := db.Exec(
		"UPDATE todos SET status = 'done', completed_at = ? WHERE id = ?",
		now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to mark todo done: %w", err)
	}
	return GetTodo(db, id)
}

// RemoveTodo deletes a todo by ID
func RemoveTodo(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM todos WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove todo: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo %d not found", id)
	}
	return nil
}

// GetTodo retrieves a single todo by ID
func GetTodo(db *sql.DB, id int64) (*Todo, error) {
	t := &Todo{}
	err := db.QueryRow(
		`SELECT id, text, status, source, source_ref, project, created_at, completed_at
		 FROM todos WHERE id = ?`, id,
	).Scan(&t.ID, &t.Text, &t.Status, &t.Source, &t.SourceRef, &t.Project, &t.CreatedAt, &t.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("todo %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}
	return t, nil
}