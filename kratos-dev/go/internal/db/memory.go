package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Memory represents a durable fact about the user (preference, habit, weak spot)
type Memory struct {
	ID        int64   `json:"id"`
	Text      string  `json:"text"`
	Category  string  `json:"category"` // preference, habit, weak-spot, context
	Project   *string `json:"project,omitempty"`
	CreatedAt int64   `json:"created_at"`
}

// MemoryListOpts filters ListMemoriesOpts. Zero values mean "no filter".
type MemoryListOpts struct {
	Category string // "" or "all" means every category
	Project  string // exact match on the project column; "" means no filter
	SinceMs  int64  // created_at >= SinceMs when > 0
	Limit    int    // > 0 caps the result (newest first)
}

// EnsureMemoryProjectColumn adds user_memories.project on databases created
// before the column existed. CREATE TABLE IF NOT EXISTS never alters an
// existing table, so InitDB calls this after applying the schema. Idempotent.
func EnsureMemoryProjectColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(user_memories)`)
	if err != nil {
		return fmt.Errorf("failed to inspect user_memories: %w", err)
	}
	defer rows.Close()
	hasProject := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("failed to scan table_info: %w", err)
		}
		if name == "project" {
			hasProject = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	if !hasProject {
		if _, err := db.Exec(`ALTER TABLE user_memories ADD COLUMN project TEXT`); err != nil {
			return fmt.Errorf("failed to add user_memories.project: %w", err)
		}
	}
	// The index lives here, not in schema.sql: on a pre-migration database the
	// schema's CREATE INDEX would run before the column exists and fail.
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_memories_project ON user_memories(project)`)
	return err
}

// AddMemory inserts a new user memory with no project scope.
func AddMemory(db *sql.DB, text, category string) (*Memory, error) {
	return AddMemoryWithProject(db, text, category, "")
}

// AddMemoryWithProject inserts a new user memory. project "" stores NULL
// (a global fact); anything else scopes the memory to that project root.
func AddMemoryWithProject(db *sql.DB, text, category, project string) (*Memory, error) {
	now := time.Now().UnixMilli()
	var proj interface{}
	var projPtr *string
	if project != "" {
		proj = project
		p := project
		projPtr = &p
	}
	result, err := db.Exec(`INSERT INTO user_memories (text, category, project, created_at) VALUES (?, ?, ?, ?)`,
		text, category, proj, now)
	if err != nil {
		return nil, fmt.Errorf("failed to add memory: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert ID: %w", err)
	}

	return &Memory{ID: id, Text: text, Category: category, Project: projPtr, CreatedAt: now}, nil
}

// ReplaceMemory rewrites an existing memory in place (same id) and refreshes
// its timestamp. Used when a new fact supersedes a near-duplicate. An empty
// project keeps the stored project.
func ReplaceMemory(db *sql.DB, id int64, text, category, project string) (*Memory, error) {
	now := time.Now().UnixMilli()
	var proj interface{}
	if project != "" {
		proj = project
	}
	result, err := db.Exec(`UPDATE user_memories SET text = ?, category = ?, project = COALESCE(?, project), created_at = ? WHERE id = ?`,
		text, category, proj, now, id)
	if err != nil {
		return nil, fmt.Errorf("failed to replace memory: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("memory %d not found", id)
	}
	return GetMemory(db, id)
}

// ListMemories retrieves memories, optionally filtered by category ("all" or "" means no filter)
func ListMemories(db *sql.DB, category string) ([]*Memory, error) {
	return ListMemoriesOpts(db, MemoryListOpts{Category: category})
}

// ListMemoriesOpts retrieves memories newest-first, applying every non-zero
// filter in opts.
func ListMemoriesOpts(db *sql.DB, opts MemoryListOpts) ([]*Memory, error) {
	query := `SELECT id, text, category, project, created_at FROM user_memories`
	var where []string
	var args []interface{}

	if opts.Category != "" && opts.Category != "all" {
		where = append(where, "category = ?")
		args = append(args, opts.Category)
	}
	if opts.Project != "" {
		where = append(where, "project = ?")
		args = append(args, opts.Project)
	}
	if opts.SinceMs > 0 {
		where = append(where, "created_at >= ?")
		args = append(args, opts.SinceMs)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC"
	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		m := &Memory{}
		var proj sql.NullString
		if err := rows.Scan(&m.ID, &m.Text, &m.Category, &proj, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan memory: %w", err)
		}
		if proj.Valid {
			p := proj.String
			m.Project = &p
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// CountMemories returns the number of rows matching opts (Limit is ignored).
func CountMemories(db *sql.DB, opts MemoryListOpts) (int, error) {
	opts.Limit = 0
	all, err := ListMemoriesOpts(db, opts)
	if err != nil {
		return 0, err
	}
	return len(all), nil
}

// RemoveMemory deletes a memory by ID
func RemoveMemory(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM user_memories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove memory: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memory %d not found", id)
	}
	return nil
}

// GetMemory retrieves a single memory by ID
func GetMemory(db *sql.DB, id int64) (*Memory, error) {
	m := &Memory{}
	var proj sql.NullString
	err := db.QueryRow(
		`SELECT id, text, category, project, created_at FROM user_memories WHERE id = ?`, id,
	).Scan(&m.ID, &m.Text, &m.Category, &proj, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}
	if proj.Valid {
		p := proj.String
		m.Project = &p
	}
	return m, nil
}

// memoryTokenRE splits memory text into comparable tokens: runs of letters
// or digits. Punctuation and case never count as a difference.
var memoryTokenRE = regexp.MustCompile(`[\p{L}\p{N}]+`)

// memoryTokens returns the lowercase token set of text. CJK runs are split
// into overlapping character bigrams so two Chinese sentences that share most
// of their characters compare as similar even without word boundaries.
func memoryTokens(text string) map[string]bool {
	set := map[string]bool{}
	for _, tok := range memoryTokenRE.FindAllString(strings.ToLower(text), -1) {
		runes := []rune(tok)
		cjk := len(runes) > 0 && runes[0] >= 0x2E80
		if cjk && len(runes) > 1 {
			for i := 0; i+1 < len(runes); i++ {
				set[string(runes[i:i+2])] = true
			}
			continue
		}
		set[tok] = true
	}
	return set
}

// MemorySimilarity is the Jaccard similarity of two memories' token sets, in
// [0, 1]. 1 means identical token sets.
func MemorySimilarity(a, b string) float64 {
	ta, tb := memoryTokens(a), memoryTokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	for t := range ta {
		if tb[t] {
			inter++
		}
	}
	union := len(ta) + len(tb) - inter
	return float64(inter) / float64(union)
}

// MemoryDuplicateThreshold is the Jaccard similarity at or above which a new
// memory counts as a rewording of an existing one.
const MemoryDuplicateThreshold = 0.6

// FindSimilarMemory returns the stored memory most similar to text and its
// score, or (nil, 0, nil) when nothing reaches MemoryDuplicateThreshold. The
// store holds hundreds of rows, so a full scan is fine.
func FindSimilarMemory(db *sql.DB, text string) (*Memory, float64, error) {
	all, err := ListMemoriesOpts(db, MemoryListOpts{})
	if err != nil {
		return nil, 0, err
	}
	var best *Memory
	bestScore := 0.0
	for _, m := range all {
		if s := MemorySimilarity(text, m.Text); s > bestScore {
			best, bestScore = m, s
		}
	}
	if best == nil || bestScore < MemoryDuplicateThreshold {
		return nil, 0, nil
	}
	return best, bestScore, nil
}
