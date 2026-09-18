package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// marshalNoEscape JSON-encodes v the way Claude Code's own transcript writer
// does: real transcripts carry literal `<` and `>` inside a string value
// (verified against a real subagent transcript's `<agent-message from=…>`
// line), but Go's json.Marshal HTML-escapes them to `<`/`>` by
// default. Using the default encoder here would build a fixture the real
// scanner never sees in production.
func marshalNoEscape(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimRight(buf.String(), "\n")
}

// handbackLaunchLine builds a synthetic transcript line for one async child
// spawn — the shape scanHandbackTranscript actually scans for: the literal
// marker plus its "agentId: <id>" line, as Claude Code writes it into a
// tool_result.
func handbackLaunchLine(agentID string) string {
	return marshalNoEscape(map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []map[string]any{
				{
					"type":    "tool_result",
					"content": "Async agent launched successfully. (internal)\nagentId: " + agentID + " (internal id)",
				},
			},
		},
	})
}

// handbackReportLine builds a synthetic transcript line for one child's
// hand-back arriving in the parent's transcript.
func handbackReportLine(agentID string) string {
	return marshalNoEscape(map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": `<agent-message from="` + agentID + `">child report</agent-message>`,
		},
	})
}

// writeHandbackTranscript lays out the exact directory shape
// handbackTranscriptPath expects — <root>/<sessionID>/subagents/
// agent-<agentID>.jsonl — next to a stand-in main transcript file, and
// returns the main transcript's path (what the payload's transcript_path
// would carry).
func writeHandbackTranscript(t *testing.T, sessionID, agentID string, lines ...string) string {
	t.Helper()
	root := t.TempDir()
	mainTranscript := filepath.Join(root, sessionID+".jsonl")
	if err := os.WriteFile(mainTranscript, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(root, sessionID, "subagents")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(subDir, "agent-"+agentID+".jsonl")
	if err := os.WriteFile(agentPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return mainTranscript
}

func TestHandbackGateDecision(t *testing.T) {
	t.Run("3 launched, 1 reported: deny and name the missing two", func(t *testing.T) {
		mainTranscript := writeHandbackTranscript(t, "sess1", "hermesA",
			handbackLaunchLine("childA"),
			handbackLaunchLine("childB"),
			handbackLaunchLine("childC"),
			handbackReportLine("childA"),
		)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesA",
			SessionID:      "sess1",
			TranscriptPath: mainTranscript,
		})
		if res.Decision != "deny" {
			t.Fatalf("expected deny, got %q", res.Decision)
		}
		if !strings.Contains(res.Reason, "childB") || !strings.Contains(res.Reason, "childC") {
			t.Errorf("reason should name the missing children, got %q", res.Reason)
		}
		if strings.Contains(res.Reason, "childA") {
			t.Errorf("reason should not name a child that already reported, got %q", res.Reason)
		}
	})

	t.Run("3 launched, 3 reported: no decision", func(t *testing.T) {
		mainTranscript := writeHandbackTranscript(t, "sess2", "hermesB",
			handbackLaunchLine("childA"),
			handbackLaunchLine("childB"),
			handbackLaunchLine("childC"),
			handbackReportLine("childA"),
			handbackReportLine("childB"),
			handbackReportLine("childC"),
		)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesB",
			SessionID:      "sess2",
			TranscriptPath: mainTranscript,
		})
		if res.Decision != "" {
			t.Fatalf("expected no decision once every child reported, got %q: %s", res.Decision, res.Reason)
		}
	})

	t.Run("non-Hermes agent: no decision regardless of transcript", func(t *testing.T) {
		mainTranscript := writeHandbackTranscript(t, "sess3", "aresA",
			handbackLaunchLine("childA"),
		)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "general-purpose",
			AgentID:        "aresA",
			SessionID:      "sess3",
			TranscriptPath: mainTranscript,
		})
		if res.Decision != "" {
			t.Fatalf("expected no decision for a non-Hermes agent, got %q", res.Decision)
		}
	})

	t.Run("unreadable transcript: fails open", func(t *testing.T) {
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesC",
			SessionID:      "sess4",
			TranscriptPath: filepath.Join(t.TempDir(), "does-not-exist.jsonl"),
		})
		if res.Decision != "" {
			t.Fatalf("expected fail-open (no decision) for an unreadable transcript, got %q", res.Decision)
		}
	})

	t.Run("wrong tool name: no decision", func(t *testing.T) {
		res := handbackGateDecision(preToolUseInput{
			ToolName:  "Write",
			AgentType: "hermes",
			AgentID:   "hermesD",
			SessionID: "sess5",
		})
		if res.Decision != "" {
			t.Fatalf("expected no decision for a non-SubagentHandback tool, got %q", res.Decision)
		}
	})

	t.Run("missing session id: fails open before touching disk", func(t *testing.T) {
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesE",
			TranscriptPath: "/some/path/sess.jsonl",
		})
		if res.Decision != "" {
			t.Fatalf("expected no decision with no session id, got %q", res.Decision)
		}
	})
}
