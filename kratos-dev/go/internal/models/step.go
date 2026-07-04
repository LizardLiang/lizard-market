package models

// Step represents a single action within a Kratos session
type Step struct {
	ID            int64   `json:"id"`
	SessionID     string  `json:"session_id"`
	StepNumber    int64   `json:"step_number"`
	StepType      string  `json:"step_type"` // agent_spawn, file_modify, decision, command
	Timestamp     int64   `json:"timestamp"` // Unix epoch ms
	AgentName     *string `json:"agent_name,omitempty"`
	AgentModel    *string `json:"agent_model,omitempty"`
	PipelineStage *int64  `json:"pipeline_stage,omitempty"`
	Action        string  `json:"action"`
	Target        *string `json:"target,omitempty"`
	Result        *string `json:"result,omitempty"`
	Context       *string `json:"context,omitempty"`
}
