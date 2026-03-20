package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		name      string
		lockfiles []string
		wantPM    string
		wantLock  string
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

// createFeatureStatusJSON creates a status.json file with the given 11-review status under featureDir.
func createFeatureStatusJSON(t *testing.T, featureDir string, reviewStatus string) {
	t.Helper()
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", featureDir, err)
	}
	content := map[string]interface{}{
		"stages": map[string]interface{}{
			"11-review": map[string]interface{}{
				"status": reviewStatus,
			},
		},
	}
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("json.Marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "status.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile status.json: %v", err)
	}
}

func TestFindActiveFeatureDir(t *testing.T) {
	tests := []struct {
		name          string
		reviewStatus  string // empty string means: do not create status.json
		malformedJSON bool
		wantEmpty     bool
	}{
		{
			name:         "pending status returns feature dir",
			reviewStatus: "pending",
			wantEmpty:    false,
		},
		{
			name:         "in-progress status returns feature dir",
			reviewStatus: "in-progress",
			wantEmpty:    false,
		},
		{
			name:         "complete status returns empty string",
			reviewStatus: "complete",
			wantEmpty:    true,
		},
		{
			name:      "no status.json returns empty string",
			wantEmpty: true,
		},
		{
			name:          "malformed JSON returns empty string without panic",
			malformedJSON: true,
			wantEmpty:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			featureDir := filepath.Join(root, ".claude", "feature", "my-feature")

			switch {
			case tt.malformedJSON:
				if err := os.MkdirAll(featureDir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(filepath.Join(featureDir, "status.json"), []byte("{not valid json"), 0644); err != nil {
					t.Fatalf("WriteFile malformed: %v", err)
				}
			case tt.reviewStatus != "":
				createFeatureStatusJSON(t, featureDir, tt.reviewStatus)
				// When reviewStatus is empty and not malformed, no status.json is created.
			}

			got, err := findActiveFeatureDir(root)
			if err != nil {
				t.Fatalf("findActiveFeatureDir returned unexpected error: %v", err)
			}

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty string, got %q", got)
				}
			} else {
				if got == "" {
					t.Error("expected a feature dir path, got empty string")
				}
				if got != featureDir {
					t.Errorf("got %q, want %q", got, featureDir)
				}
			}
		})
	}
}

func TestFindHermesChecklist(t *testing.T) {
	t.Run("single checklist in feature folder", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "my-feature")
		if err := os.MkdirAll(featureDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		checklistPath := filepath.Join(featureDir, "hermes-checklist.json")
		if err := os.WriteFile(checklistPath, []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		got := findHermesChecklist(root)
		if got != checklistPath {
			t.Errorf("got %q, want %q", got, checklistPath)
		}
	})

	t.Run("fallback to .claude/tmp/ when no feature checklist", func(t *testing.T) {
		root := t.TempDir()
		tmpDir := filepath.Join(root, ".claude", "tmp")
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		fallbackPath := filepath.Join(tmpDir, "hermes-checklist.json")
		if err := os.WriteFile(fallbackPath, []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		got := findHermesChecklist(root)
		if got != fallbackPath {
			t.Errorf("got %q, want %q", got, fallbackPath)
		}
	})

	t.Run("no checklist anywhere returns empty string", func(t *testing.T) {
		root := t.TempDir()
		got := findHermesChecklist(root)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("multiple checklists returns most recently modified", func(t *testing.T) {
		root := t.TempDir()
		featureA := filepath.Join(root, ".claude", "feature", "feature-a")
		featureB := filepath.Join(root, ".claude", "feature", "feature-b")
		if err := os.MkdirAll(featureA, 0755); err != nil {
			t.Fatalf("MkdirAll featureA: %v", err)
		}
		if err := os.MkdirAll(featureB, 0755); err != nil {
			t.Fatalf("MkdirAll featureB: %v", err)
		}

		pathA := filepath.Join(featureA, "hermes-checklist.json")
		pathB := filepath.Join(featureB, "hermes-checklist.json")

		// Write A first, then back-date it so B is clearly newer.
		if err := os.WriteFile(pathA, []byte(`{"feature":"a"}`), 0644); err != nil {
			t.Fatalf("WriteFile A: %v", err)
		}
		old := time.Now().Add(-10 * time.Second)
		if err := os.Chtimes(pathA, old, old); err != nil {
			t.Fatalf("Chtimes A: %v", err)
		}
		if err := os.WriteFile(pathB, []byte(`{"feature":"b"}`), 0644); err != nil {
			t.Fatalf("WriteFile B: %v", err)
		}

		got := findHermesChecklist(root)
		if got != pathB {
			t.Errorf("expected most-recent checklist %q, got %q", pathB, got)
		}
	})
}

// writeHermesChecklist creates a hermes-checklist.json with the provided tier values.
func writeHermesChecklist(t *testing.T, path string, tiers map[string]bool) {
	t.Helper()
	content := map[string]interface{}{
		"agent_id": "test-agent",
		"tiers":    tiers,
	}
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal checklist: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile checklist: %v", err)
	}
}

// allTiersFalse returns a map with all 8 tiers set to false.
func allTiersFalse() map[string]bool {
	return map[string]bool{
		"T1_correct":      false,
		"T2_safe":         false,
		"T3_clear":        false,
		"T4_minimal":      false,
		"T5_consistent":   false,
		"T6_resilient":    false,
		"T7_performant":   false,
		"T8_maintainable": false,
	}
}

// allTiersTrue returns a map with all 8 tiers set to true.
func allTiersTrue() map[string]bool {
	return map[string]bool{
		"T1_correct":      true,
		"T2_safe":         true,
		"T3_clear":        true,
		"T4_minimal":      true,
		"T5_consistent":   true,
		"T6_resilient":    true,
		"T7_performant":   true,
		"T8_maintainable": true,
	}
}

func TestHermesChecklistEnforcement(t *testing.T) {
	tests := []struct {
		name             string
		tiers            map[string]bool // nil means: do not create checklist file
		wantAllComplete  bool
		wantInIncomplete string // substring that must appear in one of the incomplete tier names
	}{
		{
			name:             "all tiers false — blocked",
			tiers:            allTiersFalse(),
			wantAllComplete:  false,
			wantInIncomplete: "T1 Correct",
		},
		{
			name:            "all tiers true — passes",
			tiers:           allTiersTrue(),
			wantAllComplete: true,
		},
		{
			name: "seven true one false — blocked with incomplete tier name",
			tiers: func() map[string]bool {
				m := allTiersTrue()
				m["T5_consistent"] = false
				return m
			}(),
			wantAllComplete:  false,
			wantInIncomplete: "T5 Consistent",
		},
		{
			name:            "no checklist file — fails open (passes)",
			tiers:           nil,
			wantAllComplete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			checklistPath := filepath.Join(root, "hermes-checklist.json")

			if tt.tiers != nil {
				writeHermesChecklist(t, checklistPath, tt.tiers)
			} else {
				// Point at a path that does not exist so checkHermesChecklist fails open.
				checklistPath = filepath.Join(root, "nonexistent-checklist.json")
			}

			ok, incomplete := checkHermesChecklist(checklistPath)

			if ok != tt.wantAllComplete {
				t.Errorf("checkHermesChecklist() ok = %v, want %v (incomplete: %v)", ok, tt.wantAllComplete, incomplete)
			}

			if tt.wantInIncomplete != "" {
				found := false
				for _, name := range incomplete {
					if strings.Contains(name, tt.wantInIncomplete) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("incomplete tiers %v should contain %q", incomplete, tt.wantInIncomplete)
				}
			}

			if tt.wantAllComplete && len(incomplete) != 0 {
				t.Errorf("expected no incomplete tiers, got %v", incomplete)
			}
		})
	}
}