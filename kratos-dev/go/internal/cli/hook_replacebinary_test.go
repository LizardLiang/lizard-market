package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// replaceBinaryScript runs kratos-bin.cjs's replaceBinary against target with a
// place() that writes content, printing the returned stale path (or "null").
func replaceBinaryScript(target, content string) string {
	return "const fs = require('fs'); const k = require(" + hookModule("kratos-bin.cjs") + ");" +
		"const aside = k.replaceBinary(" + strconv.Quote(filepath.ToSlash(target)) + ", (dest) => fs.writeFileSync(dest, " + strconv.Quote(content) + "));" +
		"console.log(JSON.stringify(aside));"
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestReplaceBinaryMovesToStaleThenDeletes pins the update order a running
// Windows executable requires: move the live file into bin/stale/ (a rename,
// allowed), place the new file at the fixed path, delete the stale file.
// Overwriting in place fails with EBUSY/EPERM while another session's hook
// executes the binary.
func TestReplaceBinaryMovesToStaleThenDeletes(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".kratos", "bin")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	target := filepath.Join(dir, "kratos.exe")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))

	out := runNodeHook(t, home, replaceBinaryScript(target, "new"))
	assert.Equal(t, "null", strings.TrimSpace(out), "old file deletable → no stale path returned")

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
	assert.Equal(t, []string{"kratos.exe"}, dirNames(t, dir), "stale file deleted and empty stale/ removed")
}

// TestSweepStaleBinaries: files left in bin/stale/ by an earlier update (the
// binary was still executing) are removed by the SessionStart sweep; other
// files in bin/ are untouched; a missing stale/ is a no-op.
func TestSweepStaleBinaries(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".kratos", "bin")
	stale := filepath.Join(dir, "stale")
	require.NoError(t, os.MkdirAll(stale, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stale, "kratos.exe.1-1"), []byte("stale"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stale, "kratos.exe.2-7"), []byte("stale"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kratos.exe.bak-2.120.0"), []byte("keep"), 0o755))

	script := "const k = require(" + hookModule("kratos-bin.cjs") + ");" +
		"console.log(k.sweepStaleBinaries(" + strconv.Quote(filepath.ToSlash(dir)) + "));" +
		"console.log(k.sweepStaleBinaries(" + strconv.Quote(filepath.ToSlash(dir)) + "));"
	out := runNodeHook(t, home, script)
	assert.Equal(t, "2\n0", strings.ReplaceAll(strings.TrimSpace(out), "\r\n", "\n"), "second sweep finds no stale folder")
	assert.Equal(t, []string{"kratos.exe.bak-2.120.0"}, dirNames(t, dir), "only stale/ is swept, and removed once empty")

	// No target yet: replaceBinary just places the file, nothing to move.
	target := filepath.Join(dir, "kratos.exe")
	out = runNodeHook(t, home, replaceBinaryScript(target, "fresh"))
	assert.Equal(t, "null", strings.TrimSpace(out))
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(data))
}

// TestReplaceBinaryRestoresOldOnPlaceFailure: when place() throws after the
// move to stale/, the old binary moves back so ~/.kratos/bin never goes empty.
func TestReplaceBinaryRestoresOldOnPlaceFailure(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".kratos", "bin")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	target := filepath.Join(dir, "kratos.exe")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))

	script := "const k = require(" + hookModule("kratos-bin.cjs") + ");" +
		"try { k.replaceBinary(" + strconv.Quote(filepath.ToSlash(target)) + ", () => { throw new Error('boom'); }); console.log('no-throw'); }" +
		"catch (e) { console.log('threw ' + e.message); }"
	out := runNodeHook(t, home, script)
	assert.Equal(t, "threw boom", strings.TrimSpace(out))

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "old", string(data), "old binary restored at the fixed path")
	assert.Equal(t, "", strings.Join(sweepLeft(t, dir), ","), "stale/ holds nothing after the restore")
}

// sweepLeft lists files under bin/stale/, or nothing when the folder is absent.
func sweepLeft(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, "stale"))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestBinaryUpdatePathsUseReplaceBinary pins both update paths to the helper
// and the SessionStart sweep: a bare renameSync/copyFileSync onto the target
// reintroduces the EBUSY/EPERM failure the 2026-11-09 health check found
// (binary stuck one version behind).
func TestBinaryUpdatePathsUseReplaceBinary(t *testing.T) {
	for _, tc := range []struct {
		file   string
		banned string
	}{
		{"ensure-binary.cjs", "fs.renameSync(TMP_FILE, targetPath)"},
		{"session-start.cjs", "fs.copyFileSync(srcPath, targetPath)"},
	} {
		data, err := os.ReadFile(filepath.Join(hooksDirPath(), tc.file))
		require.NoError(t, err, tc.file)
		body := string(data)
		assert.Contains(t, body, "replaceBinary(targetPath", "%s must update through replaceBinary", tc.file)
		assert.NotContains(t, body, tc.banned, "%s still writes onto the live binary", tc.file)
	}
	data, err := os.ReadFile(filepath.Join(hooksDirPath(), "session-start.cjs"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "sweepStaleBinaries(targetDir)", "session-start must sweep bin/stale/ every start")
}
