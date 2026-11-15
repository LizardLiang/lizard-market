package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runRoutineList(t *testing.T, dueOnly bool) map[string]interface{} {
	t.Helper()
	cmd := RoutineListCmd()
	if dueOnly {
		cmd.SetArgs([]string{"--due"})
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	return result
}

func TestRoutineAddListDoneRmRoundTrip(t *testing.T) {
	setupMemoryTestDB(t)

	// Add (default daily cadence → always due when never done)
	addCmd := RoutineAddCmd()
	addCmd.SetArgs([]string{"review inbox"})
	var addOutput bytes.Buffer
	addCmd.SetOut(&addOutput)
	require.NoError(t, addCmd.Execute())

	var addResult map[string]interface{}
	require.NoError(t, json.Unmarshal(addOutput.Bytes(), &addResult))
	assert.Equal(t, "added", addResult["status"])
	routine := addResult["routine"].(map[string]interface{})
	assert.Equal(t, "review inbox", routine["text"])
	assert.Equal(t, "daily", routine["cadence"])
	id := int64(routine["id"].(float64))
	require.NotZero(t, id)

	// List --due: daily never-done is due
	dueResult := runRoutineList(t, true)
	assert.NotEmpty(t, dueResult["date"], "list should include the resolved date")
	dueRoutines := dueResult["routines"].([]interface{})
	require.Len(t, dueRoutines, 1)
	first := dueRoutines[0].(map[string]interface{})
	assert.Equal(t, true, first["due_today"])

	// Done
	doneCmd := RoutineDoneCmd()
	doneCmd.SetArgs([]string{"1"})
	var doneOutput bytes.Buffer
	doneCmd.SetOut(&doneOutput)
	require.NoError(t, doneCmd.Execute())

	var doneResult map[string]interface{}
	require.NoError(t, json.Unmarshal(doneOutput.Bytes(), &doneResult))
	assert.Equal(t, "done", doneResult["status"])
	doneRoutine := doneResult["routine"].(map[string]interface{})
	assert.NotNil(t, doneRoutine["last_done"])

	// List --due: no longer due after done
	dueResult2 := runRoutineList(t, true)
	if routines := dueResult2["routines"]; routines != nil {
		assert.Len(t, routines.([]interface{}), 0)
	}

	// Full list still shows it
	allResult := runRoutineList(t, false)
	assert.Len(t, allResult["routines"].([]interface{}), 1)

	// Remove
	rmCmd := RoutineRemoveCmd()
	rmCmd.SetArgs([]string{"1"})
	var rmOutput bytes.Buffer
	rmCmd.SetOut(&rmOutput)
	require.NoError(t, rmCmd.Execute())

	var rmResult map[string]interface{}
	require.NoError(t, json.Unmarshal(rmOutput.Bytes(), &rmResult))
	assert.Equal(t, "removed", rmResult["status"])
}

func TestRoutineAddRejectsInvalidCadence(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := RoutineAddCmd()
	cmd.SetArgs([]string{"bad routine", "--cadence", "weekly:funday"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	assert.Error(t, cmd.Execute())
}

func TestRoutineDoneNonExistentID(t *testing.T) {
	setupMemoryTestDB(t)

	cmd := RoutineDoneCmd()
	cmd.SetArgs([]string{"999"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	assert.Error(t, cmd.Execute())
}
