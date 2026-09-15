package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// The ledger-keyed inline edit gate. One PreToolUse hook decides, mechanically,
// whether the god running inline in the main context may make this edit:
//
//	odysseus — plans and spec deltas only, read-only shell (ported from the
//	           retired hooks/plan-mode-guard.cjs, which could only ever see a
//	           *spawned* Odysseus because it keyed off agent_type)
//	iris     — at most two distinct project source files per user turn, then
//	           the third is denied with the kratos:ares spawn template
//
// Everything else fails open: a spawned subagent other than Odysseus, a session
// with no ledger, an unparseable ledger, no recorded god, a god with no rule, a
// user who stood the gate down, or any error on any path.
//
// The gate emits "deny" or nothing at all — never "allow". A permitted command
// or edit produces no decision and follows Claude Code's normal permission
// flow. That is the whole safety margin of the shell classifier: it is a
// pattern matcher over shell text, and the 2026-09-11 review found three
// separate ways past it (process substitution, backslash-escaped quotes,
// unspaced redirects). Under an explicit "allow" each of those ran `rm -rf`
// with the user's permission prompt skipped; with no "allow" the same miss
// degrades to the prompt the user would have seen anyway.

// irisFileBudget is the number of distinct project source files Iris may edit
// herself in one user turn before the work belongs to Ares. Three documents
// state this number in words — TestDocsPinIrisFileBudget keeps them together.
const irisFileBudget = 2

// gateResetAgents are the builder spawn targets. Two things key off them: they
// refill Iris's inline budget (work handed to them is no longer hers to do),
// and they end an inline Odysseus's turn, because the plan has left his hands.
//
// Deliberately only the builders. Clearing the lock on *any* kratos:<god>
// dispatch handed Odysseus a one-call escape: a Task(kratos:metis) for
// grounding — which the planning protocol tells him to run — unlocked the
// session and let him implement his own plan on the next Write. Without that
// exit a single /kratos:plan locked the session for life, so the exit stays,
// narrowed to the dispatch that actually transfers the work.
var gateResetAgents = regexp.MustCompile(`(?i)^kratos:(?:ares|hades)$`)

// gateDocExtensions never count against the budget. Iris's own Inline rung
// already exempts documents; notes, plans and diagrams are her job.
var gateDocExtensions = map[string]bool{
	".md": true, ".drawio": true, ".svg": true, ".png": true, ".jpg": true,
	".jpeg": true, ".gif": true, ".pptx": true, ".docx": true, ".xlsx": true,
	".pdf": true, ".txt": true, ".csv": true,
}

// gateTempClaudeRE matches an agent scratchpad path — bookkeeping, not project
// work. Same rule as hooks/tool-use.cjs isProjectFile.
var gateTempClaudeRE = regexp.MustCompile(`(^|/)(temp|tmp)/claude/`)

// gateSpecDeltaRE allows exactly one segment after spec-delta/, so
// spec-delta/archived/*.md — where `kratos spec archive` moves promoted deltas
// — stays denied.
var gateSpecDeltaRE = regexp.MustCompile(`(?i)(^|/)\.claude/feature/[^/]+/spec-delta/[^/]+\.md$`)

// gatePlanRoot is the tactical-plan directory Odysseus may write.
const gatePlanRoot = ".claude/.Arena/tactical-plans"

// gateAbsPathRE matches the absolute path shapes the edit tools actually send:
// POSIX and Windows drive-letter. Only an absolute path can be compared with
// the payload's cwd.
var gateAbsPathRE = regexp.MustCompile(`^(?:/|[A-Za-z]:/)`)

// Read-only shell allowlist, matched against the HEAD of each command segment.
// sed/head/tail/wc and `git -C <path> <read-only>` were added when this logic
// moved into Go: the JS guard denied all five during the 2026-09-11 planning
// session, and a guard that denies the agent's own inspection commands teaches
// the agent to route around it. which/command -v/stat/file came from the JS
// guard's own allowlist fix (v2.110), merged into this port.
var gateReadOnlyCommands = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^git\s+(?:-C\s+(?:"[^"]*"|'[^']*'|\S+)\s+)?(?:status|diff|show|log|rev-parse|ls-files)\b`),
	// `git branch` is its own entry because most of its surface mutates:
	// -d/-D/--delete drop a branch, -m/-M rename one, -f moves a ref, and a
	// bare `git branch <name>` creates one. Only the listing forms are
	// accepted, and only with no trailing operand — `git branch --merged main`
	// is read-only but denied, an accepted false deny in exchange for a rule
	// with no way through it.
	regexp.MustCompile(`(?i)^git\s+(?:-C\s+(?:"[^"]*"|'[^']*'|\S+)\s+)?branch(?:\s+(?:--list|--show-current|--all|--remotes|--verbose|--merged|--no-merged|-a|-r|-v|-vv|-av|-rv))*\s*$`),
	regexp.MustCompile(`(?i)^(?:ls|dir|pwd)\b`),
	regexp.MustCompile(`(?i)^(?:cat|type)\b`),
	regexp.MustCompile(`(?i)^(?:find|grep|rg)\b`),
	regexp.MustCompile(`(?i)^(?:sed|head|tail|wc)\b`),
	regexp.MustCompile(`(?i)^(?:which|stat|file)\b`),
	regexp.MustCompile(`(?i)^command\s+-v\b`),
	regexp.MustCompile(`(?i)^Get-(?:ChildItem|Content|Location)\b`),
	regexp.MustCompile(`(?i)^Select-String\b`),
	regexp.MustCompile(`(?i)^Test-Path\b`),
}

// Read-only kratos subcommands the planner is instructed to run: slug mint and
// timestamp, template fetch and delta self-validation, draft discovery, and the
// session-tracking calls references/agent-protocol.md mandates for every agent.
// Mutating subcommands — spec archive, pipeline update, session start, init,
// install — are deliberately absent and stay denied.
var gateKratosReadOnlySubcommandRE = regexp.MustCompile(`(?i)^(?:slug|now|version|--version|template\s+get|spec\s+(?:validate|list)|agent\s+(?:load|protocol)|session\s+active|pipeline\s+(?:get|status|next|discover)|feedback\s+list|memory\s+list|profile\s+list|todo\s+list|step\s+record-(?:agent|file))\b`)

// Any shell metacharacter disqualifies a kratos command. This is what makes it
// safe to check the kratos allowlist *before* the segment heuristics: without
// it, `kratos slug -d "x" && rm -rf build` matches the allowlist prefix and is
// allowed outright. The class carries CR and LF because a newline separates two
// commands exactly as `;` does — `kratos slug x\nrm -rf build` returned an
// explicit allow while they were missing.
var gateShellMetaRE = regexp.MustCompile("[;&|<>`$\r\n]")

var (
	// gateVarWrapRE matches the `VAR=$( … )` capture the agent protocol's own
	// timestamp snippet uses; the inner command is what gets classified.
	gateVarWrapRE = regexp.MustCompile(`(?s)^[A-Za-z_][A-Za-z0-9_]*=\$\((.*)\)$`)
	// gateDevNullRE strips an inert stderr redirect, `2>/dev/null`.
	gateDevNullRE = regexp.MustCompile(`\s*2>\s*/dev/null`)
	// gateDateFallbackRE strips the protocol's trailing `|| date …` fallback.
	// It stays in escaped-string form rather than a raw string because the
	// negated class needs a literal backtick, which a Go raw string cannot
	// contain.
	gateDateFallbackRE = regexp.MustCompile("\\s*\\|\\|\\s*date\\b[^;&|$`]*$")
	// gateBinArgsRE splits `<maybe-quoted binary> <args…>`, so a quoted path
	// with spaces ("C:/Program Files/kratos/kratos.exe") still yields a binary
	// name to test against the kratos allowlist.
	gateBinArgsRE = regexp.MustCompile(`(?s)^(?:"([^"]+)"|'([^']+)'|(\S+))\s+(.+)$`)

	// gateRedirectRE matches a write. The `>` alternative is deliberately
	// unanchored: requiring `(^|\s)` before it missed every unspaced form the
	// shell accepts — `ls>out.txt`, `cat foo>>bar`, `git status>out.txt`,
	// `ls 1>out.txt`, `Get-Content x>y` all passed as reads. It is safe to
	// match anywhere because the test runs on the *masked* segment, where a
	// quoted `>` is blanked, and because gateInertRedirectRE has already taken
	// the stderr plumbing (`2>/dev/null`, `2>&1`) out of the string.
	gateRedirectRE = regexp.MustCompile(`(?i)>|(^|\s)(?:Set-Content\b|Add-Content\b|Out-File\b|Remove-Item\b|Move-Item\b|Copy-Item\b|New-Item\b)`)
	// gateInertRedirectRE matches stderr plumbing that creates no file: a
	// redirect to the null device and any `N>&M` descriptor duplication
	// (`2>&1`, `1>&2`, `>&2`). It is stripped before the segment split, not
	// after: the `&` in `2>&1` is a segment separator, so `ls 2>&1 | grep x`
	// would otherwise be cut into `ls 2`, `1 ` and ` grep x` and denied.
	gateInertRedirectRE = regexp.MustCompile(`(?i)\s*\d?>\s*(?:/dev/null|nul)\b|\s*\d?>&\d`)
	// gateCmdSubstRE spots a substitution surviving inside a segment:
	// `cat $(rm -rf build)` has a reader at its head and is still a mutation.
	// `<(` and `>(` are process substitution, the same hole in bash clothing —
	// `cat <(rm -rf build)`, `sed -n 1p <(rm -rf build)` and
	// `git log --format=%h <(rm -rf b)` all ran the inner command while the
	// pattern matched only `$(` and a backtick.
	gateCmdSubstRE = regexp.MustCompile("\\$\\(|`|<\\(|>\\(")
	// gateInPlaceFlagRE matches an in-place flag anywhere in a sed invocation:
	// `sed -n -i`, `sed --quiet -i` and `sed --in-place` all edit the file. The
	// leading (^|\s) is what keeps `--quiet` (which contains an "i") out of the
	// short-flag alternative.
	gateInPlaceFlagRE = regexp.MustCompile(`(?i)(?:^|\s)(?:-[a-z]*i[a-z]*|--in-place)\b`)
	// gateFindActionRE matches the find actions that delete, run a program, or
	// write a file. The -fprint family is easy to miss: `find . -fprintf out
	// "%p"` needs no shell redirect to create a file.
	//
	// Accepted residual: `sed 's/a/b/w out.txt'` writes a file from inside the
	// sed script, and so does `sed -n '1p;w out.txt'`. Catching it needs a sed
	// script parser, which this gate deliberately does not build — the miss
	// costs a permission prompt, not an unattended write, because the gate
	// never returns "allow".
	gateFindActionRE = regexp.MustCompile(`(?i)(?:^|\s)-(?:delete|exec|execdir|ok|okdir|fprintf|fprint0|fprint|fls)\b`)
	// gateFollowFlagRE matches `tail -f`/`-F`/`--follow`, which never returns
	// and would hang the tool call until its timeout.
	gateFollowFlagRE = regexp.MustCompile(`(?i)(?:^|\s)(?:-[a-z]*f[a-z]*|--follow)\b`)
	// gateSegmentSplitRE cuts a command where a shell would start a new one.
	gateSegmentSplitRE = regexp.MustCompile("[;&|\r\n]+")
)

const odysseusWriteDenyReason = "Odysseus plans, Ares builds — save the plan, get approval, then spawn kratos:ares with the plan path."

const odysseusBashDenyReason = "Odysseus plan mode may only run read-only inspection commands. Write the command into the plan for Ares to run instead of running it now."

// editGateResult is one gate verdict. Decision "" means no output at all: the
// tool call goes through Claude Code's normal permission flow untouched, which
// is what every non-deny verdict produces. "allow" is not a value this gate
// ever sets — see the contract note at the top of the file.
type editGateResult struct {
	Decision string // "" (no decision) or "deny"
	Reason   string
	// Files is the new inline_edited_files value. nil means "do not write";
	// an empty non-nil slice clears the list.
	Files []string
	// ClearGod drops inline_god: the inline god handed the work to a spawned
	// agent, so his claim on this session ends with that dispatch.
	ClearGod bool
}

func editGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit-gate",
		Short: "Handle PreToolUse — deny source edits the inline god should dispatch instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				debugLog("edit-gate: stdin read error: %v", err)
				return nil
			}
			handleEditGate(raw)
			return nil
		},
	}
}

// handleEditGate runs the gate for one raw PreToolUse payload and prints the
// decision, if any. Every error path is silent and non-blocking.
func handleEditGate(raw []byte) {
	var input preToolUseInput
	if err := json.Unmarshal(raw, &input); err != nil {
		debugLog("edit-gate: json parse error: %v", err)
		return
	}

	var ledger map[string]any
	if gateNeedsLedger(input) {
		m, err := readInlineLedger(input.SessionID)
		if err != nil {
			debugLog("edit-gate: no readable ledger for %s: %v", input.SessionID, err)
		} else {
			ledger = m
		}
	}

	res := editGateDecision(input, ledger)

	if ledger != nil && input.SessionID != "" && (res.Files != nil || res.ClearGod) {
		if res.Files != nil {
			ledger[ledgerKeyEditedFiles] = res.Files
		}
		if res.ClearGod {
			ledger[ledgerKeyInlineGod] = ""
		}
		if err := writeInlineLedger(input.SessionID, ledger); err != nil {
			// Counting is best effort: never turn a failed write into a deny.
			debugLog("edit-gate: ledger write failed: %v", err)
		}
	}

	if res.Decision == "" {
		return
	}
	data, err := json.Marshal(preToolUseOutput{
		HookSpecificOutput: preToolUseHookSpecific{
			HookEventName:            "PreToolUse",
			PermissionDecision:       res.Decision,
			PermissionDecisionReason: res.Reason,
		},
	})
	if err != nil {
		return
	}
	fmt.Println(string(data))
}

// gateIsSpawned reports whether the payload came from a spawned subagent.
// agent_type and agent_id are the only subagent-only keys (verified on real
// payloads, 2026-09-11); a top-level subagent_type is not one of them, and
// keying on it would hand a free pass to anything that sets it.
func gateIsSpawned(input preToolUseInput) bool {
	return input.AgentType != "" || input.AgentID != ""
}

// gateNeedsLedger reports whether the verdict can depend on the session ledger.
// A spawned subagent that is not Odysseus is decided by the payload alone, and
// that is the hot path — every Ares edit and every Ares Bash call passes
// through this hook — so it must not pay for a file read it cannot use.
func gateNeedsLedger(input preToolUseInput) bool {
	if input.SessionID == "" {
		return false
	}
	if gateIsSpawned(input) {
		return gateIsSpawnedOdysseus(input)
	}
	return true
}

func gateIsSpawnedOdysseus(input preToolUseInput) bool {
	return strings.Contains(strings.ToLower(input.AgentType), "odysseus")
}

// editGateDecision is the whole policy, as a pure function of the payload and
// the ledger, so the table test can exercise every branch in process.
func editGateDecision(input preToolUseInput, ledger map[string]any) editGateResult {
	// 1. Spawned subagent: only Odysseus is gated. Ares, Hades and everyone
	//    else must never be counted against the inline god's budget — gating
	//    them would break the dispatch this gate exists to create. The marker
	//    is agent_type/agent_id, which only a spawned payload carries; a
	//    top-level subagent_type is not part of that payload, and keying on it
	//    hands a free pass to anything that sets one.
	if gateIsSpawned(input) {
		if gateIsSpawnedOdysseus(input) {
			return odysseusGate(input, ledger)
		}
		return editGateResult{}
	}

	// 2. No session id or no readable ledger: this session never ran a
	//    launcher, so there is nothing to gate.
	if input.SessionID == "" || ledger == nil {
		return editGateResult{}
	}

	god := strings.ToLower(strings.TrimSpace(ledgerString(ledger, ledgerKeyInlineGod)))

	// 3. Dispatch. Read the target from this payload rather than from the
	//    agent_spawn table: it is synchronous, ordered, and keeps the DB out of
	//    a per-edit hot path. Only a dispatch to a builder counts — it refills
	//    Iris's budget, and it ends an inline Odysseus's turn. A research or
	//    review spawn (kratos:metis for grounding, kratos:hermes for a read)
	//    changes neither: the work is still the inline god's.
	if input.ToolName == "Agent" || input.ToolName == "Task" {
		target := strings.TrimSpace(input.ToolInput.SubagentType)
		res := editGateResult{}
		if gateResetAgents.MatchString(target) {
			if len(ledgerStrings(ledger)) > 0 {
				res.Files = []string{}
			}
			if god == "odysseus" {
				res.ClearGod = true
			}
		}
		return res
	}

	// 4. No recorded god: a plain Claude Code session, or one whose inline god
	//    already handed off.
	if god == "" {
		return editGateResult{}
	}

	// 5. The user told the model to do the work itself.
	if ledgerBool(ledger) {
		return editGateResult{}
	}

	// 6. The per-god rules. Any other god has none: fail open.
	switch god {
	case "odysseus":
		return odysseusGate(input, ledger)
	case "iris":
		return irisGate(input, ledger)
	}
	return editGateResult{}
}

// odysseusGate keeps the planner on planning artifacts and read-only shell.
//
// It denies or says nothing. A tactical-plan write, a spec-delta write and a
// read-only command all return no decision and fall through to Claude Code's
// normal permission flow — see the contract note at the top of the file for
// why an explicit "allow" is not worth its failure mode.
func odysseusGate(input preToolUseInput, ledger map[string]any) editGateResult {
	// The payload's cwd is the containment root; the ledger's cwd is the
	// fallback, exactly as in irisGate. Without it a payload with an empty cwd
	// turned isUnderGateCwd into a no-op and any absolute path containing
	// ".claude/.Arena/tactical-plans/" passed the plan test.
	cwd := input.Cwd
	if cwd == "" {
		cwd = ledgerString(ledger, ledgerKeyCwd)
	}

	switch input.ToolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		p := input.ToolInput.targetPath()
		if p == "" {
			// A payload the gate cannot even name a target in is not something
			// to deny: fail open, as irisGate does on the same shape.
			debugLog("edit-gate: %s payload carries no file path", input.ToolName)
			return editGateResult{}
		}
		if isTacticalPlanPath(p, cwd) || isSpecDeltaGatePath(p, cwd) {
			return editGateResult{}
		}
		return editGateResult{Decision: "deny", Reason: odysseusWriteDenyReason}
	case "Bash", "PowerShell":
		// PowerShell's tool input carries the script under the same "command"
		// key Bash uses (see subagentStopCmd's transcript scan, which reads
		// both tools' Command field identically), and the read-only allowlist
		// above already has PowerShell verbs (Get-ChildItem, Test-Path, …) for
		// exactly this reason. Routing it through the same classification
		// closes the hole a PowerShell-only matcher left: hooks.json used to
		// gate Bash but not PowerShell, so Set-Content/Remove-Item/etc. — all
		// already in the mutation patterns above — ran ungated.
		c := input.ToolInput.Command
		if isReadOnlyKratosCommand(c) || isReadOnlyShellCommand(c) {
			return editGateResult{}
		}
		return editGateResult{Decision: "deny", Reason: odysseusBashDenyReason}
	}
	return editGateResult{}
}

// irisGate caps distinct project source files per user turn. Allows are silent
// — the gate never widens Iris's permissions, it only denies the third file.
func irisGate(input preToolUseInput, ledger map[string]any) editGateResult {
	switch input.ToolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
	default:
		// Bash is never gated for Iris: she runs builds, tests and git.
		return editGateResult{}
	}

	filePath := input.ToolInput.targetPath()
	if filePath == "" {
		return editGateResult{}
	}
	cwd := input.Cwd
	if cwd == "" {
		cwd = ledgerString(ledger, ledgerKeyCwd)
	}
	if !isGateProjectFile(filePath, cwd) || isGateDocument(filePath) {
		return editGateResult{}
	}

	norm := normalizeLedgerPath(filePath)
	files := ledgerStrings(ledger)
	for _, f := range files {
		if normalizeLedgerPath(f) == norm {
			// Repeat edits to one file are one file.
			return editGateResult{}
		}
	}
	if len(files) < irisFileBudget {
		next := make([]string, 0, len(files)+1)
		next = append(next, files...)
		next = append(next, norm)
		return editGateResult{Files: next}
	}
	return editGateResult{Decision: "deny", Reason: irisDenyReason(files, cwd)}
}

// irisDenyReason carries the spawn template verbatim so the model can act on it
// without reading commands/quick.md first. TestAresSpawnTemplateMatchesQuickMd
// keeps it in step with commands/quick.md, the copy it duplicates.
//
// The closing sentence is line-broken mid-phrase on purpose: gateBypassRE is
// multi-line anchored, so a stand-down phrase that *opens* a line grants the
// bypass. With "do it yourself" at the head of its own line, a user pasting
// this very message back into the prompt would switch the gate off.
func irisDenyReason(files []string, cwd string) string {
	shown := make([]string, 0, len(files))
	root := normalizeLedgerPath(cwd) + "/"
	for _, f := range files {
		n := normalizeLedgerPath(f)
		if root != "/" && strings.HasPrefix(n, root) {
			shown = append(shown, n[len(root):])
			continue
		}
		shown = append(shown, f)
	}
	return `Third source file this turn — this is Ares's work, not yours. Spawn him now:
Task(subagent_type: "kratos:ares", mode: "acceptEdits", prompt:
"MISSION: <Bug Fix / Refactor / Small Feature>
TARGET: <file/function/area>
REQUIREMENTS: <your reading of the request>
ORIGINAL_USER_REQUEST: <the user's words, verbatim — the scope contract>
TICKET: <#N or none>
Before any edit, follow your INTENTION protocol. No PRD or tech spec needed.")
Already edited this turn: ` + strings.Join(shown, ", ") + `. If the user explicitly tells you to do it
yourself, they can say so and the gate stands down for that turn.`
}

// gateSlashPath normalizes separators without touching case: the plan-path test
// is case sensitive on ".Arena", as it was in the JS guard.
func gateSlashPath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// gateCleanPath is gateSlashPath plus path.Clean, so `..` segments resolve and
// doubled separators collapse before any containment test. Without the clean,
// `.claude/.Arena/tactical-plans/../../../../etc/passwd.md` contained the plan
// root and was allowed.
func gateCleanPath(p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	return path.Clean(gateSlashPath(p))
}

// isUnderGateCwd reports whether an absolute path sits under cwd. An empty cwd
// or a relative path cannot be anchored, so both pass and the cleaned-path test
// is the only guard — the same fail-open discipline as the rest of the gate.
// (Precedent for the containment test: hooks/permission-read.cjs.)
func isUnderGateCwd(cleaned, cwd string) bool {
	root := normalizeLedgerPath(cwd)
	if root == "" || !gateAbsPathRE.MatchString(cleaned) {
		return true
	}
	p := normalizeLedgerPath(cleaned)
	return p == root || strings.HasPrefix(p, root+"/")
}

func isTacticalPlanPath(filePath, cwd string) bool {
	n := gateCleanPath(filePath)
	if n == "" || !strings.HasSuffix(n, ".md") {
		return false
	}
	return strings.Contains(n, gatePlanRoot+"/") && isUnderGateCwd(n, cwd)
}

// isSpecDeltaGatePath allows .claude/feature/<slug>/spec-delta/<capability>.md
// only — the rest of the feature dir (prd.md, status.json, tech-spec.md) is
// still denied.
func isSpecDeltaGatePath(filePath, cwd string) bool {
	n := gateCleanPath(filePath)
	return n != "" && gateSpecDeltaRE.MatchString(n) && isUnderGateCwd(n, cwd)
}

// isGateProjectFile reports whether the file is project work: under cwd and
// outside bookkeeping dirs. Ported from hooks/tool-use.cjs isProjectFile —
// TestGateProjectFileMatchesJS runs both over one fixture so they cannot drift.
// One difference is deliberate: the recorder logs .claude/feature/ and
// .claude/.Arena/ writes as project work, but no .claude/ path spends Iris's
// source-file budget.
func isGateProjectFile(filePath, cwd string) bool {
	file := normalizeLedgerPath(filePath)
	root := normalizeLedgerPath(cwd)
	if file == "" || root == "" {
		return false
	}
	if !strings.HasPrefix(file, root+"/") {
		return false
	}
	rel := file[len(root)+1:]
	if strings.HasPrefix(rel, ".claude/") || strings.HasPrefix(rel, ".kratos/") || strings.HasPrefix(rel, ".git/") {
		return false
	}
	if gateTempClaudeRE.MatchString(file) {
		return false
	}
	return true
}

func isGateDocument(filePath string) bool {
	return gateDocExtensions[strings.ToLower(path.Ext(gateSlashPath(filePath)))]
}

// stripSafeFallbacks removes the inert decorations the agent protocol itself
// recommends around a kratos call — a VAR=$( … ) wrapper, `2>/dev/null`, and a
// trailing `|| date …` — before the allowlist check. Anything else that leaves
// a shell metacharacter behind is still rejected.
func stripSafeFallbacks(command string) string {
	c := strings.TrimSpace(command)
	if m := gateVarWrapRE.FindStringSubmatch(c); m != nil {
		c = strings.TrimSpace(m[1])
	}
	c = gateDevNullRE.ReplaceAllString(c, "")
	c = gateDateFallbackRE.ReplaceAllString(c, "")
	return strings.TrimSpace(c)
}

// isReadOnlyKratosCommand matches `<maybe-quoted path to>/kratos[.exe] <read-only
// subcommand> …` with no shell metacharacter outside quotes, so argument text
// (a slug title, a --project path, a record-agent description containing "&")
// is inert by construction.
func isReadOnlyKratosCommand(command string) bool {
	trimmed := stripSafeFallbacks(command)
	if trimmed == "" || gateShellMetaRE.MatchString(maskQuoted(trimmed)) {
		return false
	}
	m := gateBinArgsRE.FindStringSubmatch(trimmed)
	if m == nil {
		return false
	}
	bin := gateSlashPath(firstNonEmpty(m[1], m[2], m[3]))
	base := strings.ToLower(bin[strings.LastIndex(bin, "/")+1:])
	if base != "kratos" && base != "kratos.exe" {
		return false
	}
	return gateKratosReadOnlySubcommandRE.MatchString(strings.TrimSpace(m[4]))
}

// shellSegment is one command a shell would run on its own, with a quote mask
// of the same length: in masked, quoted text is blanked, so a separator or a
// mutation word inside an argument is inert.
type shellSegment struct {
	raw    string
	masked string
}

// maskQuoted blanks quoted spans with 'x', preserving length and index
// alignment. A single-quoted span is inert in every shell and is blanked whole.
// Inside double quotes a `$`, a backtick and the parentheses of a substitution
// still expand, so those survive the mask and keep `"$(rm -rf build)"`
// detectable. An unbalanced quote leaves the remainder unmasked — the stricter
// reading.
//
// A backslash escapes the byte after it, and that byte is never a delimiter.
// Treating `\"` as an opening quote was a way straight through the gate:
// `ls \"a ; rm -rf build \"z` has no quoted span at all (the shell sees two
// literal quote characters), but the mask read `"a ; rm -rf build "` as one
// quoted argument, blanked the `;` inside it, and classified the whole line as
// a single `ls` segment.
func maskQuoted(s string) string {
	out := []byte(s)
	for i := 0; i < len(out); {
		if out[i] == '\\' {
			i += 2
			continue
		}
		q := out[i]
		if q != '"' && q != '\'' {
			i++
			continue
		}
		j := i + 1
		for j < len(out) && out[j] != q {
			// Only double quotes honor a backslash escape; inside single
			// quotes a backslash is an ordinary character in every shell.
			if q == '"' && out[j] == '\\' {
				j += 2
				continue
			}
			j++
		}
		if j >= len(out) {
			break
		}
		for k := i + 1; k < j; k++ {
			if q == '"' && survivesDoubleQuotes(out[k]) {
				continue
			}
			out[k] = 'x'
		}
		i = j + 1
	}
	return string(out)
}

// survivesDoubleQuotes reports whether a byte keeps its shell meaning inside a
// double-quoted span. `$`, a backtick and the parentheses of a substitution
// still expand there, so the mask must leave them readable.
func survivesDoubleQuotes(b byte) bool {
	switch b {
	case '$', '`', '(', ')':
		return true
	}
	return false
}

// splitShellSegments cuts a command on the separators a shell honors (`&&`,
// `||`, `;`, `|`, `&`, and a newline), using the quote mask so a separator
// inside an argument does not split.
func splitShellSegments(command string) []shellSegment {
	masked := maskQuoted(command)
	var segs []shellSegment
	start := 0
	for _, loc := range gateSegmentSplitRE.FindAllStringIndex(masked, -1) {
		if loc[0] > start {
			segs = append(segs, shellSegment{raw: command[start:loc[0]], masked: masked[start:loc[0]]})
		}
		start = loc[1]
	}
	if start < len(command) {
		segs = append(segs, shellSegment{raw: command[start:], masked: masked[start:]})
	}
	return segs
}

// isReadOnlyShellCommand reports whether every segment of command is pure
// inspection.
//
// Classification is per segment and per head, never over the raw string: a
// whole-string scan for mutation words denied
// `cat .claude/.Arena/tactical-plans/2026-09-11-move-sidebar.md` — the
// draft-plan discovery commands/plan.md mandates — because the *path* said
// "move", while checking only the prefix allowed `git status && go build -o out
// ./...`.
//
// Inert stderr plumbing comes out first, before the split: `2>&1` contains a
// segment separator, so `git status 2>&1 | head -20` was cut at the `&` and
// denied on a fragment that was never a command.
func isReadOnlyShellCommand(command string) bool {
	trimmed := strings.TrimSpace(gateInertRedirectRE.ReplaceAllString(command, " "))
	if trimmed == "" {
		return true
	}
	segs := splitShellSegments(trimmed)
	if len(segs) == 0 {
		return false
	}
	for _, seg := range segs {
		if strings.TrimSpace(seg.raw) == "" {
			continue
		}
		if !isReadOnlySegment(seg) {
			return false
		}
	}
	return true
}

// isReadOnlySegment classifies one segment by its head plus the guards a reader
// head cannot vouch for: a redirect writes, a command substitution hides a
// second command in the arguments, `sed -i` edits in place, `find -delete` or
// `-exec` runs anything, and `tail -f` never returns.
func isReadOnlySegment(seg shellSegment) bool {
	raw := strings.TrimSpace(seg.raw)
	masked := strings.TrimSpace(seg.masked)
	if raw == "" {
		return false
	}
	if gateRedirectRE.MatchString(masked) || gateCmdSubstRE.MatchString(masked) {
		return false
	}
	switch gateSegmentHead(raw) {
	case "sed":
		if gateInPlaceFlagRE.MatchString(masked) {
			return false
		}
	case "find":
		if gateFindActionRE.MatchString(masked) {
			return false
		}
	case "tail":
		if gateFollowFlagRE.MatchString(masked) {
			return false
		}
	}
	if isReadOnlyKratosCommand(raw) {
		return true
	}
	for _, re := range gateReadOnlyCommands {
		if re.MatchString(raw) {
			return true
		}
	}
	return false
}

// gateSegmentHead is the lower-cased program name a segment starts with, with
// its directory, quotes and .exe suffix stripped.
func gateSegmentHead(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	head := gateSlashPath(strings.Trim(fields[0], "\"'"))
	head = strings.ToLower(head[strings.LastIndex(head, "/")+1:])
	return strings.TrimSuffix(head, ".exe")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
