package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Fixture line builders ------------------------------------------------
//
// Every builder mirrors a real shape observed in a live Hermes transcript
// (agent-ae8cddbbbfd689d03.jsonl, GSS_VITAL-NETZERO2604, 2026-09-17 — read
// only, never copied into the repo verbatim; no customer text below).

// handbackAssistantToolUseLine builds an assistant entry whose message
// carries one tool_use block — the shape a Task/Agent spawn, or an ordinary
// Read/Grep call, both look like.
func handbackAssistantToolUseLine(toolUseID, name string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []map[string]any{
				{"type": "tool_use", "id": toolUseID, "name": name, "input": map[string]any{}},
			},
		},
	})
	return string(b)
}

// handbackLaunchResultLine builds the tool_result Claude Code returns for an
// async Agent/Task spawn: the marker text plus the child's id, both in the
// tool_result's own nested content and in the line's toolUseResult field —
// exactly as the real transcript carries both.
func handbackLaunchResultLine(toolUseID, agentID string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []map[string]any{
				{
					"type":        "tool_result",
					"tool_use_id": toolUseID,
					"content": []map[string]any{
						{"type": "text", "text": "Async agent launched successfully. (internal)\nagentId: " + agentID + " (internal id)"},
					},
				},
			},
		},
		"toolUseResult": map[string]any{"isAsync": true, "agentId": agentID},
	})
	return string(b)
}

// handbackReadResultLine builds an ordinary tool_result (a Read or Grep, not
// an Agent/Task spawn) whose text happens to quote the two literal markers —
// the Defect 1 exploit: this must never be counted as a real launch or
// report, because its tool_use_id never answers an Agent/Task tool_use.
func handbackReadResultLine(toolUseID string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []map[string]any{
				{
					"type":        "tool_result",
					"tool_use_id": toolUseID,
					"content": []map[string]any{
						{"type": "text", "text": "// const handbackAsyncLaunchMarker = \"Async agent launched successfully\"\n// matches <agent-message from=\"not-a-real-child\">"},
					},
				},
			},
		},
	})
	return string(b)
}

// handbackReportLine builds the attachment entry a child's SubagentHandback
// arrives as: attachment.prompt carries the literal <agent-message from="…">
// tag, and attachment.origin names the same id structurally.
func handbackReportLine(agentID string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "attachment",
		"attachment": map[string]any{
			"type":   "queued_command",
			"prompt": `<agent-message from="` + agentID + `">child report</agent-message>`,
			"origin": map[string]any{"kind": "peer", "from": agentID, "handback": true},
		},
	})
	return string(b)
}

// handbackTaskNotificationLine builds the attachment entry Claude Code sends
// when a child agent stops, for any status — completed, failed, or anything
// else. A dead child never sends an <agent-message>, only this.
func handbackTaskNotificationLine(taskID, status string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "attachment",
		"attachment": map[string]any{
			"type":        "queued_command",
			"commandMode": "task-notification",
			"prompt":      "<task-notification>\n<task-id>" + taskID + "</task-id>\n<status>" + status + "</status>\n</task-notification>",
		},
	})
	return string(b)
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

// rewriteHandbackTranscript overwrites an already-laid-out transcript file
// (same path shape as writeHandbackTranscript) with new lines, for tests that
// simulate the transcript growing between two handbackGateDecision calls.
func rewriteHandbackTranscript(t *testing.T, mainTranscript, sessionID, agentID string, lines ...string) {
	t.Helper()
	agentPath := filepath.Join(filepath.Dir(mainTranscript), sessionID, "subagents", "agent-"+agentID+".jsonl")
	if err := os.WriteFile(agentPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// threeLaunchLines builds the six lines a real Hermes fan-out writes for
// three children: one assistant tool_use plus one tool_result per child.
func threeLaunchLines(toolUseA, toolUseB, toolUseC, childA, childB, childC string) []string {
	return []string{
		handbackAssistantToolUseLine(toolUseA, "Agent"),
		handbackLaunchResultLine(toolUseA, childA),
		handbackAssistantToolUseLine(toolUseB, "Agent"),
		handbackLaunchResultLine(toolUseB, childB),
		handbackAssistantToolUseLine(toolUseC, "Agent"),
		handbackLaunchResultLine(toolUseC, childC),
	}
}

func TestHandbackGateDecision(t *testing.T) {
	t.Run("3 launched, 1 reported: deny and name the missing two", func(t *testing.T) {
		lines := append(threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackReportLine("childA"),
		)
		mainTranscript := writeHandbackTranscript(t, "sess1", "hermesA", lines...)
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
		lines := append(threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackReportLine("childA"),
			handbackReportLine("childB"),
			handbackReportLine("childC"),
		)
		mainTranscript := writeHandbackTranscript(t, "sess2", "hermesB", lines...)
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
			handbackAssistantToolUseLine("tu1", "Agent"),
			handbackLaunchResultLine("tu1", "childA"),
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

	// Defect 1: a Read/Grep result that merely quotes the two literal markers
	// in prose must never be mistaken for a real launch or a real report.
	t.Run("Read result quoting both literals plus 3 real launches and 3 real reports: no decision", func(t *testing.T) {
		lines := []string{
			handbackAssistantToolUseLine("tuRead", "Read"),
			handbackReadResultLine("tuRead"),
		}
		lines = append(lines, threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC")...)
		lines = append(lines,
			handbackReportLine("childA"),
			handbackReportLine("childB"),
			handbackReportLine("childC"),
		)
		mainTranscript := writeHandbackTranscript(t, "sess6", "hermesF", lines...)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesF",
			SessionID:      "sess6",
			TranscriptPath: mainTranscript,
		})
		if res.Decision != "" {
			t.Fatalf("expected no decision — the quoted Read result must not inflate launched or finished, got %q: %s", res.Decision, res.Reason)
		}
	})

	// Defect 2: a child that dies still sends a task-notification, never an
	// <agent-message>. That notification must count as finished regardless
	// of its <status>.
	t.Run("3 launches, 2 reports, 1 failed task-notification: no decision", func(t *testing.T) {
		lines := append(threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackReportLine("childA"),
			handbackReportLine("childB"),
			handbackTaskNotificationLine("childC", "failed"),
		)
		mainTranscript := writeHandbackTranscript(t, "sess7", "hermesG", lines...)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesG",
			SessionID:      "sess7",
			TranscriptPath: mainTranscript,
		})
		if res.Decision != "" {
			t.Fatalf("expected no decision — a failed child's task-notification still finishes it, got %q: %s", res.Decision, res.Reason)
		}
	})

	// An <agent-message from="X"> for an id this transcript never launched
	// must not count toward any of the real children finishing.
	t.Run("agent-message for an unlaunched id does not count", func(t *testing.T) {
		lines := append(threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackReportLine("some-other-agent-not-launched-here"),
		)
		mainTranscript := writeHandbackTranscript(t, "sess8", "hermesH", lines...)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesH",
			SessionID:      "sess8",
			TranscriptPath: mainTranscript,
		})
		if res.Decision != "deny" {
			t.Fatalf("expected deny — the unrelated report must not finish any real child, got %q", res.Decision)
		}
		for _, id := range []string{"childA", "childB", "childC"} {
			if !strings.Contains(res.Reason, id) {
				t.Errorf("reason should still name %s as missing, got %q", id, res.Reason)
			}
		}
	})
}

func TestHandbackGateBound(t *testing.T) {
	t.Run("3 launched, 1 reported: denies gateMaxBlocks times with no progress, then no decision", func(t *testing.T) {
		lines := append(threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackReportLine("childA"),
		)
		mainTranscript := writeHandbackTranscript(t, "sess9", "hermesI", lines...)
		cwd := filepath.Dir(mainTranscript)
		input := preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesI",
			SessionID:      "sess9",
			TranscriptPath: mainTranscript,
			Cwd:            cwd,
		}

		for i := 1; i <= gateMaxBlocks; i++ {
			res := handbackGateDecision(input)
			if res.Decision != "deny" {
				t.Fatalf("call %d: expected deny (no progress since last denial), got %q", i, res.Decision)
			}
		}
		// One more call, still zero progress: the bound has been reached.
		res := handbackGateDecision(input)
		if res.Decision != "" {
			t.Fatalf("call %d: expected no decision once the bound is reached, got %q", gateMaxBlocks+1, res.Decision)
		}
	})

	t.Run("progress between denials restarts the counter — never trips the bound", func(t *testing.T) {
		mainTranscript := writeHandbackTranscript(t, "sess10", "hermesJ",
			threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC")...,
		)
		cwd := filepath.Dir(mainTranscript)
		input := preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesJ",
			SessionID:      "sess10",
			TranscriptPath: mainTranscript,
			Cwd:            cwd,
		}

		// Denial 1: 0 finished.
		res := handbackGateDecision(input)
		if res.Decision != "deny" {
			t.Fatalf("denial at 0 finished: expected deny, got %q", res.Decision)
		}

		// Denial 2: 1 finished — progress since the last denial, so the
		// counter must restart rather than advance toward the bound.
		lines := append(threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackReportLine("childA"),
		)
		rewriteHandbackTranscript(t, mainTranscript, "sess10", "hermesJ", lines...)
		res = handbackGateDecision(input)
		if res.Decision != "deny" {
			t.Fatalf("denial at 1 finished: expected deny (progress resets the bound), got %q", res.Decision)
		}

		// Denial 3: 2 finished — progress again.
		lines = append(lines, handbackReportLine("childB"))
		rewriteHandbackTranscript(t, mainTranscript, "sess10", "hermesJ", lines...)
		res = handbackGateDecision(input)
		if res.Decision != "deny" {
			t.Fatalf("denial at 2 finished: expected deny (progress resets the bound), got %q", res.Decision)
		}
	})
}
