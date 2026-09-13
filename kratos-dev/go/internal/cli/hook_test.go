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
		{
			name: "relative plugin path does not trigger",
			text: "the plugins/kratos launcher",
			want: nil,
		},
		{
			name: "hyphenated dev dir path does not trigger",
			text: "kratos-dev/go",
			want: nil,
		},
		{
			name: "hyphenated compound does not trigger",
			text: "rebuild the kratos-bin copy",
			want: nil,
		},
		{
			name: "backticked binary name does not trigger",
			text: "the `kratos` binary",
			want: nil,
		},
		{
			name: "direct address triggers",
			text: "Kratos, build X",
			want: []string{"kratos"},
		},
		{
			name: "ask a god triggers",
			text: "ask Athena to write the PRD",
			want: []string{"athena"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchKeywords(sanitizePrompt(tt.text))
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
		{"bun text lockfile detected", []string{"bun.lock"}, "bun", "bun.lock"},
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
		name        string
		command     string
		pm          string
		want        string
		wantChanged bool
	}{
		{"npm install → yarn install", "npm install", "yarn", "yarn install", true},
		{"npm run build → bun run build", "npm run build", "bun", "bun run build", true},
		{"npm test → pnpm test", "npm test", "pnpm", "pnpm test", true},
		{"no npm → unchanged", "node index.js", "yarn", "node index.js", false},
		{"partial word no match", "npmrc check", "yarn", "npmrc check", false},
		{"npm ci → frozen install", "npm ci", "pnpm", "pnpm install --frozen-lockfile", true},
		{"npm ci with trailing flags", "npm ci --ignore-scripts", "bun", "bun install --frozen-lockfile --ignore-scripts", true},
		{"after &&", "cd app && npm install", "pnpm", "cd app && pnpm install", true},
		{"after ;", "echo hi; npm test", "yarn", "echo hi; yarn test", true},
		{"after ||", "true || npm test", "yarn", "true || yarn test", true},
		{"after pipe", "cat x | npm exec foo", "pnpm", "cat x | pnpm exec foo", true},
		{"env prefix", "CI=true npm test", "pnpm", "CI=true pnpm test", true},
		{"grep npm untouched", "grep npm package.json", "pnpm", "grep npm package.json", false},
		{"double-quoted untouched", `echo "npm install"`, "pnpm", `echo "npm install"`, false},
		{"single-quoted untouched", `echo 'run npm ci'`, "pnpm", `echo 'run npm ci'`, false},
		{"quoted segment separator untouched", `echo "a; npm test"`, "pnpm", `echo "a; npm test"`, false},
		{"mixed: leading rewritten, argument kept", "npm run lint && grep npm README.md", "yarn", "yarn run lint && grep npm README.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := rewriteNPMCommand(tt.command, tt.pm)
			if got != tt.want {
				t.Errorf("rewrite %q with %q = %q, want %q", tt.command, tt.pm, got, tt.want)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}

// TestFixPMCommandOutput runs the real fix-pm command: cwd comes from the hook
// payload, and the output carries updatedInput but no permissionDecision.
func TestFixPMCommandOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	run := func(command string) string {
		b, _ := json.Marshal(map[string]interface{}{
			"tool_name":  "Bash",
			"tool_input": map[string]string{"command": command},
			"cwd":        dir,
		})
		var out string
		pipeStdin(string(b), func() {
			out = captureStdout(func() {
				_ = fixPMCmd().RunE(nil, nil)
			})
		})
		return strings.TrimSpace(out)
	}

	t.Run("rewrites and leaves permission flow alone", func(t *testing.T) {
		out := run("npm install")
		if out == "" {
			t.Fatal("expected a rewrite, got no output")
		}
		if strings.Contains(out, "permissionDecision") {
			t.Errorf("output must not carry permissionDecision: %s", out)
		}
		var parsed struct {
			HookSpecificOutput struct {
				HookEventName string            `json:"hookEventName"`
				UpdatedInput  map[string]string `json:"updatedInput"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("output not valid JSON: %v\n%s", err, out)
		}
		if parsed.HookSpecificOutput.HookEventName != "PreToolUse" {
			t.Errorf("hookEventName = %q", parsed.HookSpecificOutput.HookEventName)
		}
		if got := parsed.HookSpecificOutput.UpdatedInput["command"]; got != "pnpm install" {
			t.Errorf("updatedInput.command = %q, want %q", got, "pnpm install")
		}
	})

	t.Run("argument-position npm produces no output", func(t *testing.T) {
		if out := run("grep npm package.json"); out != "" {
			t.Errorf("expected no output, got %s", out)
		}
	})
}

// allowed reports whether a SubagentStop response lets the agent stop (no block decision).
func (o subagentStopOutput) allowed() bool { return o.Decision != "block" }

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
				LastAssistantMessage: "TODO:\n1. [ ] Implement auth\nTODO:\ncreated auth.ts\nImplementation complete.\nLanded: main@abc1234",
			},
			wantOK: true,
		},
		{
			name: "ares passes with task list recap",
			input: subagentStopInput{
				AgentType:            "kratos:ares",
				LastAssistantMessage: "Task list:\n1. [x] auth\ncreated auth.ts\nImplementation complete.\nLanded: main@abc1234",
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
			wantInMsg: "no task list",
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
				var result subagentStopOutput
				json.Unmarshal([]byte("{}"), &result)
				if !result.allowed() {
					t.Error("stop_hook_active should always pass")
				}
				return
			}

			var result subagentStopOutput
			block := func(reason string) subagentStopOutput {
				return subagentStopOutput{Decision: "block", Reason: reason}
			}

			if strings.Contains(agentType, "ares") {
				var failures []string
				hasTaskList := strings.Contains(msgLower, "task list:") || strings.Contains(msgLower, "todo:")
				if !hasTaskList {
					failures = append(failures, "no task list recap was written before starting work")
				}
				hasFiles := strings.Contains(msg, "created") || strings.Contains(msg, "wrote") || strings.Contains(msg, "modified")
				fileExt := strings.Contains(msg, ".ts") || strings.Contains(msg, ".js") || strings.Contains(msg, ".go") || strings.Contains(msg, ".py")
				if !hasFiles || !fileExt {
					failures = append(failures, "no specific files were mentioned as created or modified")
				}
				done := strings.Contains(msgLower, "complete") || strings.Contains(msgLower, "done") || strings.Contains(msgLower, "finished") || strings.Contains(msgLower, "implemented")
				if !done {
					failures = append(failures, "implementation completion was not confirmed")
				}
				if len(failures) > 0 {
					result = block(strings.Join(failures, "; "))
				}
			} else if strings.Contains(agentType, "hephaestus") {
				sections := []string{"architecture", "data model", "api", "implementation", "schema", "interface"}
				var found []string
				for _, s := range sections {
					if strings.Contains(msgLower, s) {
						found = append(found, s)
					}
				}
				if len(found) < 2 {
					result = block("technical spec appears incomplete")
				}
			}

			if result.allowed() != tt.wantOK {
				t.Errorf("gate allowed = %v, want %v (reason: %s)", result.allowed(), tt.wantOK, result.Reason)
			}
			if tt.wantInMsg != "" && !strings.Contains(result.Reason, tt.wantInMsg) {
				t.Errorf("reason %q should contain %q", result.Reason, tt.wantInMsg)
			}
		})
	}
}

// TestSubagentStopOutputShape runs the real subagent-stop command and checks the
// wire format: a block is a top-level {"decision":"block","reason":...} (the only
// shape Claude Code honors for SubagentStop), an allow is an empty object.
func TestSubagentStopOutputShape(t *testing.T) {
	run := func(payload string) map[string]interface{} {
		var out string
		pipeStdin(payload, func() {
			out = captureStdout(func() {
				_ = subagentStopCmd().RunE(nil, nil)
			})
		})
		var got map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, out)
		}
		return got
	}

	t.Run("block uses decision:block", func(t *testing.T) {
		dir := t.TempDir()
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:hephaestus",
			"cwd":                    dir,
			"last_assistant_message": "This is a brief spec.",
		})
		got := run(string(b))
		if got["decision"] != "block" {
			t.Fatalf("decision = %v, want %q (full output %v)", got["decision"], "block", got)
		}
		if _, has := got["ok"]; has {
			t.Error("legacy \"ok\" field must not be emitted")
		}
		if reason, _ := got["reason"].(string); !strings.Contains(reason, "incomplete") {
			t.Errorf("reason = %q, want mention of incomplete spec", reason)
		}
	})

	t.Run("allow is an empty object", func(t *testing.T) {
		dir := t.TempDir()
		got := run(makeStopStdin("kratos:ares", dir, true))
		if len(got) != 0 {
			t.Errorf("allow output = %v, want {}", got)
		}
	})
}

// TestMalformedStopPayloadFailsClosed verifies that a SubagentStop payload that cannot be
// parsed does not silently bypass a gated agent's quality gate.
func TestMalformedStopPayloadFailsClosed(t *testing.T) {
	gatedCases := []struct {
		name string
		raw  string
	}{
		{"ares raw newline breaks json", "{\"agent_type\":\"kratos:ares\",\"last_assistant_message\":\"oops\nraw newline\"}"},
		{"hephaestus truncated json", `{"agent_type":"kratos:hephaestus","last_assistant_message":"`},
		{"hermes garbage", `not json at all but mentions hermes`},
		{"nemesis garbage", `{bad json nemesis`},
		{"athena garbage", `not json at all but mentions athena`},
	}
	for _, tt := range gatedCases {
		t.Run("blocks/"+tt.name, func(t *testing.T) {
			raw := []byte(tt.raw)
			var in subagentStopInput
			if json.Unmarshal(raw, &in) == nil {
				t.Fatalf("payload unexpectedly parsed; test needs a malformed payload")
			}
			if rawHasStopHookActive(raw) {
				t.Fatalf("payload should not look like a re-entry")
			}
			if agent := gatedAgentInRaw(raw); agent == "" {
				t.Errorf("gatedAgentInRaw = \"\" — gated agent would silently bypass the gate")
			}
		})
	}

	// A re-entry marker must short-circuit to allow, even when malformed, to avoid loops.
	t.Run("stop_hook_active short-circuits loop", func(t *testing.T) {
		raw := []byte(`{bad json ares "stop_hook_active": true`)
		if !rawHasStopHookActive(raw) {
			t.Errorf("rawHasStopHookActive = false; a re-invocation loop is possible")
		}
	})

	// Ungated agents on a malformed payload still fail open (don't break the pipeline).
	t.Run("ungated agent not blocked", func(t *testing.T) {
		raw := []byte(`{bad json cassandra`)
		if agent := gatedAgentInRaw(raw); agent != "" {
			t.Errorf("gatedAgentInRaw = %q; ungated agent should fail open", agent)
		}
	})

	// Substrings of ordinary words must not read as a gated agent.
	t.Run("substring words are not gated agents", func(t *testing.T) {
		raw := []byte(`{bad json "the diff shares and compares squares"`)
		if agent := gatedAgentInRaw(raw); agent != "" {
			t.Errorf("gatedAgentInRaw = %q; 'shares'/'compares' must not match ares", agent)
		}
	})

	// A surviving agent_type field wins over the message body.
	t.Run("agent_type field takes precedence over body", func(t *testing.T) {
		raw := []byte(`{"agent_type":"kratos:cassandra","last_assistant_message":"asked ares and hermes`)
		if agent := gatedAgentInRaw(raw); agent != "" {
			t.Errorf("gatedAgentInRaw = %q; agent_type is cassandra (ungated)", agent)
		}
		raw = []byte(`{"agent_type":"kratos:ares","last_assistant_message":"oops` + "\n" + `"`)
		if agent := gatedAgentInRaw(raw); agent != "ares" {
			t.Errorf("gatedAgentInRaw = %q, want ares", agent)
		}
	})
}

// createFeatureStatusJSON creates a status.json file with the given 9-review status under featureDir.
func createFeatureStatusJSON(t *testing.T, featureDir string, reviewStatus string) {
	t.Helper()
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", featureDir, err)
	}
	content := map[string]interface{}{
		"pipeline": map[string]interface{}{
			"9-review": map[string]interface{}{
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

// writeTranscript writes JSONL lines to a temp file and returns its path.
func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Transcript line builders for verify-gate tests.
func userPromptLine(text string) string {
	b, _ := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]any{"content": text},
	})
	return string(b)
}

func toolUseLine(sidechain bool, toolName string, input map[string]any) string {
	b, _ := json.Marshal(map[string]any{
		"type":        "assistant",
		"isSidechain": sidechain,
		"message": map[string]any{
			"content": []map[string]any{
				{"type": "tool_use", "name": toolName, "input": input},
			},
		},
	})
	return string(b)
}

func TestTranscriptTestEvidence(t *testing.T) {
	tests := []struct {
		name           string
		lines          []string
		sidechainOnly  bool
		wantEditedCode bool
		wantRanTests   bool
	}{
		{
			name: "code edit plus test run",
			lines: []string{
				userPromptLine("implement the feature"),
				toolUseLine(true, "Edit", map[string]any{"file_path": "src/auth.go"}),
				toolUseLine(true, "Bash", map[string]any{"command": "go test ./..."}),
			},
			sidechainOnly:  true,
			wantEditedCode: true,
			wantRanTests:   true,
		},
		{
			name: "code edit without test run",
			lines: []string{
				userPromptLine("implement the feature"),
				toolUseLine(true, "Write", map[string]any{"file_path": "src/auth.ts"}),
			},
			sidechainOnly:  true,
			wantEditedCode: true,
			wantRanTests:   false,
		},
		{
			name: "markdown-only edits do not count as code",
			lines: []string{
				userPromptLine("write the notes"),
				toolUseLine(true, "Write", map[string]any{"file_path": "implementation-notes.md"}),
			},
			sidechainOnly:  true,
			wantEditedCode: false,
			wantRanTests:   false,
		},
		{
			name: "main-session edits ignored when sidechainOnly",
			lines: []string{
				userPromptLine("implement the feature"),
				toolUseLine(false, "Edit", map[string]any{"file_path": "src/auth.go"}),
			},
			sidechainOnly:  true,
			wantEditedCode: false,
			wantRanTests:   false,
		},
		{
			name: "evidence resets at a new user prompt",
			lines: []string{
				userPromptLine("first task"),
				toolUseLine(true, "Edit", map[string]any{"file_path": "src/old.go"}),
				toolUseLine(true, "Bash", map[string]any{"command": "go test ./..."}),
				userPromptLine("second task"),
				toolUseLine(true, "Edit", map[string]any{"file_path": "src/new.go"}),
			},
			sidechainOnly:  true,
			wantEditedCode: true,
			wantRanTests:   false,
		},
		{
			name: "word boundary rejects attest and npmrc test",
			lines: []string{
				userPromptLine("task"),
				toolUseLine(true, "Edit", map[string]any{"file_path": "src/a.py"}),
				toolUseLine(true, "Bash", map[string]any{"command": "attest --verify"}),
				toolUseLine(true, "Bash", map[string]any{"command": "npmrc test"}),
			},
			sidechainOnly:  true,
			wantEditedCode: true,
			wantRanTests:   false,
		},
		{
			name: "npm run test and pytest -k both match",
			lines: []string{
				userPromptLine("task"),
				toolUseLine(true, "Edit", map[string]any{"file_path": "src/a.py"}),
				toolUseLine(true, "Bash", map[string]any{"command": "cd app && npm run test"}),
				toolUseLine(true, "Bash", map[string]any{"command": "pytest -k auth"}),
			},
			sidechainOnly:  true,
			wantEditedCode: true,
			wantRanTests:   true,
		},
		{
			name: "garbage lines are tolerated",
			lines: []string{
				"not json at all {{{",
				userPromptLine("task"),
				"",
				toolUseLine(true, "Edit", map[string]any{"file_path": "src/a.rs"}),
				`{"type":"assistant","message":{"content":"plain string content"}}`,
				toolUseLine(true, "Bash", map[string]any{"command": "cargo test"}),
			},
			sidechainOnly:  true,
			wantEditedCode: true,
			wantRanTests:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTranscript(t, tt.lines...)
			editedCode, ranTests, err := transcriptTestEvidence(path, tt.sidechainOnly)
			if err != nil {
				t.Fatalf("transcriptTestEvidence() error = %v", err)
			}
			if editedCode != tt.wantEditedCode {
				t.Errorf("editedCode = %v, want %v", editedCode, tt.wantEditedCode)
			}
			if ranTests != tt.wantRanTests {
				t.Errorf("ranTests = %v, want %v", ranTests, tt.wantRanTests)
			}
		})
	}
}

func TestAresVerifyGateFailure(t *testing.T) {
	codeEditNoTest := []string{
		userPromptLine("implement"),
		toolUseLine(true, "Edit", map[string]any{"file_path": "src/auth.go"}),
	}
	codeEditWithTest := append(append([]string{}, codeEditNoTest...),
		toolUseLine(true, "Bash", map[string]any{"command": "go test ./..."}))

	t.Run("blocks on code edit without test", func(t *testing.T) {
		input := subagentStopInput{TranscriptPath: writeTranscript(t, codeEditNoTest...)}
		if f := aresVerifyGateFailure(input); f == "" {
			t.Error("expected a failure, got none")
		}
	})

	t.Run("passes on code edit with test", func(t *testing.T) {
		input := subagentStopInput{TranscriptPath: writeTranscript(t, codeEditWithTest...)}
		if f := aresVerifyGateFailure(input); f != "" {
			t.Errorf("expected no failure, got %q", f)
		}
	})

	t.Run("escape phrase waives the gate", func(t *testing.T) {
		input := subagentStopInput{
			TranscriptPath:       writeTranscript(t, codeEditNoTest...),
			LastAssistantMessage: "Docs-only change. TESTS-NOT-APPLICABLE: no runtime surface.",
		}
		if f := aresVerifyGateFailure(input); f != "" {
			t.Errorf("expected escape phrase to waive gate, got %q", f)
		}
	})

	t.Run("missing transcript fails open", func(t *testing.T) {
		input := subagentStopInput{TranscriptPath: filepath.Join(t.TempDir(), "missing.jsonl")}
		if f := aresVerifyGateFailure(input); f != "" {
			t.Errorf("expected fail-open on missing file, got %q", f)
		}
	})

	t.Run("no transcript path means gate inactive", func(t *testing.T) {
		if f := aresVerifyGateFailure(subagentStopInput{}); f != "" {
			t.Errorf("expected inactive gate, got %q", f)
		}
	})

	t.Run("agent transcript path scans without sidechain filter", func(t *testing.T) {
		lines := []string{
			userPromptLine("implement"),
			toolUseLine(false, "Edit", map[string]any{"file_path": "src/auth.go"}),
		}
		input := subagentStopInput{AgentTranscriptPath: writeTranscript(t, lines...)}
		if f := aresVerifyGateFailure(input); f == "" {
			t.Error("expected failure via agent_transcript_path without sidechain flag")
		}
	})
}
