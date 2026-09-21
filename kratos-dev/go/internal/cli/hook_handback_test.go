package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rfc3339 formats a timestamp the way every real Claude Code transcript line
// carries its own: RFC 3339 with fractional seconds, "Z" suffix.
func rfc3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// fixHandbackClock points handbackNow at a fixed instant for the life of the
// test, restoring the real clock on cleanup.
func fixHandbackClock(t *testing.T, when time.Time) {
	t.Helper()
	prev := handbackNow
	handbackNow = func() time.Time { return when }
	t.Cleanup(func() { handbackNow = prev })
}

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
// exactly as the real transcript carries both. Timestamped "now": tests that
// do not care about the stall clock stay well inside handbackStallTimeout.
func handbackLaunchResultLine(toolUseID, agentID string) string {
	return handbackLaunchResultLineAt(toolUseID, agentID, time.Now())
}

// handbackLaunchResultLineAt is handbackLaunchResultLine with an explicit
// timestamp, for the stall-timeout tests that control the clock precisely.
func handbackLaunchResultLineAt(toolUseID, agentID string, ts time.Time) string {
	b, _ := json.Marshal(map[string]any{
		"type":      "user",
		"timestamp": rfc3339(ts),
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
// tag, and attachment.origin names the same id structurally. Timestamped
// "now" — see handbackLaunchResultLine.
func handbackReportLine(agentID string) string {
	return handbackReportLineAt(agentID, time.Now())
}

// handbackReportLineAt is handbackReportLine with an explicit timestamp.
func handbackReportLineAt(agentID string, ts time.Time) string {
	b, _ := json.Marshal(map[string]any{
		"type":      "attachment",
		"timestamp": rfc3339(ts),
		"attachment": map[string]any{
			"type":   "queued_command",
			"prompt": `<agent-message from="` + agentID + `">child report</agent-message>`,
			"origin": map[string]any{"kind": "peer", "from": agentID, "handback": true},
		},
	})
	return string(b)
}

// handbackUserOriginReportLine builds the OTHER real report shape: a
// top-level `origin` on a type:"user" entry whose message content is a plain
// string carrying the hand-back wrapper — 2 of 3 real reports in the 09-17
// NETZERO transcript took this shape, not the attachment shape above. The
// body text deliberately carries no `<agent-message from=` literal, so a
// case using this fixture alone proves the structured origin path, not the
// prose fallback, is what finishes the child.
func handbackUserOriginReportLine(agentID string, ts time.Time) string {
	b, _ := json.Marshal(map[string]any{
		"type":      "user",
		"timestamp": rfc3339(ts),
		"origin":    map[string]any{"kind": "peer", "from": agentID, "senderTaskId": agentID, "handback": true},
		"message": map[string]any{
			"content": "Another Claude session sent a message while you were working:\nchild report body, no literal tag here",
		},
	})
	return string(b)
}

// handbackArrayPromptReportLine builds the same attachment report shape as
// handbackReportLine, but with `prompt` encoded as a content-block array
// instead of a plain string — a real shape a queued_command attachment can
// carry. Before Prompt was typed json.RawMessage, a string-typed field made
// the whole line fail json.Unmarshal, silently dropping this attachment's
// origin.handback along with it.
func handbackArrayPromptReportLine(agentID string, ts time.Time) string {
	b, _ := json.Marshal(map[string]any{
		"type":      "attachment",
		"timestamp": rfc3339(ts),
		"attachment": map[string]any{
			"type": "queued_command",
			"prompt": []map[string]any{
				{"type": "text", "text": `<agent-message from="` + agentID + `">child report</agent-message>`},
			},
			"origin": map[string]any{"kind": "peer", "from": agentID, "handback": true},
		},
	})
	return string(b)
}

// handbackTaskNotificationLine builds the attachment entry Claude Code sends
// when a child agent stops, for any status — completed, failed, or anything
// else. A dead child never sends an <agent-message>, only this. Timestamped
// "now" — see handbackLaunchResultLine.
func handbackTaskNotificationLine(taskID, status string) string {
	return handbackTaskNotificationLineAt(taskID, status, time.Now())
}

// handbackTaskNotificationLineAt is handbackTaskNotificationLine with an
// explicit timestamp.
func handbackTaskNotificationLineAt(taskID, status string, ts time.Time) string {
	b, _ := json.Marshal(map[string]any{
		"type":      "attachment",
		"timestamp": rfc3339(ts),
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

// threeLaunchLines builds the six lines a real Hermes fan-out writes for
// three children: one assistant tool_use plus one tool_result per child.
func threeLaunchLines(toolUseA, toolUseB, toolUseC, childA, childB, childC string) []string {
	return threeLaunchLinesAt(time.Now(), toolUseA, toolUseB, toolUseC, childA, childB, childC)
}

// threeLaunchLinesAt is threeLaunchLines with every launch stamped at the
// same explicit time, for the stall-timeout tests.
func threeLaunchLinesAt(ts time.Time, toolUseA, toolUseB, toolUseC, childA, childB, childC string) []string {
	return []string{
		handbackAssistantToolUseLine(toolUseA, "Agent"),
		handbackLaunchResultLineAt(toolUseA, childA, ts),
		handbackAssistantToolUseLine(toolUseB, "Agent"),
		handbackLaunchResultLineAt(toolUseB, childB, ts),
		handbackAssistantToolUseLine(toolUseC, "Agent"),
		handbackLaunchResultLineAt(toolUseC, childC, ts),
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

// TestHandbackGateStallTimeout covers the stateless redesign (Part 3a): no
// state file, no denial counter — the gate denies while children are
// outstanding unless the transcript itself shows no launch or finish event
// for handbackStallTimeout, measured against the injected clock.
func TestHandbackGateStallTimeout(t *testing.T) {
	t.Run("3 launched, 0 finished, last progress 1 minute ago: deny on ten consecutive calls", func(t *testing.T) {
		ref := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
		launchedAt := ref.Add(-1 * time.Minute)
		mainTranscript := writeHandbackTranscript(t, "sess9", "hermesI",
			threeLaunchLinesAt(launchedAt, "tu1", "tu2", "tu3", "childA", "childB", "childC")...,
		)
		fixHandbackClock(t, ref)
		input := preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesI",
			SessionID:      "sess9",
			TranscriptPath: mainTranscript,
			Cwd:            filepath.Dir(mainTranscript),
		}

		for i := 1; i <= 10; i++ {
			res := handbackGateDecision(input)
			if res.Decision != "deny" {
				t.Fatalf("call %d: expected deny (1 minute since last progress), got %q", i, res.Decision)
			}
		}
	})

	t.Run("same transcript, clock 31 minutes past last progress: no deny, additionalContext names the three ids", func(t *testing.T) {
		ref := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
		launchedAt := ref.Add(-1 * time.Minute)
		mainTranscript := writeHandbackTranscript(t, "sess9b", "hermesI2",
			threeLaunchLinesAt(launchedAt, "tu1", "tu2", "tu3", "childA", "childB", "childC")...,
		)
		fixHandbackClock(t, launchedAt.Add(31*time.Minute))
		input := preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesI2",
			SessionID:      "sess9b",
			TranscriptPath: mainTranscript,
			Cwd:            filepath.Dir(mainTranscript),
		}

		res := handbackGateDecision(input)
		if res.Decision != "" {
			t.Fatalf("expected no decision past the stall timeout, got %q: %s", res.Decision, res.Reason)
		}
		for _, id := range []string{"childA", "childB", "childC"} {
			if !strings.Contains(res.AdditionalContext, id) {
				t.Errorf("additionalContext should name %s as missing, got %q", id, res.AdditionalContext)
			}
		}
		if !strings.Contains(res.AdditionalContext, "parent-only") {
			t.Errorf("additionalContext should say the missing tiers are parent-only, got %q", res.AdditionalContext)
		}
	})

	t.Run("3 launched, 1 finished 2 minutes ago: deny even when the launch was 40 minutes ago", func(t *testing.T) {
		ref := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
		launchedAt := ref.Add(-40 * time.Minute)
		finishedAt := ref.Add(-2 * time.Minute)
		lines := append(threeLaunchLinesAt(launchedAt, "tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackReportLineAt("childA", finishedAt),
		)
		mainTranscript := writeHandbackTranscript(t, "sess9c", "hermesI3", lines...)
		fixHandbackClock(t, ref)
		input := preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesI3",
			SessionID:      "sess9c",
			TranscriptPath: mainTranscript,
			Cwd:            filepath.Dir(mainTranscript),
		}

		res := handbackGateDecision(input)
		if res.Decision != "deny" {
			t.Fatalf("expected deny — the finish 2 minutes ago is progress, not the 40-minute-old launch, got %q", res.Decision)
		}
		if strings.Contains(res.Reason, "childA") {
			t.Errorf("reason should not name the child that already reported, got %q", res.Reason)
		}
		for _, id := range []string{"childB", "childC"} {
			if !strings.Contains(res.Reason, id) {
				t.Errorf("reason should name %s as missing, got %q", id, res.Reason)
			}
		}
	})

	t.Run("all finished: no output at all, regardless of how stale the launch is", func(t *testing.T) {
		ref := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
		launchedAt := ref.Add(-90 * time.Minute)
		lines := append(threeLaunchLinesAt(launchedAt, "tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackReportLineAt("childA", ref.Add(-70*time.Minute)),
			handbackReportLineAt("childB", ref.Add(-60*time.Minute)),
			handbackReportLineAt("childC", ref.Add(-50*time.Minute)),
		)
		mainTranscript := writeHandbackTranscript(t, "sess9d", "hermesI4", lines...)
		fixHandbackClock(t, ref)
		input := preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesI4",
			SessionID:      "sess9d",
			TranscriptPath: mainTranscript,
			Cwd:            filepath.Dir(mainTranscript),
		}

		res := handbackGateDecision(input)
		if res.Decision != "" || res.AdditionalContext != "" {
			t.Fatalf("expected no output at all once every child has reported, got decision %q, context %q", res.Decision, res.AdditionalContext)
		}
	})

	t.Run("no usable timestamp anywhere: fails open", func(t *testing.T) {
		mainTranscript := writeHandbackTranscript(t, "sess9e", "hermesI5",
			handbackAssistantToolUseLine("tu1", "Agent"),
			// A launch line with no timestamp field at all.
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":[{"type":"text","text":"Async agent launched successfully.\nagentId: childA"}]}]},"toolUseResult":{"agentId":"childA"}}`,
		)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesI5",
			SessionID:      "sess9e",
			TranscriptPath: mainTranscript,
			Cwd:            filepath.Dir(mainTranscript),
		})
		if res.Decision != "" {
			t.Fatalf("expected fail-open with no usable timestamp, got %q", res.Decision)
		}
	})
}

// TestHandbackGateStructuredFinishShapes covers Part 3c and 3d: the
// top-level `origin` shape on a type:"user" entry, and an attachment whose
// `prompt` is a content-block array rather than a plain string.
func TestHandbackGateStructuredFinishShapes(t *testing.T) {
	t.Run("top-level origin on a user entry finishes the child, no <agent-message> literal needed", func(t *testing.T) {
		now := time.Now()
		lines := append(threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackUserOriginReportLine("childA", now),
			handbackUserOriginReportLine("childB", now),
			handbackUserOriginReportLine("childC", now),
		)
		for _, l := range lines {
			if strings.Contains(l, `<agent-message from=`) {
				t.Fatalf("fixture must carry no <agent-message from= literal, got line: %s", l)
			}
		}
		mainTranscript := writeHandbackTranscript(t, "sess11", "hermesK", lines...)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesK",
			SessionID:      "sess11",
			TranscriptPath: mainTranscript,
		})
		if res.Decision != "" {
			t.Fatalf("expected no decision — the structured origin alone finishes every child, got %q: %s", res.Decision, res.Reason)
		}
	})

	t.Run("an attachment with an array-shaped prompt still counts as finished", func(t *testing.T) {
		now := time.Now()
		lines := append(threeLaunchLines("tu1", "tu2", "tu3", "childA", "childB", "childC"),
			handbackArrayPromptReportLine("childA", now),
			handbackReportLine("childB"),
			handbackReportLine("childC"),
		)
		mainTranscript := writeHandbackTranscript(t, "sess12", "hermesL", lines...)
		res := handbackGateDecision(preToolUseInput{
			ToolName:       "SubagentHandback",
			AgentType:      "hermes",
			AgentID:        "hermesL",
			SessionID:      "sess12",
			TranscriptPath: mainTranscript,
		})
		if res.Decision != "" {
			t.Fatalf("expected no decision — an array-shaped prompt must not fail the whole line's unmarshal, got %q: %s", res.Decision, res.Reason)
		}
	})
}
