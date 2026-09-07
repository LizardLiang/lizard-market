package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/models"
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
		nil, // initial_request — filled by SetInitialRequestIfEmpty on the first prompt
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

	return scanSessions(rows)
}

// EnsureSession returns the session row for sessionID, creating an active one
// (with the given project) when none exists. Hooks record steps against the
// Claude Code session id; before this existed a missing row surfaced as
// "FOREIGN KEY constraint failed" and the step was lost. The bool reports
// whether a row was created.
func EnsureSession(db *sql.DB, sessionID, project string) (*models.Session, bool, error) {
	if sessionID == "" {
		return nil, false, fmt.Errorf("session id is required")
	}
	if existing, err := GetSession(db, sessionID); err == nil {
		return existing, false, nil
	}
	if project == "" {
		project = "unknown"
	}
	session := &models.Session{
		SessionID: sessionID,
		Project:   project,
		StartedAt: time.Now().UnixMilli(),
		Status:    "active",
	}
	if err := CreateSession(db, session); err != nil {
		// Lost a race with a concurrent creator: the row exists now.
		if existing, gerr := GetSession(db, sessionID); gerr == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return session, true, nil
}

// ReactivateSession re-opens a session that was ended. A resumed Claude Code
// session keeps its id across the gap, so the same row continues. No-op when
// the row is already active.
func ReactivateSession(db *sql.DB, sessionID string) error {
	_, err := db.Exec(`UPDATE sessions SET status = 'active', ended_at = NULL WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("failed to reactivate session: %w", err)
	}
	return nil
}

// SetInitialRequestIfEmpty records the first user prompt of a session. Later
// prompts never overwrite it, so the column always answers "what did this
// session start out doing". Text is capped so a pasted document never lands
// in the ledger.
func SetInitialRequestIfEmpty(db *sql.DB, sessionID, text string) error {
	const maxLen = 500
	if len(text) > maxLen {
		text = text[:maxLen]
	}
	_, err := db.Exec(`UPDATE sessions SET initial_request = ? WHERE session_id = ? AND (initial_request IS NULL OR initial_request = '')`,
		text, sessionID)
	if err != nil {
		return fmt.Errorf("failed to set initial request: %w", err)
	}
	return nil
}
