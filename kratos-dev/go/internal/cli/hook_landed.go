package cli

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// landedLineRE matches the line Ares must end with once its files are
// committed: `Landed: <branch>@<hash>` (branch optional, 7-40 hex hash).
var landedLineRE = regexp.MustCompile(`(?im)^\s*\**landed\**\s*:\**\s*(?:([\w./-]+)@)?([0-9a-f]{7,40})\b`)

// landedWaiverRE is the only accepted waiver: a mission that changed no files
// (User Mode task creation, a pure report) or a directory that is not a git
// repository.
var landedWaiverRE = regexp.MustCompile(`(?i)landed-not-applicable:`)

// aresLandedGateFailure enforces the landed-work rule at Ares's SubagentStop:
// the final message must name the commit the work landed in (or waive with
// LANDED-NOT-APPLICABLE: <reason>), and when a cwd is known and is a git
// repository, that commit must exist there. Uncommitted work "pending manual
// check" lost an entire Ares+Hermes cycle on LizMeter #63 (2026-09 review).
//
// The message-level rule always applies: Ares must say where the work landed
// or say explicitly why nothing could land. Infrastructure problems (no git on
// PATH, cwd unknown or outside a work tree) fail OPEN only on the commit
// verification.
func aresLandedGateFailure(input subagentStopInput) string {
	msg := input.LastAssistantMessage
	if landedWaiverRE.MatchString(msg) {
		return ""
	}

	m := landedLineRE.FindStringSubmatch(msg)
	if m == nil {
		return "no `Landed: <branch>@<hash>` line — stage exactly the files you created or modified (git add <files>), commit them on the current branch, and report the hash; or state LANDED-NOT-APPLICABLE: <reason> when the mission changed no files or the directory is not a git repository"
	}
	hash := m[2]
	if input.Cwd == "" || !isGitWorkTree(input.Cwd) {
		return ""
	}
	if err := gitCommitExists(input.Cwd, hash); err != nil {
		return fmt.Sprintf("Landed: names commit %s but git cannot find it in %s (%v) — commit for real and report the new hash", hash, input.Cwd, err)
	}
	return ""
}

// isGitWorkTree reports whether dir is inside a git work tree. Missing git is
// treated as "unknown → assume a work tree" so the message rule still applies.
func isGitWorkTree(dir string) bool {
	git, err := exec.LookPath("git")
	if err != nil {
		return true
	}
	out, err := exec.Command(git, "-C", dir, "rev-parse", "--is-inside-work-tree").CombinedOutput()
	return err == nil && strings.Contains(string(out), "true")
}

// gitCommitExists returns nil when hash names a commit reachable in dir's
// repository. A missing git binary fails open (nil).
func gitCommitExists(dir, hash string) error {
	git, err := exec.LookPath("git")
	if err != nil {
		return nil
	}
	out, err := exec.Command(git, "-C", dir, "cat-file", "-e", hash+"^{commit}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
