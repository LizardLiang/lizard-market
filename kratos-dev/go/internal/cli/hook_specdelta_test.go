package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func specDeltaPayload(filePath string) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"tool_name":  "Write",
		"tool_input": map[string]string{"file_path": filePath},
	})
	return b
}

func TestSpecDeltaPathRE(t *testing.T) {
	cases := []struct {
		path    string
		feature string
	}{
		{`.claude/feature/my-feat/spec-delta/auth.md`, "my-feat"},
		{`C:\Users\x\proj\.claude\feature\my-feat\spec-delta\auth.md`, "my-feat"},
		{`/home/x/proj/.claude/feature/a-b_c/spec-delta/cap.md`, "a-b_c"},
		{`.claude/feature/my-feat/spec-delta/archived/auth.md`, ""}, // archived deltas are not pending
		{`.claude/feature/my-feat/prd.md`, ""},
		{`src/app/spec-delta/notes.md`, ""},
		{`.claude/feature/my-feat/spec-delta/auth.txt`, ""},
	}
	for _, c := range cases {
		m := specDeltaPathRE.FindStringSubmatch(c.path)
		got := ""
		if m != nil {
			got = m[1]
		}
		if got != c.feature {
			t.Errorf("path %q: got feature %q, want %q", c.path, got, c.feature)
		}
	}
}

func TestSpecDeltaCheck_ValidDeltaSilent(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)
	setupFeatureDelta(t, root, "feat-mfa", "auth", validAddedOnlyDelta)

	var out bytes.Buffer
	payload := specDeltaPayload(".claude/feature/feat-mfa/spec-delta/auth.md")
	if err := specDeltaCheckIn(root, payload, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected silent pass, got output: %s", out.String())
	}
}

func TestSpecDeltaCheck_ProseDeltaBlocks(t *testing.T) {
	root := t.TempDir()
	setupFeatureDelta(t, root, "feat-bad", "auth",
		"# Auth changes\n\nThis feature adds a popover to the toolbar and keeps behavior the same.\n")

	var out bytes.Buffer
	payload := specDeltaPayload(`.claude\feature\feat-bad\spec-delta\auth.md`)
	if err := specDeltaCheckIn(root, payload, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp map[string]string
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("expected block JSON, got %q: %v", out.String(), err)
	}
	if resp["decision"] != "block" {
		t.Errorf("expected decision=block, got %+v", resp)
	}
	if !strings.Contains(resp["reason"], "feat-bad") {
		t.Errorf("reason should name the feature, got %q", resp["reason"])
	}
}

func TestSpecDeltaCheck_NonDeltaPathSilent(t *testing.T) {
	var out bytes.Buffer
	if err := specDeltaCheckIn(t.TempDir(), specDeltaPayload("src/app/main.ts"), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected silence for non-delta path, got %s", out.String())
	}
}

func TestSpecDeltaCheck_MissingFeatureFailsOpen(t *testing.T) {
	var out bytes.Buffer
	payload := specDeltaPayload(".claude/feature/ghost/spec-delta/auth.md")
	if err := specDeltaCheckIn(t.TempDir(), payload, &out); err != nil {
		t.Fatalf("expected fail-open nil error, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected silence when feature dir missing, got %s", out.String())
	}
}

func TestSpecDeltaCheck_GarbagePayloadFailsOpen(t *testing.T) {
	var out bytes.Buffer
	if err := specDeltaCheckIn(t.TempDir(), []byte("not json"), &out); err != nil {
		t.Fatalf("expected fail-open nil error, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected silence on garbage payload, got %s", out.String())
	}
}
