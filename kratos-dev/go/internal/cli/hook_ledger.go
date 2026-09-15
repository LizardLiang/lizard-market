package cli

import (
	"encoding/json"
	"path"
	"strings"
	"time"

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

	// Session-ledger side effect first: it must not depend on the database
	// being reachable, because the edit gate reads it on every Write/Edit.
	recordInlineGod(input.SessionID, input.Cwd, input.Prompt)

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

// recordInlineGod keeps the session ledger's edit-gate fields current from the
// UserPromptSubmit payload. It is the only writer of inline_god and
// gate_bypass. Every failure is swallowed: a session with no readable ledger
// simply has no gate.
//
// Two prompt shapes matter:
//
//   - a launcher invocation (`/kratos:iris …`, or an expanded launcher body on
//     harnesses that send one) names the god now running inline;
//   - anything else the user typed is a new turn: the per-turn file budget
//     refills and gate_bypass is re-evaluated from the user's own words.
//
// A `<task-notification>` pseudo-prompt (Claude Code posts one when a spawned
// subagent finishes — verified on a real payload, 2026-09-11) is neither: it is
// not user text, so it must not grant or clear a bypass.
func recordInlineGod(sessionID, cwd, prompt string) {
	if sessionID == "" {
		return
	}
	god := inlineGodFromPrompt(prompt)
	userTurn := isUserTurnPrompt(prompt)
	if god == "" && !userTurn {
		return
	}

	changed := false
	m, err := readInlineLedger(sessionID)
	if err != nil || m == nil {
		// session-start.cjs has not written (or could not write) a ledger for
		// this session — start one, with the fields it would have set, from the
		// payload this hook already holds.
		m = newInlineLedger(sessionID, cwd)
		changed = true
	}

	if god != "" && ledgerString(m, ledgerKeyInlineGod) != god {
		// Relaunching the same god is not a way to refill the budget: only a
		// change of god resets the timestamp and the file list here.
		m[ledgerKeyInlineGod] = god
		m[ledgerKeyInlineSince] = now()
		m[ledgerKeyEditedFiles] = []string{}
		changed = true
	}
	if userTurn {
		if _, ok := m[ledgerKeyEditedFiles]; !ok || len(ledgerStrings(m)) > 0 {
			m[ledgerKeyEditedFiles] = []string{}
			changed = true
		}
		bypass := gateBypassRE.MatchString(prompt)
		if _, ok := m[ledgerKeyGateBypass]; !ok || ledgerBool(m) != bypass {
			m[ledgerKeyGateBypass] = bypass
			changed = true
		}
	}
	if !changed {
		// Nothing to store. Rewriting an identical ledger on every prompt is a
		// file replacement (and a temp file) for no reason.
		return
	}

	if err := writeInlineLedger(sessionID, m); err != nil {
		debugLog("ledger: inline god write failed: %v", err)
	}
}

// newInlineLedger is the stub written when a session has no ledger file yet. It
// carries the same fields hooks/session-start.cjs writes, from the payload the
// caller holds: a stub with only session_id dropped the cwd the gate's
// project-root test falls back to, plus the project name and start time the
// session notice reads.
func newInlineLedger(sessionID, cwd string) map[string]any {
	m := map[string]any{
		"session_id": sessionID,
		"started_at": time.Now().UnixMilli(),
		"source":     "prompt-submit",
	}
	if cwd != "" {
		m[ledgerKeyCwd] = cwd
		m["project"] = path.Base(strings.ReplaceAll(cwd, "\\", "/"))
	}
	return m
}

// inlineGodFromPrompt names the god a prompt launches inline, or "" when the
// prompt launches none. A slash command only counts when it resolves to a real
// agent definition, so /kratos:status or /kratos:main leave the field alone.
//
// Only the typed slash command counts. Matching `agent load <god> --resolve`
// anywhere in the prompt was measured dead (Claude Code 2.1.268 never delivers
// an expanded body here) and was live as an injection vector: any prompt
// quoting a launcher line — a review of this very file — rebound the session's
// inline god.
func inlineGodFromPrompt(prompt string) string {
	if m := slashGodRE.FindStringSubmatch(prompt); m != nil {
		name := strings.ToLower(m[1])
		if alias, ok := inlineGodAliases[name]; ok {
			name = alias
		}
		if isEmbeddedGod(name) {
			return name
		}
	}
	return ""
}

// isEmbeddedGod reports whether name has an agent definition in the embedded
// FS. ReadFile rather than Open+Close, which is how agent.go reads the same FS;
// an embedded read is a slice of a byte array already in the binary, so there
// is no I/O to save by not reading it.
func isEmbeddedGod(name string) bool {
	if name == "" {
		return false
	}
	_, err := agentsFS.ReadFile("agents/" + name + ".md")
	return err == nil
}

// isUserTurnPrompt reports whether the prompt is text the user typed, as
// opposed to an expanded launcher body or a harness notification.
//
// Only the <task-notification> pseudo-prompt is a harness event. Rejecting
// every prompt that opens with "<" also rejected the user's own text — an XML
// tag, a quoted snippet, "<br> renders wrong" — and such a turn silently kept
// the previous turn's spent budget and bypass.
func isUserTurnPrompt(prompt string) bool {
	p := strings.TrimSpace(prompt)
	if p == "" {
		return false
	}
	if strings.HasPrefix(p, "<task-notification") {
		return false
	}
	return !isExpandedLauncherBody(p)
}
