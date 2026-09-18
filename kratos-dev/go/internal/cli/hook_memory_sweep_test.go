package cli

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// runCountMessages calls memory-sweep.cjs's countMessages(text) through node,
// the same node-exec pattern runBuildMemoryReport uses for session-start.cjs.
func runCountMessages(t *testing.T, text string) (human, assistant int) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	hook, err := filepath.Abs(filepath.Join(hooksDirPath(), "memory-sweep.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	script := "const { countMessages } = require(" + strconv.Quote(filepath.ToSlash(hook)) + ");\n" +
		"process.stdout.write(JSON.stringify(countMessages(process.argv[1])));\n"
	out, err := exec.Command(node, "-e", script, text).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("node run failed: %v (%s)", err, ee.Stderr)
		}
		t.Fatalf("node run failed: %v", err)
	}
	var counts struct {
		Human     int `json:"human"`
		Assistant int `json:"assistant"`
	}
	if err := json.Unmarshal(out, &counts); err != nil {
		t.Fatalf("cannot parse node output %q: %v", out, err)
	}
	return counts.Human, counts.Assistant
}

// TestMemorySweepCountMessagesSkipsHarnessPseudoPrompts pins the same defect
// class Part 2 fixed on the Go side: isSystemPrompt knew a bare
// <task-notification> but not the "[SYSTEM NOTIFICATION" preamble Claude
// Code 2.1.276 prepends to it, or the "Another Claude session sent a
// message" hand-back wrapper, so both counted as human turns in
// countMessages and could arm a sweep over a stretch the human never typed
// into.
func TestMemorySweepCountMessagesSkipsHarnessPseudoPrompts(t *testing.T) {
	real := `{"type":"user","message":{"content":"a real question from the human"}}`
	barePseudo := `{"type":"user","message":{"content":"<task-notification>\n<task-id>abc</task-id>\n</task-notification>"}}`
	preamblePseudo := `{"type":"user","message":{"content":"[SYSTEM NOTIFICATION - NOT USER INPUT]\nThis is an automated background-task event, NOT a message from the user.\n\n<task-notification>\n<task-id>abc</task-id>\n</task-notification>"}}`
	handbackPseudo := `{"type":"user","message":{"content":"Another Claude session sent a message while you were working:\n<agent-message from=\"a\">Done.</agent-message>"}}`
	assistantLine := `{"type":"assistant","message":{"content":[{"type":"text","text":"ok"}]}}`

	text := real + "\n" + barePseudo + "\n" + preamblePseudo + "\n" + handbackPseudo + "\n" + assistantLine

	human, assistant := runCountMessages(t, text)
	if human != 1 {
		t.Errorf("human = %d, want 1 (only the real question) — a harness pseudo-prompt counted as a human turn", human)
	}
	if assistant != 1 {
		t.Errorf("assistant = %d, want 1", assistant)
	}
}
