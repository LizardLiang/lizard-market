package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mixedLegacySettingsFixture mixes legacy ~/.claude/hooks/kratos/ entries with
// foreign hooks (codegraph, token-monitor) in the same groups, across the
// events the old installer wrote plus PreToolUse.
const mixedLegacySettingsFixture = `{
  "model": "opus",
  "hooks": {
    "UserPromptSubmit": [
      {"matcher": "", "hooks": [
        {"type": "command", "command": "\"/home/u/.claude/hooks/kratos/kratos\" hook prompt-submit", "timeout": 3000}
      ]}
    ],
    "SessionStart": [
      {"matcher": "", "hooks": [
        {"type": "command", "command": "node \"/home/u/.claude/hooks/kratos/session-start.cjs\"", "timeout": 5000},
        {"type": "command", "command": "codegraph session-start"}
      ]}
    ],
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [
        {"type": "command", "command": "node \"C:\\Users\\u\\.claude\\hooks\\kratos\\fix-pm.cjs\""}
      ]},
      {"matcher": "Read", "hooks": [
        {"type": "command", "command": "codegraph pre-read"}
      ]}
    ],
    "PostToolUse": [
      {"matcher": "Task|Write|Edit", "hooks": [
        {"type": "command", "command": "node \"/home/u/.claude/hooks/kratos/tool-use.cjs\"", "timeout": 5000}
      ]}
    ],
    "Stop": [
      {"matcher": "", "hooks": [
        {"type": "command", "command": "token-monitor stop"},
        {"type": "command", "command": "node \"/home/u/.claude/hooks/kratos/session-end.cjs\"", "timeout": 10000}
      ]}
    ]
  },
  "permissions": {
    "allow": [
      "Read(~/.claude/plugins/cache/lizard-plugins/kratos/**)",
      "Bash(/home/u/.claude/hooks/kratos/kratos:*)",
      "Bash(~/.kratos/bin/kratos:*)",
      "Bash(git status:*)"
    ]
  }
}`

func TestRemoveHooksFromSettings_KeepsForeignHooks(t *testing.T) {
	settingsFile := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(settingsFile, []byte(mixedLegacySettingsFixture), 0644))
	require.True(t, hasLegacyHooks(settingsFile))

	require.NoError(t, removeHooksFromSettings(settingsFile))
	assert.False(t, hasLegacyHooks(settingsFile))

	data, err := os.ReadFile(settingsFile)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &got))

	want := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(`{
  "model": "opus",
  "hooks": {
    "SessionStart": [
      {"matcher": "", "hooks": [{"type": "command", "command": "codegraph session-start"}]}
    ],
    "PreToolUse": [
      {"matcher": "Read", "hooks": [{"type": "command", "command": "codegraph pre-read"}]}
    ],
    "Stop": [
      {"matcher": "", "hooks": [{"type": "command", "command": "token-monitor stop"}]}
    ]
  },
  "permissions": {"allow": ["Bash(git status:*)"]}
}`), &want))
	assert.Equal(t, want, got)
}

// With only Kratos hooks and permissions, the hooks and permissions keys go
// away and the other settings stay.
func TestRemoveHooksFromSettings_DropsEmptiedKeys(t *testing.T) {
	settingsFile := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(settingsFile, []byte(`{
  "theme": "dark",
  "hooks": {"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "node ~/.claude/hooks/kratos/session-end.cjs"}]}]},
  "permissions": {"allow": ["Bash(~/.kratos/bin/kratos:*)"]}
}`), 0644))

	require.NoError(t, removeHooksFromSettings(settingsFile))

	data, err := os.ReadFile(settingsFile)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, map[string]interface{}{"theme": "dark"}, got)
}
