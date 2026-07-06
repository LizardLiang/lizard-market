package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileSetGetListRmRoundTrip(t *testing.T) {
	setupMemoryTestDB(t)

	// Set
	setCmd := ProfileSetCmd()
	setCmd.SetArgs([]string{"timezone", "Asia/Taipei"})
	var setOutput bytes.Buffer
	setCmd.SetOut(&setOutput)
	require.NoError(t, setCmd.Execute())

	var setResult map[string]interface{}
	require.NoError(t, json.Unmarshal(setOutput.Bytes(), &setResult))
	assert.Equal(t, "set", setResult["status"])
	profile := setResult["profile"].(map[string]interface{})
	assert.Equal(t, "timezone", profile["key"])
	assert.Equal(t, "Asia/Taipei", profile["value"])

	// Get
	getCmd := ProfileGetCmd()
	getCmd.SetArgs([]string{"timezone"})
	var getOutput bytes.Buffer
	getCmd.SetOut(&getOutput)
	require.NoError(t, getCmd.Execute())

	var getResult map[string]interface{}
	require.NoError(t, json.Unmarshal(getOutput.Bytes(), &getResult))
	got := getResult["profile"].(map[string]interface{})
	assert.Equal(t, "Asia/Taipei", got["value"])

	// List
	listCmd := ProfileListCmd()
	var listOutput bytes.Buffer
	listCmd.SetOut(&listOutput)
	require.NoError(t, listCmd.Execute())

	var listResult map[string]interface{}
	require.NoError(t, json.Unmarshal(listOutput.Bytes(), &listResult))
	entries := listResult["profile"].([]interface{})
	require.Len(t, entries, 1)

	// Remove
	rmCmd := ProfileRemoveCmd()
	rmCmd.SetArgs([]string{"timezone"})
	var rmOutput bytes.Buffer
	rmCmd.SetOut(&rmOutput)
	require.NoError(t, rmCmd.Execute())

	var rmResult map[string]interface{}
	require.NoError(t, json.Unmarshal(rmOutput.Bytes(), &rmResult))
	assert.Equal(t, "removed", rmResult["status"])

	// Confirm removal
	getCmd2 := ProfileGetCmd()
	getCmd2.SetArgs([]string{"timezone"})
	var getOutput2 bytes.Buffer
	getCmd2.SetOut(&getOutput2)
	getCmd2.SetErr(&getOutput2)
	assert.Error(t, getCmd2.Execute())
}

func TestProfileSetOverwrites(t *testing.T) {
	setupMemoryTestDB(t)

	set := func(key, value string) {
		cmd := ProfileSetCmd()
		cmd.SetArgs([]string{key, value})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())
	}
	set("current_focus", "payments launch")
	set("current_focus", "kratos v3")

	getCmd := ProfileGetCmd()
	getCmd.SetArgs([]string{"current_focus"})
	var out bytes.Buffer
	getCmd.SetOut(&out)
	require.NoError(t, getCmd.Execute())

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	profile := result["profile"].(map[string]interface{})
	assert.Equal(t, "kratos v3", profile["value"])
}

func TestProfileSetRejectsInvalidKey(t *testing.T) {
	setupMemoryTestDB(t)

	for _, key := range []string{"Timezone", "time-zone", "1zone", "time zone", ""} {
		cmd := ProfileSetCmd()
		cmd.SetArgs([]string{key, "value"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		assert.Error(t, cmd.Execute(), "key %q should be rejected", key)
	}
}

func TestProfileSetRejectsValueOver500Chars(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := ProfileSetCmd()
	cmd.SetArgs([]string{"goals", strings.Repeat("a", 501)})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestProfileRemoveNonExistentKey(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := ProfileRemoveCmd()
	cmd.SetArgs([]string{"missing"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	assert.Error(t, cmd.Execute())
}
