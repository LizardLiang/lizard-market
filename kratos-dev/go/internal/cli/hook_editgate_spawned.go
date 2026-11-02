package cli

import (
	"io"
	"os"
	"regexp"
	"strings"
)

// Rules for spawned agents, added after the 2026-10-20 → 10-31 transcript
// review. Until then a spawned non-Odysseus agent passed the edit gate
// untouched (editGateDecisionRest step 1). Three incidents that week were
// shell commands a prompt rule already forbade:
//
//   - 2026-10-23 14:47: an Ares `git stash push -q -m … -- <path>` created no
//     stash (flags misordered), and the following `git stash pop` applied the
//     user's own stash@{0} into the feature worktree — worktrees share one
//     stash list.
//   - 2026-10-24 09:11: Ares committed with `SKIP_LOCAL_CI=1`, bypassing the
//     pre-commit hook it was told to respect.
//   - 2026-10-21 07:00: a Hermes review child ran `git checkout -q --detach`
//     and left the user's worktree off its branch, although its prompt
//     carried the WORKING TREE IS READ-ONLY block word for word.
//
// Prose rules teach the model what to avoid; the gate makes the avoidance
// mechanical. Everything here is fail-closed only for the listed heads and
// fail-open for anything it cannot classify.

// aresGitDenyReason is the deny text for a forbidden git form in an Ares
// subagent.
const aresGitDenyReason = "Ares never runs git stash/checkout/switch/restore/clean or a hard reset: worktrees share one stash list and the user's dirty files are off-limits. Baseline = copy the files to the scratchpad or use a temporary worktree (git worktree add). Creating a branch (checkout -b / switch -c) is allowed."

// aresHookBypassDenyReason is the deny text for a commit that skips the
// project's own hooks.
const aresHookBypassDenyReason = "Ares never bypasses the project's hooks (SKIP_LOCAL_CI, HUSKY=0, --no-verify). A slow or failing hook goes into the report as `Not run:` with the reason; the user decides."

// gateAresGitRE matches the git forms Ares may not run, on the effective text
// of one shell segment (wrappers and leading assignments stripped). Creating a
// branch is excluded by the negative lookahead-free form below: the branch
// creation flags are matched first and the segment is then left alone.
var (
	gateAresGitHeadRE   = regexp.MustCompile(`(?i)^git\s+(?:-C\s+(?:"[^"]*"|'[^']*'|\S+)\s+)?(checkout|switch|stash|clean|restore|reset)\b(.*)$`)
	gateAresBranchNewRE = regexp.MustCompile(`(?i)^\s*(?:-b|-B|-c|-C)\b`)
	gateAresResetHardRE = regexp.MustCompile(`(?i)--(?:hard|merge|keep)\b`)
	gateAresStashReadRE = regexp.MustCompile(`(?i)^\s*(?:list|show)\b`)
	// Hook bypass tokens are checked on the masked segment text so a commit
	// message that merely mentions them does not trip the rule.
	gateHookBypassRE = regexp.MustCompile(`(?i)(?:^|[\s;&|])(?:SKIP_LOCAL_CI=|HUSKY=0\b|--no-verify\b)`)
)

// aresForbiddenGit reports whether one segment is a git form Ares may not run.
func aresForbiddenGit(seg shellSegment) bool {
	m := gateAresGitHeadRE.FindStringSubmatch(strings.TrimSpace(gateEffectiveSegmentText(seg.raw)))
	if m == nil {
		return false
	}
	sub, rest := strings.ToLower(m[1]), m[2]
	switch sub {
	case "checkout", "switch":
		return !gateAresBranchNewRE.MatchString(rest)
	case "stash":
		return !gateAresStashReadRE.MatchString(rest)
	case "reset":
		return gateAresResetHardRE.MatchString(rest)
	}
	return true // clean, restore
}

// aresBashGate denies the git forms and hook bypasses listed above. Any other
// command is left to Claude Code's normal permission flow.
func aresBashGate(command string) editGateResult {
	for _, seg := range splitShellSegments(command) {
		if gateHookBypassRE.MatchString(seg.masked) {
			return editGateResult{Decision: "deny", Reason: aresHookBypassDenyReason}
		}
		if aresForbiddenGit(seg) {
			return editGateResult{Decision: "deny", Reason: aresGitDenyReason}
		}
	}
	return editGateResult{}
}

// readOnlyChildMarker is the first line of the block hermes.md puts in every
// review child and validator prompt. A spawned agent whose transcript opens
// with it has promised not to touch the working tree.
const readOnlyChildMarker = "WORKING TREE IS READ-ONLY"

// readOnlyChildDenyReason is the deny text for a mutating git form in a
// read-only review child.
const readOnlyChildDenyReason = "This agent's prompt declares the working tree read-only (Hermes review child or validator). git checkout/switch/reset/stash/clean/restore are not available here; inspect with git show <rev>:<path>, git diff and git log instead."

// gateMutatingGitRE matches the git forms a read-only child may not run, on
// the effective text of one segment. Unlike the Ares rule, branch creation is
// denied too: a reviewer has no business creating branches.
var gateMutatingGitRE = regexp.MustCompile(`(?i)^git\s+(?:-C\s+(?:"[^"]*"|'[^']*'|\S+)\s+)?(?:checkout|switch|stash|clean|restore|reset)\b`)

// readOnlyChildTranscriptBytes bounds how much of a child's transcript is read
// to find the marker: the prompt is the first user entry, so the marker sits
// in the first few kilobytes when it is there at all.
const readOnlyChildTranscriptBytes = 256 * 1024

// gateReadTranscriptHead returns up to readOnlyChildTranscriptBytes of the
// file, or "" when it cannot be read.
func gateReadTranscriptHead(p string) string {
	if p == "" {
		return ""
	}
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, readOnlyChildTranscriptBytes))
	if err != nil {
		return ""
	}
	return string(data)
}

// spawnedTranscriptPath resolves the spawned agent's own transcript. Claude
// Code hands a subagent's hook the parent's transcript_path; its own file is
// the sibling <dir>/<session_id>/subagents/agent-<agent_id>.jsonl
// (subagentTranscriptPath). A payload that already names a subagent file is
// used as is.
func spawnedTranscriptPath(input preToolUseInput) string {
	tp := strings.ReplaceAll(input.TranscriptPath, `\`, "/")
	if strings.Contains(tp, "/subagents/agent-") {
		return input.TranscriptPath
	}
	return subagentTranscriptPath(input.TranscriptPath, input.SessionID, input.AgentID)
}

// mutatingGitSegment reports whether any segment of the command is a mutating
// git form.
func mutatingGitSegment(command string) bool {
	for _, seg := range splitShellSegments(command) {
		if gateMutatingGitRE.MatchString(strings.TrimSpace(gateEffectiveSegmentText(seg.raw))) {
			return true
		}
	}
	return false
}

// readOnlyChildGitGate denies a mutating git form in a spawned agent whose
// transcript head carries the read-only marker. The transcript is read only
// once a mutating form is seen — a rare event — so the hot path of every
// other spawned agent's Bash call stays a pure string check. An unreadable
// or marker-less transcript fails open.
func readOnlyChildGitGate(input preToolUseInput) editGateResult {
	if !mutatingGitSegment(input.ToolInput.Command) {
		return editGateResult{}
	}
	head := gateReadTranscriptHead(spawnedTranscriptPath(input))
	if head == "" || !strings.Contains(head, readOnlyChildMarker) {
		return editGateResult{}
	}
	return editGateResult{Decision: "deny", Reason: readOnlyChildDenyReason}
}

// spawnedAgentGate is step 1 of editGateDecisionRest for every spawned agent
// that is not Odysseus: an Ares payload gets the Ares shell rules; any other
// spawned agent gets the read-only-child git guard. Only Bash and PowerShell
// are examined; edits by spawned agents are never gated here.
func spawnedAgentGate(input preToolUseInput) editGateResult {
	switch input.ToolName {
	case "Bash", "PowerShell":
	default:
		return editGateResult{}
	}
	if strings.Contains(strings.ToLower(input.AgentType), "ares") {
		return aresBashGate(input.ToolInput.Command)
	}
	return readOnlyChildGitGate(input)
}

// gateSpawnScopeRE reads the scope line an orchestrator puts in a review or
// fix prompt when several worktrees are in play at once. The per-turn review
// cap (irisSpawnCapScoped) counts per scope when the line is present, so two
// Hermes reviews of two worktrees in one turn both pass while a second review
// of the same worktree still trips. 2026-10-25 16:28: the second Hermes (button
// branch, its own worktree) was denied and waited ten hours for the next turn.
// A `SCOPE:` line splits one worktree by module: 2026-10-30, four of five
// parallel module reviews in one worktree were denied.
var gateSpawnScopeRE = regexp.MustCompile(`(?mi)^\s*(?:WORKTREE|WORKDIR|BRANCH|SCOPE):\s*(\S.*?)\s*$`)

// gateSpawnScope returns the normalized scope named in a spawn prompt, or "".
// The first worktree/branch line and the first SCOPE line key together, so two
// modules of one worktree get two scopes and an extra worktree line cannot
// split one.
func gateSpawnScope(prompt string) string {
	var base, module string
	for _, m := range gateSpawnScopeRE.FindAllStringSubmatch(prompt, -1) {
		v := strings.ToLower(strings.TrimRight(strings.ReplaceAll(strings.Trim(m[1], "`\"'"), `\`, "/"), "/"))
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(m[0])), "SCOPE") {
			if module == "" {
				module = v
			}
		} else if base == "" {
			base = v
		}
	}
	if base != "" && module != "" {
		return base + "|" + module
	}
	return base + module
}

// gateControlMessageRE recognizes a SendMessage body that steers a running
// agent rather than adding work: stop, hand back, status. Such a message is
// allowed at any context size (gateMaxResumeTurns otherwise denies it), so a
// long-running Ares can be told to wrap up without TaskStop killing the run
// mid-suite — 2026-10-28 15:02 and 16:11, the only remaining option was
// TaskStop and the finished code was lost once.
var gateControlMessageRE = regexp.MustCompile(`(?i)^\s*(?:please\s+)?(?:stop|halt|pause|wrap(?:\s+it)?\s+up|hand\s*back|finish\s+up|status\??|progress\??|how\s+long|eta\??)\b`)

// gateControlMessageMaxLen bounds a control message: anything longer carries
// new scope and pays for the agent's context like any other resume.
const gateControlMessageMaxLen = 300

func isGateControlMessage(message string) bool {
	m := strings.TrimSpace(message)
	return m != "" && len(m) <= gateControlMessageMaxLen && gateControlMessageRE.MatchString(m)
}
