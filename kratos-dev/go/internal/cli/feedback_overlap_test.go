package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// One lesson goes in one store: a feedback lesson that repeats a stored user
// memory is rejected and names the memory id; --force keeps both.
func TestFeedbackAdd_RejectsMemoryDuplicate(t *testing.T) {
	setupMemoryTestDB(t)

	mem := MemoryAddCmd()
	mem.SetArgs([]string{"Prefers terse replies with the conclusion first", "--category", "preference"})
	mem.SetOut(&bytes.Buffer{})
	require.NoError(t, mem.Execute())

	fb := FeedbackAddCmd()
	fb.SetArgs([]string{"Prefers terse replies, conclusion first", "--agent", "iris"})
	fb.SetOut(&bytes.Buffer{})
	fb.SetErr(&bytes.Buffer{})
	err := fb.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicates memory 1")
	assert.Contains(t, err.Error(), "--force")

	forced := FeedbackAddCmd()
	forced.SetArgs([]string{"Prefers terse replies, conclusion first", "--agent", "iris", "--force"})
	var out bytes.Buffer
	forced.SetOut(&out)
	require.NoError(t, forced.Execute())
	assert.Contains(t, out.String(), `"status":"added"`)

	// An unrelated lesson passes the check.
	other := FeedbackAddCmd()
	other.SetArgs([]string{"run the migration dry-run before applying", "--agent", "ares"})
	other.SetOut(&bytes.Buffer{})
	require.NoError(t, other.Execute())
}

// Length errors say how many characters to cut.
func TestLengthErrorsSayHowMuchToCut(t *testing.T) {
	setupMemoryTestDB(t)

	fb := FeedbackAddCmd()
	fb.SetArgs([]string{strings.Repeat("a", 216), "--agent", "ares"})
	fb.SetOut(&bytes.Buffer{})
	fb.SetErr(&bytes.Buffer{})
	err := fb.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lesson exceeds 200 characters (got 216) — cut 16")

	mem := MemoryAddCmd()
	mem.SetArgs([]string{strings.Repeat("字", 203)})
	mem.SetOut(&bytes.Buffer{})
	mem.SetErr(&bytes.Buffer{})
	err = mem.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory text exceeds 200 characters (got 203) — cut 3")
}
