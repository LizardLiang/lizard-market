package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStep_JSONMarshaling tests marshaling Step to JSON
func TestStep_JSONMarshaling(t *testing.T) {
	agentName := "athena"
	agentModel := "opus"
	pipelineStage := int64(1)
	target := "prd.md"
	result := "success"
	context := "Created PRD for auth feature"

	step := Step{
		ID:            1,
		SessionID:     "sess-123",
		StepNumber:    1,
		StepType:      "agent_spawn",
		Timestamp:     1707738000000,
		AgentName:     &agentName,
		AgentModel:    &agentModel,
		PipelineStage: &pipelineStage,
		Action:        "create_prd",
		Target:        &target,
		Result:        &result,
		Context:       &context,
	}

	// Marshal
	jsonBytes, err := json.Marshal(step)
	require.NoError(t, err)

	// Verify JSON structure
	var resultMap map[string]interface{}
	err = json.Unmarshal(jsonBytes, &resultMap)
	require.NoError(t, err)

	assert.Equal(t, float64(1), resultMap["id"])
	assert.Equal(t, "sess-123", resultMap["session_id"])
	assert.Equal(t, float64(1), resultMap["step_number"])
	assert.Equal(t, "agent_spawn", resultMap["step_type"])
	assert.Equal(t, "athena", resultMap["agent_name"])
	assert.Equal(t, "opus", resultMap["agent_model"])
	assert.Equal(t, float64(1), resultMap["pipeline_stage"])
	assert.Equal(t, "create_prd", resultMap["action"])
	assert.Equal(t, "prd.md", resultMap["target"])
	assert.Equal(t, "success", resultMap["result"])
	assert.Equal(t, "Created PRD for auth feature", resultMap["context"])
}

// TestStep_JSONUnmarshaling tests unmarshaling JSON to Step
func TestStep_JSONUnmarshaling(t *testing.T) {
	jsonStr := `{
		"id": 1,
		"session_id": "sess-123",
		"step_number": 1,
		"step_type": "agent_spawn",
		"timestamp": 1707738000000,
		"agent_name": "athena",
		"agent_model": "opus",
		"pipeline_stage": 1,
		"action": "create_prd",
		"target": "prd.md",
		"result": "success",
		"context": "Created PRD for auth feature"
	}`

	var step Step
	err := json.Unmarshal([]byte(jsonStr), &step)
	require.NoError(t, err)

	assert.Equal(t, int64(1), step.ID)
	assert.Equal(t, "sess-123", step.SessionID)
	assert.Equal(t, int64(1), step.StepNumber)
	assert.Equal(t, "agent_spawn", step.StepType)
	assert.Equal(t, int64(1707738000000), step.Timestamp)
	assert.NotNil(t, step.AgentName)
	assert.Equal(t, "athena", *step.AgentName)
	assert.NotNil(t, step.AgentModel)
	assert.Equal(t, "opus", *step.AgentModel)
	assert.NotNil(t, step.PipelineStage)
	assert.Equal(t, int64(1), *step.PipelineStage)
	assert.Equal(t, "create_prd", step.Action)
	assert.NotNil(t, step.Target)
	assert.Equal(t, "prd.md", *step.Target)
	assert.NotNil(t, step.Result)
	assert.Equal(t, "success", *step.Result)
	assert.NotNil(t, step.Context)
	assert.Equal(t, "Created PRD for auth feature", *step.Context)
}

// TestStep_NullOptionalFields tests Step with null optional fields
func TestStep_NullOptionalFields(t *testing.T) {
	step := Step{
		ID:            1,
		SessionID:     "sess-123",
		StepNumber:    1,
		StepType:      "command",
		Timestamp:     1707738000000,
		AgentName:     nil, // NULL
		AgentModel:    nil, // NULL
		PipelineStage: nil, // NULL
		Action:        "git_commit",
		Target:        nil, // NULL
		Result:        nil, // NULL
		Context:       nil, // NULL
	}

	jsonBytes, err := json.Marshal(step)
	require.NoError(t, err)

	// Verify omitempty works for nil pointers
	jsonStr := string(jsonBytes)
	assert.NotContains(t, jsonStr, "agent_name")
	assert.NotContains(t, jsonStr, "agent_model")
	assert.NotContains(t, jsonStr, "pipeline_stage")
	assert.NotContains(t, jsonStr, "target")
	assert.NotContains(t, jsonStr, "result")
	assert.NotContains(t, jsonStr, "context")
}
