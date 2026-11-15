package gencmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// pinnedModelIDRe matches hardcoded model IDs like claude-sonnet-4-6 or
// claude-haiku-4-5-20251001 — anything that isn't the haiku|sonnet|opus
// alias. See modes/modes.md and agents/*.md frontmatter for the alias
// routing scheme these pins should never bypass.
var pinnedModelIDRe = regexp.MustCompile(`claude-[a-z]+-[0-9]`)

// TestNoPinnedModelIDs bans hardcoded model IDs from plugins/kratos/**/*.md.
// Aliases resolve to the current model generation at spawn time; a pin goes
// stale the moment a newer model ships (the drift class the v2.41.0 "model
// upgrades" commit introduced, which this todo cleans up). If a specific
// agent genuinely needs to stay pinned, repin deliberately and extend this
// test with an explicit allowlist entry.
func TestNoPinnedModelIDs(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to resolve this test file's path")
	}
	repoRoot, err := FindRepoRoot(thisFile)
	if err != nil {
		t.Fatalf("locating repo root: %v", err)
	}

	pluginDir := filepath.Join(repoRoot, "plugins", "kratos")
	var hits []string
	err = filepath.Walk(pluginDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer f.Close()

		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		scanner := bufio.NewScanner(f)
		line := 0
		for scanner.Scan() {
			line++
			if pinnedModelIDRe.MatchString(scanner.Text()) {
				hits = append(hits, fmt.Sprintf("%s:%d", rel, line))
			}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("walking %s: %v", pluginDir, err)
	}
	sort.Strings(hits)

	if len(hits) > 0 {
		t.Errorf("%d pinned model ID(s) found in plugins/kratos/**/*.md — use the haiku|sonnet|opus alias instead:", len(hits))
		for _, h := range hits {
			t.Errorf("  %s", h)
		}
	}
}
