package db

import (
	"database/sql"
	"fmt"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/models"
)

// GetLastSessionForProject returns the most recent session for a project
func GetLastSessionForProject(db *sql.DB, project string) (*models.Session, error) {
	query := `
		SELECT id, session_id, project, feature_name, started_at, ended_at,
		       status, summary, total_steps, total_agents_spawned
		FROM sessions
		WHERE project = ?
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
		return nil, nil // No sessions found is valid
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last session: %w", err)
	}

	return session, nil
}

// GetIncompleteFeatures returns sessions for features that aren't completed
func GetIncompleteFeatures(db *sql.DB, project string) ([]*models.Session, error) {
	query := `
		SELECT id, session_id, project, feature_name, started_at, ended_at,
		       status, summary, total_steps, total_agents_spawned
		FROM sessions
		WHERE project = ?
		  AND feature_name IS NOT NULL
		  AND status != 'completed'
		ORDER BY started_at DESC
	`

	rows, err := db.Query(query, project)
	if err != nil {
		return nil, fmt.Errorf("failed to get incomplete features: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// GetSessionContext retrieves a session with all its steps and metadata
func GetSessionContext(db *sql.DB, sessionID string) (*models.SessionContext, error) {
	session, err := GetSession(db, sessionID)
	if err != nil {
		return nil, err
	}

	steps, err := GetSessionTimeline(db, sessionID)
	if err != nil {
		return nil, err
	}

	return &models.SessionContext{
		Session:     session,
		Steps:       steps,
		TotalSteps:  session.TotalSteps,
		TotalAgents: session.TotalAgentsSpawned,
	}, nil
}

// GetRecentSessionsGlobal returns recent sessions across all projects.
// Delegates to GetRecentSessions which performs the same query.
func GetRecentSessionsGlobal(db *sql.DB, limit int) ([]*models.Session, error) {
	return GetRecentSessions(db, limit)
}
