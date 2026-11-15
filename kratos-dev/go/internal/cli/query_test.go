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

// TestQuerySessionsCmd tests querying recent sessions
func TestQuerySessionsCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Initialize DB
	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Create some sessions
	for i := 1; i <= 3; i++ {
		startCmd := SessionStartCmd()
		startCmd.SetArgs([]string{"/test/project"})
		var startOutput bytes.Buffer
		startCmd.SetOut(&startOutput)
		startCmd.Execute()

		// End previous sessions to create variety
		if i < 3 {
			var startResult map[string]interface{}
			json.Unmarshal(startOutput.Bytes(), &startResult)
			sessionID := startResult["session_id"].(string)

			endCmd := SessionEndCmd()
			endCmd.SetArgs([]string{sessionID})
			var endOutput bytes.Buffer
			endCmd.SetOut(&endOutput)
			endCmd.Execute()
		}
	}

	// Test: Query sessions with limit
	cmd := QuerySessionsCmd()
	cmd.SetArgs([]string{"--limit", "2"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	sessions := result["sessions"].([]interface{})
	assert.Len(t, sessions, 2)
}

// TestQuerySessionsByStatusCmd tests filtering sessions by status
func TestQuerySessionsByStatusCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Create active session
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	startCmd.Execute()

	// Test: Query by status
	cmd := QuerySessionsCmd()
	cmd.SetArgs([]string{"--status", "active"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	sessions := result["sessions"].([]interface{})
	assert.GreaterOrEqual(t, len(sessions), 1)
}

// TestQuerySessionsByProjectCmd tests filtering sessions by project
func TestQuerySessionsByProjectCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Create sessions for different projects
	projects := []string{"/project/a", "/project/b"}
	for _, proj := range projects {
		startCmd := SessionStartCmd()
		startCmd.SetArgs([]string{proj})
		var startOutput bytes.Buffer
		startCmd.SetOut(&startOutput)
		startCmd.Execute()
	}

	// Test: Query by project
	cmd := QuerySessionsCmd()
	cmd.SetArgs([]string{"--project", "/project/a"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	sessions := result["sessions"].([]interface{})
	assert.GreaterOrEqual(t, len(sessions), 1)

	// Verify project matches
	firstSession := sessions[0].(map[string]interface{})
	assert.Equal(t, "/project/a", firstSession["project"])
}

// TestQuerySearchCmd tests full-text search
func TestQuerySearchCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Create session with feature name
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project", "authentication"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	startCmd.Execute()

	// Test: Search for "auth"
	cmd := QuerySearchCmd()
	cmd.SetArgs([]string{"auth"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	sessions := result["sessions"].([]interface{})
	assert.GreaterOrEqual(t, len(sessions), 1)
}

// TestQueryStepsCmd tests querying steps for a session
func TestQueryStepsCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Create session
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	startCmd.Execute()

	var startResult map[string]interface{}
	json.Unmarshal(startOutput.Bytes(), &startResult)
	sessionID := startResult["session_id"].(string)

	// Test: Query steps (should be empty)
	cmd := QueryStepsCmd()
	cmd.SetArgs([]string{sessionID})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	steps := result["steps"].([]interface{})
	assert.Len(t, steps, 0) // No steps yet
}

// TestQueryCountCmd tests getting session count
func TestQueryCountCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Initially count should be 0
	cmd1 := QueryCountCmd()
	var output1 bytes.Buffer
	cmd1.SetOut(&output1)
	cmd1.Execute()

	var result1 map[string]interface{}
	json.Unmarshal(output1.Bytes(), &result1)
	assert.Equal(t, float64(0), result1["count"])

	// Create sessions
	for i := 0; i < 3; i++ {
		startCmd := SessionStartCmd()
		startCmd.SetArgs([]string{"/test/project"})
		var startOutput bytes.Buffer
		startCmd.SetOut(&startOutput)
		startCmd.Execute()

		// End session
		var startResult map[string]interface{}
		json.Unmarshal(startOutput.Bytes(), &startResult)
		sessionID := startResult["session_id"].(string)

		endCmd := SessionEndCmd()
		endCmd.SetArgs([]string{sessionID})
		var endOutput bytes.Buffer
		endCmd.SetOut(&endOutput)
		endCmd.Execute()
	}

	// Count should be 3
	cmd2 := QueryCountCmd()
	var output2 bytes.Buffer
	cmd2.SetOut(&output2)
	cmd2.Execute()

	var result2 map[string]interface{}
	json.Unmarshal(output2.Bytes(), &result2)
	assert.Equal(t, float64(3), result2["count"])
}
