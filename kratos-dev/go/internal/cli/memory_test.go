package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMemoryTestDB points KRATOS_MEMORY_DB at a fresh temp DB and returns a
// cleanup function to unset it.
func setupMemoryTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("KRATOS_MEMORY_DB", dbPath)
	t.Cleanup(func() { os.Unsetenv("KRATOS_MEMORY_DB") })
}

func TestMemoryAddListRmRoundTrip(t *testing.T) {
	setupMemoryTestDB(t)

	// Add
	addCmd := MemoryAddCmd()
	addCmd.SetArgs([]string{"prefers terse output", "--category", "preference"})
	var addOutput bytes.Buffer
	addCmd.SetOut(&addOutput)
	require.NoError(t, addCmd.Execute())

	var addResult map[string]interface{}
	require.NoError(t, json.Unmarshal(addOutput.Bytes(), &addResult))
	assert.Equal(t, "added", addResult["status"])
	memory := addResult["memory"].(map[string]interface{})
	assert.Equal(t, "prefers terse output", memory["text"])
	assert.Equal(t, "preference", memory["category"])
	id := int64(memory["id"].(float64))
	require.NotZero(t, id)

	// List
	listCmd := MemoryListCmd()
	var listOutput bytes.Buffer
	listCmd.SetOut(&listOutput)
	require.NoError(t, listCmd.Execute())

	var listResult map[string]interface{}
	require.NoError(t, json.Unmarshal(listOutput.Bytes(), &listResult))
	memories := listResult["memories"].([]interface{})
	require.Len(t, memories, 1)

	// Remove
	rmCmd := MemoryRemoveCmd()
	rmCmd.SetArgs([]string{"1"})
	var rmOutput bytes.Buffer
	rmCmd.SetOut(&rmOutput)
	require.NoError(t, rmCmd.Execute())

	var rmResult map[string]interface{}
	require.NoError(t, json.Unmarshal(rmOutput.Bytes(), &rmResult))
	assert.Equal(t, "removed", rmResult["status"])

	// Confirm removal
	listCmd2 := MemoryListCmd()
	var listOutput2 bytes.Buffer
	listCmd2.SetOut(&listOutput2)
	require.NoError(t, listCmd2.Execute())

	var listResult2 map[string]interface{}
	require.NoError(t, json.Unmarshal(listOutput2.Bytes(), &listResult2))
	memories2 := listResult2["memories"]
	if memories2 != nil {
		assert.Len(t, memories2.([]interface{}), 0)
	}
}

func TestMemoryListFilterByCategory(t *testing.T) {
	setupMemoryTestDB(t)

	add := func(text, category string) {
		cmd := MemoryAddCmd()
		cmd.SetArgs([]string{text, "--category", category})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())
	}
	add("prefers terse output", "preference")
	add("often forgets to run tests", "weak-spot")

	listCmd := MemoryListCmd()
	listCmd.SetArgs([]string{"--category", "weak-spot"})
	var out bytes.Buffer
	listCmd.SetOut(&out)
	require.NoError(t, listCmd.Execute())

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	memories := result["memories"].([]interface{})
	require.Len(t, memories, 1)
	first := memories[0].(map[string]interface{})
	assert.Equal(t, "often forgets to run tests", first["text"])
}

func TestMemoryAddRejectsTextOver200Chars(t *testing.T) {
	setupMemoryTestDB(t)

	longText := strings.Repeat("a", 201)

	cmd := MemoryAddCmd()
	cmd.SetArgs([]string{longText})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "200")
}

func TestMemoryAddAccepts200Chars(t *testing.T) {
	setupMemoryTestDB(t)

	exactText := strings.Repeat("a", 200)

	cmd := MemoryAddCmd()
	cmd.SetArgs([]string{exactText})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())
}

func TestMemoryAddRejectsUnknownCategory(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := MemoryAddCmd()
	cmd.SetArgs([]string{"put the actual list in the ticket body", "--category", "feedback"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown category "feedback"`)
	assert.Contains(t, err.Error(), "kratos feedback add")
}

// TestMemoryAddAcceptsRuleCategory covers the new standing-rule category:
// `memory add` must accept it, and `memory list --category rule` must return
// only rows saved with it.
func TestMemoryAddAcceptsRuleCategory(t *testing.T) {
	setupMemoryTestDB(t)

	addCmd := MemoryAddCmd()
	addCmd.SetArgs([]string{"never touch his credentials", "--category", "rule"})
	var addOutput bytes.Buffer
	addCmd.SetOut(&addOutput)
	require.NoError(t, addCmd.Execute())

	var addResult map[string]interface{}
	require.NoError(t, json.Unmarshal(addOutput.Bytes(), &addResult))
	assert.Equal(t, "added", addResult["status"])
	memory := addResult["memory"].(map[string]interface{})
	assert.Equal(t, "rule", memory["category"])

	otherCmd := MemoryAddCmd()
	otherCmd.SetArgs([]string{"prefers terse output", "--category", "preference"})
	var otherOutput bytes.Buffer
	otherCmd.SetOut(&otherOutput)
	require.NoError(t, otherCmd.Execute())

	listCmd := MemoryListCmd()
	listCmd.SetArgs([]string{"--category", "rule"})
	var listOutput bytes.Buffer
	listCmd.SetOut(&listOutput)
	require.NoError(t, listCmd.Execute())

	var listResult map[string]interface{}
	require.NoError(t, json.Unmarshal(listOutput.Bytes(), &listResult))
	memories := listResult["memories"].([]interface{})
	require.Len(t, memories, 1)
	first := memories[0].(map[string]interface{})
	assert.Equal(t, "never touch his credentials", first["text"])
}

func TestMemoryRemoveNonExistentID(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := MemoryRemoveCmd()
	cmd.SetArgs([]string{"999"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	assert.Error(t, err)
}
