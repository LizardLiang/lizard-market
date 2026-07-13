package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Feedback represents a lesson a specific agent should apply next time,
// distilled from a user correction of that agent's finished work.
type Feedback struct {
	ID        int64  `json:"id"`
	Agent     string `json:"agent"`
	Lesson    string `json:"lesson"`
	Project   string `json:"project"`
	CreatedAt int64  `json:"created_at"`
}

// AddFeedback inserts a new agent feedback lesson
func AddFeedback(db *sql.DB, agent, lesson, project string) (*Feedback, error) {
	now := time.Now().UnixMilli()
	query := `
		INSERT INTO agent_feedback (agent, lesson, project, created_at)
		VALUES (?, ?, ?, ?)
	`
	result, err := db.Exec(query, agent, lesson, project, now)
	if err != nil {
		return nil, fmt.Errorf("failed to add feedback: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert ID: %w", err)
	}

	return &Feedback{
		ID:        id,
		Agent:     agent,
		Lesson:    lesson,
		Project:   project,
		CreatedAt: now,
	}, nil
}

// ListFeedback retrieves feedback lessons across all projects, optionally
// filtered by agent ("" or "all" means no filter). When preferProject is
// non-empty, that project's lessons sort first (then by recency). limit <= 0
// means unlimited.
func ListFeedback(db *sql.DB, agent, preferProject string, limit int) ([]*Feedback, error) {
	query := `SELECT id, agent, lesson, project, created_at FROM agent_feedback`
	var args []interface{}

	if agent != "" && agent != "all" {
		query += " WHERE agent = ?"
		args = append(args, agent)
	}

	if preferProject != "" {
		query += " ORDER BY (project = ?) DESC, created_at DESC"
		args = append(args, preferProject)
	} else {
		query += " ORDER BY created_at DESC"
	}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list feedback: %w", err)
	}
	defer rows.Close()

	var feedback []*Feedback
	for rows.Next() {
		f := &Feedback{}
		if err := rows.Scan(&f.ID, &f.Agent, &f.Lesson, &f.Project, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan feedback: %w", err)
		}
		feedback = append(feedback, f)
	}
	return feedback, rows.Err()
}

// RemoveFeedback deletes a feedback lesson by ID
func RemoveFeedback(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM agent_feedback WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove feedback: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feedback %d not found", id)
	}
	return nil
}

// GetFeedback retrieves a single feedback lesson by ID
func GetFeedback(db *sql.DB, id int64) (*Feedback, error) {
	f := &Feedback{}
	err := db.QueryRow(
		`SELECT id, agent, lesson, project, created_at FROM agent_feedback WHERE id = ?`, id,
	).Scan(&f.ID, &f.Agent, &f.Lesson, &f.Project, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("feedback %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feedback: %w", err)
	}
	return f, nil
}
