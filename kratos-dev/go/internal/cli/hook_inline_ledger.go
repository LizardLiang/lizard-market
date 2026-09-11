package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
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
// whole file.
func writeInlineLedger(sessionID string, m map[string]any) error {
	path := sessionLedgerFile(sessionID)
	if path == "" {
		return os.ErrNotExist
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// normalizeLedgerPath canonicalizes a file path for list membership, using the
// same rule as stored project paths (slashes, no trailing slash, lower case on
// Windows).
func normalizeLedgerPath(p string) string {
	return normalizeProjectPath(p)
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

// nowRFC3339 is the timestamp format the ledger stores, in local time with an
// offset — the same shape `kratos now` prints.
func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
