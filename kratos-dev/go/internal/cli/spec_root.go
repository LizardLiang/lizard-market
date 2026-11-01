package cli

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// specArenaRoot is the --arena-root override on the spec command group: the
// checkout whose .claude/ holds the living specs and the feature deltas.
var specArenaRoot string

// specRoot resolves the repository root the spec commands read and write.
//
// A tactical plan and its spec delta are written in the main checkout, while
// Ares lands the code in a worktree; `spec archive` run from the worktree
// then found the delta missing and the living spec untracked, and the
// orchestrator copied the feature dir across by hand (2026-10-25 16:28).
// Worktrees share one git common dir, so its parent — the main checkout —
// is the one place both sides agree on. --arena-root overrides it; a plain
// repository resolves to its own top level as before.
func specRoot() string {
	if specArenaRoot != "" {
		return specArenaRoot
	}
	if root := gitMainWorktreeRoot(); root != "" {
		return root
	}
	return gitRoot()
}

// gitMainWorktreeRoot returns the main checkout of the repository containing
// the working directory: the parent of `git rev-parse --git-common-dir`.
// "" when git is unavailable, the directory is not a repository, or the
// common dir is not a plain .git directory (a bare or detached layout).
func gitMainWorktreeRoot() string {
	out, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	common := strings.TrimSpace(string(out))
	if common == "" || filepath.Base(common) != ".git" {
		return ""
	}
	return filepath.Dir(common)
}
