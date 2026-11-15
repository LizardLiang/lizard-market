package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func waiverJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func waiverMission(t *testing.T, text string) string {
	return waiverJSON(t, map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}})
}

func waiverCoordinator(t *testing.T, text string) string {
	return waiverJSON(t, map[string]any{"type": "attachment", "attachment": map[string]any{
		"type": "queued_command", "prompt": text, "origin": map[string]any{"kind": "coordinator"}}})
}

func waiverAssistant(t *testing.T, text string) string {
	return waiverJSON(t, map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant",
		"content": []map[string]any{{"type": "text", "text": text}}}})
}

func waiverHuman(t *testing.T, text string) string {
	return waiverJSON(t, map[string]any{"type": "user", "origin": map[string]any{"kind": "human"},
		"message": map[string]any{"role": "user", "content": []map[string]any{{"type": "text", "text": text}}}})
}

// waiverInput writes a parent transcript and an Ares transcript under one temp
// dir and returns the SubagentStop input that points at both.
func waiverInput(t *testing.T, parent, agent []string) subagentStopInput {
	t.Helper()
	dir := t.TempDir()
	main := filepath.Join(dir, "sessW.jsonl")
	if err := os.WriteFile(main, []byte(strings.Join(parent, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sessW", "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "agent-aW.jsonl"), []byte(strings.Join(agent, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return subagentStopInput{TranscriptPath: main, SessionID: "sessW", AgentID: "aW", Cwd: t.TempDir()}
}

const waiverLine = `USER-WAIVER: speed — "skip all unit test, just direct edit"`

func TestAresUserWaiver(t *testing.T) {
	human := waiverHuman(t, "please skip all unit test, just direct edit. Thanks")
	filesOnly := "Changed files:\n- src/app/formula.cs\n- src/app/formula.ts\n"

	t.Run("mission line grants the waiver", func(t *testing.T) {
		in := waiverInput(t, []string{human}, []string{waiverMission(t, "MISSION: x\n"+waiverLine)})
		if got := aresReportFailures(filesOnly, in); len(got) != 0 {
			t.Fatalf("failures = %v, want none", got)
		}
	})
	t.Run("coordinator message grants the waiver", func(t *testing.T) {
		in := waiverInput(t, []string{human}, []string{waiverMission(t, "MISSION: x"), waiverCoordinator(t, waiverLine)})
		if got := aresReportFailures(filesOnly, in); len(got) != 0 {
			t.Fatalf("failures = %v, want none", got)
		}
	})
	t.Run("same report without a waiver fails", func(t *testing.T) {
		in := waiverInput(t, []string{human}, []string{waiverMission(t, "MISSION: x")})
		if got := aresReportFailures(filesOnly, in); len(got) == 0 {
			t.Fatal("want failures without a waiver")
		}
	})
	t.Run("waiver only in the report of Ares is refused", func(t *testing.T) {
		in := waiverInput(t, []string{human}, []string{waiverMission(t, "MISSION: x"), waiverAssistant(t, waiverLine)})
		if got := aresReportFailures(filesOnly+waiverLine+"\n", in); len(got) == 0 {
			t.Fatal("want failures: Ares cannot grant its own waiver")
		}
	})
	t.Run("quote missing from the human prompts is refused", func(t *testing.T) {
		in := waiverInput(t, []string{waiverHuman(t, "please add tests for everything")}, []string{waiverMission(t, "MISSION: x\n"+waiverLine)})
		if got := aresReportFailures(filesOnly, in); len(got) == 0 {
			t.Fatal("want failures for an invented quote")
		}
	})
	t.Run("quote in a non-human parent entry is refused", func(t *testing.T) {
		notHuman := waiverJSON(t, map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "skip all unit test, just direct edit"}})
		in := waiverInput(t, []string{waiverHuman(t, "other words"), notHuman}, []string{waiverMission(t, "MISSION: x\n"+waiverLine)})
		if got := aresReportFailures(filesOnly, in); len(got) == 0 {
			t.Fatal("want failures when no human prompt holds the quote")
		}
	})
	t.Run("unreadable parent transcript fails open", func(t *testing.T) {
		in := waiverInput(t, []string{human}, []string{waiverMission(t, "MISSION: x\n"+waiverLine)})
		in.TranscriptPath = filepath.Join(filepath.Dir(in.TranscriptPath), "sessW.jsonl")
		if err := os.Remove(in.TranscriptPath); err != nil {
			t.Fatal(err)
		}
		// Removing the parent also leaves the sibling path derivation intact.
		if got := aresReportFailures(filesOnly, in); len(got) != 0 {
			t.Fatalf("failures = %v, want none", got)
		}
	})
	t.Run("no-code report passes under a waiver", func(t *testing.T) {
		in := waiverInput(t, []string{human}, []string{waiverMission(t, "MISSION: x\n"+waiverLine)})
		rep := "LANDED-NOT-APPLICABLE: read-only\nReach: not applicable — no code changed\n"
		if got := aresReportFailures(rep, in); len(got) != 0 {
			t.Fatalf("failures = %v, want none", got)
		}
	})
	t.Run("quote typed mid-turn and stored as a queued_command counts", func(t *testing.T) {
		queued := waiverJSON(t, map[string]any{"type": "attachment", "attachment": map[string]any{
			"type": "queued_command", "prompt": []map[string]any{{"type": "text", "text": "ok, skip all unit test, just direct edit"}},
			"origin": map[string]any{"kind": "human"}}})
		in := waiverInput(t, []string{waiverHuman(t, "other words entirely"), queued}, []string{waiverMission(t, "MISSION: x\n"+waiverLine)})
		if got := aresReportFailures(filesOnly, in); len(got) != 0 {
			t.Fatalf("failures = %v, want none", got)
		}
	})
	t.Run("queued_command from a coordinator does not count as a human prompt", func(t *testing.T) {
		in := waiverInput(t, []string{waiverHuman(t, "other words entirely"), waiverCoordinator(t, "skip all unit test, just direct edit")}, []string{waiverMission(t, "MISSION: x\n"+waiverLine)})
		if got := aresReportFailures(filesOnly, in); len(got) == 0 {
			t.Fatal("want failures: a coordinator message is not a human prompt")
		}
	})
	t.Run("short quote is refused", func(t *testing.T) {
		in := waiverInput(t, []string{human}, []string{waiverMission(t, "MISSION: x\nUSER-WAIVER: speed — \"a\"")})
		if got := aresReportFailures(filesOnly, in); len(got) == 0 {
			t.Fatal("want failures for a one-letter quote")
		}
	})
	t.Run("files check stays under a waiver", func(t *testing.T) {
		in := waiverInput(t, []string{human}, []string{waiverMission(t, "MISSION: x\n"+waiverLine)})
		got := strings.Join(aresReportFailures("done, nothing else to say", in), "; ")
		if !strings.Contains(got, "no specific files") {
			t.Fatalf("failures = %q, want the files check", got)
		}
	})
}

func TestMentionsFilesRE(t *testing.T) {
	pass := []string{
		"Changes:\n- lua/liz_diff/git.lua\n- spec/diff_spec.lua",
		"Changed files:\n- Models/A.cs\n- Views/B.cshtml",
		"Files created/modified: kratos-dev/go/internal/cli/hook.go",
		"edited web/app.vue and styles/site.scss",
		"Changed: `檔案.ts`",
		"Changed: src/檔案.ts",
		"Changed: C:\\repo\\src\\a.go",
	}
	for _, r := range pass {
		if !mentionsFilesRE.MatchString(r) {
			t.Errorf("want a file match in %q", r)
		}
	}
	fail := []string{
		"M.show is called by the plugin", "nothing changed", "Files created/modified:\n- nothing",
		"Upgraded the Node.js runtime", "Moved to Next.js and Vue.js", "see section 3.c", "bare a.go only",
	}
	for _, r := range fail {
		if mentionsFilesRE.MatchString(r) {
			t.Errorf("want no file match in %q", r)
		}
	}
}

func TestReportNamesFilesBareOnFilesLine(t *testing.T) {
	pass := []string{
		"Files created/modified: main.go, util.go",
		"Files created/modified:\n- main.go\n- util.go\nLanded: master@abc1234",
		"- **Files created/modified:** Makefile.sh",
		"Changed files: main.go",
	}
	for _, r := range pass {
		if !reportNamesFiles(r) {
			t.Errorf("want files named in %q", r)
		}
	}
	fail := []string{
		"We moved to Node.js and main.go stays the same",
		"Reach: main.go entry point - none beyond the request",
		"Files created/modified: none",
	}
	for _, r := range fail {
		if reportNamesFiles(r) {
			t.Errorf("want no files named in %q", r)
		}
	}
}
