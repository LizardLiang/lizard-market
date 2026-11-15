package gencmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRepoRoot walks up from start looking for plugins/kratos/agents, which
// only exists at the lizard-market monorepo root. Used by both the CLI and
// the drift test so neither depends on the caller's working directory.
func FindRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "plugins", "kratos", "agents")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate repo root (no plugins/kratos/agents found walking up from %s)", start)
		}
		dir = parent
	}
}
