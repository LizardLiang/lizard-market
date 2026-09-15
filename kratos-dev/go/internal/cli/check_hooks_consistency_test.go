package cli

import (
	"encoding/json"
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
	hooksPath := filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks", "hooks.json")
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

// TestHooksJSONTimeoutsAreSeconds guards against hook `timeout` values regressing to
// milliseconds. Claude Code documents the field as "Seconds before canceling"
// (https://code.claude.com/docs/en/hooks.md); a leftover millisecond value (e.g. 5000)
// silently becomes an 83-minute timeout instead of 5 seconds — the exact bug this test
// pins (plugins/kratos/hooks/hooks.json used to carry 1500/3000/5000/10000). The
// threshold is 600: Claude Code's own longest documented default timeout, in seconds,
// for a command/http/mcp_tool hook. No legitimate seconds-based value in this file
// should ever need to reach that, and any leftover ms value trips it immediately.
func TestHooksJSONTimeoutsAreSeconds(t *testing.T) {
	hooksPath := filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("cannot read hooks.json at %s: %v", hooksPath, err)
	}

	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Timeout float64 `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("hooks.json is not valid JSON: %v", err)
	}

	const maxSeconds = 600
	found := 0
	for event, matchers := range doc.Hooks {
		for _, matcher := range matchers {
			for _, h := range matcher.Hooks {
				found++
				if h.Timeout >= maxSeconds {
					t.Errorf("hooks.json %s hook has timeout %v — looks like milliseconds, not seconds (must be < %d)", event, h.Timeout, maxSeconds)
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("no timeout fields found in hooks.json; JSON shape or file layout changed")
	}
}
