package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The settings.json shape the pre-plugin installer produced, mixed with hooks
// and rules that belong to other tools and must survive the migration.
const legacySettingsFixture = `{
  "permissions": {
    "allow": [
      "Bash(ask-pi:*)",
      "Bash(C:/Users/u/.claude/hooks/kratos/kratos:*)",
      "Bash(~/.kratos/bin/kratos:*)"
    ],
    "deny": ["Read(~/secrets/**)"]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Read|Edit",
        "hooks": [{"type": "command", "command": "node \"C:/Users/u/.claude/hooks/guard.cjs\"", "timeout": 5}]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Task|Write|Edit",
        "hooks": [{"type": "command", "command": "node \"C:/Users/u/.claude/hooks/kratos/tool-use.cjs\"", "timeout": 5000}]
      }
    ],
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "node \"C:/Users/u/.claude/hooks/kratos/session-start.cjs\"", "timeout": 5000},
          {"type": "command", "command": "node \"C:/Users/u/.claude/hooks/other-tool.cjs\"", "timeout": 5000}
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [{"type": "command", "command": "node \"C:\\Users\\u\\.claude\\hooks\\kratos\\session-end.cjs\"", "timeout": 10000}]
      }
    ]
  },
  "editorMode": "vim"
}`

func TestRemoveLegacyHookEntriesKeepsOtherHooks(t *testing.T) {
	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(legacySettingsFixture), &settings))

	removed := removeLegacyHookEntries(settings)
	assert.Equal(t, 4, removed, "tool-use, session-start, session-end, and the hooks/kratos permission rule")

	hooks := settings["hooks"].(map[string]interface{})
	assert.Contains(t, hooks, "PreToolUse", "another tool's hook event must survive")
	_, hasPost := hooks["PostToolUse"]
	assert.False(t, hasPost, "event with only kratos hooks is dropped")
	_, hasStop := hooks["Stop"]
	assert.False(t, hasStop, "backslash paths are recognized too")

	start := hooks["SessionStart"].([]interface{})
	require.Len(t, start, 1)
	entries := start[0].(map[string]interface{})["hooks"].([]interface{})
	require.Len(t, entries, 1, "the non-kratos SessionStart hook stays in the same group")
	assert.Contains(t, entries[0].(map[string]interface{})["command"], "other-tool.cjs")

	allow := settings["permissions"].(map[string]interface{})["allow"].([]interface{})
	assert.ElementsMatch(t, []interface{}{"Bash(ask-pi:*)", "Bash(~/.kratos/bin/kratos:*)"}, allow,
		"only the hooks/kratos rule goes; the ~/.kratos binary rule is still used by the plugin")
	assert.Equal(t, "vim", settings["editorMode"])
}

func TestRemoveLegacyHooksFromSettingsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(file, []byte(legacySettingsFixture), 0644))

	removed, err := removeLegacyHooksFromSettingsFile(file)
	require.NoError(t, err)
	assert.Equal(t, 4, removed)

	data, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "hooks/kratos/")
	assert.NotContains(t, string(data), `\u0026`, "no HTML escaping in the rewritten file")
	assert.Contains(t, string(data), "Read|Edit", "matcher regex survives the round trip")

	// Second run is a no-op that leaves the file untouched.
	before, _ := os.Stat(file)
	removed, err = removeLegacyHooksFromSettingsFile(file)
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
	after, _ := os.Stat(file)
	assert.Equal(t, before.ModTime(), after.ModTime())
}

func TestRemoveLegacyHooksFromSettingsFileMissing(t *testing.T) {
	_, err := removeLegacyHooksFromSettingsFile(filepath.Join(t.TempDir(), "settings.json"))
	assert.True(t, os.IsNotExist(err))
}
