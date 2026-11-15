package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompareVersions pins the numeric dotted compare both update paths use:
// a string compare would call "2.99.0" newer than "2.100.0".
func TestCompareVersions(t *testing.T) {
	home := t.TempDir()
	script := "const k = require(" + hookModule("kratos-bin.cjs") + ");" +
		"console.log(JSON.stringify([" +
		"k.compareVersions('2.126.1', '2.126.0')," +
		"k.compareVersions('2.126.0', '2.126.1')," +
		"k.compareVersions('2.99.0', '2.100.0')," +
		"k.compareVersions('v2.126.1', '2.126.1')," +
		"k.compareVersions('2.126', '2.126.0')," +
		"k.compareVersions(null, '2.126.0')," +
		"]));"
	out := runNodeHook(t, home, script)
	assert.Equal(t, "[1,-1,-1,0,0,-1]", strings.TrimSpace(out))
}

// TestVersionMismatchNeverRefreshesNewerBinary: a session still pinned to an
// older plugin root sees a newer shared binary. That line must tell the user
// to start a new session, never claim a refresh — and ensure-binary.cjs must
// not download the older version (the 2026-11-09 downgrade path).
func TestVersionMismatchNeverRefreshesNewerBinary(t *testing.T) {
	home := t.TempDir()
	script := "const s = require(" + hookModule("session-start.cjs") + ");" +
		"console.log(JSON.stringify([" +
		"s.formatVersionMismatchFor('2.126.1', '2.126.0', true)," +
		"s.formatVersionMismatchFor('2.125.0', '2.126.0', true)," +
		"s.formatVersionMismatchFor('2.125.0', '2.126.0', false)," +
		"s.formatVersionMismatchFor('2.126.0', '2.126.0', true)," +
		"s.formatVersionMismatchFor(null, '2.126.0', true)," +
		"]));"
	out := runNodeHook(t, home, script)
	var lines []*string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &lines), out)
	require.Len(t, lines, 5)

	require.NotNil(t, lines[0])
	assert.Contains(t, *lines[0], "plugin v2.126.0 in this session is older than binary v2.126.1")
	assert.Contains(t, *lines[0], "start a new session")
	assert.NotContains(t, *lines[0], "refreshing")

	require.NotNil(t, lines[1])
	assert.Equal(t, "Kratos: binary v2.125.0 ≠ plugin v2.126.0 — refreshing in background", *lines[1])
	require.NotNil(t, lines[2])
	assert.Contains(t, *lines[2], "rebuild: cd kratos-dev/go && make build")
	assert.Nil(t, lines[3], "equal versions print nothing")
	assert.Nil(t, lines[4], "unknown binary version prints nothing")

	data, err := os.ReadFile(filepath.Join(hooksDirPath(), "ensure-binary.cjs"))
	require.NoError(t, err)
	body := string(data)
	guard := "compareVersions(currentVersion, wantedVersion) > 0) return;"
	assert.Contains(t, body, guard, "ensure-binary.cjs must refuse to replace a newer binary")
	assert.Less(t, strings.Index(body, guard), strings.Index(body, "if (throttled()) return;"),
		"the downgrade guard must run before the throttle so a stale session never records an attempt")
}
