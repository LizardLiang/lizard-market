package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runNodeHook runs script under node with an isolated HOME and no
// KRATOS_STATE_DIR, returning stdout.
func runNodeHook(t *testing.T, home, script string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	cmd := exec.Command(node, "-e", script)
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "KRATOS_STATE_DIR=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

func hookModule(name string) string {
	return strconv.Quote(filepath.ToSlash(filepath.Join(hooksDirPath(), name)))
}

func writeShippedConfig(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "kratos.config.json"), []byte(content), 0o644))
	return filepath.ToSlash(root)
}

func TestSessionStartWritesShippedStateDir(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	plugin := writeShippedConfig(t, `{"state_dir":".agents"}`)
	script := "const s = require(" + hookModule("session-start.cjs") + ");" +
		"console.log(JSON.stringify(s.formatStateDirNotices(" + strconv.Quote(filepath.ToSlash(cwd)) + "," + strconv.Quote(plugin) + ")));"
	out := runNodeHook(t, home, script)
	var lines []string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &lines), out)
	assert.Equal(t, []string{"Kratos: state dir .agents (from ~/.kratos/config.json)"}, lines)

	data, err := os.ReadFile(filepath.Join(home, ".kratos", "config.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"state_dir": ".agents"`)

	// Second run: equal value, file untouched.
	info1, _ := os.Stat(filepath.Join(home, ".kratos", "config.json"))
	runNodeHook(t, home, script)
	info2, _ := os.Stat(filepath.Join(home, ".kratos", "config.json"))
	assert.Equal(t, info1.ModTime(), info2.ModTime(), "an equal setting is not rewritten")
}

func TestSessionStartMergesExistingConfig(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".kratos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".kratos", "config.json"), []byte(`{"other":1}`), 0o644))
	plugin := writeShippedConfig(t, `{"state_dir":".agents"}`)
	script := "require(" + hookModule("kratos-bin.cjs") + ").syncStateDirConfig(" + strconv.Quote(plugin) + ");"
	runNodeHook(t, home, script)
	data, err := os.ReadFile(filepath.Join(home, ".kratos", "config.json"))
	require.NoError(t, err)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(data, &cfg))
	assert.Equal(t, ".agents", cfg["state_dir"])
	assert.EqualValues(t, 1, cfg["other"], "other keys survive the merge")
}

func TestSessionStartVersionAWritesNothing(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	plugin := t.TempDir() // no kratos.config.json
	script := "const s = require(" + hookModule("session-start.cjs") + ");" +
		"console.log(JSON.stringify(s.formatStateDirNotices(" + strconv.Quote(filepath.ToSlash(cwd)) + "," + strconv.Quote(filepath.ToSlash(plugin)) + ")));"
	out := runNodeHook(t, home, script)
	assert.Equal(t, "[]", strings.TrimSpace(out))
	_, err := os.Stat(filepath.Join(home, ".kratos", "config.json"))
	assert.True(t, os.IsNotExist(err), "no shipped config means no ~/.kratos/config.json")
}

func TestSessionStartIgnoresInvalidShippedConfig(t *testing.T) {
	home := t.TempDir()
	plugin := writeShippedConfig(t, `{"state_dir":"../x"}`)
	runNodeHook(t, home, "require("+hookModule("kratos-bin.cjs")+").syncStateDirConfig("+strconv.Quote(plugin)+");")
	_, err := os.Stat(filepath.Join(home, ".kratos", "config.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestSessionStartLegacyStateNotice(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, ".claude", "feature", "x"), 0o755))
	plugin := writeShippedConfig(t, `{"state_dir":".agents"}`)
	script := "const s = require(" + hookModule("session-start.cjs") + ");" +
		"console.log(JSON.stringify(s.formatStateDirNotices(" + strconv.Quote(filepath.ToSlash(cwd)) + "," + strconv.Quote(plugin) + ")));"
	var lines []string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(runNodeHook(t, home, script))), &lines))
	require.Len(t, lines, 2)
	assert.Equal(t, "Kratos: found old state in .claude/ — move it with: mv .claude/feature .claude/.Arena .agents/", lines[1])
}

// The JS ledger counts feature/ and .Arena/ under the resolved state dir as
// project work and ignores the rest of that dir, as it does for .claude/.
func TestLedgerClassificationFollowsStateDir(t *testing.T) {
	cwd := filepath.ToSlash(t.TempDir())
	files := []string{
		cwd + "/.agents/feature/x/prd.md", cwd + "/.agents/.Arena/index.md", cwd + "/.agents/tmp/s.json", cwd + "/.agents/other.md",
		cwd + "/.claude/feature/x/prd.md", cwd + "/.claude/tmp/s.json", cwd + "/src/a.go",
	}
	cases := []struct {
		name, cfg string
		want      []bool
	}{
		{"B", `{"state_dir":".agents"}`, []bool{true, true, false, false, true, false, true}},
		{"A", "", []bool{true, true, true, true, true, false, true}},
	}
	for _, tc := range cases {
		home := t.TempDir()
		if tc.cfg != "" {
			require.NoError(t, os.MkdirAll(filepath.Join(home, ".kratos"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(home, ".kratos", "config.json"), []byte(tc.cfg), 0o644))
		}
		list, _ := json.Marshal(files)
		script := "const { isProjectFile } = require(" + hookModule("tool-use.cjs") + ");" +
			"console.log(JSON.stringify(" + string(list) + ".map((f) => isProjectFile(f, " + strconv.Quote(cwd) + "))));"
		var js []bool
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(runNodeHook(t, home, script))), &js))
		assert.Equal(t, tc.want, js, "version %s", tc.name)
	}
}
