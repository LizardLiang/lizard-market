package cli

import (
	"encoding/json"
	"strings"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
)

// recordPromptLedger is the UserPromptSubmit hook's side effect on the session
// ledger: it makes sure a row exists for this Claude Code session id (keyed by
// the harness id, so `step record-agent` can never hit a missing foreign key)
// and stores the first real prompt as initial_request. Every failure is
// swallowed — the ledger must never change or block the hook's output.
func recordPromptLedger(raw []byte) {
	var input hookInput
	if err := json.Unmarshal(raw, &input); err != nil || input.SessionID == "" {
		return
	}

	conn, err := db.GetConnection()
	if err != nil {
		debugLog("ledger: db unavailable: %v", err)
		return
	}
	defer conn.Close()
	if err := db.InitDB(conn); err != nil {
		debugLog("ledger: init failed: %v", err)
		return
	}

	project := ""
	if input.Cwd != "" {
		project = normalizeProjectPath(input.Cwd)
	}
	if _, _, err := db.EnsureSession(conn, input.SessionID, project); err != nil {
		debugLog("ledger: ensure session failed: %v", err)
		return
	}

	if prompt := initialRequestText(input.Prompt); prompt != "" {
		if err := db.SetInitialRequestIfEmpty(conn, input.SessionID, prompt); err != nil {
			debugLog("ledger: initial request failed: %v", err)
		}
	}
}

// initialRequestText returns the prompt text worth storing as a session's
// initial request, or "" for slash-command echoes, expanded launcher bodies,
// and other non-request input.
func initialRequestText(prompt string) string {
	p := strings.TrimSpace(prompt)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "!") || strings.HasPrefix(p, "<") {
		return ""
	}
	if strings.Contains(p, "KRATOS_ROOT=") || strings.Contains(p, "agent load ") {
		return ""
	}
	return p
}
