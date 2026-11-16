package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Replays of the commands the gates denied during the week of 2026-11-08
// (leweigen-erp-v2 sessions on v2.124.0–v2.126.2), plus the forms that must
// stay denied next to each new allowance.

func TestOdysseusReadOnlyWeek1115(t *testing.T) {
	allow := []string{
		// His own loader, as commands/plan.md tells him to run it.
		`node "C:/Users/x/.claude/plugins/cache/lizard-plugins/kratos/2.126.2/hooks/launch.cjs" agent load odysseus --resolve --part body`,
		`node "C:/Users/x/.claude/plugins/cache/lizard-plugins/kratos/2.126.2/hooks/launch.cjs" agent load odysseus --resolve --part extras`,
		`node C:\Users\x\.claude\plugins\cache\lizard-plugins\kratos\2.126.2\hooks\launch.cjs agent protocol odysseus`,
		`git log --oneline -3 main; git ls-tree -r --name-only main -- backend/app/domains/sync`,
		`git -C D:/repo grep -n -E "class .*Sync"`,
		`git grep -o -n pattern`,
		`git ls-remote origin | Select-String main`,
		`git status --short | Select-Object -First 5`,
		`git blame -L 10,20 src/a.py`,
		`git merge-base main HEAD`,
		`git cat-file -p HEAD:README.md`,
		`git worktree list`,
		`Get-ChildItem backend -Recurse | Where-Object { $_.Name -match 'sync' }`,
		`Get-ChildItem . | Where-Object { $_.Name -match 'a|b' -and -not $_.PSIsContainer } | Select-String foo`,
		`Get-ChildItem . | Where-Object { $_.Length -gt 1000 }`,
		`Get-ChildItem . | ForEach-Object { $_.Name }`,
		`Get-ChildItem . | Where-Object Name -like '*.go'`,
	}
	deny := []string{
		`node "C:/x/hooks/launch.cjs" agent load odysseus; rm -rf build`,
		`node "C:/x/hooks/launch.cjs" agent load odysseus && npm run build`,
		`node "C:/x/hooks/launch.cjs" hook edit-gate`,
		`node "C:/x/hooks/launch.cjs" pipeline update --stage 7`,
		`node "C:/x/hooks/other.cjs" agent load odysseus`,
		`node build.js`,
		`node "$(rm -rf x)/hooks/launch.cjs" agent load odysseus`,
		`node "C:/x/hooks/launch.cjs" agent load $(rm -rf x)`,
		`git grep -Oless pattern`,
		`git grep --open-files-in-pager=less pattern`,
		`git ls-remote --upload-pack=./evil origin`,
		`git worktree add ../wt-x`,
		`git worktree remove ../wt-x`,
		`git push`,
		`Get-ChildItem . | Where-Object { Remove-Item $_ }`,
		`Get-ChildItem . | Where-Object { $_.Delete() }`,
		`Get-ChildItem . | Where-Object { $_.Name -match (npm run build) }`,
		`Get-ChildItem . | Where-Object { $_.Name -match "$(rm x)" }`,
		`Get-ChildItem . | Where-Object { $_.Name -eq 'a'; npm run build }`,
	}
	for _, c := range allow {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q (%s), want allow", c, got.Decision, got.Reason)
		}
	}
	for _, c := range deny {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
}

// The deny reason has to name the way out of plan mode, and that way has to
// exist: a Skill load of Iris replaces the planner.
func TestOdysseusDenyReasonNamesTheExit(t *testing.T) {
	got := odysseusBashDecision(`git push`)
	if got.Decision != "deny" {
		t.Fatalf("git push: decision %q, want deny", got.Decision)
	}
	for _, want := range []string{"kratos:ares", "kratos:iris"} {
		if !strings.Contains(got.Reason, want) {
			t.Errorf("deny reason does not name %s: %q", want, got.Reason)
		}
	}
	if res := skillLoadResult("kratos:iris"); res.SetGod != "iris" {
		t.Errorf("Skill(kratos:iris) sets god %q, want iris", res.SetGod)
	}
}

func week1115Line(t *testing.T, v map[string]any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func week1115ToolUse(t *testing.T, id, name string, input map[string]any) string {
	return week1115Line(t, map[string]any{
		"type":        "assistant",
		"isSidechain": true,
		"message": map[string]any{"content": []map[string]any{
			{"type": "tool_use", "id": id, "name": name, "input": input},
		}},
	})
}

func week1115ToolResult(t *testing.T, id string, isError bool, childID string) string {
	line := map[string]any{
		"type":        "user",
		"isSidechain": true,
		"message": map[string]any{"content": []map[string]any{
			{"type": "tool_result", "tool_use_id": id, "is_error": isError, "content": "result text"},
		}},
	}
	if childID != "" {
		line["toolUseResult"] = map[string]any{"agentId": childID, "isAsync": true}
	}
	return week1115Line(t, line)
}

// TestHandbackEvidenceWeek1115 replays the two false "code files were edited
// but no test command was run" denials: Ares had no shell and a child ran the
// tests (36 of 42 runs), and every Edit was refused by worktree isolation.
func TestHandbackEvidenceWeek1115(t *testing.T) {
	write := func(dir, name string, lines ...string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	edit := func(id string) string {
		return week1115ToolUse(t, id, "Edit", map[string]any{"file_path": "backend/app/sync.py"})
	}

	tests := []struct {
		name       string
		ares       []string
		children   map[string][]string
		wantEdited bool
		wantTests  bool
		wantDenied bool
	}{
		{
			name: "a child ran pytest",
			ares: []string{
				edit("e1"), week1115ToolResult(t, "e1", false, ""),
				week1115ToolUse(t, "a1", "Agent", map[string]any{"prompt": "run the tests"}),
				week1115ToolResult(t, "a1", false, "achild01"),
			},
			children: map[string][]string{"achild01": {
				week1115ToolUse(t, "c1", "PowerShell", map[string]any{"command": "docker compose exec api pytest -q tests/test_sync.py"}),
			}},
			wantEdited: true, wantTests: true,
		},
		{
			name: "the child ran no test",
			ares: []string{
				edit("e1"), week1115ToolResult(t, "e1", false, ""),
				week1115ToolUse(t, "a1", "Agent", map[string]any{"prompt": "commit it"}),
				week1115ToolResult(t, "a1", false, "achild02"),
			},
			children: map[string][]string{"achild02": {
				week1115ToolUse(t, "c1", "PowerShell", map[string]any{"command": "git commit -m x"}),
			}},
			wantEdited: true, wantDenied: true,
		},
		{
			name: "the child transcript is missing",
			ares: []string{
				edit("e1"), week1115ToolResult(t, "e1", false, ""),
				week1115ToolUse(t, "a1", "Agent", map[string]any{"prompt": "run the tests"}),
				week1115ToolResult(t, "a1", false, "agone"),
			},
			wantEdited: true, wantDenied: true,
		},
		{
			name: "a child id that is not an id is never opened",
			ares: []string{
				edit("e1"), week1115ToolResult(t, "e1", false, ""),
				week1115ToolUse(t, "a1", "Agent", map[string]any{"prompt": "run the tests"}),
				week1115ToolResult(t, "a1", false, "../../other/agent"),
			},
			wantEdited: true, wantDenied: true,
		},
		{
			name: "every edit was refused by worktree isolation",
			ares: []string{
				edit("e1"), week1115ToolResult(t, "e1", true, ""),
				edit("e2"), week1115ToolResult(t, "e2", true, ""),
			},
		},
		{
			name: "one edit refused, one landed, nobody tested",
			ares: []string{
				edit("e1"), week1115ToolResult(t, "e1", true, ""),
				edit("e2"), week1115ToolResult(t, "e2", false, ""),
			},
			wantEdited: true, wantDenied: true,
		},
		{
			name: "edited and tested himself with PowerShell",
			ares: []string{
				edit("e1"), week1115ToolResult(t, "e1", false, ""),
				week1115ToolUse(t, "s1", "PowerShell", map[string]any{"command": "pnpm test"}),
			},
			wantEdited: true, wantTests: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := write(dir, "agent-aares.jsonl", tt.ares...)
			for id, lines := range tt.children {
				write(dir, "agent-"+id+".jsonl", lines...)
			}
			edited, tested, err := transcriptTestEvidence(path, false)
			if err != nil {
				t.Fatal(err)
			}
			if edited != tt.wantEdited || tested != tt.wantTests {
				t.Errorf("edited=%v tested=%v, want edited=%v tested=%v", edited, tested, tt.wantEdited, tt.wantTests)
			}
			failure := aresVerifyGateFailure("ARES COMPLETE", subagentStopInput{AgentTranscriptPath: path})
			if (failure != "") != tt.wantDenied {
				t.Errorf("gate failure %q, want denied=%v", failure, tt.wantDenied)
			}
		})
	}
}
