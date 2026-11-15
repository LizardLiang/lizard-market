package cli

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Dirty-file baseline for `verify --landed`.
//
// ares.md tells Ares to run `git status --porcelain` before its first edit and
// treat every path already modified or untracked as the user's. The landed
// check had no copy of that list, so it counted the user's own untracked
// file as unlanded work (2026-10-20 15:53, `frontend/vite.walkthrough.config.ts`,
// present before Ares started) and the orchestrator learned to ignore a
// BLOCKED verdict — which makes the gate decorative. The Ares SubagentStart
// hook now records the snapshot per repository root under
// ~/.kratos/dirty-baseline/, and evaluateLanded skips the paths in it while
// it is fresh.

// dirtyBaselineMaxAge bounds how long a snapshot is trusted: a mission does
// not run for days, and a stale snapshot would hide real unlanded work.
const dirtyBaselineMaxAge = 48 * time.Hour

// dirtyBaseline is the stored snapshot.
type dirtyBaseline struct {
	Root       string   `json:"root"`
	RecordedAt int64    `json:"recorded_at"` // unix ms
	Paths      []string `json:"paths"`       // repo-root-relative, slash separated
}

// dirtyBaselineDir returns ~/.kratos/dirty-baseline, or "" when the home
// directory cannot be resolved. Tests override it through
// dirtyBaselineDirFn.
var dirtyBaselineDirFn = func() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".kratos", "dirty-baseline")
}

// gitTopLevel returns the repository root of dir as git prints it, slash
// separated, or "" when dir is not inside a work tree.
func gitTopLevel(dir string) string {
	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	out, err := exec.Command(git, "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(string(out), `\`, "/"))
}

func dirtyBaselinePath(root string) string {
	dir := dirtyBaselineDirFn()
	if dir == "" || root == "" {
		return ""
	}
	sum := sha1.Sum([]byte(strings.ToLower(root)))
	return filepath.Join(dir, hex.EncodeToString(sum[:8])+".json")
}

// porcelainPaths parses `git status --porcelain` output into repo-relative
// paths, keeping a rename's destination, exactly as evaluateLanded does.
func porcelainPaths(out string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		if i := strings.Index(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		p = strings.Trim(p, `"`)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// recordDirtyBaseline snapshots the dirty paths of the repository containing
// cwd. Every failure is silent: the baseline is an aid to verify --landed,
// never a reason to block a spawn.
func recordDirtyBaseline(cwd string) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	root := gitTopLevel(cwd)
	p := dirtyBaselinePath(root)
	if p == "" {
		return
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return
	}
	out, err := exec.Command(git, "-C", root, "status", "--porcelain", "--untracked-files=all").Output()
	if err != nil {
		return
	}
	b := dirtyBaseline{Root: root, RecordedAt: time.Now().UnixMilli(), Paths: porcelainPaths(string(out))}
	if b.Paths == nil {
		b.Paths = []string{}
	}
	data, err := json.Marshal(b)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		debugLog("dirty-baseline: write failed: %v", err)
	}
}

// loadDirtyBaseline returns the fresh baseline for the repository containing
// repoDir as a set of paths, or nil when there is none.
func loadDirtyBaseline(repoDir string) map[string]bool {
	root := gitTopLevel(repoDir)
	p := dirtyBaselinePath(root)
	if p == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var b dirtyBaseline
	if json.Unmarshal(data, &b) != nil {
		return nil
	}
	if time.Since(time.UnixMilli(b.RecordedAt)) > dirtyBaselineMaxAge {
		return nil
	}
	set := map[string]bool{}
	for _, path := range b.Paths {
		set[path] = true
	}
	return set
}
