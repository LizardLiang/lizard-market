package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeedbackAddListRmRoundTrip(t *testing.T) {
	setupMemoryTestDB(t)

	// Add
	addCmd := FeedbackAddCmd()
	addCmd.SetArgs([]string{"no magic strings — use exported consts", "--agent", "ares"})
	var addOutput bytes.Buffer
	addCmd.SetOut(&addOutput)
	require.NoError(t, addCmd.Execute())

	var addResult map[string]interface{}
	require.NoError(t, json.Unmarshal(addOutput.Bytes(), &addResult))
	assert.Equal(t, "added", addResult["status"])
	feedback := addResult["feedback"].(map[string]interface{})
	assert.Equal(t, "no magic strings — use exported consts", feedback["lesson"])
	assert.Equal(t, "ares", feedback["agent"])
	assert.NotEmpty(t, feedback["project"])
	id := int64(feedback["id"].(float64))
	require.NotZero(t, id)

	// List
	listCmd := FeedbackListCmd()
	listCmd.SetArgs([]string{"--agent", "ares"})
	var listOutput bytes.Buffer
	listCmd.SetOut(&listOutput)
	require.NoError(t, listCmd.Execute())

	var listResult map[string]interface{}
	require.NoError(t, json.Unmarshal(listOutput.Bytes(), &listResult))
	lessons := listResult["feedback"].([]interface{})
	require.Len(t, lessons, 1)

	// Remove
	rmCmd := FeedbackRemoveCmd()
	rmCmd.SetArgs([]string{"1"})
	var rmOutput bytes.Buffer
	rmCmd.SetOut(&rmOutput)
	require.NoError(t, rmCmd.Execute())

	var rmResult map[string]interface{}
	require.NoError(t, json.Unmarshal(rmOutput.Bytes(), &rmResult))
	assert.Equal(t, "removed", rmResult["status"])

	// Confirm removal
	listCmd2 := FeedbackListCmd()
	var listOutput2 bytes.Buffer
	listCmd2.SetOut(&listOutput2)
	require.NoError(t, listCmd2.Execute())

	var listResult2 map[string]interface{}
	require.NoError(t, json.Unmarshal(listOutput2.Bytes(), &listResult2))
	lessons2 := listResult2["feedback"]
	if lessons2 != nil {
		assert.Len(t, lessons2.([]interface{}), 0)
	}
}

func TestFeedbackAddRequiresAgent(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := FeedbackAddCmd()
	cmd.SetArgs([]string{"a lesson without an agent"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent")
}

func TestFeedbackAddStripsKratosPrefix(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := FeedbackAddCmd()
	cmd.SetArgs([]string{"prefer table-driven tests", "--agent", "kratos:Ares"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	feedback := result["feedback"].(map[string]interface{})
	assert.Equal(t, "ares", feedback["agent"])

	// Listing with the bare name finds it
	listCmd := FeedbackListCmd()
	listCmd.SetArgs([]string{"--agent", "ares"})
	var listOut bytes.Buffer
	listCmd.SetOut(&listOut)
	require.NoError(t, listCmd.Execute())

	var listResult map[string]interface{}
	require.NoError(t, json.Unmarshal(listOut.Bytes(), &listResult))
	assert.Len(t, listResult["feedback"].([]interface{}), 1)
}

func TestFeedbackAddRejectsLessonOver200Chars(t *testing.T) {
	setupMemoryTestDB(t)

	longLesson := strings.Repeat("a", 201)

	cmd := FeedbackAddCmd()
	cmd.SetArgs([]string{longLesson, "--agent", "ares"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "200")
}

func TestFeedbackListLimit(t *testing.T) {
	setupMemoryTestDB(t)

	for i := 0; i < 7; i++ {
		cmd := FeedbackAddCmd()
		cmd.SetArgs([]string{strings.Repeat("x", i+1), "--agent", "ares"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())
	}

	listCmd := FeedbackListCmd()
	listCmd.SetArgs([]string{"--agent", "ares", "--limit", "5"})
	var out bytes.Buffer
	listCmd.SetOut(&out)
	require.NoError(t, listCmd.Execute())

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Len(t, result["feedback"].([]interface{}), 5)
	assert.Equal(t, float64(5), result["count"])
}

func TestFeedbackRmNonExistentID(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := FeedbackRemoveCmd()
	cmd.SetArgs([]string{"999"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	assert.Error(t, err)
}
