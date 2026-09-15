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

// The npm-to-lockfile-PM auto-correction (fix-pm, rewriteNPMCommand,
// detectPackageManager) was removed — the user no longer wants Kratos
// rewriting their package-manager invocations. See item 8 of the 2026-09-15
// bug-fix batch.

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
		// stop_hook_active is no longer a special case: TestStopHookActiveGate
		// below drives the real subagentStopCmd handler to pin that a
		// re-invocation with stop_hook_active=true is checked exactly like
		// one with it false, instead of duplicating that behavior here.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentType := strings.ToLower(tt.input.AgentType)
			msg := tt.input.LastAssistantMessage
			msgLower := strings.ToLower(msg)

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
		// A compliant Ares message with stop_hook_active=true: this must
		// still allow (its checks pass), pairing with
		// TestStopHookActiveGate's negative case below which pins that an
		// unmet gate on a stop_hook_active=true retry blocks instead of
		// bypassing.
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:ares",
			"cwd":                    dir,
			"stop_hook_active":       true,
			"last_assistant_message": "Task list:\n1. [x] auth\ncreated auth.ts\nImplementation complete.\nLanded: main@abc1234",
		})
		got := run(string(b))
		if len(got) != 0 {
			t.Errorf("allow output = %v, want {}", got)
		}
	})
}

// TestStopHookActiveGate drives the real subagentStopCmd handler (not a
// re-implemented copy) to pin that stop_hook_active=true no longer bypasses
// the per-agent quality gate. Regression coverage for the bug where hook.go's
// SubagentStop handler returned {} unconditionally whenever
// stop_hook_active was true, before any per-agent check — including
// handleHermesStop's block_count guard — ever ran.
func TestStopHookActiveGate(t *testing.T) {
	run := func(payload string) subagentStopOutput {
		var out string
		pipeStdin(payload, func() {
			out = captureStdout(func() {
				_ = subagentStopCmd().RunE(nil, nil)
			})
		})
		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, out)
		}
		return resp
	}

	t.Run("ares still blocked on a re-invocation with an unmet gate", func(t *testing.T) {
		dir := t.TempDir()
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:ares",
			"cwd":                    dir,
			"stop_hook_active":       true,
			"last_assistant_message": "I did some work.", // no task list, no files, no "complete"
		})
		resp := run(string(b))
		if resp.allowed() {
			t.Error("stop_hook_active=true must not bypass Ares's quality gate — got allow")
		}
	})

	t.Run("hephaestus still blocked on a re-invocation with a thin spec", func(t *testing.T) {
		dir := t.TempDir()
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:hephaestus",
			"cwd":                    dir,
			"stop_hook_active":       true,
			"last_assistant_message": "This is a brief spec.",
		})
		resp := run(string(b))
		if resp.allowed() {
			t.Error("stop_hook_active=true must not bypass Hephaestus's quality gate — got allow")
		}
	})

	// Hermes block_count guard: reproduces "a blocked agent's second stop is
	// never re-checked" for handleHermesStop specifically. Before the fix,
	// every one of these three stops returned allow without ever touching
	// checklistPath's block_count, since the blanket StopHookActive check ran
	// first. With the fix, block_count must advance to 1, then 2 across the
	// stop_hook_active=true re-invocations, then hit the >=3 cap and allow.
	t.Run("hermes block_count advances across stop_hook_active retries then caps", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "review-feature")
		// findHermesChecklist mirrors handleHermesStart: it only resolves this
		// feature dir when 9-review is active there (see TestFindHermesChecklist).
		createFeatureStatusJSON(t, featureDir, "in-progress")
		checklistPath := filepath.Join(featureDir, "hermes-checklist.json")
		checklist := map[string]interface{}{
			"agent_id":    "hermes-1",
			"block_count": 0,
			"tiers":       map[string]bool{}, // every tier incomplete
		}
		data, _ := json.Marshal(checklist)
		if err := os.WriteFile(checklistPath, data, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		payload := func(stopHookActive bool) string {
			b, _ := json.Marshal(map[string]interface{}{
				"agent_type":       "kratos:hermes",
				"cwd":              root,
				"stop_hook_active": stopHookActive,
			})
			return string(b)
		}
		readBlockCount := func() int {
			raw, err := os.ReadFile(checklistPath)
			if err != nil {
				t.Fatalf("ReadFile checklist: %v", err)
			}
			var c struct {
				BlockCount int `json:"block_count"`
			}
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatalf("Unmarshal checklist: %v", err)
			}
			return c.BlockCount
		}

		// Stop 1: fresh, stop_hook_active=false. Blocked, block_count -> 1.
		resp1 := run(payload(false))
		if resp1.allowed() {
			t.Fatal("stop 1: expected block (all tiers incomplete), got allow")
		}
		if got := readBlockCount(); got != 1 {
			t.Fatalf("stop 1: block_count = %d, want 1", got)
		}

		// Stop 2: Claude Code's re-invocation, stop_hook_active=true. Must
		// still be blocked and advance block_count to 2 — this is the exact
		// call the old bypass skipped.
		resp2 := run(payload(true))
		if resp2.allowed() {
			t.Fatal("stop 2 (stop_hook_active=true): expected block, got allow — the bypass regressed")
		}
		if got := readBlockCount(); got != 2 {
			t.Fatalf("stop 2: block_count = %d, want 2 (must advance on stop_hook_active=true too)", got)
		}

		// Stop 3: another re-invocation, stop_hook_active=true. block_count
		// is now >= 3's cap boundary after this call's own increment check
		// (guard fires at BlockCount>=3 BEFORE incrementing), so the third
		// stop is where it should still block once more (2 < 3) and reach 3.
		resp3 := run(payload(true))
		if resp3.allowed() {
			t.Fatal("stop 3: expected block (block_count 2 < 3 cap), got allow")
		}
		if got := readBlockCount(); got != 3 {
			t.Fatalf("stop 3: block_count = %d, want 3", got)
		}

		// Stop 4: block_count is now 3, hits the cap — allowed through with
		// tiers still incomplete.
		resp4 := run(payload(true))
		if !resp4.allowed() {
			t.Fatalf("stop 4: expected allow once block_count cap (3) is reached, got block: %q", resp4.Reason)
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
	t.Run("active feature's checklist is used", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "my-feature")
		createFeatureStatusJSON(t, featureDir, "in-progress")
		checklistPath := filepath.Join(featureDir, "hermes-checklist.json")
		if err := os.WriteFile(checklistPath, []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		got := findHermesChecklist(root)
		if got != checklistPath {
			t.Errorf("got %q, want %q", got, checklistPath)
		}
	})

	t.Run("fallback to .claude/tmp/ when no feature has an active 9-review", func(t *testing.T) {
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

	// Regression for the review finding: findHermesChecklist used to glob
	// every feature dir and return "whichever file exists" (most recently
	// modified when there were several), instead of mirroring
	// handleHermesStart's own findActiveFeatureDir resolution. A stale
	// checklist sitting in an already-reviewed feature — even one with a
	// newer mtime than the real review's tmp checklist — must never be
	// picked up as the current review's state.
	t.Run("stale checklist in an inactive feature is not used, even if newer", func(t *testing.T) {
		root := t.TempDir()

		staleFeatureDir := filepath.Join(root, ".claude", "feature", "a-old")
		createFeatureStatusJSON(t, staleFeatureDir, "complete")
		staleChecklist := filepath.Join(staleFeatureDir, "hermes-checklist.json")
		writeHermesChecklist(t, staleChecklist, map[string]bool{
			"T1_correct": true, "T2_safe": true, "T3_clear": true, "T4_minimal": true,
			"T5_consistent": true, "T6_resilient": true, "T7_performant": true, "T8_maintainable": true,
		})

		// No feature has an active 9-review (a-old is complete), so
		// handleHermesStart would have written to tmp. Give tmp a checklist
		// that is OLDER than the stale one to prove mtime is not the
		// deciding factor.
		tmpDir := filepath.Join(root, ".claude", "tmp")
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			t.Fatalf("MkdirAll tmp: %v", err)
		}
		freshChecklist := filepath.Join(tmpDir, "hermes-checklist.json")
		writeHermesChecklist(t, freshChecklist, map[string]bool{}) // 0/8, real review in progress
		old := time.Now().Add(-1 * time.Hour)
		if err := os.Chtimes(freshChecklist, old, old); err != nil {
			t.Fatalf("Chtimes fresh: %v", err)
		}
		newer := time.Now()
		if err := os.Chtimes(staleChecklist, newer, newer); err != nil {
			t.Fatalf("Chtimes stale: %v", err)
		}

		got := findHermesChecklist(root)
		if got != freshChecklist {
			t.Errorf("got %q, want the tmp checklist %q (mirroring handleHermesStart's own resolution) — the stale a-old checklist must not shadow it", got, freshChecklist)
		}
	})

	// The mirror-image case: an active feature's OWN checklist must win even
	// when an unrelated feature's stale (complete) checklist has a newer
	// mtime.
	t.Run("active feature's checklist wins over a newer stale one elsewhere", func(t *testing.T) {
		root := t.TempDir()

		staleFeatureDir := filepath.Join(root, ".claude", "feature", "a-old")
		createFeatureStatusJSON(t, staleFeatureDir, "complete")
		staleChecklist := filepath.Join(staleFeatureDir, "hermes-checklist.json")
		writeHermesChecklist(t, staleChecklist, map[string]bool{
			"T1_correct": true, "T2_safe": true, "T3_clear": true, "T4_minimal": true,
			"T5_consistent": true, "T6_resilient": true, "T7_performant": true, "T8_maintainable": true,
		})

		activeFeatureDir := filepath.Join(root, ".claude", "feature", "b-new")
		createFeatureStatusJSON(t, activeFeatureDir, "in-progress")
		activeChecklist := filepath.Join(activeFeatureDir, "hermes-checklist.json")
		writeHermesChecklist(t, activeChecklist, map[string]bool{}) // 0/8, real review

		old := time.Now().Add(-1 * time.Hour)
		if err := os.Chtimes(activeChecklist, old, old); err != nil {
			t.Fatalf("Chtimes active: %v", err)
		}
		newer := time.Now()
		if err := os.Chtimes(staleChecklist, newer, newer); err != nil {
			t.Fatalf("Chtimes stale: %v", err)
		}

		got := findHermesChecklist(root)
		if got != activeChecklist {
			t.Errorf("got %q, want the active feature's checklist %q", got, activeChecklist)
		}
	})
}

// TestHermesStopUsesStartResolution is the end-to-end repro from the review:
// an old feature whose review already completed (all 8 tiers true) sits next
// to a fresh checklist for the review actually in progress (0/8, in tmp
// because no feature is marked active yet). Before the fix, handleHermesStop
// found the old feature's checklist (the only, or most-recently-modified,
// glob match) and reported "all 8 tiers complete" — satisfying a brand new
// review with zero real work. The fix must block, because the fresh
// tmp checklist (the one handleHermesStart actually wrote) has 0/8 tiers.
func TestHermesStopUsesStartResolution(t *testing.T) {
	root := t.TempDir()

	oldFeatureDir := filepath.Join(root, ".claude", "feature", "a-old")
	createFeatureStatusJSON(t, oldFeatureDir, "complete")
	writeHermesChecklist(t, filepath.Join(oldFeatureDir, "hermes-checklist.json"), map[string]bool{
		"T1_correct": true, "T2_safe": true, "T3_clear": true, "T4_minimal": true,
		"T5_consistent": true, "T6_resilient": true, "T7_performant": true, "T8_maintainable": true,
	})

	tmpDir := filepath.Join(root, ".claude", "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("MkdirAll tmp: %v", err)
	}
	writeHermesChecklist(t, filepath.Join(tmpDir, "hermes-checklist.json"), map[string]bool{}) // 0/8

	b, _ := json.Marshal(map[string]interface{}{
		"agent_type": "kratos:hermes",
		"cwd":        root,
	})
	var out string
	pipeStdin(string(b), func() {
		out = captureStdout(func() {
			_ = subagentStopCmd().RunE(nil, nil)
		})
	})
	var resp subagentStopOutput
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, out)
	}
	if resp.allowed() {
		t.Error("stale a-old checklist must not satisfy the in-progress review — got allow (\"all 8 tiers complete\")")
	}
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

// runSubagentStop pipes payload through the real subagent-stop handler and
// decodes its response.
func runSubagentStop(t *testing.T, payload string) subagentStopOutput {
	t.Helper()
	var out string
	pipeStdin(payload, func() {
		out = captureStdout(func() {
			_ = subagentStopCmd().RunE(nil, nil)
		})
	})
	var resp subagentStopOutput
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, out)
	}
	return resp
}

// TestHephaestusStopScopesToOwnFeature reproduces the review finding: the
// Hephaestus disk check globbed every .claude/feature/*/ dir and accepted a
// tech-spec.md/tech-spec-proposal.md found in ANY of them, so an old
// feature's valid spec satisfied the gate for a different feature (the one
// Hephaestus is actually working on right now) that has none. The fix must
// resolve only the feature whose pipeline is currently on 4-tech-spec
// (findFeatureDirByStage, the same resolver check.go uses) and check that
// dir specifically.
func TestHephaestusStopScopesToOwnFeature(t *testing.T) {
	specMessage := "## Architecture\n...\n## API\n...\n## Data Model\n..." // passes the section-count check

	t.Run("old feature's valid spec does not satisfy a different feature with none", func(t *testing.T) {
		root := t.TempDir()

		oldFeatureDir := filepath.Join(root, ".claude", "feature", "a-old")
		writeFile(t, filepath.Join(oldFeatureDir, "status.json"),
			`{"feature":"a-old","stage":"9-review","pipeline":{"4-tech-spec":{"status":"complete"}}}`)
		writeFile(t, filepath.Join(oldFeatureDir, "tech-spec.md"), "# Old feature's real tech spec")

		newFeatureDir := filepath.Join(root, ".claude", "feature", "b-new")
		writeFile(t, filepath.Join(newFeatureDir, "status.json"),
			`{"feature":"b-new","stage":"4-tech-spec","pipeline":{"4-tech-spec":{"status":"in-progress"}}}`)
		// b-new has no tech-spec.md / tech-spec-proposal.md yet.

		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:hephaestus",
			"cwd":                    root,
			"last_assistant_message": specMessage,
		})
		resp := runSubagentStop(t, string(b))
		if resp.allowed() {
			t.Error("a-old's tech-spec.md must not satisfy b-new's Hephaestus stop — got allow")
		}
		if !strings.Contains(resp.Reason, "b-new") {
			t.Errorf("reason should name the b-new feature dir, got: %q", resp.Reason)
		}
	})

	t.Run("own feature's spec file allows stop", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "my-feature")
		writeFile(t, filepath.Join(featureDir, "status.json"),
			`{"feature":"my-feature","stage":"4-tech-spec","pipeline":{"4-tech-spec":{"status":"in-progress"}}}`)
		writeFile(t, filepath.Join(featureDir, "tech-spec.md"), "# Real tech spec")

		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:hephaestus",
			"cwd":                    root,
			"last_assistant_message": specMessage,
		})
		resp := runSubagentStop(t, string(b))
		if !resp.allowed() {
			t.Errorf("own feature's tech-spec.md should allow stop, got block: %q", resp.Reason)
		}
	})

	t.Run("no feature on stage 4-tech-spec fails open", func(t *testing.T) {
		root := t.TempDir() // no .claude/feature/ at all — pure command mode
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:hephaestus",
			"cwd":                    root,
			"last_assistant_message": specMessage,
		})
		resp := runSubagentStop(t, string(b))
		if !resp.allowed() {
			t.Errorf("no pipeline feature should fail open, got block: %q", resp.Reason)
		}
	})
}

// TestNemesisStopScopesToOwnFeature reproduces the same review finding for
// Nemesis: handleNemesisStop globbed every feature dir for prd-challenge.md,
// so an old feature's valid challenge satisfied the gate for a different
// feature (the one on stage 2-prd-review right now) with none.
func TestNemesisStopScopesToOwnFeature(t *testing.T) {
	t.Run("old feature's valid challenge does not satisfy a different feature with none", func(t *testing.T) {
		root := t.TempDir()

		oldFeatureDir := filepath.Join(root, ".claude", "feature", "a-old")
		writeFile(t, filepath.Join(oldFeatureDir, "status.json"),
			`{"feature":"a-old","stage":"9-review","pipeline":{"2-prd-review":{"status":"complete"}}}`)
		writeFile(t, filepath.Join(oldFeatureDir, "prd-challenge.md"), "## Challenge: assumption A\ncontent")

		newFeatureDir := filepath.Join(root, ".claude", "feature", "b-new")
		writeFile(t, filepath.Join(newFeatureDir, "status.json"),
			`{"feature":"b-new","stage":"2-prd-review","pipeline":{"2-prd-review":{"status":"in-progress"}}}`)
		// b-new has no prd-challenge.md yet.

		b, _ := json.Marshal(map[string]interface{}{
			"agent_type": "kratos:nemesis",
			"cwd":        root,
		})
		resp := runSubagentStop(t, string(b))
		if resp.allowed() {
			t.Error("a-old's prd-challenge.md must not satisfy b-new's Nemesis stop — got allow")
		}
		if !strings.Contains(resp.Reason, "b-new") {
			t.Errorf("reason should name the b-new feature dir, got: %q", resp.Reason)
		}
	})

	t.Run("own feature's challenge allows stop", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "my-feature")
		writeFile(t, filepath.Join(featureDir, "status.json"),
			`{"feature":"my-feature","stage":"2-prd-review","pipeline":{"2-prd-review":{"status":"in-progress"}}}`)
		writeFile(t, filepath.Join(featureDir, "prd-challenge.md"), "## Challenge: assumption A\ncontent")

		b, _ := json.Marshal(map[string]interface{}{
			"agent_type": "kratos:nemesis",
			"cwd":        root,
		})
		resp := runSubagentStop(t, string(b))
		if !resp.allowed() {
			t.Errorf("own feature's prd-challenge.md should allow stop, got block: %q", resp.Reason)
		}
	})

	t.Run("no feature on stage 2-prd-review fails open", func(t *testing.T) {
		root := t.TempDir()
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type": "kratos:nemesis",
			"cwd":        root,
		})
		resp := runSubagentStop(t, string(b))
		if !resp.allowed() {
			t.Errorf("no pipeline feature should fail open, got block: %q", resp.Reason)
		}
	})
}

// TestAresBlockCountCapsThenAllows reproduces item B's diagnosis: before evaluateGateBlock,
// Ares's SubagentStop content gate had no cap of its own — unlike Hermes's block_count or
// check --verify's MaxRetries — so an unsatisfiable gate (a message that never satisfies the
// task-list/files/completion checks) forced continuations indefinitely. Drives the real
// subagentStopCmd handler through repeated stop_hook_active=true stops to prove it now blocks
// up to gateMaxBlocks, then allows — and that a fresh spawn (a different agent_id) starts its
// own count at 0 rather than inheriting the exhausted one.
func TestAresBlockCountCapsThenAllows(t *testing.T) {
	dir := t.TempDir()
	payload := func(agentID string, stopHookActive bool) string {
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:ares",
			"agent_id":               agentID,
			"cwd":                    dir,
			"stop_hook_active":       stopHookActive,
			"last_assistant_message": "I did some work.", // no task list, no files, no "complete" — never passes
		})
		return string(b)
	}

	for i := 1; i <= gateMaxBlocks; i++ {
		resp := runSubagentStop(t, payload("ares-1", i > 1))
		if resp.allowed() {
			t.Fatalf("stop %d: expected block (block_count %d < cap %d), got allow", i, i-1, gateMaxBlocks)
		}
	}

	resp := runSubagentStop(t, payload("ares-1", true))
	if !resp.allowed() {
		t.Fatalf("stop %d: expected allow once block-count cap (%d) is reached, got block: %q", gateMaxBlocks+1, gateMaxBlocks, resp.Reason)
	}

	// A fresh spawn (different agent_id) must start its own count at 0, not inherit ares-1's
	// exhausted cap — the cross-spawn carry-over class of bug item 3 fixed for Hermes and
	// check --verify.
	resp = runSubagentStop(t, payload("ares-2", false))
	if resp.allowed() {
		t.Fatal("fresh spawn (new agent_id) allowed through immediately — it must start its own block count at 0")
	}
}

// TestAresBlockCountResetsOnPass proves the block-count guard resets once the gate passes
// (item B.2's "make the counter reset when the gate passes"), so a later failure in a new stop
// sequence starts counting from 0 rather than picking up where an earlier, resolved sequence
// left off.
func TestAresBlockCountResetsOnPass(t *testing.T) {
	dir := t.TempDir()
	fail := func(stopHookActive bool) string {
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:ares",
			"agent_id":               "ares-1",
			"cwd":                    dir,
			"stop_hook_active":       stopHookActive,
			"last_assistant_message": "I did some work.",
		})
		return string(b)
	}
	pass := func() string {
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:ares",
			"agent_id":               "ares-1",
			"cwd":                    dir,
			"last_assistant_message": "Task list:\n1. [x] auth\ncreated auth.ts\nImplementation complete.\nLanded: main@abc1234",
		})
		return string(b)
	}

	for i := 0; i < gateMaxBlocks-1; i++ {
		if resp := runSubagentStop(t, fail(i > 0)); resp.allowed() {
			t.Fatalf("expected block on attempt %d", i+1)
		}
	}

	if resp := runSubagentStop(t, pass()); !resp.allowed() {
		t.Fatalf("expected allow on a compliant message, got block: %q", resp.Reason)
	}

	// A fresh failure right after a pass must start counting from 1 again, not from the
	// pre-pass count (which would otherwise hit the cap on this very next block).
	if resp := runSubagentStop(t, fail(false)); resp.allowed() {
		t.Fatal("expected block on the first failure after a pass — the count must have reset")
	}
}

// TestHephaestusBlockCountCapsThenAllows mirrors TestAresBlockCountCapsThenAllows for
// Hephaestus's content gate (section-count check), which also had no cap before item B.
func TestHephaestusBlockCountCapsThenAllows(t *testing.T) {
	dir := t.TempDir()
	payload := func(agentID string, stopHookActive bool) string {
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":             "kratos:hephaestus",
			"agent_id":               agentID,
			"cwd":                    dir,
			"stop_hook_active":       stopHookActive,
			"last_assistant_message": "This is a brief spec.", // too few sections — never passes
		})
		return string(b)
	}

	for i := 1; i <= gateMaxBlocks; i++ {
		resp := runSubagentStop(t, payload("heph-1", i > 1))
		if resp.allowed() {
			t.Fatalf("stop %d: expected block, got allow", i)
		}
	}

	resp := runSubagentStop(t, payload("heph-1", true))
	if !resp.allowed() {
		t.Fatalf("expected allow once block-count cap (%d) is reached, got block: %q", gateMaxBlocks, resp.Reason)
	}

	resp = runSubagentStop(t, payload("heph-2", false))
	if resp.allowed() {
		t.Fatal("fresh spawn (new agent_id) allowed through immediately — it must start its own block count at 0")
	}
}

// TestNemesisBlockCountCapsThenAllows mirrors TestAresBlockCountCapsThenAllows for Nemesis's
// content gate (prd-challenge.md checks), which also had no cap before item B.
func TestNemesisBlockCountCapsThenAllows(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, ".claude", "feature", "my-feature")
	writeFile(t, filepath.Join(featureDir, "status.json"),
		`{"feature":"my-feature","stage":"2-prd-review","pipeline":{"2-prd-review":{"status":"in-progress"}}}`)
	// No prd-challenge.md — never passes.

	payload := func(agentID string, stopHookActive bool) string {
		b, _ := json.Marshal(map[string]interface{}{
			"agent_type":       "kratos:nemesis",
			"agent_id":         agentID,
			"cwd":              root,
			"stop_hook_active": stopHookActive,
		})
		return string(b)
	}

	for i := 1; i <= gateMaxBlocks; i++ {
		resp := runSubagentStop(t, payload("nem-1", i > 1))
		if resp.allowed() {
			t.Fatalf("stop %d: expected block, got allow", i)
		}
	}

	resp := runSubagentStop(t, payload("nem-1", true))
	if !resp.allowed() {
		t.Fatalf("expected allow once block-count cap (%d) is reached, got block: %q", gateMaxBlocks, resp.Reason)
	}

	resp = runSubagentStop(t, payload("nem-2", false))
	if resp.allowed() {
		t.Fatal("fresh spawn (new agent_id) allowed through immediately — it must start its own block count at 0")
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
