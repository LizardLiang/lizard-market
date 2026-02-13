package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndWorkflow tests complete session flow from start to end
func TestEndToEndWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	// 1. Initialize database
	initCmd := InitCmd()
	err := initCmd.Execute()
	require.NoError(t, err, "Init should succeed")

	// 2. Start a session
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project", "test-feature"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	err = startCmd.Execute()
	require.NoError(t, err, "Session start should succeed")

	var startResult map[string]interface{}
	err = json.Unmarshal(startOutput.Bytes(), &startResult)
	require.NoError(t, err, "Should parse start JSON")
	sessionID := startResult["session_id"].(string)
	assert.NotEmpty(t, sessionID, "Session ID should be returned")

	// 3. Record some steps (agent spawns and file changes)
	recordAgentCmd := StepRecordAgentCmd()
	recordAgentCmd.SetArgs([]string{sessionID, "athena", "opus", "create_prd"})
	err = recordAgentCmd.Execute()
	require.NoError(t, err, "Record agent spawn should succeed")

	recordFileCmd := StepRecordFileCmd()
	recordFileCmd.SetArgs([]string{sessionID, "Write", "prd.md"})
	err = recordFileCmd.Execute()
	require.NoError(t, err, "Record file change should succeed")

	recordAgentCmd2 := StepRecordAgentCmd()
	recordAgentCmd2.SetArgs([]string{sessionID, "hephaestus", "opus", "create_spec"})
	err = recordAgentCmd2.Execute()
	require.NoError(t, err, "Second agent spawn should succeed")

	// 4. Query steps
	listStepsCmd := StepListCmd()
	listStepsCmd.SetArgs([]string{sessionID})
	var stepsOutput bytes.Buffer
	listStepsCmd.SetOut(&stepsOutput)
	err = listStepsCmd.Execute()
	require.NoError(t, err, "List steps should succeed")

	var stepsResult map[string]interface{}
	err = json.Unmarshal(stepsOutput.Bytes(), &stepsResult)
	require.NoError(t, err, "Should parse steps JSON")
	steps := stepsResult["steps"].([]interface{})
	assert.Len(t, steps, 3, "Should have 3 steps recorded")

	// 5. End the session
	endCmd := SessionEndCmd()
	endCmd.SetArgs([]string{sessionID, "Completed test workflow"})
	var endOutput bytes.Buffer
	endCmd.SetOut(&endOutput)
	err = endCmd.Execute()
	require.NoError(t, err, "Session end should succeed")

	// 6. Verify session was updated
	queryCmd := QuerySessionsCmd()
	queryCmd.SetArgs([]string{"--project", "/test/project"})
	var queryOutput bytes.Buffer
	queryCmd.SetOut(&queryOutput)
	err = queryCmd.Execute()
	require.NoError(t, err, "Query should succeed")

	var queryResult map[string]interface{}
	err = json.Unmarshal(queryOutput.Bytes(), &queryResult)
	require.NoError(t, err, "Should parse query JSON")

	sessions := queryResult["sessions"].([]interface{})
	require.Len(t, sessions, 1, "Should have 1 session")

	session := sessions[0].(map[string]interface{})
	assert.Equal(t, "completed", session["status"], "Session should be completed")
	assert.Equal(t, float64(3), session["total_steps"], "Should have 3 steps")
	assert.Equal(t, float64(2), session["total_agents_spawned"], "Should have 2 agents spawned")
	assert.NotNil(t, session["ended_at"], "Should have end time")
	assert.NotNil(t, session["summary"], "Should have summary")
}

// TestConcurrentSessions tests handling multiple sessions across different projects
func TestConcurrentSessions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	err := InitCmd().Execute()
	require.NoError(t, err)

	// Start multiple sessions in different projects (avoids active session conflict)
	sessionIDs := []string{}
	for i := 0; i < 3; i++ {
		startCmd := SessionStartCmd()
		startCmd.SetArgs([]string{"/test/project" + string(rune('1'+i)), "feature-" + string(rune('A'+i))})
		var output bytes.Buffer
		startCmd.SetOut(&output)
		err := startCmd.Execute()
		require.NoError(t, err)

		var result map[string]interface{}
		json.Unmarshal(output.Bytes(), &result)
		sessionIDs = append(sessionIDs, result["session_id"].(string))
	}

	// Verify all sessions are active
	queryCmd := QuerySessionsCmd()
	queryCmd.SetArgs([]string{"--status", "active"})
	var output bytes.Buffer
	queryCmd.SetOut(&output)
	err = queryCmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	sessions := result["sessions"].([]interface{})
	assert.Len(t, sessions, 3, "Should have 3 active sessions")

	// End one session
	endCmd := SessionEndCmd()
	endCmd.SetArgs([]string{sessionIDs[1], "Completed middle session"})
	err = endCmd.Execute()
	require.NoError(t, err)

	// Verify counts
	queryCmd2 := QuerySessionsCmd()
	queryCmd2.SetArgs([]string{"--status", "active"})
	var output2 bytes.Buffer
	queryCmd2.SetOut(&output2)
	err = queryCmd2.Execute()
	require.NoError(t, err)

	var result2 map[string]interface{}
	json.Unmarshal(output2.Bytes(), &result2)
	activeSessions := result2["sessions"].([]interface{})
	assert.Len(t, activeSessions, 2, "Should have 2 active sessions remaining")
}

// TestRecallWorkflow tests recall functionality
func TestRecallWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	err := InitCmd().Execute()
	require.NoError(t, err)

	// Start and complete a session
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project", "auth-system"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	err = startCmd.Execute()
	require.NoError(t, err)

	var startResult map[string]interface{}
	json.Unmarshal(startOutput.Bytes(), &startResult)
	sessionID := startResult["session_id"].(string)

	// Record activity
	cmd1 := StepRecordAgentCmd()
	cmd1.SetArgs([]string{sessionID, "athena", "opus", "PRD"})
	cmd1.Execute()

	cmd2 := StepRecordFileCmd()
	cmd2.SetArgs([]string{sessionID, "Write", "auth.md"})
	cmd2.Execute()

	cmd3 := SessionEndCmd()
	cmd3.SetArgs([]string{sessionID, "Completed auth design"})
	cmd3.Execute()

	// Start a new incomplete session
	startCmd2 := SessionStartCmd()
	startCmd2.SetArgs([]string{"/test/project", "payment-system"})
	var startOutput2 bytes.Buffer
	startCmd2.SetOut(&startOutput2)
	err = startCmd2.Execute()
	require.NoError(t, err)

	var startResult2 map[string]interface{}
	json.Unmarshal(startOutput2.Bytes(), &startResult2)
	sessionID2 := startResult2["session_id"].(string)

	cmd4 := StepRecordAgentCmd()
	cmd4.SetArgs([]string{sessionID2, "athena", "opus", "PRD"})
	cmd4.Execute()

	// Recall should show the incomplete session (use --global to see all projects)
	recallCmd := RecallCmd()
	recallCmd.SetArgs([]string{"--global"})
	var recallOutput bytes.Buffer
	recallCmd.SetOut(&recallOutput)
	err = recallCmd.Execute()
	require.NoError(t, err)

	output := recallOutput.String()
	assert.Contains(t, output, "payment-system", "Should show incomplete feature")
	assert.Contains(t, output, "active", "Should show active status")
}

// TestStepOrderingIntegration tests that steps are recorded in correct order
func TestStepOrderingIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	err := InitCmd().Execute()
	require.NoError(t, err)

	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	err = startCmd.Execute()
	require.NoError(t, err)

	var startResult map[string]interface{}
	json.Unmarshal(startOutput.Bytes(), &startResult)
	sessionID := startResult["session_id"].(string)

	// Record steps with slight delays to ensure different timestamps
	steps := []struct {
		agent  string
		action string
	}{
		{"metis", "research"},
		{"athena", "prd"},
		{"hephaestus", "spec"},
		{"ares", "code"},
		{"hermes", "review"},
	}

	for _, step := range steps {
		cmd := StepRecordAgentCmd()
		cmd.SetArgs([]string{sessionID, step.agent, "sonnet", step.action})
		cmd.Execute()
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Verify steps are in order
	listCmd := StepListCmd()
	listCmd.SetArgs([]string{sessionID})
	var output bytes.Buffer
	listCmd.SetOut(&output)
	err = listCmd.Execute()
	require.NoError(t, err)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	stepsList := result["steps"].([]interface{})

	require.Len(t, stepsList, 5, "Should have 5 steps")

	for i, stepData := range stepsList {
		step := stepData.(map[string]interface{})
		assert.Equal(t, float64(i+1), step["step_number"], "Step %d should have correct number", i+1)
		assert.Equal(t, steps[i].agent, step["agent_name"], "Step %d should have correct agent", i+1)
	}
}

// TestQueryPerformance tests query performance with many sessions
func TestQueryPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	err := InitCmd().Execute()
	require.NoError(t, err)

	// Create many sessions
	numSessions := 100
	for i := 0; i < numSessions; i++ {
		startCmd := SessionStartCmd()
		startCmd.SetArgs([]string{"/test/project", "feature-" + string(rune(i))})
		var output bytes.Buffer
		startCmd.SetOut(&output)
		err := startCmd.Execute()
		require.NoError(t, err)

		// Add some steps to half of them
		if i%2 == 0 {
			var result map[string]interface{}
			json.Unmarshal(output.Bytes(), &result)
			sessionID := result["session_id"].(string)
			cmd := StepRecordAgentCmd()
			cmd.SetArgs([]string{sessionID, "athena", "opus", "prd"})
			cmd.Execute()
		}
	}

	// Query all sessions (should be < 50ms for 100 sessions)
	start := time.Now()
	queryCmd := QuerySessionsCmd()
	queryCmd.SetArgs([]string{"--limit", "100"})
	var output bytes.Buffer
	queryCmd.SetOut(&output)
	err = queryCmd.Execute()
	require.NoError(t, err)
	duration := time.Since(start)

	var result map[string]interface{}
	json.Unmarshal(output.Bytes(), &result)
	sessions := result["sessions"].([]interface{})

	assert.Len(t, sessions, numSessions, "Should return all sessions")
	assert.Less(t, duration.Milliseconds(), int64(50), "Query should complete in < 50ms")
	t.Logf("Query %d sessions took %v", numSessions, duration)
}

// TestDatabaseIntegrity tests database consistency
func TestDatabaseIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	defer os.Unsetenv("KRATOS_MEMORY_DB")

	err := InitCmd().Execute()
	require.NoError(t, err)

	// Start session
	startCmd := SessionStartCmd()
	startCmd.SetArgs([]string{"/test/project", "test-feature"})
	var startOutput bytes.Buffer
	startCmd.SetOut(&startOutput)
	err = startCmd.Execute()
	require.NoError(t, err)

	var startResult map[string]interface{}
	json.Unmarshal(startOutput.Bytes(), &startResult)
	sessionID := startResult["session_id"].(string)

	// Add steps
	cmd1 := StepRecordAgentCmd()
	cmd1.SetArgs([]string{sessionID, "athena", "opus", "prd"})
	cmd1.Execute()

	cmd2 := StepRecordFileCmd()
	cmd2.SetArgs([]string{sessionID, "Write", "file.md"})
	cmd2.Execute()

	// Verify counts match
	queryCmd := QuerySessionsCmd()
	queryCmd.SetArgs([]string{"--project", "/test/project"})
	var queryOutput bytes.Buffer
	queryCmd.SetOut(&queryOutput)
	err = queryCmd.Execute()
	require.NoError(t, err)

	var queryResult map[string]interface{}
	json.Unmarshal(queryOutput.Bytes(), &queryResult)
	sessions := queryResult["sessions"].([]interface{})
	session := sessions[0].(map[string]interface{})

	// Verify counts
	assert.Equal(t, float64(2), session["total_steps"], "Session total_steps should match")
	assert.Equal(t, float64(1), session["total_agents_spawned"], "Session total_agents_spawned should match")

	// Verify steps table matches
	listCmd := StepListCmd()
	listCmd.SetArgs([]string{sessionID})
	var stepsOutput bytes.Buffer
	listCmd.SetOut(&stepsOutput)
	err = listCmd.Execute()
	require.NoError(t, err)

	var stepsResult map[string]interface{}
	json.Unmarshal(stepsOutput.Bytes(), &stepsResult)
	steps := stepsResult["steps"].([]interface{})

	assert.Len(t, steps, 2, "Steps table should have 2 records")
	assert.Equal(t, float64(session["total_steps"].(float64)), float64(len(steps)), "Counts should be consistent")
}
