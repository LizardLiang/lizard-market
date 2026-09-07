package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runAgentLoadOut(t *testing.T, god string) string {
	t.Helper()
	cmd := AgentCmd()
	cmd.SetArgs([]string{"load", god})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())
	return out.String()
}

// Inline gods load through `agent load`; their stored lessons must ride along,
// with a retro nudge once enough have piled up.
func TestAgentLoad_InjectsLessons(t *testing.T) {
	useTempDB(t)
	conn, err := db.GetConnection()
	require.NoError(t, err)
	defer conn.Close()

	assert.NotContains(t, runAgentLoadOut(t, "odysseus"), "Lessons from past user corrections", "no lessons → no block")

	_, err = db.AddFeedback(conn, "odysseus", "Ask one clarity question at a time; he rejects batches.", "some-project")
	require.NoError(t, err)
	out := runAgentLoadOut(t, "odysseus")
	assert.Contains(t, out, "Lessons from past user corrections of odysseus")
	assert.Contains(t, out, "Ask one clarity question at a time")
	assert.NotContains(t, out, "/kratos:retro", "below the nudge threshold")
	assert.NotContains(t, runAgentLoadOut(t, "ares"), "Ask one clarity question", "lessons are per god")

	for i := 0; i < 5; i++ {
		_, err = db.AddFeedback(conn, "odysseus", "lesson "+strings.Repeat("x", i+1), "some-project")
		require.NoError(t, err)
	}
	out = runAgentLoadOut(t, "odysseus")
	assert.Contains(t, out, "/kratos:retro odysseus")
	block := out[strings.Index(out, "Lessons from past user corrections"):]
	if nudge := strings.Index(block, "lessons pending"); nudge > 0 {
		block = block[:nudge]
	}
	assert.LessOrEqual(t, strings.Count(block, "\n- "), lessonsInjectMax, "at most five injected")
}

func TestSpecArchiveGuard(t *testing.T) {
	root := t.TempDir()
	write := func(feature, body string) {
		dir := filepath.Join(root, ".claude", "feature", feature)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "status.json"), []byte(body), 0o644))
	}
	write("reviewing", `{"pipeline":{"9-review":{"status":"in-progress"}}}`)
	write("reviewed", `{"pipeline":{"9-review":{"status":"complete","code_review_verdict":"approved"}}}`)

	err := specArchiveGuard(root, "reviewing", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")
	assert.NoError(t, specArchiveGuard(root, "reviewing", true))
	assert.NoError(t, specArchiveGuard(root, "reviewed", false))
	assert.NoError(t, specArchiveGuard(root, "plan-only-no-status", false))
}

func TestProfileList_StaleFlag(t *testing.T) {
	useTempDB(t)
	conn, err := db.GetConnection()
	require.NoError(t, err)
	defer conn.Close()

	_, err = db.SetProfile(conn, "current_focus", "Shipping a product feature")
	require.NoError(t, err)
	_, err = db.SetProfile(conn, "timezone", "Asia/Taipei")
	require.NoError(t, err)
	old := time.Now().Add(-45 * 24 * time.Hour).UnixMilli()
	_, err = conn.Exec(`UPDATE user_profile SET updated_at = ? WHERE key = 'current_focus'`, old)
	require.NoError(t, err)

	cmd := ProfileListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())
	var res struct {
		Profile []profileEntryOut `json:"profile"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &res))
	byKey := map[string]profileEntryOut{}
	for _, p := range res.Profile {
		byKey[p.Key] = p
	}
	assert.True(t, byKey["current_focus"].Stale)
	assert.GreaterOrEqual(t, byKey["current_focus"].AgeDays, 44)
	assert.False(t, byKey["timezone"].Stale)
}
