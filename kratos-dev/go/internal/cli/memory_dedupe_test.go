package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A rewording of a stored fact is rejected and names the existing id;
// --replace supersedes it in place; --force keeps both.
func TestMemoryAdd_RejectsNearDuplicate(t *testing.T) {
	useTempDB(t)

	first, err := runCLI(t, "memory", "add", "Prefers terse replies with the conclusion first", "--category", "preference")
	require.NoError(t, err)
	mem := first["memory"].(map[string]interface{})
	id := mem["id"].(float64)

	_, err = runCLI(t, "memory", "add", "Prefers terse replies, conclusion first", "--category", "preference")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "near-duplicate")
	assert.Contains(t, err.Error(), "--replace")

	replaced, err := runCLI(t, "memory", "add", "Prefers terse replies: conclusion first, then evidence", "--category", "preference", "--replace", "1")
	require.NoError(t, err)
	assert.Equal(t, "replaced", replaced["status"])
	rm := replaced["memory"].(map[string]interface{})
	assert.Equal(t, id, rm["id"])

	forced, err := runCLI(t, "memory", "add", "Prefers terse replies, conclusion first", "--category", "preference", "--force")
	require.NoError(t, err)
	assert.Equal(t, "added", forced["status"])

	list, err := runCLI(t, "memory", "list")
	require.NoError(t, err)
	assert.Equal(t, float64(2), list["count"])
}

func TestMemoryAdd_ProjectScopeAndCJKLength(t *testing.T) {
	useTempDB(t)

	res, err := runCLI(t, "memory", "add", "draw.io 桌面版 CLI 的 -p 是 1-based 頁碼", "--category", "context", "--project", "C:\\Work\\NETZERO")
	require.NoError(t, err)
	mem := res["memory"].(map[string]interface{})
	assert.Equal(t, normalizeProjectPath("C:\\Work\\NETZERO"), mem["project"])

	// 150 CJK characters are 450 bytes but only 150 characters: accepted.
	_, err = runCLI(t, "memory", "add", strings.Repeat("字", 150), "--category", "context", "--force")
	require.NoError(t, err)

	_, err = runCLI(t, "memory", "add", strings.Repeat("字", 201), "--force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 200 characters")
}

func TestMemoryList_LimitProjectSinceIds(t *testing.T) {
	useTempDB(t)

	for _, text := range []string{
		"Uses LizMeter tickets as the system of record",
		"Runs stock checks mid-session before the close",
		"Diagram cards drop the outer container when the name is shown",
	} {
		_, err := runCLI(t, "memory", "add", text, "--force")
		require.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}
	_, err := runCLI(t, "memory", "add", "NETZERO decks use the BGTO master", "--project", "/work/netzero", "--force")
	require.NoError(t, err)

	limited, err := runCLI(t, "memory", "list", "--limit", "2")
	require.NoError(t, err)
	assert.Equal(t, float64(2), limited["count"])
	assert.Equal(t, float64(4), limited["total"])
	mems := limited["memories"].([]interface{})
	assert.Equal(t, "NETZERO decks use the BGTO master", mems[0].(map[string]interface{})["text"], "newest first")

	scoped, err := runCLI(t, "memory", "list", "--project", "/work/netzero")
	require.NoError(t, err)
	assert.Equal(t, float64(1), scoped["count"])

	ids, err := runCLI(t, "memory", "list", "--ids-only", "--limit", "3")
	require.NoError(t, err)
	assert.Len(t, ids["ids"], 3)
	assert.Equal(t, float64(4), ids["total"])

	recent, err := runCLI(t, "memory", "list", "--since", "1d")
	require.NoError(t, err)
	assert.Equal(t, float64(4), recent["count"])

	_, err = runCLI(t, "memory", "list", "--since", "soon")
	require.Error(t, err)
}

func TestParseSince(t *testing.T) {
	d, err := parseSince("7d")
	require.NoError(t, err)
	assert.Equal(t, 7*24*time.Hour, d)
	d, err = parseSince("36h")
	require.NoError(t, err)
	assert.Equal(t, 36*time.Hour, d)
	d, err = parseSince("30m")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, d)
	d, err = parseSince("3")
	require.NoError(t, err)
	assert.Equal(t, 72*time.Hour, d)
	_, err = parseSince("-1d")
	assert.Error(t, err)
}
