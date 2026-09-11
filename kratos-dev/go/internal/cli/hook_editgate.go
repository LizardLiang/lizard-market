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

// irisFileBudget is the number of distinct project source files Iris may edit
// herself in one user turn before the work belongs to Ares.
const irisFileBudget = 2

// gateResetAgents are the spawn targets that refill the inline budget: work
// handed to them is no longer the inline god's to do.
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

// Read-only shell allowlist. sed/head/tail/wc and `git -C <path> <read-only>`
// were added when this logic moved into Go: the JS guard denied all five during
// the 2026-09-11 planning session, and a guard that denies the agent's own
// inspection commands teaches the agent to route around it.
var gateReadOnlyCommands = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^git\s+(?:-C\s+(?:"[^"]*"|'[^']*'|\S+)\s+)?(?:status|diff|show|log|branch|rev-parse|ls-files)\b`),
	regexp.MustCompile(`(?i)^(?:ls|dir|pwd)\b`),
	regexp.MustCompile(`(?i)^(?:cat|type)\b`),
	regexp.MustCompile(`(?i)^(?:find|grep|rg)\b`),
	regexp.MustCompile(`(?i)^(?:sed|head|tail|wc)\b`),
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
// safe to check the kratos allowlist *before* the generic deny heuristics:
// without it, `kratos slug -d "x" && rm -rf build` matches the allowlist prefix
// and is allowed outright.
var gateShellMetaRE = regexp.MustCompile("[;&|<>`$]")

var (
	gateVarWrapRE      = regexp.MustCompile(`(?s)^[A-Za-z_][A-Za-z0-9_]*=\$\((.*)\)$`)
	gateDevNullRE      = regexp.MustCompile(`\s*2>\s*/dev/null`)
	gateDateFallbackRE = regexp.MustCompile("\\s*\\|\\|\\s*date\\b[^;&|$`]*$")
	gateBinArgsRE      = regexp.MustCompile(`(?s)^(?:"([^"]+)"|'([^']+)'|(\S+))\s+(.+)$`)

	gateChainedMutationRE = regexp.MustCompile(`(?i)[;&|]\s*(?:rm|del|erase|mv|move|cp|copy|mkdir|rmdir|npm\s+install|pnpm\s+install|yarn\s+add|bun\s+add)\b`)
	gateRedirectRE        = regexp.MustCompile(`(?i)(^|\s)(?:>>?|Set-Content\b|Add-Content\b|Out-File\b|Remove-Item\b|Move-Item\b|Copy-Item\b|New-Item\b)`)
	gateMutationWordRE    = regexp.MustCompile(`(?i)\b(?:rm|del|erase|mv|move|mkdir|rmdir|npm\s+install|pnpm\s+install|yarn\s+add|bun\s+add|git\s+(?:push|commit|reset|checkout|switch|merge|rebase|pull))\b`)
	// sed is read-only inspection only without an in-place flag.
	gateSedInPlaceRE = regexp.MustCompile(`(?i)\bsed\s+(?:-[a-z]*i\b|--in-place)`)
)

const odysseusWriteDenyReason = "Odysseus plans, Ares builds — save the plan, get approval, then spawn kratos:ares with the plan path."

const odysseusBashDenyReason = "Odysseus plan mode may only run read-only inspection commands."

// editGateResult is one gate verdict. Decision "" means no output at all: the tool
// call goes through Claude Code's normal permission flow untouched.
type editGateResult struct {
	Decision   string // "", "allow", "deny"
	Reason     string
	Files      []string // new inline_edited_files value
	WriteFiles bool     // write Files back to the ledger
}

func editGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit-gate",
		Short: "Handle PreToolUse — deny source edits the inline god should dispatch instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
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
		return
	}

	var ledger map[string]any
	if input.SessionID != "" {
		if m, err := readInlineLedger(input.SessionID); err == nil {
			ledger = m
		}
	}

	res := editGateDecision(input, ledger)

	if res.WriteFiles && ledger != nil && input.SessionID != "" {
		ledger[ledgerKeyEditedFiles] = res.Files
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

// editGateDecision is the whole policy, as a pure function of the payload and
// the ledger, so the table test can exercise every branch in process.
func editGateDecision(input preToolUseInput, ledger map[string]any) editGateResult {
	// 1. Spawned subagent: only Odysseus is gated. Ares, Hades and everyone
	//    else must never be counted against the inline god's budget — gating
	//    them would break the dispatch this gate exists to create.
	if spawned := strings.ToLower(strings.TrimSpace(input.AgentType + " " + input.SubagentType)); spawned != "" {
		if strings.Contains(spawned, "odysseus") {
			return odysseusGate(input)
		}
		return editGateResult{}
	}

	// 2. No session id or no readable ledger: this session never ran a
	//    launcher, so there is nothing to gate.
	if input.SessionID == "" || ledger == nil {
		return editGateResult{}
	}

	// 3. Dispatching to a builder refills the budget, whatever the recorded god
	//    is. Read from this payload rather than from the agent_spawn table: it
	//    is synchronous, ordered, and keeps the DB out of a per-edit hot path.
	if input.ToolName == "Agent" || input.ToolName == "Task" {
		if gateResetAgents.MatchString(strings.TrimSpace(input.ToolInput.SubagentType)) &&
			len(ledgerStrings(ledger, ledgerKeyEditedFiles)) > 0 {
			return editGateResult{Files: []string{}, WriteFiles: true}
		}
		return editGateResult{}
	}

	god := strings.ToLower(strings.TrimSpace(ledgerString(ledger, ledgerKeyInlineGod)))
	if god == "" {
		return editGateResult{}
	}
	// 4. The user told the model to do the work itself.
	if ledgerBool(ledger, ledgerKeyGateBypass) {
		return editGateResult{}
	}

	switch god {
	case "odysseus":
		return odysseusGate(input)
	case "iris":
		return irisGate(input, ledger)
	}
	return editGateResult{}
}

// odysseusGate keeps the planner on planning artifacts and read-only shell.
func odysseusGate(input preToolUseInput) editGateResult {
	switch input.ToolName {
	case "Write", "Edit", "MultiEdit":
		p := input.ToolInput.FilePath
		if isTacticalPlanPath(p) {
			return editGateResult{Decision: "allow", Reason: "Odysseus may write tactical plan markdown files."}
		}
		if isSpecDeltaGatePath(p) {
			return editGateResult{Decision: "allow", Reason: "Odysseus may write the pending spec delta."}
		}
		return editGateResult{Decision: "deny", Reason: odysseusWriteDenyReason}
	case "Bash":
		c := input.ToolInput.Command
		if isReadOnlyKratosCommand(c) {
			return editGateResult{Decision: "allow", Reason: "Odysseus may run read-only kratos subcommands."}
		}
		if isReadOnlyShellCommand(c) {
			return editGateResult{Decision: "allow", Reason: "Odysseus may run read-only inspection commands."}
		}
		return editGateResult{Decision: "deny", Reason: odysseusBashDenyReason}
	}
	return editGateResult{}
}

// irisGate caps distinct project source files per user turn. Allows are silent
// — the gate never widens Iris's permissions, it only denies the third file.
func irisGate(input preToolUseInput, ledger map[string]any) editGateResult {
	switch input.ToolName {
	case "Write", "Edit", "MultiEdit":
	default:
		// Bash is never gated for Iris: she runs builds, tests and git.
		return editGateResult{}
	}

	filePath := input.ToolInput.FilePath
	if filePath == "" {
		return editGateResult{}
	}
	cwd := input.Cwd
	if cwd == "" {
		cwd = ledgerString(ledger, "cwd")
	}
	if !isGateProjectFile(filePath, cwd) || isGateDocument(filePath) {
		return editGateResult{}
	}

	norm := normalizeLedgerPath(filePath)
	files := ledgerStrings(ledger, ledgerKeyEditedFiles)
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
		return editGateResult{Files: next, WriteFiles: true}
	}
	return editGateResult{Decision: "deny", Reason: irisDenyReason(files, cwd)}
}

// irisDenyReason carries the spawn template verbatim so the model can act on it
// without reading commands/quick.md first.
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
Already edited this turn: ` + strings.Join(shown, ", ") + `. If the user explicitly tells you to
do it yourself, they can say so and the gate stands down for that turn.`
}

// gateSlashPath normalizes separators without touching case: the plan-path test
// is case sensitive on ".Arena", as it was in the JS guard.
func gateSlashPath(p string) string {
	out := strings.ReplaceAll(p, "\\", "/")
	for strings.Contains(out, "//") {
		out = strings.ReplaceAll(out, "//", "/")
	}
	return out
}

func isTacticalPlanPath(filePath string) bool {
	n := gateSlashPath(filePath)
	return strings.Contains(n, gatePlanRoot+"/") && strings.HasSuffix(n, ".md")
}

// isSpecDeltaGatePath allows .claude/feature/<slug>/spec-delta/<capability>.md
// only — the rest of the feature dir (prd.md, status.json, tech-spec.md) is
// still denied.
func isSpecDeltaGatePath(filePath string) bool {
	return gateSpecDeltaRE.MatchString(gateSlashPath(filePath))
}

// isGateProjectFile reports whether the file is project work: under cwd and
// outside bookkeeping dirs. Ported from hooks/tool-use.cjs isProjectFile.
func isGateProjectFile(filePath, cwd string) bool {
	file := strings.ToLower(gateSlashPath(filePath))
	root := strings.TrimRight(strings.ToLower(gateSlashPath(cwd)), "/")
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
// subcommand> …` with no shell metacharacters anywhere, so argument text (task
// titles) is inert by construction. Checked before the generic heuristics
// below, because those scan the whole command string and would false-deny
// `kratos slug --dated "move the sidebar"` on their \bmove\b pattern.
func isReadOnlyKratosCommand(command string) bool {
	trimmed := stripSafeFallbacks(command)
	if trimmed == "" || gateShellMetaRE.MatchString(trimmed) {
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

// isReadOnlyShellCommand is the generic inspection allowlist, checked after the
// kratos one.
func isReadOnlyShellCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return true
	}
	if gateChainedMutationRE.MatchString(trimmed) ||
		gateRedirectRE.MatchString(trimmed) ||
		gateSedInPlaceRE.MatchString(trimmed) ||
		gateMutationWordRE.MatchString(trimmed) {
		return false
	}
	for _, re := range gateReadOnlyCommands {
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
