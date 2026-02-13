package models

// Session represents a Kratos development session
type Session struct {
	ID                 int64   `json:"id"`
	SessionID          string  `json:"session_id"`
	Project            string  `json:"project"`
	FeatureName        *string `json:"feature_name,omitempty"`
	StartedAt          int64   `json:"started_at"`
	EndedAt            *int64  `json:"ended_at,omitempty"`
	Status             string  `json:"status"`
	Summary            *string `json:"summary,omitempty"`
	TotalSteps         int64   `json:"total_steps"`
	TotalAgentsSpawned int64   `json:"total_agents_spawned"`
}

// SessionContext represents a full session with all related data
type SessionContext struct {
	Session     *Session `json:"session"`
	Steps       []*Step  `json:"steps"`
	TotalSteps  int64    `json:"total_steps"`
	TotalAgents int64    `json:"total_agents"`
}
