package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRoot wires the subcommands these tests exercise under one root so a
// test can run "session start …" the way the hooks do.
func testRoot() *cobra.Command {
	root := &cobra.Command{Use: "kratos", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(InitCmd(), SessionCmd(), StepCmd(), MemoryCmd())
	return root
}

func runCLI(t *testing.T, args ...string) (map[string]interface{}, error) {
	t.Helper()
	cmd := testRoot()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	var result map[string]interface{}
	if out.Len() > 0 {
		_ = json.Unmarshal(out.Bytes(), &result)
	}
	return result, err
}

func useTempDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	t.Setenv("KRATOS_MEMORY_DB", dbPath)
	_, err := runCLI(t, "init")
	require.NoError(t, err)
}

// `session start --session-id` is idempotent on the Claude Code session id:
// the first call creates, the second returns the same row, and an ended row
// is re-activated (a resumed session keeps its id).
func TestSessionStart_SessionIDIdempotent(t *testing.T) {
	useTempDB(t)

	first, err := runCLI(t, "session", "start", "/proj/a", "--session-id", "cc-123")
	require.NoError(t, err)
	assert.Equal(t, "cc-123", first["session_id"])
	assert.Equal(t, true, first["created"])
	assert.Equal(t, false, first["resumed"])

	second, err := runCLI(t, "session", "start", "/proj/a", "--session-id", "cc-123")
	require.NoError(t, err)
	assert.Equal(t, false, second["created"])
	assert.Equal(t, false, second["resumed"])
	assert.Equal(t, first["id"], second["id"])

	// Two concurrent sessions in the same project are both allowed.
	other, err := runCLI(t, "session", "start", "/proj/a", "--session-id", "cc-456")
	require.NoError(t, err)
	assert.Equal(t, true, other["created"])

	_, err = runCLI(t, "session", "end", "cc-123", "bye")
	require.NoError(t, err)
	resumed, err := runCLI(t, "session", "start", "/proj/a", "--session-id", "cc-123")
	require.NoError(t, err)
	assert.Equal(t, true, resumed["resumed"])
	assert.Equal(t, "active", resumed["status"])
	assert.Nil(t, resumed["ended_at"])
}

// Regression: `step record-agent` against a session id that has no row used
// to fail with "FOREIGN KEY constraint failed". The row is now created.
func TestStepRecordAgent_CreatesMissingSession(t *testing.T) {
	useTempDB(t)

	res, err := runCLI(t, "step", "record-agent", "cc-new", "odysseus", "sonnet", "plan ticket #52", "--project", "C:\\Proj\\Whiteboard")
	require.NoError(t, err)
	assert.Equal(t, "success", res["status"])

	_, err = runCLI(t, "step", "record-file", "cc-new", "modified", "src/app.ts", "--project", "C:\\Proj\\Whiteboard")
	require.NoError(t, err)

	list, err := runCLI(t, "step", "list", "cc-new")
	require.NoError(t, err)
	assert.Equal(t, float64(2), list["count"])

	active, err := runCLI(t, "session", "active", "C:/Proj/Whiteboard")
	require.NoError(t, err)
	sess, _ := active["session"].(map[string]interface{})
	require.NotNil(t, sess, "row was created with the normalized project path")
	assert.Equal(t, "cc-new", sess["session_id"])
}

func TestInitialRequestText(t *testing.T) {
	cases := map[string]string{
		"fix the login bug":                   "fix the login bug",
		"  spaced  ":                          "spaced",
		"/clear":                              "",
		"/kratos:iris do #26":                 "",
		"!echo \"KRATOS_ROOT=x\"":             "",
		"<command-name>/model</command-name>": "",
		"You ARE Iris. KRATOS_ROOT=/x agent load iris": "",
		"": "",
	}
	for in, want := range cases {
		assert.Equal(t, want, initialRequestText(in), "input %q", in)
	}
}
