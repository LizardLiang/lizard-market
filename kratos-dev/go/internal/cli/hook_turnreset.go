package cli

import (
	"encoding/json"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// resetTurnFields applies the per-turn resets a new user message triggers in
// recordInlineGod: the inline god's file budget refills and the review/fix
// round counters start over. Returns whether the ledger changed.
func resetTurnFields(m map[string]any) bool {
	changed := false
	if _, ok := m[ledgerKeyEditedFiles]; !ok || len(ledgerStrings(m)) > 0 {
		m[ledgerKeyEditedFiles] = []string{}
		changed = true
	}
	if len(ledgerTurnSpawns(m)) > 0 {
		m[ledgerKeyTurnSpawns] = map[string]int{}
		changed = true
	}
	return changed
}

// turnResetInput is the PostToolUse payload the turn-reset hook reads.
type turnResetInput struct {
	ToolName  string `json:"tool_name"`
	SessionID string `json:"session_id"`
	AgentType string `json:"agent_type"`
	AgentID   string `json:"agent_id"`
}

// resetTurnLedger treats an answered AskUserQuestion as a user turn. The user
// typed that answer, but it never passes through UserPromptSubmit, so
// recordInlineGod never saw it: on 2026-10-20 23:25 the user picked "Run
// Hermes round 2" in the question box and the spawn cap denied the round
// with "an answer in the question box doesn't count as a new message — send
// 'go'". Only the main context's AskUserQuestion counts; a spawned agent's
// payload carries agent_type/agent_id and is ignored.
func resetTurnLedger(input turnResetInput) {
	if input.SessionID == "" || input.ToolName != "AskUserQuestion" {
		return
	}
	if input.AgentType != "" || input.AgentID != "" {
		return
	}
	m, err := readInlineLedger(input.SessionID)
	if err != nil || m == nil {
		return
	}
	if !resetTurnFields(m) {
		return
	}
	if err := writeInlineLedger(input.SessionID, m); err != nil {
		debugLog("turn-reset: ledger write failed: %v", err)
	}
}

func turnResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "turn-reset",
		Short: "PostToolUse(AskUserQuestion) — an answered question is a user turn: refill the inline file budget and the review-round counters",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				debugLog("turn-reset: stdin read error: %v", err)
				return nil
			}
			var input turnResetInput
			if err := json.Unmarshal(raw, &input); err != nil {
				debugLog("turn-reset: json parse error: %v", err)
				return nil
			}
			resetTurnLedger(input)
			return nil
		},
	}
}
