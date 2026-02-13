package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// CreateStep inserts a step into the database
func CreateStep(db *sql.DB, step *models.Step) error {
	query := `
		INSERT INTO steps (
			session_id, step_number, step_type, timestamp,
			agent_name, agent_model, pipeline_stage,
			action, target, result, context
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(query,
		step.SessionID,
		step.StepNumber,
		step.StepType,
		step.Timestamp,
		step.AgentName,
		step.AgentModel,
		step.PipelineStage,
		step.Action,
		step.Target,
		step.Result,
		step.Context,
	)
	if err != nil {
		return fmt.Errorf("failed to create step: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get insert ID: %w", err)
	}

	step.ID = id
	return nil
}

// GetStepsForSession retrieves all steps for a session
func GetStepsForSession(db *sql.DB, sessionID string) ([]*models.Step, error) {
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
		return nil, fmt.Errorf("failed to get steps: %w", err)
	}
	defer rows.Close()

	steps := []*models.Step{}
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

	return steps, rows.Err()
}

// IncrementSessionSteps increments the total_steps counter for a session
func IncrementSessionSteps(db *sql.DB, sessionID string) error {
	query := `
		UPDATE sessions
		SET total_steps = total_steps + 1
		WHERE session_id = ?
	`

	_, err := db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to increment steps: %w", err)
	}

	return nil
}

// RecordAgentSpawn records an agent spawn step
func RecordAgentSpawn(db *sql.DB, sessionID, agentName, agentModel, action string) error {
	// Get next step number
	var stepNum int64
	err := db.QueryRow("SELECT COALESCE(MAX(step_number), 0) + 1 FROM steps WHERE session_id = ?", sessionID).Scan(&stepNum)
	if err != nil {
		return fmt.Errorf("failed to get step number: %w", err)
	}

	step := &models.Step{
		SessionID:  sessionID,
		StepNumber: stepNum,
		StepType:   "agent_spawn",
		Timestamp:  time.Now().UnixMilli(),
		AgentName:  &agentName,
		AgentModel: &agentModel,
		Action:     action,
	}

	if err := CreateStep(db, step); err != nil {
		return err
	}

	// Increment counters
	if err := IncrementSessionSteps(db, sessionID); err != nil {
		return err
	}

	// Increment agent counter
	_, err = db.Exec("UPDATE sessions SET total_agents_spawned = total_agents_spawned + 1 WHERE session_id = ?", sessionID)
	return err
}

// RecordFileChange records a file modification step
func RecordFileChange(db *sql.DB, sessionID, action, filePath string) error {
	// Get next step number
	var stepNum int64
	err := db.QueryRow("SELECT COALESCE(MAX(step_number), 0) + 1 FROM steps WHERE session_id = ?", sessionID).Scan(&stepNum)
	if err != nil {
		return fmt.Errorf("failed to get step number: %w", err)
	}

	step := &models.Step{
		SessionID:  sessionID,
		StepNumber: stepNum,
		StepType:   "file_modify",
		Timestamp:  time.Now().UnixMilli(),
		Action:     action,
		Target:     &filePath,
	}

	if err := CreateStep(db, step); err != nil {
		return err
	}

	return IncrementSessionSteps(db, sessionID)
}
