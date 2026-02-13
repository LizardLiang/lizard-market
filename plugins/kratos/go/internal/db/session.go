package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// CreateSession inserts a new session into the database
func CreateSession(db *sql.DB, session *models.Session) error {
	query := `
		INSERT INTO sessions (
			session_id, project, feature_name, initial_request,
			started_at, ended_at, status, summary, total_steps, total_agents_spawned
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(query,
		session.SessionID,
		session.Project,
		session.FeatureName,
		nil, // initial_request (future)
		session.StartedAt,
		session.EndedAt,
		session.Status,
		session.Summary,
		session.TotalSteps,
		session.TotalAgentsSpawned,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get insert ID: %w", err)
	}

	session.ID = id
	return nil
}

// GetSession retrieves a session by session_id
func GetSession(db *sql.DB, sessionID string) (*models.Session, error) {
	query := `
		SELECT id, session_id, project, feature_name, started_at, ended_at,
		       status, summary, total_steps, total_agents_spawned
		FROM sessions
		WHERE session_id = ?
	`

	session := &models.Session{}
	err := db.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.SessionID,
		&session.Project,
		&session.FeatureName,
		&session.StartedAt,
		&session.EndedAt,
		&session.Status,
		&session.Summary,
		&session.TotalSteps,
		&session.TotalAgentsSpawned,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// GetActiveSession gets the active session for a project (if any)
func GetActiveSession(db *sql.DB, project string) (*models.Session, error) {
	query := `
		SELECT id, session_id, project, feature_name, started_at, ended_at,
		       status, summary, total_steps, total_agents_spawned
		FROM sessions
		WHERE project = ? AND status = 'active' AND ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`

	session := &models.Session{}
	err := db.QueryRow(query, project).Scan(
		&session.ID,
		&session.SessionID,
		&session.Project,
		&session.FeatureName,
		&session.StartedAt,
		&session.EndedAt,
		&session.Status,
		&session.Summary,
		&session.TotalSteps,
		&session.TotalAgentsSpawned,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No active session is valid
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	return session, nil
}

// EndSession marks a session as completed with optional summary
func EndSession(db *sql.DB, sessionID string, summary string) error {
	now := time.Now().UnixMilli()

	query := `
		UPDATE sessions
		SET ended_at = ?, status = 'completed', summary = ?
		WHERE session_id = ?
	`

	_, err := db.Exec(query, now, summary, sessionID)
	if err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	return nil
}

// ListRecentSessions returns the N most recent sessions for a project
func ListRecentSessions(db *sql.DB, project string, limit int) ([]*models.Session, error) {
	query := `
		SELECT id, session_id, project, feature_name, started_at, ended_at,
		       status, summary, total_steps, total_agents_spawned
		FROM sessions
		WHERE project = ?
		ORDER BY started_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, project, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		session := &models.Session{}
		err := rows.Scan(
			&session.ID,
			&session.SessionID,
			&session.Project,
			&session.FeatureName,
			&session.StartedAt,
			&session.EndedAt,
			&session.Status,
			&session.Summary,
			&session.TotalSteps,
			&session.TotalAgentsSpawned,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return sessions, nil
}
