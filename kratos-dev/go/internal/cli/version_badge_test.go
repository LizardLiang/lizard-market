package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestReadmeBadgeMatchesPluginVersion keeps the two places the version is
// written by hand together. The README badge stayed at 2.122.0 through two
// releases (v2.123, v2.124) because nothing compared them.
func TestReadmeBadgeMatchesPluginVersion(t *testing.T) {
	root := filepath.Join(hooksDirPath(), "..")
	manifest, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("cannot read plugin.json: %v", err)
	}
	var plugin struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(manifest, &plugin); err != nil || plugin.Version == "" {
		t.Fatalf("plugin.json has no version (err %v)", err)
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("cannot read README.md: %v", err)
	}
	m := regexp.MustCompile(`badge/version-([0-9][0-9A-Za-z.]*)-`).FindSubmatch(readme)
	if m == nil {
		t.Fatal("README.md has no version badge")
	}
	if string(m[1]) != plugin.Version {
		t.Errorf("README badge says %s, plugin.json says %s", m[1], plugin.Version)
	}
}
