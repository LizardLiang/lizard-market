package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A database created before user_memories.project existed must gain the
// column on InitDB — CREATE TABLE IF NOT EXISTS alone never alters a table.
func TestEnsureMemoryProjectColumn_MigratesOldTable(t *testing.T) {
	db := NewTestDB(t)
	_, err := db.Exec(`CREATE TABLE user_memories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		text TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT 'context',
		created_at INTEGER NOT NULL
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_memories (text, category, created_at) VALUES ('old fact', 'habit', 1)`)
	require.NoError(t, err)

	require.NoError(t, InitDB(db))
	require.NoError(t, InitDB(db), "migration must be idempotent")

	all, err := ListMemoriesOpts(db, MemoryListOpts{})
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Nil(t, all[0].Project, "pre-migration rows are global")

	m, err := AddMemoryWithProject(db, "scoped fact", "context", "/proj/a")
	require.NoError(t, err)
	require.NotNil(t, m.Project)
	assert.Equal(t, "/proj/a", *m.Project)
}

func TestListMemoriesOpts_Filters(t *testing.T) {
	db := NewTestDBWithSchema(t)
	_, err := AddMemoryWithProject(db, "global one", "preference", "")
	require.NoError(t, err)
	_, err = AddMemoryWithProject(db, "project a fact", "context", "/proj/a")
	require.NoError(t, err)
	_, err = AddMemoryWithProject(db, "project b fact", "context", "/proj/b")
	require.NoError(t, err)

	byProject, err := ListMemoriesOpts(db, MemoryListOpts{Project: "/proj/a"})
	require.NoError(t, err)
	require.Len(t, byProject, 1)
	assert.Equal(t, "project a fact", byProject[0].Text)

	limited, err := ListMemoriesOpts(db, MemoryListOpts{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, limited, 2)

	total, err := CountMemories(db, MemoryListOpts{Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, total, "count ignores the limit")

	future := time.Now().Add(time.Hour).UnixMilli()
	none, err := ListMemoriesOpts(db, MemoryListOpts{SinceMs: future})
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestMemorySimilarity(t *testing.T) {
	a := "寫 docs/ 圖說時絕不從圖卡既有註記抄語彙，改用系統名詞"
	b := "寫 docs/ 圖說時絕不抄圖卡既有註記的語彙，改用系統名詞"
	assert.GreaterOrEqual(t, MemorySimilarity(a, b), MemoryDuplicateThreshold, "rewording of the same CJK lesson")

	c := "Prefers terse replies with the conclusion first"
	d := "Prefers terse replies, conclusion first"
	assert.GreaterOrEqual(t, MemorySimilarity(c, d), MemoryDuplicateThreshold)

	e := "Runs stock checks mid-session before the close"
	assert.Less(t, MemorySimilarity(c, e), MemoryDuplicateThreshold, "unrelated facts stay distinct")
	assert.Equal(t, 0.0, MemorySimilarity("", c))
}

// The four paraphrase duplicates the sweep saved in the 2026-09-08 week: each
// slipped under the Jaccard threshold and would be caught by content-word
// overlap. Texts are the real stored memories.
var paraphrasePairs = []struct{ name, stored, incoming string }{
	{"sed -i CRLF",
		"sed -i in Git Bash rewrites CRLF files as LF on Windows: git warns 'LF will be replaced by CRLF' and the whole file shows as changed. Use the Edit tool or convert back to CRLF.",
		"sed -i in Git Bash rewrites a CRLF file to LF, flooding the diff with whole-file churn. On this Windows repo edit .cs via the Edit tool, or convert back to CRLF and verify."},
	{"here-strings",
		"In the Bash tool, never commit with PowerShell here-string (-m @'...'@) — bash leaves a literal '@ ' on the commit subject (recurring bug he hates). Use git commit -F - with a heredoc.",
		"Bash tool is Git Bash: PowerShell here-strings (@'...'@) are NOT supported and leak a literal @ into the argument (polluted a commit subject). Use bash heredoc <<'EOF'."},
	{"LizMeter todos",
		"All todos go to LizMeter via the lizmeter-todo MCP inline - never the kratos todo store. Ananke has no MCP tools, so do not route todo work to her.",
		"Todo MCP tools (mcp__*todo*, e.g. LizMeter) are the system of record; Kratos v2.104+ calls them inline and treats Ananke as fallback only. Never file tickets in the Kratos todo store."},
	{"heredoc backslashes",
		"Bash tool heredocs collapse one backslash level: a JS regex written /\\\\n/ lands on disk as /\\n/ (silent no-op, parser returns 0 rows). Use the Write tool for scripts containing backslash escapes.",
		"Bash heredocs in this environment collapse backslashes: a JS file written with '\\\\n' lands as a real newline and fails to parse. Use the Write tool for any file containing escape sequences."},
}

func TestMemoryOverlap_CatchesParaphrases(t *testing.T) {
	for _, p := range paraphrasePairs {
		assert.Less(t, MemorySimilarity(p.stored, p.incoming), MemoryDuplicateThreshold, "%s: Jaccard alone missed this pair in production", p.name)
		assert.GreaterOrEqual(t, MemoryOverlap(p.stored, p.incoming), MemoryOverlapThreshold, "%s: overlap must flag the paraphrase", p.name)
	}
}

func TestMemoryOverlap_UnrelatedStaysBelow(t *testing.T) {
	terse := "Prefers terse replies with the conclusion first"
	assert.Less(t, MemoryOverlap(terse, "NETZERO decks use the BGTO master"), MemoryOverlapThreshold)
	assert.Less(t, MemoryOverlap(terse, paraphrasePairs[0].stored), MemoryOverlapThreshold)
	assert.Less(t, MemoryOverlap("Windows console is cp950; run python CLIs with PYTHONIOENCODING=utf-8", "Uses LizMeter tickets as the system of record"), MemoryOverlapThreshold)
	assert.Equal(t, 0.0, MemoryOverlap("the and of", "a an or"), "stopwords only → empty sets → 0")
	assert.Equal(t, 0.0, MemoryOverlap("", terse))
}

func TestMemoryContentTokens_DropsStopwordsAndSingles(t *testing.T) {
	got := memoryContentTokens("a CRLF file to LF in Git, x 1")
	assert.Equal(t, map[string]bool{"crlf": true, "file": true, "lf": true, "git": true}, got)
	cjk := memoryContentTokens("改用系統名詞")
	assert.True(t, cjk["改用"] && cjk["系統"] && cjk["名詞"], "CJK bigrams survive: %v", cjk)
}

func TestFindSimilarMemory_AndReplace(t *testing.T) {
	db := NewTestDBWithSchema(t)
	orig, err := AddMemory(db, "Prefers terse replies with the conclusion first", "preference")
	require.NoError(t, err)
	_, err = AddMemory(db, "Windows console is cp950; run python CLIs with PYTHONIOENCODING=utf-8", "context")
	require.NoError(t, err)

	dup, err := FindSimilarMemory(db, "Prefers terse replies, conclusion first")
	require.NoError(t, err)
	require.NotNil(t, dup)
	assert.Equal(t, orig.ID, dup.Memory.ID)
	assert.GreaterOrEqual(t, dup.Score, MemoryDuplicateThreshold)
	assert.Equal(t, MetricJaccard, dup.Metric)

	none, err := FindSimilarMemory(db, "Uses LizMeter tickets as the system of record")
	require.NoError(t, err)
	assert.Nil(t, none)

	replaced, err := ReplaceMemory(db, orig.ID, "Prefers terse replies: conclusion first, then evidence", "preference", "")
	require.NoError(t, err)
	assert.Equal(t, orig.ID, replaced.ID)
	assert.Equal(t, "Prefers terse replies: conclusion first, then evidence", replaced.Text)

	_, err = ReplaceMemory(db, 9999, "x", "context", "")
	assert.Error(t, err)
}

// A paraphrase that Jaccard lets through is caught by the overlap metric and
// reported as such; an unrelated fact still passes.
func TestFindSimilarMemory_OverlapMetric(t *testing.T) {
	db := NewTestDBWithSchema(t)
	stored, err := AddMemory(db, paraphrasePairs[0].stored, "context")
	require.NoError(t, err)
	_, err = AddMemory(db, "Windows console is cp950; run python CLIs with PYTHONIOENCODING=utf-8", "context")
	require.NoError(t, err)

	match, err := FindSimilarMemory(db, paraphrasePairs[0].incoming)
	require.NoError(t, err)
	require.NotNil(t, match)
	assert.Equal(t, stored.ID, match.Memory.ID)
	assert.Equal(t, MetricOverlap, match.Metric)
	assert.GreaterOrEqual(t, match.Score, MemoryOverlapThreshold)

	none, err := FindSimilarMemory(db, "Uses LizMeter tickets as the system of record")
	require.NoError(t, err)
	assert.Nil(t, none)
}
