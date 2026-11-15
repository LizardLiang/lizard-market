package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

// The validation root comes from the written file's path, not cwd: a session
// rooted in one repo that writes a delta into another project still gets the
// gate. With cwd's root the feature is missing and the gate fails open.
func TestSpecDeltaCheck_RootFromFilePath(t *testing.T) {
	cwdRoot := t.TempDir()
	project := t.TempDir()
	path := setupFeatureDelta(t, project, "feat-bad", "auth",
		"# Auth changes\n\nThis feature adds a popover to the toolbar and keeps behavior the same.\n")

	var out bytes.Buffer
	if err := specDeltaCheckIn(cwdRoot, specDeltaPayload(path), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp map[string]string
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("expected block JSON from the file's project, got %q: %v", out.String(), err)
	}
	if resp["decision"] != "block" {
		t.Errorf("expected decision=block, got %+v", resp)
	}
}

func TestSpecDeltaRoot(t *testing.T) {
	fallback := t.TempDir()
	project := t.TempDir()
	setupFeatureDelta(t, project, "feat", "auth", validAddedOnlyDelta)

	cases := []struct {
		name, prefix, want string
	}{
		{"absolute project", project + string(filepath.Separator), project},
		{"relative empty prefix", "", fallback},
		{"dir without .claude/feature", t.TempDir(), fallback},
	}
	for _, c := range cases {
		if got := specDeltaRoot(c.prefix, fallback); got != c.want {
			t.Errorf("%s: specDeltaRoot(%q) = %q, want %q", c.name, c.prefix, got, c.want)
		}
	}
}
