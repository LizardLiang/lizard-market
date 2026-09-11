package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// The per-session ledger (~/.kratos/sessions/<session_id>.json) is written by
// hooks/session-start.cjs. The inline edit gate stores four extra keys in it:
//
//	inline_god          the god currently running inline in the main context
//	inline_god_since    RFC3339 timestamp of the last god change
//	inline_edited_files normalized project source paths edited this turn
//	gate_bypass         the user told the model to do the work itself
//
// Every read and write is best-effort. A missing, unreadable or corrupt ledger
// means the gate emits no decision at all — the file is a discipline aid, never
// a lock on the session.
const (
	ledgerKeyInlineGod   = "inline_god"
	ledgerKeyInlineSince = "inline_god_since"
	ledgerKeyEditedFiles = "inline_edited_files"
	ledgerKeyGateBypass  = "gate_bypass"
	// ledgerKeyCwd is session-start.cjs's own key, read here as the fallback
	// project root when a payload carries no cwd.
	ledgerKeyCwd = "cwd"
)

// sessionLedgerFile is the ledger path for one Claude Code session id, or ""
// when the home dir or the id is unknown.
func sessionLedgerFile(sessionID string) string {
	dir := sessionLedgerDir()
	if dir == "" || sessionID == "" {
		return ""
	}
	return filepath.Join(dir, sessionID+".json")
}

// readInlineLedger returns the ledger as a generic map so unknown keys written
// by session-start.cjs (session_id, project, cwd, started_at, source) survive a
// round trip. Missing file, unreadable file and invalid JSON all return an
// error the callers treat as fail-open.
func readInlineLedger(sessionID string) (map[string]any, error) {
	path := sessionLedgerFile(sessionID)
	if path == "" {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, os.ErrNotExist
	}
	return m, nil
}

// writeInlineLedger replaces the ledger atomically: a half-written file read by
// the next edit hook must never be possible, and one counted edit writes the
// whole file. The temp file is uniquely named (see atomicWriteJSON) because two
// writers share this path — prompt-submit and the edit gate, with the async
// PostToolUse recorder able to overlap both.
func writeInlineLedger(sessionID string, m map[string]any) error {
	path := sessionLedgerFile(sessionID)
	if path == "" {
		return os.ErrNotExist
	}
	return atomicWriteJSON(path, m)
}

// normalizeLedgerPath canonicalizes a file path for both the budget list and
// the project-root test: forward slashes, `..` and doubled separators resolved,
// no trailing slash, lower case.
//
// One normalizer, deliberately: while the list used one rule and the
// project-root test another, an already-counted file arriving as
// C:/repo//src//a.ts burned a second budget slot. Case folds on every platform
// (not only Windows, as normalizeProjectPath does) because the paths this gate
// compares come from Windows payloads that differ in drive-letter case, and
// because the JS isProjectFile it ports folds case unconditionally.
func normalizeLedgerPath(p string) string {
	cleaned := gateCleanPath(p)
	if cleaned == "" {
		return ""
	}
	trimmed := strings.TrimRight(cleaned, "/")
	if trimmed == "" {
		trimmed = cleaned // a bare "/" is the root, not an empty path
	}
	return strings.ToLower(trimmed)
}

// ledgerString reads a string key, "" when absent or of another type.
func ledgerString(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// ledgerBool reads a bool key, false when absent or of another type.
func ledgerBool(m map[string]any, key string) bool {
	if b, ok := m[key].(bool); ok {
		return b
	}
	return false
}

// ledgerStrings reads a string-array key, dropping non-string members. JSON
// round-trips arrays as []any, so both shapes are accepted.
func ledgerStrings(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
