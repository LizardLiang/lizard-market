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

// Test: session start
func TestSessionStartCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Initialize DB first
	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	err := initCmd.Execute()
	require.NoError(t, err)

	// Test: Start session
	cmd := SessionStartCmd()
	cmd.SetArgs([]string{"/test/project"})

	var output bytes.Buffer
	cmd.SetOut(&output)

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify JSON output
	var result map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result["session_id"])
	assert.Equal(t, "/test/project", result["project"])
	assert.Equal(t, "active", result["status"])
	assert.NotNil(t, result["started_at"])
}

// Test: session start with feature
func TestSessionStartCmd_WithFeature(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	cmd := SessionStartCmd()
	cmd.SetArgs([]string{"/test/project", "auth-feature"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	assert.Equal(t, "auth-feature", result["feature_name"])
}

// Test: session active (no active session)
func TestSessionActiveCmd_NoSession(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	cmd := SessionActiveCmd()
	cmd.SetArgs([]string{"/test/project"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	assert.Nil(t, result["session"])
	assert.Equal(t, "no active session", result["message"])
}

// Test: session active (with active session)
func TestSessionActiveCmd_WithSession(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Start session first
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	startCmd.Execute()

	// Test: Get active
	cmd := SessionActiveCmd()
	cmd.SetArgs([]string{"/test/project"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)

	session := result["session"].(map[string]interface{})
	assert.Equal(t, "/test/project", session["project"])
	assert.Equal(t, "active", session["status"])
}

// Test: session end
func TestSessionEndCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Start session
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	startCmd.Execute()

	var startResult map[string]interface{}
	json.Unmarshal(startOutput.Bytes(), &startResult)
	sessionID := startResult["session_id"].(string)

	// Test: End session
	cmd := SessionEndCmd()
	cmd.SetArgs([]string{sessionID, "Work completed"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	assert.Equal(t, "completed", result["status"])
	assert.NotNil(t, result["ended_at"])
}
