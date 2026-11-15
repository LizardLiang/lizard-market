package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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

// runSweepStop drives memory-sweep.cjs with a Stop payload against a scratch
// HOME holding the given marker and a transcript of 12 human and 30 assistant
// lines. It returns the hook's stdout and the marker left behind.
func runSweepStop(t *testing.T, marker map[string]any) (string, map[string]any) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	hook, err := filepath.Abs(filepath.Join(hooksDirPath(), "memory-sweep.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	root := t.TempDir()
	binName := "kratos"
	if runtime.GOOS == "windows" {
		binName = "kratos.exe"
	}
	for path, body := range map[string]string{
		filepath.Join(root, "bin", binName):                  "",
		filepath.Join(root, "references", "memory-sweep.md"): "rules",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	transcript := filepath.Join(t.TempDir(), "t.jsonl")
	var lines []string
	for i := 0; i < 12; i++ {
		lines = append(lines, `{"type":"user","message":{"content":"hi"}}`)
	}
	for i := 0; i < 30; i++ {
		lines = append(lines, `{"type":"assistant","message":{"content":"ok"}}`)
	}
	if err := os.WriteFile(transcript, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	markerFile := filepath.Join(home, ".kratos", "sweeps", "sess-a.json")
	if err := os.MkdirAll(filepath.Dir(markerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(marker)
	if err := os.WriteFile(markerFile, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{
		"session_id": "sess-a", "transcript_path": transcript, "hook_event_name": "Stop",
	})
	cmd := exec.Command(node, hook)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home,
		"CLAUDE_PLUGIN_ROOT="+root, "KRATOS_MEMORY_SWEEP=")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hook failed: %v", err)
	}
	after := map[string]any{}
	data, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &after); err != nil {
		t.Fatal(err)
	}
	return string(out), after
}

func sweepMarkerAt(sweeps int, age time.Duration) map[string]any {
	return map[string]any{
		"offset": 0, "human": 0, "assistant": 0, "sweeps": sweeps,
		"last_sweep_at": time.Now().Add(-age).UnixMilli(),
	}
}

// A session that outlives the per-window cap sweeps again once the last sweep
// is more than six hours old, and the count restarts at one.
func TestMemorySweepResumesAfterSixHoursPastCap(t *testing.T) {
	out, after := runSweepStop(t, sweepMarkerAt(8, 7*time.Hour))
	if !strings.Contains(out, "memory sweep") {
		t.Fatalf("expected a sweep instruction, got %q", out)
	}
	if got := after["sweeps"]; got != float64(1) {
		t.Fatalf("sweeps = %v, want 1", got)
	}
}

func TestMemorySweepStaysCappedInsideSixHours(t *testing.T) {
	out, after := runSweepStop(t, sweepMarkerAt(8, time.Hour))
	if out != "" {
		t.Fatalf("expected no sweep, got %q", out)
	}
	if got := after["sweeps"]; got != float64(8) {
		t.Fatalf("sweeps = %v, want 8", got)
	}
}

// A marker capped before last_sweep_at existed counts as elapsed and arms.
func TestMemorySweepCappedMarkerWithoutTimestampArms(t *testing.T) {
	m := sweepMarkerAt(8, 48*time.Hour)
	delete(m, "last_sweep_at")
	out, after := runSweepStop(t, m)
	if !strings.Contains(out, "memory sweep") {
		t.Fatalf("expected a sweep instruction, got %q", out)
	}
	if got := after["sweeps"]; got != float64(1) {
		t.Fatalf("sweeps = %v, want 1", got)
	}
	if _, ok := after["last_sweep_at"].(float64); !ok {
		t.Errorf("last_sweep_at not stamped: %v", after["last_sweep_at"])
	}
}
