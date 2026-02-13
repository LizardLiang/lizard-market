package db

import (
	"database/sql"
	"fmt"

	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// GetRecentSessions returns the N most recent sessions across all projects
func GetRecentSessions(db *sql.DB, limit int) ([]*models.Session, error) {
	query := `
		SELECT id, session_id, project, feature_name, started_at, ended_at,
		       status, summary, total_steps, total_agents_spawned
		FROM sessions
		ORDER BY started_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent sessions: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// GetSessionsByStatus returns all sessions with a specific status
func GetSessionsByStatus(db *sql.DB, status string) ([]*models.Session, error) {
	query := `
		SELECT id, session_id, project, feature_name, started_at, ended_at,
		       status, summary, total_steps, total_agents_spawned
		FROM sessions
		WHERE status = ?
		ORDER BY started_at DESC
	`

	rows, err := db.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions by status: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// GetSessionsByProject returns all sessions for a specific project
func GetSessionsByProject(db *sql.DB, project string) ([]*models.Session, error) {
	query := `
		SELECT id, session_id, project, feature_name, started_at, ended_at,
		       status, summary, total_steps, total_agents_spawned
		FROM sessions
		WHERE project = ?
		ORDER BY started_at DESC
	`

	rows, err := db.Query(query, project)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions by project: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// SearchSessions performs full-text search across sessions using FTS5
// Searches in project, feature_name, and summary fields
func SearchSessions(db *sql.DB, searchTerm string) ([]*models.Session, error) {
	// Use FTS5 MATCH syntax for better search
	// Search across project, feature_name, and summary
	query := `
		SELECT s.id, s.session_id, s.project, s.feature_name, s.started_at, s.ended_at,
		       s.status, s.summary, s.total_steps, s.total_agents_spawned
		FROM sessions s
		WHERE s.project LIKE ?
		   OR s.feature_name LIKE ?
		   OR s.summary LIKE ?
		ORDER BY s.started_at DESC
	`

	searchPattern := "%" + searchTerm + "%"
	rows, err := db.Query(query, searchPattern, searchPattern, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search sessions: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// GetSessionTimeline returns all steps for a session in chronological order
func GetSessionTimeline(db *sql.DB, sessionID string) ([]*models.Step, error) {
	query := `
		SELECT id, session_id, step_number, step_type, timestamp,
		       agent_name, agent_model, pipeline_stage,
		       action, target, result, context
		FROM steps
		WHERE session_id = ?
		ORDER BY step_number ASC
	`

	rows, err := db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session timeline: %w", err)
	}
	defer rows.Close()

	steps := []*models.Step{} // Initialize as empty slice, not nil
	for rows.Next() {
		step := &models.Step{}
		err := rows.Scan(
			&step.ID,
			&step.SessionID,
			&step.StepNumber,
			&step.StepType,
			&step.Timestamp,
			&step.AgentName,
			&step.AgentModel,
			&step.PipelineStage,
			&step.Action,
			&step.Target,
			&step.Result,
			&step.Context,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan step: %w", err)
		}
		steps = append(steps, step)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return steps, nil
}

// GetSessionCount returns the total number of sessions
func GetSessionCount(db *sql.DB) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM sessions`

	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get session count: %w", err)
	}

	return count, nil
}

// scanSessions is a helper function to scan multiple session rows
func scanSessions(rows *sql.Rows) ([]*models.Session, error) {
	sessions := []*models.Session{} // Initialize as empty slice, not nil
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return sessions, nil
}
