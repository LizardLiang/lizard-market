package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- helpers ----------

// captureStdout redirects os.Stdout to a buffer for the duration of fn.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// writeFile writes content to a file, creating parent dirs as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("writeFile: MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: WriteFile: %v", err)
	}
}

// makeFeatureDir creates a temp root with .claude/feature/<name>/ and a basic status.json.
func makeFeatureDir(t *testing.T, name, stage string) (root, featureDir string) {
	t.Helper()
	root = t.TempDir()
	featureDir = filepath.Join(root, ".claude", "feature", name)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("makeFeatureDir: %v", err)
	}
	statusJSON := fmt.Sprintf(`{"feature":%q,"stage":%q,"pipeline":{%q:{"status":"in-progress"}}}`,
		name, stage, stage)
	writeFile(t, filepath.Join(featureDir, "status.json"), statusJSON)
	return root, featureDir
}

// pipeStdin replaces os.Stdin with a reader containing content for the duration of fn.
func pipeStdin(content string, fn func()) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	w.WriteString(content)
	w.Close()
	fn()
	os.Stdin = old
}

// makeStopStdin returns a JSON string for a SubagentStop event (properly escaped paths).
func makeStopStdin(agentType, cwd string, stopHookActive bool) string {
	b, _ := json.Marshal(map[string]interface{}{
		"agent_type":      agentType,
		"cwd":             cwd,
		"stop_hook_active": stopHookActive,
	})
	return string(b)
}

// makeStartStdin returns a JSON string for a SubagentStart event (properly escaped paths).
func makeStartStdin(agentID, agentType, cwd string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"agent_id":   agentID,
		"agent_type": agentType,
		"cwd":        cwd,
	})
	return string(b)
}

// ---------- TC-050/051: Stage mapping ----------

func TestStageChecks(t *testing.T) {
	t.Run("all 9 tier1 stages present", func(t *testing.T) {
		expected := []string{
			"1-prd", "2-prd-review", "3-decomposition", "4-discuss",
			"6-spec-review-pm", "7-spec-review-sa", "8-test-plan",
			"10-prd-alignment", "11-review",
		}
		for _, s := range expected {
			if _, ok := stageChecks[s]; !ok {
				t.Errorf("stageChecks missing expected stage %q", s)
			}
		}
	})

	t.Run("excluded stages not present", func(t *testing.T) {
		excluded := []string{"0-research", "5-tech-spec", "9-implementation"}
		for _, s := range excluded {
			if _, ok := stageChecks[s]; ok {
				t.Errorf("stageChecks should NOT contain excluded stage %q", s)
			}
		}
	})

	t.Run("1-prd has correct fields", func(t *testing.T) {
		sc := stageChecks["1-prd"]
		if sc.Tier != 1 {
			t.Errorf("1-prd Tier = %d, want 1", sc.Tier)
		}
		if sc.MaxRetries != 2 {
			t.Errorf("1-prd MaxRetries = %d, want 2", sc.MaxRetries)
		}
		if len(sc.Files) != 2 {
			t.Errorf("1-prd Files len = %d, want 2: %v", len(sc.Files), sc.Files)
		}
		wantFiles := map[string]bool{"prd.md": true, "decisions.md": true}
		for _, f := range sc.Files {
			if !wantFiles[f] {
				t.Errorf("1-prd unexpected file %q", f)
			}
		}
	})

	t.Run("2-prd-review has verdicts", func(t *testing.T) {
		sc := stageChecks["2-prd-review"]
		if len(sc.Verdicts) == 0 {
			t.Error("2-prd-review should have Verdicts")
		}
		v, ok := sc.Verdicts["prd-review.md"]
		if !ok {
			t.Fatal("2-prd-review missing Verdicts for prd-review.md")
		}
		wantVerdicts := map[string]bool{"approved": true, "revisions": true, "rejected": true}
		for _, vv := range v {
			if !wantVerdicts[vv] {
				t.Errorf("2-prd-review unexpected verdict %q", vv)
			}
		}
	})

	t.Run("3-decomposition is optional", func(t *testing.T) {
		sc := stageChecks["3-decomposition"]
		if !sc.Optional {
			t.Error("3-decomposition should be Optional=true")
		}
	})

	t.Run("4-discuss is optional", func(t *testing.T) {
		sc := stageChecks["4-discuss"]
		if !sc.Optional {
			t.Error("4-discuss should be Optional=true")
		}
	})

	t.Run("8-test-plan has correct file", func(t *testing.T) {
		sc := stageChecks["8-test-plan"]
		if len(sc.Files) != 1 || sc.Files[0] != "test-plan.md" {
			t.Errorf("8-test-plan Files = %v, want [test-plan.md]", sc.Files)
		}
	})

	t.Run("11-review uses AgentDispatch", func(t *testing.T) {
		sc := stageChecks["11-review"]
		if len(sc.Files) != 0 {
			t.Errorf("11-review should have no direct Files, got %v", sc.Files)
		}
		if len(sc.AgentDispatch) == 0 {
			t.Error("11-review should have AgentDispatch")
		}
		cassandra, ok := sc.AgentDispatch["kratos:cassandra"]
		if !ok {
			t.Fatal("11-review missing AgentDispatch[kratos:cassandra]")
		}
		if len(cassandra.Files) != 1 || cassandra.Files[0] != "risk-analysis.md" {
			t.Errorf("cassandra sub-check Files = %v, want [risk-analysis.md]", cassandra.Files)
		}
	})

	t.Run("unknown stage returns zero value", func(t *testing.T) {
		sc, ok := stageChecks["99-nonexistent"]
		if ok {
			t.Errorf("expected stageChecks[99-nonexistent] to be absent, got %+v", sc)
		}
	})
}

// ---------- TC-010/011/012/013: File existence ----------

func TestVerifyFileExists(t *testing.T) {
	tests := []struct {
		name    string
		content string
		size    int // override: -1 to use content, 0 = empty file
		wantErr bool
	}{
		{name: "file exists non-empty", content: "hello", wantErr: false},
		{name: "file with single newline", content: "\n", wantErr: false},
		{name: "file missing", content: "", size: -2 /* do not create */, wantErr: true},
		{name: "file empty 0 bytes", content: "", size: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.size != -2 {
				if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte(tt.content), 0o644); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			err := verifyFileExists(dir, "test.md")
			if (err != nil) != tt.wantErr {
				t.Errorf("verifyFileExists() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// ---------- TC-030/031/032/033/034: Verdict checks ----------

func TestVerifyVerdictPresent(t *testing.T) {
	verdicts := []string{"approved", "revisions", "rejected"}

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "approved mixed case", content: "## Verdict: Approved", wantErr: false},
		{name: "approved lowercase", content: "verdict: approved", wantErr: false},
		{name: "APPROVED uppercase", content: "VERDICT: APPROVED", wantErr: false},
		{name: "revisions present", content: "Verdict: Revisions required", wantErr: false},
		{name: "no verdict", content: "This is a review without any verdict keyword", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "prd-review.md"), tt.content)
			err := verifyVerdictPresent(dir, "prd-review.md", verdicts)
			if (err != nil) != tt.wantErr {
				t.Errorf("verifyVerdictPresent() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyVerdictPerStage(t *testing.T) {
	t.Run("SA review uses different verdict set", func(t *testing.T) {
		dir := t.TempDir()
		// "sound" is valid SA verdict
		writeFile(t, filepath.Join(dir, "spec-review-sa.md"), "Architecture is sound.")
		err := verifyVerdictPresent(dir, "spec-review-sa.md", []string{"sound", "concerns", "unsound"})
		if err != nil {
			t.Errorf("SA verdict 'sound' should pass: %v", err)
		}
	})

	t.Run("SA review rejects PM verdict", func(t *testing.T) {
		dir := t.TempDir()
		// "approved" is NOT valid for SA review
		writeFile(t, filepath.Join(dir, "spec-review-sa.md"), "approved — looks good")
		err := verifyVerdictPresent(dir, "spec-review-sa.md", []string{"sound", "concerns", "unsound"})
		if err == nil {
			t.Error("SA verdict 'approved' should fail (not in SA vocabulary)")
		}
	})
}

// ---------- TC-040/041/042/043: Retry counting ----------

func TestRetry(t *testing.T) {
	t.Run("increment creates counter 1", func(t *testing.T) {
		dir := t.TempDir()
		count, err := incrementRetry(dir, "1-prd")
		if err != nil {
			t.Fatalf("incrementRetry: %v", err)
		}
		if count != 1 {
			t.Errorf("first increment = %d, want 1", count)
		}
		state, _ := readCheckState(dir)
		if state["1-prd"] != 1 {
			t.Errorf("check-state.json[1-prd] = %d, want 1", state["1-prd"])
		}
	})

	t.Run("increment to 2 on second call", func(t *testing.T) {
		dir := t.TempDir()
		incrementRetry(dir, "1-prd")
		count, err := incrementRetry(dir, "1-prd")
		if err != nil {
			t.Fatalf("incrementRetry: %v", err)
		}
		if count != 2 {
			t.Errorf("second increment = %d, want 2", count)
		}
	})

	t.Run("reset removes counter", func(t *testing.T) {
		dir := t.TempDir()
		incrementRetry(dir, "1-prd")
		if err := resetRetry(dir, "1-prd"); err != nil {
			t.Fatalf("resetRetry: %v", err)
		}
		state, _ := readCheckState(dir)
		if v, ok := state["1-prd"]; ok {
			t.Errorf("check-state.json[1-prd] after reset = %d, want key removed", v)
		}
	})

	t.Run("per-stage isolation", func(t *testing.T) {
		dir := t.TempDir()
		// Pre-populate two stages
		initial := map[string]int{"1-prd": 1, "8-test-plan": 0}
		if err := writeCheckState(dir, initial); err != nil {
			t.Fatalf("writeCheckState: %v", err)
		}
		// Increment only 1-prd
		incrementRetry(dir, "1-prd")
		state, _ := readCheckState(dir)
		if state["1-prd"] != 2 {
			t.Errorf("1-prd after increment = %d, want 2", state["1-prd"])
		}
		if state["8-test-plan"] != 0 {
			t.Errorf("8-test-plan should be unchanged at 0, got %d", state["8-test-plan"])
		}
	})

	t.Run("readCheckState returns empty map if file absent", func(t *testing.T) {
		dir := t.TempDir()
		state, err := readCheckState(dir)
		if err != nil {
			t.Fatalf("readCheckState: %v", err)
		}
		if len(state) != 0 {
			t.Errorf("expected empty map, got %v", state)
		}
	})
}

// ---------- TC-020/021/022/023: Auto-discovery ----------

func TestFindFeatureDir(t *testing.T) {
	t.Run("finds by top-level stage field", func(t *testing.T) {
		root, featureDir := makeFeatureDir(t, "my-feature", "1-prd")
		got, err := findFeatureDirByStage(root, "1-prd")
		if err != nil {
			t.Fatalf("findFeatureDirByStage: %v", err)
		}
		if got != featureDir {
			t.Errorf("got %q, want %q", got, featureDir)
		}
	})

	t.Run("finds by pipeline stage status in-progress", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "my-feature")
		os.MkdirAll(featureDir, 0o755)
		// Top-level stage does NOT match, but pipeline has in-progress
		statusJSON := `{"feature":"my-feature","stage":"5-tech-spec","pipeline":{"8-test-plan":{"status":"in-progress"}}}`
		writeFile(t, filepath.Join(featureDir, "status.json"), statusJSON)

		got, err := findFeatureDirByStage(root, "8-test-plan")
		if err != nil {
			t.Fatalf("findFeatureDirByStage: %v", err)
		}
		if got != featureDir {
			t.Errorf("got %q, want %q", got, featureDir)
		}
	})

	t.Run("returns empty when no match", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "my-feature")
		os.MkdirAll(featureDir, 0o755)
		statusJSON := `{"feature":"my-feature","stage":"5-tech-spec","pipeline":{}}`
		writeFile(t, filepath.Join(featureDir, "status.json"), statusJSON)

		got, err := findFeatureDirByStage(root, "1-prd")
		if err != nil {
			t.Fatalf("findFeatureDirByStage: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("multiple features returns most recently updated", func(t *testing.T) {
		root := t.TempDir()

		featureA := filepath.Join(root, ".claude", "feature", "feature-a")
		featureB := filepath.Join(root, ".claude", "feature", "feature-b")
		os.MkdirAll(featureA, 0o755)
		os.MkdirAll(featureB, 0o755)

		// feature-a has older timestamp
		writeFile(t, filepath.Join(featureA, "status.json"),
			`{"feature":"feature-a","stage":"1-prd","updated":"2026-01-01T00:00:00Z","pipeline":{"1-prd":{"status":"in-progress"}}}`)
		// feature-b has newer timestamp
		writeFile(t, filepath.Join(featureB, "status.json"),
			`{"feature":"feature-b","stage":"1-prd","updated":"2026-03-25T00:00:00Z","pipeline":{"1-prd":{"status":"in-progress"}}}`)

		got, err := findFeatureDirByStage(root, "1-prd")
		if err != nil {
			t.Fatalf("findFeatureDirByStage: %v", err)
		}
		if got != featureB {
			t.Errorf("got %q, want %q (most recently updated feature-b)", got, featureB)
		}
	})

	t.Run("no features directory returns empty", func(t *testing.T) {
		root := t.TempDir() // No .claude/feature/ at all
		got, err := findFeatureDirByStage(root, "1-prd")
		if err != nil {
			t.Fatalf("findFeatureDirByStage: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

// ---------- TC-060/061/062/063: Stage 11 dispatch ----------

func TestDispatch(t *testing.T) {
	t.Run("cassandra with risk-analysis.md passes", func(t *testing.T) {
		root, featureDir := makeFeatureDir(t, "review-feature", "11-review")
		writeFile(t, filepath.Join(featureDir, "risk-analysis.md"), "Risk analysis content")

		var output string
		pipeStdin(makeStopStdin("kratos:cassandra", root, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("11-review", "review-feature")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if !resp.OK {
			t.Errorf("cassandra with risk-analysis.md should pass, got ok=false reason=%q", resp.Reason)
		}
	})

	t.Run("hermes returns ok immediately (no Tier 1 check)", func(t *testing.T) {
		root, _ := makeFeatureDir(t, "review-feature", "11-review")
		// No risk-analysis.md or code-review.md in featureDir

		var output string
		pipeStdin(makeStopStdin("kratos:hermes", root, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("11-review", "review-feature")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if !resp.OK {
			t.Errorf("hermes at stage 11 should return ok=true (no Tier 1 check), got ok=false")
		}
	})

	t.Run("cassandra fails when risk-analysis.md missing", func(t *testing.T) {
		root, _ := makeFeatureDir(t, "review-feature", "11-review")
		// No risk-analysis.md

		var output string
		pipeStdin(makeStopStdin("kratos:cassandra", root, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("11-review", "review-feature")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if resp.OK {
			t.Error("cassandra without risk-analysis.md should fail")
		}
		if !strings.Contains(resp.Reason, "risk-analysis.md") {
			t.Errorf("reason should mention risk-analysis.md, got: %q", resp.Reason)
		}
	})

	t.Run("unknown agent_type at stage 11 returns ok (fail-open)", func(t *testing.T) {
		root, _ := makeFeatureDir(t, "review-feature", "11-review")

		var output string
		pipeStdin(makeStopStdin("kratos:unknown-agent", root, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("11-review", "review-feature")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if !resp.OK {
			t.Error("unknown agent_type at stage 11 should fail-open (ok=true)")
		}
	})
}

// ---------- TC-064/065: Optional stage skip ----------

func TestOptional(t *testing.T) {
	t.Run("skipped stage returns ok without file check", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "test-feature")
		os.MkdirAll(featureDir, 0o755)
		// Stage 3 is skipped
		statusJSON := `{"feature":"test-feature","stage":"3-decomposition","pipeline":{"3-decomposition":{"status":"skipped"}}}`
		writeFile(t, filepath.Join(featureDir, "status.json"), statusJSON)
		// No decomposition.md present

		var output string
		pipeStdin(makeStopStdin("kratos:daedalus", root, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("3-decomposition", "test-feature")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if !resp.OK {
			t.Error("skipped optional stage should return ok=true without file checks")
		}
	})

	t.Run("non-skipped optional stage checks files", func(t *testing.T) {
		root := t.TempDir()
		featureDir := filepath.Join(root, ".claude", "feature", "test-feature")
		os.MkdirAll(featureDir, 0o755)
		// Stage 3 is complete (not skipped)
		statusJSON := `{"feature":"test-feature","stage":"3-decomposition","pipeline":{"3-decomposition":{"status":"complete"}}}`
		writeFile(t, filepath.Join(featureDir, "status.json"), statusJSON)
		// No decomposition.md

		var output string
		pipeStdin(makeStopStdin("kratos:daedalus", root, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("3-decomposition", "test-feature")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if resp.OK {
			t.Error("non-skipped stage with missing file should return ok=false")
		}
		if !strings.Contains(resp.Reason, "decomposition.md") {
			t.Errorf("reason should mention decomposition.md, got: %q", resp.Reason)
		}
	})
}

// ---------- TC-014/015/016/017: Fail-open cases ----------

func TestFailOpen(t *testing.T) {
	t.Run("stop_hook_active returns ok immediately", func(t *testing.T) {
		dir := t.TempDir()
		var output string
		pipeStdin(makeStopStdin("kratos:athena", dir, true), func() {
			output = captureStdout(func() {
				handleCheckVerify("1-prd", "")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if !resp.OK {
			t.Error("stop_hook_active=true should return ok=true immediately")
		}
	})

	t.Run("unknown stage returns ok", func(t *testing.T) {
		dir := t.TempDir()
		var output string
		pipeStdin(makeStopStdin("kratos:athena", dir, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("99-nonexistent", "")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if !resp.OK {
			t.Error("unknown stage should fail-open (ok=true)")
		}
	})

	t.Run("malformed stdin JSON returns ok", func(t *testing.T) {
		var output string
		pipeStdin("{not valid json", func() {
			output = captureStdout(func() {
				handleCheckVerify("1-prd", "")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if !resp.OK {
			t.Error("malformed stdin should fail-open (ok=true)")
		}
	})

	t.Run("missing feature dir returns ok", func(t *testing.T) {
		dir := t.TempDir() // No .claude/feature/ structure
		var output string
		pipeStdin(makeStopStdin("kratos:athena", dir, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("1-prd", "")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if !resp.OK {
			t.Error("missing feature dir should fail-open (ok=true)")
		}
	})
}

// ---------- TC-001/002/003: handleCheckInit ----------

func TestCheckInit(t *testing.T) {
	t.Run("known stage returns SubagentStart JSON with deliverables", func(t *testing.T) {
		root, _ := makeFeatureDir(t, "test-feature", "1-prd")
		var output string
		pipeStdin(makeStartStdin("x", "kratos:athena", root), func() {
			output = captureStdout(func() {
				handleCheckInit("1-prd", "test-feature")
			})
		})

		var resp subagentStartOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if resp.HookSpecificOutput.HookEventName != "SubagentStart" {
			t.Errorf("hookEventName = %q, want SubagentStart", resp.HookSpecificOutput.HookEventName)
		}
		ctx := resp.HookSpecificOutput.AdditionalContext
		if !strings.Contains(ctx, "VERIFICATION GATE ACTIVE") {
			t.Error("additionalContext should contain 'VERIFICATION GATE ACTIVE'")
		}
		if !strings.Contains(ctx, "prd.md") {
			t.Error("additionalContext should mention prd.md")
		}
		if !strings.Contains(ctx, "decisions.md") {
			t.Error("additionalContext should mention decisions.md")
		}
	})

	t.Run("review stage mentions verdict requirement", func(t *testing.T) {
		root, _ := makeFeatureDir(t, "test-feature", "2-prd-review")
		var output string
		pipeStdin(makeStartStdin("x", "kratos:athena", root), func() {
			output = captureStdout(func() {
				handleCheckInit("2-prd-review", "test-feature")
			})
		})

		var resp subagentStartOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}
		ctx := resp.HookSpecificOutput.AdditionalContext
		if !strings.Contains(ctx, "prd-review.md") {
			t.Error("additionalContext should mention prd-review.md")
		}
		// Should mention verdict keywords
		if !strings.Contains(ctx, "approved") && !strings.Contains(ctx, "verdict") {
			t.Error("additionalContext should mention verdict requirement")
		}
	})

	t.Run("unknown stage returns empty context (fail-open)", func(t *testing.T) {
		dir := t.TempDir()
		var output string
		pipeStdin(makeStartStdin("x", "kratos:athena", dir), func() {
			output = captureStdout(func() {
				handleCheckInit("99-nonexistent", "")
			})
		})

		var resp subagentStartOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
		}
		if resp.HookSpecificOutput.HookEventName != "SubagentStart" {
			t.Errorf("hookEventName = %q, want SubagentStart", resp.HookSpecificOutput.HookEventName)
		}
		if resp.HookSpecificOutput.AdditionalContext != "" {
			t.Errorf("unknown stage should have empty additionalContext, got: %q",
				resp.HookSpecificOutput.AdditionalContext)
		}
	})

	t.Run("malformed stdin returns empty context (fail-open)", func(t *testing.T) {
		var output string
		pipeStdin("{not valid json", func() {
			output = captureStdout(func() {
				handleCheckInit("1-prd", "")
			})
		})

		var resp subagentStartOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}
		if resp.HookSpecificOutput.HookEventName != "SubagentStart" {
			t.Errorf("hookEventName = %q, want SubagentStart", resp.HookSpecificOutput.HookEventName)
		}
	})

	t.Run("stage 11 cassandra sees only risk-analysis.md", func(t *testing.T) {
		var output string
		pipeStdin(makeStartStdin("x", "kratos:cassandra", t.TempDir()), func() {
			output = captureStdout(func() {
				handleCheckInit("11-review", "")
			})
		})

		var resp subagentStartOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}
		ctx := resp.HookSpecificOutput.AdditionalContext
		if !strings.Contains(ctx, "risk-analysis.md") {
			t.Error("Cassandra context should mention risk-analysis.md")
		}
		if strings.Contains(ctx, "code-review.md") {
			t.Error("Cassandra context should NOT mention code-review.md (that's Hermes's deliverable)")
		}
	})

	t.Run("stage 11 unknown agent type returns empty context (fail-open)", func(t *testing.T) {
		var output string
		pipeStdin(makeStartStdin("x", "kratos:unknown-agent", t.TempDir()), func() {
			output = captureStdout(func() {
				handleCheckInit("11-review", "")
			})
		})

		var resp subagentStartOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}
		if resp.HookSpecificOutput.AdditionalContext != "" {
			t.Errorf("unknown agent at stage 11 should have empty context, got: %q",
				resp.HookSpecificOutput.AdditionalContext)
		}
	})
}

// ---------- TC-041: recordCheckFailure ----------

func TestRecordCheckFailure(t *testing.T) {
	t.Run("appends check_failures to status.json", func(t *testing.T) {
		dir := t.TempDir()
		// Create a minimal status.json
		statusJSON := `{"feature":"test","pipeline":{"1-prd":{"status":"in-progress"}}}`
		writeFile(t, filepath.Join(dir, "status.json"), statusJSON)

		failedChecks := []string{"prd.md not found"}
		if err := recordCheckFailure(dir, "1-prd", 1, failedChecks); err != nil {
			t.Fatalf("recordCheckFailure: %v", err)
		}

		// Read back and verify
		status, err := readStatusJSON(filepath.Join(dir, "status.json"))
		if err != nil {
			t.Fatalf("readStatusJSON: %v", err)
		}
		pipeline, _ := status["pipeline"].(map[string]interface{})
		stageData, _ := pipeline["1-prd"].(map[string]interface{})
		failures, ok := stageData["check_failures"].([]interface{})
		if !ok || len(failures) == 0 {
			t.Fatal("check_failures array should be present and non-empty")
		}

		entry, _ := failures[0].(map[string]interface{})
		if entry["tier"].(float64) != 1 {
			t.Errorf("tier = %v, want 1", entry["tier"])
		}
		if entry["retries_exhausted"] != true {
			t.Errorf("retries_exhausted = %v, want true", entry["retries_exhausted"])
		}
		checksArr, _ := entry["checks_failed"].([]interface{})
		if len(checksArr) == 0 {
			t.Error("checks_failed should be non-empty")
		}
		if entry["timestamp"] == nil || entry["timestamp"] == "" {
			t.Error("timestamp should be set")
		}
	})
}

// ---------- Integration tests ----------

func TestCheckIntegration(t *testing.T) {
	t.Run("TC-100: full init verify lifecycle happy path", func(t *testing.T) {
		root, featureDir := makeFeatureDir(t, "test-feature", "1-prd")

		// Step 1: --init
		var initOutput string
		pipeStdin(makeStartStdin("x", "kratos:athena", root), func() {
			initOutput = captureStdout(func() {
				handleCheckInit("1-prd", "test-feature")
			})
		})

		var initResp subagentStartOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(initOutput)), &initResp); err != nil {
			t.Fatalf("--init output not valid JSON: %v\noutput: %q", err, initOutput)
		}
		ctx := initResp.HookSpecificOutput.AdditionalContext
		if !strings.Contains(ctx, "VERIFICATION GATE ACTIVE") {
			t.Error("--init should mention VERIFICATION GATE ACTIVE")
		}
		if !strings.Contains(ctx, "prd.md") {
			t.Error("--init should mention prd.md")
		}

		// Step 2: write files
		writeFile(t, filepath.Join(featureDir, "prd.md"), "# PRD content")
		writeFile(t, filepath.Join(featureDir, "decisions.md"), "# Decisions")

		// Step 3: --verify (should pass)
		var verifyOutput string
		pipeStdin(makeStopStdin("kratos:athena", root, false), func() {
			verifyOutput = captureStdout(func() {
				handleCheckVerify("1-prd", "test-feature")
			})
		})

		var verifyResp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(verifyOutput)), &verifyResp); err != nil {
			t.Fatalf("--verify output not valid JSON: %v\noutput: %q", err, verifyOutput)
		}
		if !verifyResp.OK {
			t.Errorf("--verify with all files present should pass, got ok=false reason=%q", verifyResp.Reason)
		}

		// Confirm no check-state.json or counter is 0
		state, _ := readCheckState(featureDir)
		if state["1-prd"] != 0 {
			t.Errorf("counter should be 0 after success, got %d", state["1-prd"])
		}
	})

	t.Run("TC-101: block then succeed lifecycle", func(t *testing.T) {
		root, featureDir := makeFeatureDir(t, "test-feature", "8-test-plan")
		stopStdin := makeStopStdin("kratos:artemis", root, false)

		// First verify: no test-plan.md → should block
		var out1 string
		pipeStdin(stopStdin, func() {
			out1 = captureStdout(func() {
				handleCheckVerify("8-test-plan", "test-feature")
			})
		})

		var resp1 subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(out1)), &resp1); err != nil {
			t.Fatalf("resp1 not valid JSON: %v", err)
		}
		if resp1.OK {
			t.Error("first verify with no file should block (ok=false)")
		}
		if !strings.Contains(resp1.Reason, "attempt 1/2") {
			t.Errorf("reason should say attempt 1/2, got: %q", resp1.Reason)
		}

		// Write the file
		writeFile(t, filepath.Join(featureDir, "test-plan.md"), "# Test plan")

		// Second verify: file present → should pass
		var out2 string
		pipeStdin(stopStdin, func() {
			out2 = captureStdout(func() {
				handleCheckVerify("8-test-plan", "test-feature")
			})
		})

		var resp2 subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(out2)), &resp2); err != nil {
			t.Fatalf("resp2 not valid JSON: %v", err)
		}
		if !resp2.OK {
			t.Errorf("second verify with file present should pass, got ok=false reason=%q", resp2.Reason)
		}

		// Counter should be reset
		state, _ := readCheckState(featureDir)
		if _, ok := state["8-test-plan"]; ok {
			t.Errorf("counter should be removed after success, got %d", state["8-test-plan"])
		}
	})

	t.Run("TC-102: max retries exhausted records failure and returns ok", func(t *testing.T) {
		root, featureDir := makeFeatureDir(t, "test-feature", "8-test-plan")
		stopStdin := makeStopStdin("kratos:artemis", root, false)

		// Attempt 1 → ok=false (attempt 1/2)
		var out1 string
		pipeStdin(stopStdin, func() {
			out1 = captureStdout(func() {
				handleCheckVerify("8-test-plan", "test-feature")
			})
		})
		var r1 subagentStopOutput
		json.Unmarshal([]byte(strings.TrimSpace(out1)), &r1)
		if r1.OK {
			t.Error("attempt 1 should block")
		}

		// Attempt 2 → ok=false (attempt 2/2)
		var out2 string
		pipeStdin(stopStdin, func() {
			out2 = captureStdout(func() {
				handleCheckVerify("8-test-plan", "test-feature")
			})
		})
		var r2 subagentStopOutput
		json.Unmarshal([]byte(strings.TrimSpace(out2)), &r2)
		if r2.OK {
			t.Error("attempt 2 should block")
		}
		if !strings.Contains(r2.Reason, "attempt 2/2") {
			t.Errorf("attempt 2 reason should say attempt 2/2, got: %q", r2.Reason)
		}

		// Attempt 3 → ok=true (max exhausted)
		var out3 string
		pipeStdin(stopStdin, func() {
			out3 = captureStdout(func() {
				handleCheckVerify("8-test-plan", "test-feature")
			})
		})
		var r3 subagentStopOutput
		json.Unmarshal([]byte(strings.TrimSpace(out3)), &r3)
		if !r3.OK {
			t.Errorf("attempt 3 (max exhausted) should return ok=true, got ok=false reason=%q", r3.Reason)
		}

		// Verify check_failures written to status.json
		status, err := readStatusJSON(filepath.Join(featureDir, "status.json"))
		if err != nil {
			t.Fatalf("readStatusJSON: %v", err)
		}
		pipeline, _ := status["pipeline"].(map[string]interface{})
		stageData, _ := pipeline["8-test-plan"].(map[string]interface{})
		failures, ok := stageData["check_failures"].([]interface{})
		if !ok || len(failures) == 0 {
			t.Fatal("check_failures should be recorded in status.json after max retries")
		}
		entry := failures[0].(map[string]interface{})
		if entry["retries_exhausted"] != true {
			t.Error("retries_exhausted should be true")
		}
		if entry["tier"].(float64) != 1 {
			t.Errorf("tier should be 1, got %v", entry["tier"])
		}

		// Counter should be reset
		state, _ := readCheckState(featureDir)
		if _, ok := state["8-test-plan"]; ok {
			t.Errorf("counter should be reset after exhaustion, got %d", state["8-test-plan"])
		}
	})

	t.Run("TC-103: explicit --feature flag bypasses auto-discovery", func(t *testing.T) {
		root := t.TempDir()

		// feature-a HAS prd.md
		featureA := filepath.Join(root, ".claude", "feature", "feature-a")
		os.MkdirAll(featureA, 0o755)
		writeFile(t, filepath.Join(featureA, "status.json"),
			`{"feature":"feature-a","stage":"1-prd","pipeline":{"1-prd":{"status":"in-progress"}}}`)
		writeFile(t, filepath.Join(featureA, "prd.md"), "PRD content")
		writeFile(t, filepath.Join(featureA, "decisions.md"), "Decisions content")

		// feature-b does NOT have prd.md
		featureB := filepath.Join(root, ".claude", "feature", "feature-b")
		os.MkdirAll(featureB, 0o755)
		writeFile(t, filepath.Join(featureB, "status.json"),
			`{"feature":"feature-b","stage":"1-prd","pipeline":{"1-prd":{"status":"in-progress"}}}`)

		// Explicitly target feature-a — should pass
		var output string
		pipeStdin(makeStopStdin("kratos:athena", root, false), func() {
			output = captureStdout(func() {
				handleCheckVerify("1-prd", "feature-a")
			})
		})

		var resp subagentStopOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
			t.Fatalf("output not valid JSON: %v", err)
		}
		if !resp.OK {
			t.Errorf("explicit --feature=feature-a with files present should pass, got ok=false reason=%q", resp.Reason)
		}
	})

	t.Run("TC-104: check-state.json uses atomic write", func(t *testing.T) {
		root, featureDir := makeFeatureDir(t, "test-feature", "1-prd")

		pipeStdin(makeStopStdin("kratos:athena", root, false), func() {
			captureStdout(func() {
				handleCheckVerify("1-prd", "test-feature")
			})
		})

		// Verify check-state.json exists and is valid JSON
		stateFile := filepath.Join(featureDir, "check-state.json")
		data, err := os.ReadFile(stateFile)
		if err != nil {
			t.Fatalf("check-state.json should exist after failed verify: %v", err)
		}
		var state map[string]int
		if err := json.Unmarshal(data, &state); err != nil {
			t.Errorf("check-state.json should be valid JSON: %v\ncontent: %q", err, string(data))
		}
		// No tmp file should remain
		if _, err := os.Stat(stateFile + ".tmp"); !os.IsNotExist(err) {
			t.Error("check-state.json.tmp should not remain after atomic write")
		}
	})
}

// ---------- TC-035: H-001 verdict anchoring ----------

func TestVerdictAnchoring(t *testing.T) {
	t.Run("incidental keyword before verdict section does not pass (H-001)", func(t *testing.T) {
		dir := t.TempDir()
		// "approved" appears in a quoted historical note far from the verdict section.
		// 50 lines × 20 bytes ≈ 1000 bytes, pushing "approved" well outside the 500-byte tail.
		prefix := "History: the original PRD was approved in sprint 1.\n" + strings.Repeat("review content line\n", 50)
		suffix := "\n## Verdict\n\nRevisions required."
		writeFile(t, filepath.Join(dir, "prd-review.md"), prefix+suffix)

		// Should fail for "approved" (keyword not in last 500 bytes)
		err := verifyVerdictPresent(dir, "prd-review.md", []string{"approved"})
		if err == nil {
			t.Error("'approved' appearing only in early content should not pass verdict check")
		}
		// Should pass for "revisions" (keyword IS in last 500 bytes)
		err = verifyVerdictPresent(dir, "prd-review.md", []string{"approved", "revisions", "rejected"})
		if err != nil {
			t.Errorf("'revisions' in verdict section should pass: %v", err)
		}
	})

	t.Run("short document searches full content", func(t *testing.T) {
		dir := t.TempDir()
		// Document shorter than 500 bytes — full content is the tail
		writeFile(t, filepath.Join(dir, "prd-review.md"), "## Verdict: Approved")
		err := verifyVerdictPresent(dir, "prd-review.md", []string{"approved", "revisions", "rejected"})
		if err != nil {
			t.Errorf("short document with 'approved' should pass: %v", err)
		}
	})
}

// ---------- TC-025: M-001 detectCurrentStage ordering ----------

func TestDetectCurrentStageOrdering(t *testing.T) {
	t.Run("returns stage from most-recently-updated feature (M-001)", func(t *testing.T) {
		root := t.TempDir()

		featureA := filepath.Join(root, ".claude", "feature", "aaa-feature")
		featureB := filepath.Join(root, ".claude", "feature", "zzz-feature")
		os.MkdirAll(featureA, 0o755)
		os.MkdirAll(featureB, 0o755)

		// aaa-feature is lexicographically first but updated earlier
		writeFile(t, filepath.Join(featureA, "status.json"),
			`{"feature":"aaa-feature","stage":"1-prd","updated":"2026-01-01T00:00:00Z"}`)
		// zzz-feature is lexicographically last but updated more recently
		writeFile(t, filepath.Join(featureB, "status.json"),
			`{"feature":"zzz-feature","stage":"8-test-plan","updated":"2026-03-25T00:00:00Z"}`)

		stage, err := detectCurrentStage(root, "")
		if err != nil {
			t.Fatalf("detectCurrentStage: %v", err)
		}
		// Should return "8-test-plan" from zzz-feature (most recently updated), not "1-prd" from aaa-feature
		if stage != "8-test-plan" {
			t.Errorf("detectCurrentStage = %q, want %q (most recently updated feature)", stage, "8-test-plan")
		}
	})
}

// ---------- TC-018: M-003 path traversal prevention ----------

func TestResolveFeatureDirPathTraversal(t *testing.T) {
	t.Run("rejects path traversal sequences (M-003)", func(t *testing.T) {
		root := t.TempDir()
		_, err := resolveFeatureDir(root, "../../../etc/passwd", "1-prd")
		if err == nil {
			t.Error("expected error for path traversal in --feature flag")
		}
	})

	t.Run("rejects feature name with slashes", func(t *testing.T) {
		root := t.TempDir()
		_, err := resolveFeatureDir(root, "foo/bar", "1-prd")
		if err == nil {
			t.Error("expected error for feature name containing slash")
		}
	})

	t.Run("accepts valid feature name", func(t *testing.T) {
		root, featureDir := makeFeatureDir(t, "my-feature-01", "1-prd")
		got, err := resolveFeatureDir(root, "my-feature-01", "1-prd")
		if err != nil {
			t.Fatalf("resolveFeatureDir: %v", err)
		}
		if got != featureDir {
			t.Errorf("got %q, want %q", got, featureDir)
		}
	})
}

// ---------- TC-080/081: Backward compatibility ----------

func TestBackwardCompatibility(t *testing.T) {
	t.Run("stageChecks does not contain hephaestus stages", func(t *testing.T) {
		// Hephaestus is excluded from Tier 1 (dual-hook interaction risk)
		if _, ok := stageChecks["5-tech-spec"]; ok {
			t.Error("5-tech-spec should NOT be in stageChecks (Hephaestus excluded from Tier 1)")
		}
	})

	t.Run("stageChecks does not contain ares stage", func(t *testing.T) {
		if _, ok := stageChecks["9-implementation"]; ok {
			t.Error("9-implementation should NOT be in stageChecks (Tier 2 deferred)")
		}
	})
}
