package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestHooksJSONStageKeysExist guards against stage renumbering drift: every
// `--stage <key>` literal passed by hooks/hooks.json must exist in stageChecks.
// handleCheckInit/handleCheckVerify fail open on unknown stages, so a stale key
// silently disables that agent's deliverable gate (this happened in v2.81-2.83:
// hooks.json passed 4-spec-review-sa/5-test-plan/7-prd-alignment/8-review while
// stageChecks used 5-/6-/8-/9- keys).
func TestHooksJSONStageKeysExist(t *testing.T) {
	hooksPath := filepath.Join("..", "..", "..", "hooks", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("cannot read hooks.json at %s: %v", hooksPath, err)
	}

	re := regexp.MustCompile(`--stage\s+([0-9]+-[a-z-]+)`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		t.Fatal("no --stage literals found in hooks.json; regex or file layout changed")
	}

	seen := map[string]bool{}
	for _, m := range matches {
		key := m[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, ok := stageChecks[key]; !ok {
			t.Errorf("hooks.json passes --stage %q but stageChecks has no such key (gate fails open silently)", key)
		}
	}
}
