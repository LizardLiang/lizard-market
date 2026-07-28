package cli

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlanModeGuardDecisions pins the Odysseus plan-mode write/bash allowlist.
//
// Odysseus journals the user's answers to a draft tactical plan during his
// clarification loop, and authors a pending spec delta in step 4. Both are
// instructed in agents/odysseus.md, so a guard that denies either one silently
// breaks the agent — which is exactly what happened before this test existed:
// the spec-delta path and every `kratos` invocation were denied, and it went
// unnoticed only because isOdysseus() fails open for inline runs.
func TestPlanModeGuardDecisions(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; plan-mode-guard is a .cjs hook")
	}
	guard := filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks", "plan-mode-guard.cjs")

	cases := []struct {
		name    string
		payload map[string]any
		want    string // "allow", "deny", or "" for no decision (fail open)
	}{
		{
			name: "draft plan write allowed",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Write",
				"tool_input": map[string]any{"file_path": ".claude/.Arena/tactical-plans/2026-07-28-thing.md"},
			},
			want: "allow",
		},
		{
			name: "per-answer edit of the draft allowed",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Edit",
				"tool_input": map[string]any{"file_path": "C:/repo/.claude/.Arena/tactical-plans/2026-07-28-thing.md"},
			},
			want: "allow",
		},
		{
			name: "spec delta write allowed",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Write",
				"tool_input": map[string]any{"file_path": ".claude/feature/2026-07-28-thing/spec-delta/planning.md"},
			},
			want: "allow",
		},
		{
			// `kratos spec archive` moves promoted deltas into spec-delta/archived/.
			// Odysseus never archives, so he must not be able to write there.
			name: "archived spec delta denied",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Write",
				"tool_input": map[string]any{"file_path": ".claude/feature/2026-07-28-thing/spec-delta/archived/planning.md"},
			},
			want: "deny",
		},
		{
			name: "other feature deliverables still denied",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Write",
				"tool_input": map[string]any{"file_path": ".claude/feature/2026-07-28-thing/prd.md"},
			},
			want: "deny",
		},
		{
			name: "source write denied",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Write",
				"tool_input": map[string]any{"file_path": "src/index.ts"},
			},
			want: "deny",
		},
		{
			// Regression: the mutation blacklist matches \bmove\b, so a task
			// title containing "move" would deny the slug mint if the kratos
			// allowlist were checked after it.
			name: "slug mint with a mutating word in the title allowed",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": `kratos slug --dated "move the sidebar"`},
			},
			want: "allow",
		},
		{
			name: "quoted absolute binary path allowed",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": `"C:/Program Files/kratos/kratos.exe" spec validate my-slug`},
			},
			want: "allow",
		},
		{
			name: "template get allowed",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": "~/.kratos/bin/kratos template get spec-delta-template"},
			},
			want: "allow",
		},
		{
			// The kratos allowlist is checked before the generic deny heuristics,
			// so it must reject shell metacharacters itself or an allowed prefix
			// smuggles an arbitrary command past every other check.
			name: "chained command after an allowed kratos prefix denied",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": `kratos slug -d "x" && rm -rf build`},
			},
			want: "deny",
		},
		{
			name: "command substitution after an allowed kratos prefix denied",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": "kratos slug --dated \"$(rm -rf build)\""},
			},
			want: "deny",
		},
		{
			name: "timestamp subcommand allowed",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": "kratos now"},
			},
			want: "allow",
		},
		{
			name: "spec archive denied",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": "kratos spec archive my-slug"},
			},
			want: "deny",
		},
		{
			name: "pipeline update denied",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": "kratos pipeline update --feature x --stage 7 --status complete"},
			},
			want: "deny",
		},
		{
			name: "destructive shell denied",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": "rm -rf build"},
			},
			want: "deny",
		},
		{
			name: "read-only git allowed",
			payload: map[string]any{
				"agent_type": "kratos:odysseus",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": "git status"},
			},
			want: "allow",
		},
		{
			name: "non-odysseus agents unaffected",
			payload: map[string]any{
				"agent_type": "kratos:ares",
				"tool_name":  "Write",
				"tool_input": map[string]any{"file_path": "src/index.ts"},
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}

			cmd := exec.Command(node, guard)
			cmd.Stdin = strings.NewReader(string(raw))
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("guard exited with error: %v", err)
			}

			trimmed := strings.TrimSpace(string(out))
			if tc.want == "" {
				if trimmed != "" {
					t.Fatalf("expected no decision (fail open), got %s", trimmed)
				}
				return
			}

			var resp struct {
				HookSpecificOutput struct {
					PermissionDecision string `json:"permissionDecision"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
				t.Fatalf("guard output is not valid JSON (%q): %v", trimmed, err)
			}
			if got := resp.HookSpecificOutput.PermissionDecision; got != tc.want {
				t.Errorf("permissionDecision = %q, want %q", got, tc.want)
			}
		})
	}
}
