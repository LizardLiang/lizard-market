package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test: step record agent
func TestStepRecordAgentCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Initialize and create session
	err := InitCmd().Execute()
	require.NoError(t, err)

	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	err = startCmd.Execute()
	require.NoError(t, err)

	var startResult map[string]interface{}
	err = json.Unmarshal(startOutput.Bytes(), &startResult)
	require.NoError(t, err)
	sessionID := startResult["session_id"].(string)

	// Test: Record agent spawn
	cmd := StepRecordAgentCmd()
	cmd.SetArgs([]string{sessionID, "athena", "opus", "create_prd"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err = cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "success", result["status"])
	assert.Equal(t, float64(1), result["step_number"])
}

// Test: step record file
func TestStepRecordFileCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Initialize and create session
	err := InitCmd().Execute()
	require.NoError(t, err)

	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	err = startCmd.Execute()
	require.NoError(t, err)

	var startResult map[string]interface{}
	err = json.Unmarshal(startOutput.Bytes(), &startResult)
	require.NoError(t, err)
	sessionID := startResult["session_id"].(string)

	// Test: Record file change
	cmd := StepRecordFileCmd()
	cmd.SetArgs([]string{sessionID, "Write", "src/main.go"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err = cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "success", result["status"])
}

// Test: step list
func TestStepListCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Initialize and create session
	err := InitCmd().Execute()
	require.NoError(t, err)

	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	err = startCmd.Execute()
	require.NoError(t, err)

	var startResult map[string]interface{}
	err = json.Unmarshal(startOutput.Bytes(), &startResult)
	require.NoError(t, err)
	sessionID := startResult["session_id"].(string)

	// Add a step
	recordCmd := StepRecordAgentCmd()
	recordCmd.SetArgs([]string{sessionID, "athena", "opus", "create_prd"})
	err = recordCmd.Execute()
	require.NoError(t, err)

	// Test: List steps
	cmd := StepListCmd()
	cmd.SetArgs([]string{sessionID})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err = cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, sessionID, result["session_id"])
	steps := result["steps"].([]interface{})
	assert.Len(t, steps, 1)
	assert.Equal(t, float64(1), result["count"])
}

// Test: multiple steps recorded correctly
func TestStepMultipleRecords(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Initialize and create session
	err := InitCmd().Execute()
	require.NoError(t, err)

	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	err = startCmd.Execute()
	require.NoError(t, err)

	var startResult map[string]interface{}
	err = json.Unmarshal(startOutput.Bytes(), &startResult)
	require.NoError(t, err)
	sessionID := startResult["session_id"].(string)

	// Record multiple steps
	agentCmd := StepRecordAgentCmd()
	agentCmd.SetArgs([]string{sessionID, "athena", "opus", "create_prd"})
	err = agentCmd.Execute()
	require.NoError(t, err)

	fileCmd := StepRecordFileCmd()
	fileCmd.SetArgs([]string{sessionID, "Write", "prd.md"})
	err = fileCmd.Execute()
	require.NoError(t, err)

	agentCmd2 := StepRecordAgentCmd()
	agentCmd2.SetArgs([]string{sessionID, "hephaestus", "opus", "create_spec"})
	err = agentCmd2.Execute()
	require.NoError(t, err)

	// Verify all steps recorded
	listCmd := StepListCmd()
	listCmd.SetArgs([]string{sessionID})

	var output bytes.Buffer
	listCmd.SetOut(&output)
	err = listCmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err)
	steps := result["steps"].([]interface{})
	assert.Len(t, steps, 3)
	assert.Equal(t, float64(3), result["count"])
}
