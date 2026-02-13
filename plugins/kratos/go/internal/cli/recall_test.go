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

// TestRecallCmd_NoSessions tests recall when no sessions exist
func TestRecallCmd_NoSessions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// Initialize DB
	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Test: Recall with no sessions
	cmd := RecallCmd()
	cmd.SetArgs([]string{"/test/project"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	assert.Nil(t, result["last_session"])
}

// TestRecallCmd_WithSession tests recall with an existing session
func TestRecallCmd_WithSession(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Create a session
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project", "test-feature"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	startCmd.Execute()

	var startResult map[string]interface{}
	json.Unmarshal(startOutput.Bytes(), &startResult)
	sessionID := startResult["session_id"].(string)

	// End the session
	endCmd := SessionEndCmd()
	endCmd.SetArgs([]string{sessionID, "Test completed"})
	var endOutput bytes.Buffer
	endCmd.SetOut(&endOutput)
	endCmd.Execute()

	// Test: Recall
	cmd := RecallCmd()
	cmd.SetArgs([]string{"/test/project"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	assert.NotNil(t, result["last_session"])

	lastSession := result["last_session"].(map[string]interface{})
	assert.Equal(t, "/test/project", lastSession["project"])
	assert.Equal(t, "test-feature", lastSession["feature_name"])
}

// TestRecallCmd_Global tests recall across all projects
func TestRecallCmd_Global(t *testing.T) {
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

		var startResult map[string]interface{}
		json.Unmarshal(startOutput.Bytes(), &startResult)
		sessionID := startResult["session_id"].(string)

		endCmd := SessionEndCmd()
		endCmd.SetArgs([]string{sessionID})
		var endOutput bytes.Buffer
		endCmd.SetOut(&endOutput)
		endCmd.Execute()
	}

	// Test: Global recall
	cmd := RecallCmd()
	cmd.SetArgs([]string{"--global"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	assert.NotNil(t, result["recent_sessions"])

	sessions := result["recent_sessions"].([]interface{})
	assert.GreaterOrEqual(t, len(sessions), 2)
}

// TestRecallCmd_IncompleteFeatures tests recalling incomplete features
func TestRecallCmd_IncompleteFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Create an active session with feature
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project", "incomplete-feature"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	startCmd.Execute()

	// Test: Recall incomplete features
	cmd := RecallCmd()
	cmd.SetArgs([]string{"/test/project", "--incomplete"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	assert.NotNil(t, result["incomplete_features"])

	incomplete := result["incomplete_features"].([]interface{})
	assert.GreaterOrEqual(t, len(incomplete), 1)
}

// TestRecallCmd_WithLimit tests recall with limit flag
func TestRecallCmd_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	initCmd := InitCmd()
	var initOutput bytes.Buffer
	initCmd.SetOut(&initOutput)
	initCmd.Execute()

	// Create 3 sessions
	for i := 0; i < 3; i++ {
		startCmd := SessionStartCmd()
		startCmd.SetArgs([]string{"/test/project"})
		var startOutput bytes.Buffer
		startCmd.SetOut(&startOutput)
		startCmd.Execute()

		var startResult map[string]interface{}
		json.Unmarshal(startOutput.Bytes(), &startResult)
		sessionID := startResult["session_id"].(string)

		endCmd := SessionEndCmd()
		endCmd.SetArgs([]string{sessionID})
		var endOutput bytes.Buffer
		endCmd.SetOut(&endOutput)
		endCmd.Execute()
	}

	// Test: Recall with limit 2
	cmd := RecallCmd()
	cmd.SetArgs([]string{"--global", "--limit", "2"})

	var output bytes.Buffer
	cmd.SetOut(&output)
	err := cmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	sessions := result["recent_sessions"].([]interface{})
	assert.Len(t, sessions, 2)
}
