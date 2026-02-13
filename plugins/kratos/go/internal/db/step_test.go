package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// Test 1: Create step
func TestCreateStep(t *testing.T) {
	db := NewTestDBWithSchema(t)

	// First create a session
	session := &models.Session{
		SessionID: "test-session",
		Project:   "/test/project",
		StartedAt: time.Now().UnixMilli(),
		Status:    "active",
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Create step
	agentName := "athena"
	agentModel := "opus"
	target := "prd.md"
	result := "success"

	step := &models.Step{
		SessionID:  "test-session",
		StepNumber: 1,
		StepType:   "agent_spawn",
		Timestamp:  time.Now().UnixMilli(),
		AgentName:  &agentName,
		AgentModel: &agentModel,
		Action:     "create_prd",
		Target:     &target,
		Result:     &result,
	}

	err = CreateStep(db, step)
	require.NoError(t, err)
	assert.Greater(t, step.ID, int64(0))
}

// Test 2: Get steps for session
func TestGetStepsForSession(t *testing.T) {
	db := NewTestDBWithSchema(t)

	// Create session
	session := &models.Session{
		SessionID: "test-session",
		Project:   "/test/project",
		StartedAt: time.Now().UnixMilli(),
		Status:    "active",
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Create 3 steps
	for i := 1; i <= 3; i++ {
		action := "test_action"
		step := &models.Step{
			SessionID:  "test-session",
			StepNumber: int64(i),
			StepType:   "command",
			Timestamp:  time.Now().UnixMilli(),
			Action:     action,
		}
		err := CreateStep(db, step)
		require.NoError(t, err)
	}

	// Test: Get all steps
	steps, err := GetStepsForSession(db, "test-session")
	require.NoError(t, err)
	assert.Len(t, steps, 3)
	assert.Equal(t, int64(1), steps[0].StepNumber)
}

// Test 3: Increment session step count
func TestIncrementSessionSteps(t *testing.T) {
	db := NewTestDBWithSchema(t)

	session := &models.Session{
		SessionID:  "test-session",
		Project:    "/test/project",
		StartedAt:  time.Now().UnixMilli(),
		Status:     "active",
		TotalSteps: 0,
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Increment
	err = IncrementSessionSteps(db, "test-session")
	require.NoError(t, err)

	// Verify
	updated, err := GetSession(db, "test-session")
	require.NoError(t, err)
	assert.Equal(t, int64(1), updated.TotalSteps)
}

// Test 4: Record agent spawn
func TestRecordAgentSpawn(t *testing.T) {
	db := NewTestDBWithSchema(t)

	session := &models.Session{
		SessionID:          "test-session",
		Project:            "/test/project",
		StartedAt:          time.Now().UnixMilli(),
		Status:             "active",
		TotalAgentsSpawned: 0,
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Record agent spawn
	err = RecordAgentSpawn(db, "test-session", "athena", "opus", "create_prd")
	require.NoError(t, err)

	// Verify step created
	steps, err := GetStepsForSession(db, "test-session")
	require.NoError(t, err)
	assert.Len(t, steps, 1)
	assert.Equal(t, "agent_spawn", steps[0].StepType)
	assert.Equal(t, "athena", *steps[0].AgentName)
	assert.Equal(t, "opus", *steps[0].AgentModel)

	// Verify count incremented
	updated, err := GetSession(db, "test-session")
	require.NoError(t, err)
	assert.Equal(t, int64(1), updated.TotalAgentsSpawned)
	assert.Equal(t, int64(1), updated.TotalSteps)
}

// Test 5: Record file change
func TestRecordFileChange(t *testing.T) {
	db := NewTestDBWithSchema(t)

	session := &models.Session{
		SessionID: "test-session",
		Project:   "/test/project",
		StartedAt: time.Now().UnixMilli(),
		Status:    "active",
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Record file change
	err = RecordFileChange(db, "test-session", "Write", "src/main.go")
	require.NoError(t, err)

	// Verify step created
	steps, err := GetStepsForSession(db, "test-session")
	require.NoError(t, err)
	assert.Len(t, steps, 1)
	assert.Equal(t, "file_modify", steps[0].StepType)
	assert.Equal(t, "Write", steps[0].Action)
	assert.Equal(t, "src/main.go", *steps[0].Target)
}

// Test 6: Multiple steps maintain order
func TestStepOrdering(t *testing.T) {
	db := NewTestDBWithSchema(t)

	session := &models.Session{
		SessionID: "test-session",
		Project:   "/test/project",
		StartedAt: time.Now().UnixMilli(),
		Status:    "active",
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Record multiple steps
	err = RecordAgentSpawn(db, "test-session", "athena", "opus", "create_prd")
	require.NoError(t, err)

	err = RecordFileChange(db, "test-session", "Write", "prd.md")
	require.NoError(t, err)

	err = RecordAgentSpawn(db, "test-session", "hephaestus", "opus", "create_spec")
	require.NoError(t, err)

	// Verify ordering
	steps, err := GetStepsForSession(db, "test-session")
	require.NoError(t, err)
	assert.Len(t, steps, 3)
	assert.Equal(t, int64(1), steps[0].StepNumber)
	assert.Equal(t, int64(2), steps[1].StepNumber)
	assert.Equal(t, int64(3), steps[2].StepNumber)
	assert.Equal(t, "agent_spawn", steps[0].StepType)
	assert.Equal(t, "file_modify", steps[1].StepType)
	assert.Equal(t, "agent_spawn", steps[2].StepType)
}
