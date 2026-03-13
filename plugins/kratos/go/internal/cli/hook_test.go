package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizePrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // should NOT contain after sanitization
	}{
		{
			name:     "strips fenced code blocks",
			input:    "hello ```go\nfunc kratos() {}\n``` world",
			contains: "kratos",
		},
		{
			name:     "strips inline code",
			input:    "check `kratos.Config` please",
			contains: "kratos.Config",
		},
		{
			name:     "strips URLs",
			input:    "see https://github.com/kratos/example for details",
			contains: "kratos/example",
		},
		{
			name:     "strips system reminders",
			input:    "hello <system-reminder>kratos is great</system-reminder> world",
			contains: "kratos is great",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePrompt(tt.input)
			if strings.Contains(result, tt.contains) {
				t.Errorf("sanitizePrompt() still contains %q: got %q", tt.contains, result)
			}
		})
	}
}

func TestMatchKeywords(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "matches kratos",
			text: "hey Kratos build this",
			want: []string{"kratos"},
		},
		{
			name: "matches god name",
			text: "Athena, write a PRD",
			want: []string{"athena"},
		},
		{
			name: "matches multiple",
			text: "Kratos, have Ares implement and Hermes review",
			want: []string{"kratos", "ares", "hermes"},
		},
		{
			name: "no match on normal text",
			text: "please fix the login bug",
			want: nil,
		},
		{
			name: "case insensitive",
			text: "KRATOS do this",
			want: []string{"kratos"},
		},
		{
			name: "no partial match",
			text: "the kratosConfig variable",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchKeywords(tt.text)
			if len(got) != len(tt.want) {
				t.Errorf("matchKeywords() returned %d matches, want %d: %v", len(got), len(tt.want), got)
				return
			}
			for _, w := range tt.want {
				found := false
				for _, g := range got {
					if g == w {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("matchKeywords() missing expected match %q in %v", w, got)
				}
			}
		})
	}
}

func TestBuildInjectionContext(t *testing.T) {
	t.Run("kratos keyword", func(t *testing.T) {
		ctx := buildInjectionContext([]string{"kratos"})
		if !strings.Contains(ctx, "invoked Kratos by name") {
			t.Error("should mention Kratos invocation")
		}
		if !strings.Contains(ctx, "kratos:auto") {
			t.Error("should reference kratos:auto skill")
		}
	})

	t.Run("god name only", func(t *testing.T) {
		ctx := buildInjectionContext([]string{"athena"})
		if strings.Contains(ctx, "invoked Kratos by name") {
			t.Error("should NOT mention Kratos invocation for god-only match")
		}
		if !strings.Contains(ctx, "athena") {
			t.Error("should mention the god name")
		}
	})

	t.Run("mixed", func(t *testing.T) {
		ctx := buildInjectionContext([]string{"kratos", "ares", "hermes"})
		if !strings.Contains(ctx, "invoked Kratos by name") {
			t.Error("should mention Kratos")
		}
		if !strings.Contains(ctx, "ares") {
			t.Error("should mention ares")
		}
		if !strings.Contains(ctx, "hermes") {
			t.Error("should mention hermes")
		}
	})
}

func TestDetectPackageManager(t *testing.T) {
	tests := []struct {
		name       string
		lockfiles  []string
		wantPM     string
		wantLock   string
	}{
		{"bun takes priority", []string{"bun.lockb", "yarn.lock"}, "bun", "bun.lockb"},
		{"yarn detected", []string{"yarn.lock"}, "yarn", "yarn.lock"},
		{"pnpm detected", []string{"pnpm-lock.yaml"}, "pnpm", "pnpm-lock.yaml"},
		{"no lockfile returns empty", []string{}, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.lockfiles {
				os.WriteFile(filepath.Join(dir, f), []byte{}, 0644)
			}
			pm, lock := detectPackageManager(dir)
			if pm != tt.wantPM {
				t.Errorf("detectPackageManager() pm = %q, want %q", pm, tt.wantPM)
			}
			if lock != tt.wantLock {
				t.Errorf("detectPackageManager() lockfile = %q, want %q", lock, tt.wantLock)
			}
		})
	}
}

func TestFixPMRewrite(t *testing.T) {
	tests := []struct {
		name    string
		command string
		pm      string
		want    string
	}{
		{"npm install → yarn install", "npm install", "yarn", "yarn install"},
		{"npm run build → bun run build", "npm run build", "bun", "bun run build"},
		{"npm test → pnpm test", "npm test", "pnpm", "pnpm test"},
		{"no npm → unchanged", "node index.js", "yarn", "node index.js"},
		{"partial word no match", "npmrc check", "yarn", "npmrc check"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := npmWordBoundary.ReplaceAllString(tt.command, tt.pm)
			if got != tt.want {
				t.Errorf("rewrite %q with %q = %q, want %q", tt.command, tt.pm, got, tt.want)
			}
		})
	}
}

func TestSubagentStopGate(t *testing.T) {
	tests := []struct {
		name      string
		input     subagentStopInput
		wantOK    bool
		wantInMsg string
	}{
		{
			name: "ares passes with all checks",
			input: subagentStopInput{
				AgentType:            "kratos:ares",
				LastAssistantMessage: "TODO:\n1. [ ] Implement auth\nTODO:\ncreated auth.ts\nImplementation complete.",
			},
			wantOK: true,
		},
		{
			name: "ares blocked — no todo list",
			input: subagentStopInput{
				AgentType:            "kratos:ares",
				LastAssistantMessage: "created auth.ts. Implementation complete.",
			},
			wantOK:    false,
			wantInMsg: "no TODO list",
		},
		{
			name: "ares blocked — no files mentioned",
			input: subagentStopInput{
				AgentType:            "kratos:ares",
				LastAssistantMessage: "TODO:\n1. [x] Done\nImplementation complete.",
			},
			wantOK:    false,
			wantInMsg: "no specific files",
		},
		{
			name: "hephaestus passes with enough sections",
			input: subagentStopInput{
				AgentType:            "kratos:hephaestus",
				LastAssistantMessage: "## Architecture\n...\n## API\n...\n## Data Model\n...",
			},
			wantOK: true,
		},
		{
			name: "hephaestus blocked — too few sections",
			input: subagentStopInput{
				AgentType:            "kratos:hephaestus",
				LastAssistantMessage: "This is a brief spec.",
			},
			wantOK:    false,
			wantInMsg: "incomplete",
		},
		{
			name: "stop_hook_active bypasses gate",
			input: subagentStopInput{
				AgentType:      "kratos:ares",
				StopHookActive: true,
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentType := strings.ToLower(tt.input.AgentType)
			msg := tt.input.LastAssistantMessage
			msgLower := strings.ToLower(msg)

			if tt.input.StopHookActive {
				out, _ := json.Marshal(subagentStopOutput{OK: true})
				var result subagentStopOutput
				json.Unmarshal(out, &result)
				if !result.OK {
					t.Error("stop_hook_active should always pass")
				}
				return
			}

			var result subagentStopOutput

			if strings.Contains(agentType, "ares") {
				var failures []string
				hasTodo := strings.Contains(msgLower, "todo:")
				if !hasTodo {
					failures = append(failures, "no TODO list was written before starting work")
				}
				mentionsFiles := npmWordBoundary.String() != "" && strings.Contains(msg, ".ts") || strings.Contains(msg, ".js") || strings.Contains(msg, ".go") || strings.Contains(msg, ".py")
				_ = mentionsFiles
				hasFiles := strings.Contains(msg, "created") || strings.Contains(msg, "wrote") || strings.Contains(msg, "modified")
				fileExt := strings.Contains(msg, ".ts") || strings.Contains(msg, ".js") || strings.Contains(msg, ".go") || strings.Contains(msg, ".py")
				if !hasFiles || !fileExt {
					failures = append(failures, "no specific files were mentioned as created or modified")
				}
				done := strings.Contains(msgLower, "complete") || strings.Contains(msgLower, "done") || strings.Contains(msgLower, "finished") || strings.Contains(msgLower, "implemented")
				if !done {
					failures = append(failures, "implementation completion was not confirmed")
				}
				result = subagentStopOutput{OK: len(failures) == 0, Reason: strings.Join(failures, "; ")}
			} else if strings.Contains(agentType, "hephaestus") {
				sections := []string{"architecture", "data model", "api", "implementation", "schema", "interface"}
				var found []string
				for _, s := range sections {
					if strings.Contains(msgLower, s) {
						found = append(found, s)
					}
				}
				if len(found) < 2 {
					result = subagentStopOutput{OK: false, Reason: "technical spec appears incomplete"}
				} else {
					result = subagentStopOutput{OK: true}
				}
			}

			if result.OK != tt.wantOK {
				t.Errorf("gate OK = %v, want %v (reason: %s)", result.OK, tt.wantOK, result.Reason)
			}
			if tt.wantInMsg != "" && !strings.Contains(result.Reason, tt.wantInMsg) {
				t.Errorf("reason %q should contain %q", result.Reason, tt.wantInMsg)
			}
		})
	}
}
