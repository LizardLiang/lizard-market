package cli

import (
	"fmt"
	"path/filepath"
	"strings"
)

// specArchiveGuard refuses to promote a feature's spec delta into the living
// spec while its stage-9 review is still running: the archived spec inherits
// the reviewers' verdicts, and a delta archived seven seconds after Hermes was
// spawned promoted behaviour nobody had approved yet (2026-08-31). Plan-only
// folders (no status.json) and finished features pass; --force overrides.
func specArchiveGuard(root, feature string, force bool) error {
	if force {
		return nil
	}
	status, err := readStatusJSON(filepath.Join(root, ".claude", "feature", feature, "status.json"))
	if err != nil {
		return nil
	}
	pipeline, _ := status["pipeline"].(map[string]interface{})
	stage, _ := pipeline["9-review"].(map[string]interface{})
	if st, _ := stage["status"].(string); strings.EqualFold(strings.TrimSpace(st), "in-progress") {
		return fmt.Errorf("stage 9 review is in progress for %q — archive after Hermes and Cassandra finish, or pass --force", feature)
	}
	return nil
}
