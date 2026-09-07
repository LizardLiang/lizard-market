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

func TestFindSimilarMemory_AndReplace(t *testing.T) {
	db := NewTestDBWithSchema(t)
	orig, err := AddMemory(db, "Prefers terse replies with the conclusion first", "preference")
	require.NoError(t, err)
	_, err = AddMemory(db, "Windows console is cp950; run python CLIs with PYTHONIOENCODING=utf-8", "context")
	require.NoError(t, err)

	dup, score, err := FindSimilarMemory(db, "Prefers terse replies, conclusion first")
	require.NoError(t, err)
	require.NotNil(t, dup)
	assert.Equal(t, orig.ID, dup.ID)
	assert.GreaterOrEqual(t, score, MemoryDuplicateThreshold)

	none, _, err := FindSimilarMemory(db, "Uses LizMeter tickets as the system of record")
	require.NoError(t, err)
	assert.Nil(t, none)

	replaced, err := ReplaceMemory(db, orig.ID, "Prefers terse replies: conclusion first, then evidence", "preference", "")
	require.NoError(t, err)
	assert.Equal(t, orig.ID, replaced.ID)
	assert.Equal(t, "Prefers terse replies: conclusion first, then evidence", replaced.Text)

	_, err = ReplaceMemory(db, 9999, "x", "context", "")
	assert.Error(t, err)
}
