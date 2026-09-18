package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

// debugLog writes a message to stderr (visible in Claude Code debug mode)
func debugLog(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[kratos-hook] "+format+"\n", args...)
}

// hookInput is the JSON Claude Code sends on stdin for UserPromptSubmit
type hookInput struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
}

// hookOutput is the JSON we return to Claude Code
type hookOutput struct {
	Continue           bool                `json:"continue"`
	HookSpecificOutput *hookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// kratosKeywordPattern maps each keyword to a pre-compiled word-boundary regex.
var kratosKeywordPatterns []keywordPattern

type keywordPattern struct {
	keyword string
	re      *regexp.Regexp
}

func init() {
	keywords := []string{
		"kratos",
		"athena",
		"ares",
		"metis",
		"apollo",
		"artemis",
		"hermes",
		"hephaestus",
		"daedalus",
		"clio",
		"mimir",
		"hades",
		"cassandra",
		"ananke",
		"odysseus",
		"prometheus",
		"themis",
		"nemesis",
		"hera",
		"iris",
	}
	kratosKeywordPatterns = make([]keywordPattern, len(keywords))
	for i, kw := range keywords {
		kratosKeywordPatterns[i] = keywordPattern{
			keyword: kw,
			re:      regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`),
		}
	}
}

// resumePhraseREs match phrases signaling the user wants to resume prior work. Matched
// against the same sanitizePrompt() output as god keywords (word-boundary,
// case-insensitive), so code-fenced or quoted mentions don't false-positive.
var resumePhraseREs = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bcontinue\b`),
	regexp.MustCompile(`(?i)\bresume\b`),
	regexp.MustCompile(`(?i)\bkeep\s+going\b`),
	regexp.MustCompile(`(?i)\bwhere\s+were\s+we\b`),
	regexp.MustCompile(`(?i)\bwhere\s+did\s+we\s+stop\b`),
	regexp.MustCompile(`(?i)\bpick\s+up\b`),
}

func matchesResumePhrase(text string) bool {
	for _, re := range resumePhraseREs {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// Patterns to strip before keyword matching (prevent false positives)
var stripPatterns = []*regexp.Regexp{
	regexp.MustCompile("(?s)```.*?```"),                              // fenced code blocks
	regexp.MustCompile("`[^`]+`"),                                    // inline code
	regexp.MustCompile(`<[^>]+>[^<]*</[^>]+>`),                       // XML tags with content
	regexp.MustCompile(`https?://\S+`),                               // URLs
	regexp.MustCompile(`(?:^|\s)[/\\]\S+`),                           // absolute file paths
	regexp.MustCompile(`\S+[/\\]\S+`),                                // relative paths (plugins/kratos, kratos-dev/go)
	regexp.MustCompile(`[A-Za-z0-9_.]+(?:-[A-Za-z0-9_.]+)+`),         // hyphenated compounds (kratos-dev, kratos-bin)
	regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`), // system reminders
}

// subagentStartInput is the JSON Claude Code sends for SubagentStart
type subagentStartInput struct {
	AgentID   string `json:"agent_id"`
	AgentType string `json:"agent_type"`
	Cwd       string `json:"cwd"`
}

// subagentStartOutput is returned to inject context into the subagent
type subagentStartOutput struct {
	HookSpecificOutput subagentStartHookSpecific `json:"hookSpecificOutput"`
}

type subagentStartHookSpecific struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// subagentStopInput is the JSON Claude Code sends for SubagentStop
type subagentStopInput struct {
	AgentType string `json:"agent_type"`
	// AgentID is Claude Code's unique identifier for this specific subagent spawn
	// (documented as a common SubagentStop field, mirroring subagentStartInput's
	// AgentID). Used to key the block-count guards below so a fresh spawn never
	// inherits an earlier, unrelated spawn's count.
	AgentID              string `json:"agent_id"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
	Cwd                  string `json:"cwd"`
	// Transcript paths for the verify gate. AgentTranscriptPath points at the
	// subagent's own sidechain JSONL; TranscriptPath may point at the main
	// session transcript depending on Claude Code version. Both optional —
	// the gate is inactive when neither resolves.
	AgentTranscriptPath string `json:"agent_transcript_path"`
	TranscriptPath      string `json:"transcript_path"`
}

// subagentStopOutput is the SubagentStop/Stop hook response. Claude Code honors
// only a top-level {"decision":"block","reason":"..."} to block; an allow is an
// empty object (or no output) with exit 0. The former {"ok":true|false} shape was
// silently ignored by the harness, so every gate was advisory.
type subagentStopOutput struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// preToolUseInput is the JSON Claude Code sends for PreToolUse.
//
// agent_type is the discriminator the edit gate is built on: a payload from a
// spawned subagent carries it (plus agent_id), a payload from the main context
// carries neither. Verified on real payloads from Claude Code 2.1.268 on
// 2026-09-11; without it the gate would count a subagent's edits against the
// inline god's budget.
type preToolUseInput struct {
	ToolName  string              `json:"tool_name"`
	ToolInput preToolUseToolInput `json:"tool_input"`
	SessionID string              `json:"session_id"`
	Cwd       string              `json:"cwd"`
	AgentType string              `json:"agent_type"`
	AgentID   string              `json:"agent_id"`
	// TranscriptPath is the calling session's own transcript file — the main
	// session's when the caller is inline, or the spawning parent's when the
	// caller is a subagent. handback-gate (hook_handback.go) derives a
	// subagent's own transcript from it: <dir>/<session_id>/subagents/
	// agent-<agent_id>.jsonl.
	TranscriptPath string `json:"transcript_path"`
}

type preToolUseToolInput struct {
	Command string `json:"command"`
	// Write/Edit/MultiEdit target, plus the spellings other tools use for the
	// same thing. NotebookEdit sends notebook_path; some harness versions send
	// path or filePath.
	FilePath     string `json:"file_path"`
	Path         string `json:"path"`
	FilePathAlt  string `json:"filePath"`
	NotebookPath string `json:"notebook_path"`
	// Agent/Task spawn target, e.g. "kratos:ares".
	SubagentType string `json:"subagent_type"`
	// Skill load target, e.g. "kratos:iris" (verified on a real Skill
	// PreToolUse payload, 2026-09-18: a project-level skill sends its bare
	// name; a plugin skill is namespaced "plugin:skill").
	Skill string `json:"skill"`
}

// targetPath is the file an edit tool is about to change, whichever key the
// tool used. "" means the payload names no target — the gate fails open there
// rather than denying a call it cannot even describe.
func (t preToolUseToolInput) targetPath() string {
	return firstNonEmpty(t.FilePath, t.Path, t.FilePathAlt, t.NotebookPath)
}

// preToolUseOutput is the hookSpecificOutput response for PreToolUse
type preToolUseOutput struct {
	HookSpecificOutput preToolUseHookSpecific `json:"hookSpecificOutput"`
}

// preToolUseHookSpecific carries a PreToolUse hook's decision: the edit gate
// sets PermissionDecision/PermissionDecisionReason (only ever "deny" — see
// the contract note in hook_editgate.go) and never AdditionalContext.
type preToolUseHookSpecific struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
}

// HookCmd returns the 'hook' command group
func HookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Hook handlers for Claude Code events",
	}

	cmd.AddCommand(promptSubmitCmd())
	cmd.AddCommand(subagentStartCmd())
	cmd.AddCommand(subagentStopCmd())
	cmd.AddCommand(editGateCmd())
	cmd.AddCommand(specDeltaCheckCmd())
	cmd.AddCommand(handbackGateCmd())
	return cmd
}

// specDeltaPathRE extracts the feature slug from a spec-delta file path,
// tolerating both / and \ separators. Anchored on the .claude/feature/…/
// spec-delta/…md shape so ordinary writes never match.
var specDeltaPathRE = regexp.MustCompile(`\.claude[/\\]feature[/\\]([^/\\]+)[/\\]spec-delta[/\\][^/\\]+\.md$`)

// postToolUseInput is the JSON Claude Code sends for PostToolUse (Write/Edit).
type postToolUseInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

func specDeltaCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spec-delta-check",
		Short: "PostToolUse gate — validate a just-written spec delta so malformed deltas fail immediately, not at archive time",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleSpecDeltaCheck(os.Stdin, cmd.OutOrStdout())
		},
	}
}

// handleSpecDeltaCheck validates the feature whose spec-delta file was just
// written. Fail-open everywhere: any payload/parse/lookup problem exits
// silently — the gate must never break ordinary Write/Edit calls.
func handleSpecDeltaCheck(stdin io.Reader, stdout io.Writer) error {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		debugLog("spec-delta-check: stdin read error: %v", err)
		return nil
	}
	return specDeltaCheckIn(gitRoot(), raw, stdout)
}

// specDeltaRoot returns the project that owns a spec-delta write: prefix is
// the file path up to its ".claude/feature/" segment, i.e. the directory that
// holds .claude/feature/. A relative prefix resolves against fallback. When
// that directory has no .claude/feature/ (unusual payload), fallback wins.
// The file path, not cwd, decides: a session in one repo can write a delta
// into another (sibling checkout, worktree).
func specDeltaRoot(prefix, fallback string) string {
	prefix = strings.TrimRight(prefix, `/\`)
	if prefix == "" {
		return fallback
	}
	if !filepath.IsAbs(prefix) {
		prefix = filepath.Join(fallback, prefix)
	}
	if info, err := os.Stat(filepath.Join(prefix, ".claude", "feature")); err == nil && info.IsDir() {
		return prefix
	}
	return fallback
}

func specDeltaCheckIn(root string, raw []byte, stdout io.Writer) error {
	var input postToolUseInput
	if err := json.Unmarshal(raw, &input); err != nil {
		debugLog("spec-delta-check: json parse error: %v", err)
		return nil
	}

	filePath := input.ToolInput.FilePath
	m := specDeltaPathRE.FindStringSubmatchIndex(filePath)
	if m == nil {
		return nil
	}
	feature := filePath[m[2]:m[3]]
	root = specDeltaRoot(filePath[:m[0]], root)

	ok, messages, err := specValidateIn(root, feature, false)
	if err != nil {
		// Missing feature dir, unreadable delta, invalid slug — not this
		// gate's problem. The write itself succeeded; archive will report.
		debugLog("spec-delta-check: validate error for %s: %v", feature, err)
		return nil
	}
	if ok {
		return nil
	}
	// "no spec delta files found" means this process's root doesn't see the
	// just-written file (cwd mismatch, unusual layout) — an infra condition,
	// not a malformed delta. Fail open rather than block on it.
	if len(messages) == 1 && strings.Contains(messages[0], "no spec delta files found") {
		debugLog("spec-delta-check: root mismatch for %s — failing open", feature)
		return nil
	}

	reason := fmt.Sprintf(
		"Spec delta validation failed for feature %q: %s — fix the delta now (the file must start directly with ## ADDED/MODIFIED/REMOVED/RENAMED Requirements; every ADDED/MODIFIED requirement needs a SHALL statement and ≥1 #### Scenario:; ADDED vs MODIFIED is relative to the living spec at .claude/.Arena/specs/<capability>/spec.md, not the code: if the capability has no living spec or the requirement isn't recorded there yet, it is ADDED — even for a bug fix to existing behavior).",
		feature, strings.Join(messages, "; "),
	)
	return json.NewEncoder(stdout).Encode(map[string]string{
		"decision": "block",
		"reason":   reason,
	})
}

func promptSubmitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prompt-submit",
		Short: "Handle UserPromptSubmit hook — detect Kratos keywords and inject skill activation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handlePromptSubmit()
		},
	}
}

func handlePromptSubmit() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		debugLog("stdin read error: %v", err)
		return outputPassthrough()
	}

	// Ledger side effect (fail-open): make sure this Claude Code session has a
	// row and remember its first real prompt. Never changes the hook output.
	recordPromptLedger(raw)

	return outputJSON(promptSubmitIn(raw))
}

// promptSubmitIn computes the UserPromptSubmit hook response for a raw JSON payload.
// Factored out from handlePromptSubmit so tests can exercise the merged
// keyword+handoff logic without redirecting os.Stdin.
func promptSubmitIn(raw []byte) hookOutput {
	var input hookInput
	if err := json.Unmarshal(raw, &input); err != nil {
		debugLog("json parse error: %v", err)
		return passthroughOutput()
	}

	prompt := input.Prompt
	if prompt == "" {
		return passthroughOutput()
	}

	// Direct Kratos skill invocations handle their own agent routing via CLI —
	// suppress auto-routing injection so the skill is not double-routed through auto/quick.
	if strings.HasPrefix(strings.TrimSpace(prompt), "/kratos:") {
		return passthroughOutput()
	}

	// Expanded slash-command bodies reach this hook without the "/kratos:"
	// prefix: the launcher's KRATOS_ROOT echo and `agent load <god> --resolve`
	// lines plus the user's arguments. They are not user prose — a god name in
	// the arguments ("have ares on it") must not trigger a second routing
	// (observed on a /kratos:iris turn, 2026-08-31).
	if isExpandedLauncherBody(prompt) {
		return passthroughOutput()
	}

	// A subagent hand-back (or the harness's own <task-notification>) is model
	// output, not user prose: a report naming "ares, hermes" fired the keyword
	// injection in the 2026-09-18 review, steering a system-level instruction
	// from text the user never wrote.
	if isHarnessPseudoPrompt(prompt) {
		return passthroughOutput()
	}

	// Sanitize: strip code blocks, URLs, paths, system reminders
	cleaned := sanitizePrompt(prompt)

	// Match keywords (case-insensitive, word-boundary)
	matched := matchKeywords(cleaned)

	var keywordContext string
	if len(matched) > 0 {
		debugLog("matched keywords: %v", matched)
		keywordContext = buildKeywordContext(matched, cleaned)
	}

	// Independent of keyword matching — a bare "continue" must inject the handoff
	// even when no god keyword was mentioned. Never let one context's absence
	// suppress the other.
	handoffContext := handoffInjectionContext(cleaned, input)

	merged := mergeContexts(keywordContext, handoffContext)
	if merged == "" {
		return passthroughOutput()
	}

	return hookOutput{
		Continue: true,
		HookSpecificOutput: &hookSpecificOutput{
			HookEventName:     "UserPromptSubmit",
			AdditionalContext: merged,
		},
	}
}

// mergeContexts joins non-empty context blocks with a blank-line separator. Returns ""
// when every part is empty, which signals the caller to pass the prompt through untouched.
func mergeContexts(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

const (
	handoffMaxAge       = 7 * 24 * time.Hour
	handoffMaxBytes     = 8 * 1024
	handoffMarkerMaxAge = 7 * 24 * time.Hour
)

// handoffInjectionContext returns the on-demand session-handoff injection for a
// resume-phrase prompt, or "" when no injection should happen: no resume phrase
// matched, no handoff file, the handoff is stale (>=7 days), or it was already
// injected this session. Fails open on every error: a missing/unreadable handoff
// file or unresolvable cwd degrades to "no injection"; a marker I/O failure does NOT
// suppress this run's injection — it only means the once-per-session guard may not
// take effect on the next matching prompt. No error path can block the prompt.
func handoffInjectionContext(cleanedPrompt string, input hookInput) string {
	if !matchesResumePhrase(cleanedPrompt) {
		return ""
	}

	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if cwd == "" {
		return ""
	}

	handoffPath := filepath.Join(cwd, ".claude", ".Arena", "handoff.md")
	info, err := os.Stat(handoffPath)
	if err != nil {
		return ""
	}
	if time.Since(info.ModTime()) >= handoffMaxAge {
		return ""
	}

	// Once-per-session guard. A missing session_id can't be keyed, so degrade to
	// "always inject" rather than silently dropping the handoff every time.
	if input.SessionID != "" && handoffMarkerExists(input.SessionID) {
		return ""
	}

	content, err := os.ReadFile(handoffPath)
	if err != nil {
		return ""
	}

	capped := capUTF8Bytes(string(content), handoffMaxBytes)

	if input.SessionID != "" {
		markHandoffInjected(input.SessionID)
	}

	return fmt.Sprintf(
		"[KRATOS SESSION HANDOFF]\n\nResume phrase detected — injecting the handoff from last session (%s). Use /kratos:recall for the full picture.\n\n%s",
		formatHandoffAge(info.ModTime()), capped,
	)
}

// capUTF8Bytes truncates s to at most maxBytes bytes without splitting a multi-byte
// UTF-8 rune. Walks back from the byte cut point to the last full rune boundary, then
// strings.ToValidUTF8 as a second safety net for any remaining partial rune.
func capUTF8Bytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := s[:maxBytes]
	for len(b) > 0 && !utf8.RuneStart(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	b = strings.ToValidUTF8(b, "")
	return b + "\n... (truncated)"
}

// formatHandoffAge renders a human-readable "N units ago" string for the handoff
// injection notice.
func formatHandoffAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/24/7))
	}
}

// handoffMarkerDir returns ~/.kratos/handoff-injections, or "" if the home directory
// can't be resolved (fails open — caller treats "" as "no guard, always inject").
func handoffMarkerDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".kratos", "handoff-injections")
}

func handoffMarkerExists(sessionID string) bool {
	dir := handoffMarkerDir()
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, sessionID))
	return err == nil
}

// markHandoffInjected writes the once-per-session marker and prunes markers older
// than 7 days (mirrors memory-sweep.cjs's pruneOldMarkers). Best-effort: any failure
// is logged and swallowed — a marker write failure must still let this run's
// injection through, never block the prompt.
func markHandoffInjected(sessionID string) {
	dir := handoffMarkerDir()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		debugLog("handoff marker: mkdir failed: %v", err)
		return
	}
	path := filepath.Join(dir, sessionID)
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d", time.Now().Unix())), 0644); err != nil {
		debugLog("handoff marker: write failed: %v", err)
	}
	pruneHandoffMarkers(dir)
}

// pruneHandoffMarkers removes marker files older than 7 days so the directory doesn't
// grow forever. Best-effort — errors are ignored.
func pruneHandoffMarkers(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-handoffMarkerMaxAge)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func sanitizePrompt(prompt string) string {
	cleaned := prompt
	for _, pattern := range stripPatterns {
		cleaned = pattern.ReplaceAllString(cleaned, " ")
	}
	return cleaned
}

func matchKeywords(text string) []string {
	var matched []string
	for _, kp := range kratosKeywordPatterns {
		if kp.re.MatchString(text) {
			matched = append(matched, kp.keyword)
		}
	}
	return matched
}

func buildInjectionContext(matched []string) string {
	// Determine if it's "kratos" itself or a specific god name
	hasKratos := false
	var godNames []string

	for _, kw := range matched {
		if kw == "kratos" {
			hasKratos = true
		} else {
			godNames = append(godNames, kw)
		}
	}

	var sb strings.Builder
	sb.WriteString("[KRATOS KEYWORD DETECTED]\n\n")

	if hasKratos {
		sb.WriteString("The user invoked Kratos by name. ")
	}
	if len(godNames) > 0 {
		sb.WriteString("God-agent(s) mentioned: ")
		sb.WriteString(strings.Join(godNames, ", "))
		sb.WriteString(". ")
	}

	sb.WriteString("\nYou MUST invoke the Kratos skill using the Skill tool:\n")
	sb.WriteString("Skill(skill: \"kratos:auto\")\n\n")
	sb.WriteString("Do NOT respond to the user's message directly. Invoke the skill FIRST, then follow its instructions to handle the user's request.")

	return sb.String()
}

// passthroughOutput is the hookOutput signaling "no injection, let the prompt through
// unchanged."
func passthroughOutput() hookOutput {
	return hookOutput{Continue: true}
}

func outputPassthrough() error {
	return outputJSON(passthroughOutput())
}

func outputJSON(output hookOutput) error {
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

const todoQualityGate = `
## KRATOS QUALITY GATE — MANDATORY BEFORE ANY TOOL CALL

1. Write your complete numbered TODO list FIRST. Format:
   TODO:
   1. [ ] Task description
   2. [ ] Task description
   ...
2. Work through each item in order.
3. Mark each item [x] as you complete it.
4. Do NOT call any tool before your TODO list is written.
`

// aresTaskGate is injected for Ares specifically — and only in subagent mode,
// where the harness does NOT expose TaskCreate/TaskUpdate/TaskList to subagents
// (agent-teams only). The gate therefore mandates a markdown task list, and warns
// against calling the Task* tools Ares's own instructions mention for inline mode.
// The closing "Task list:" recap is what the SubagentStop gate matches on (it can
// only see the final message text, not tool calls), so the recap keeps the gate
// meaningful.
//
// Neither gate repeats the output-format constraint: the protocol block that
// path-inject.cjs injects for the same spawn already carries it.
const aresTaskGate = `
## KRATOS QUALITY GATE — CREATE YOUR TASK LIST FIRST

You are a SUBAGENT: TaskCreate/TaskUpdate/TaskList are NOT available here — do not call them, do not retry on denial.

1. Write a markdown task checklist BEFORE any other work — one item per file/module, not one vague "implement" (small missions ≤2 files: one umbrella item is enough).
2. Mark an item in-progress when you start it.
3. Mark [x] ONLY when truly done (tests green).
4. Add any new work that surfaces mid-mission to the list.
5. End with a "Task list:" recap of every item + status.
`

// subagentStartCmd injects a mandatory TODO-first instruction into Ares and Hephaestus agents.
// For Hermes it creates a tier checklist file and injects instructions to update it.
func subagentStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "subagent-start",
		Short: "Handle SubagentStart hook — inject TODO-first quality gate or Hermes tier checklist",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				debugLog("subagent-start: stdin read error: %v", err)
				return outputSubagentStartContext(todoQualityGate)
			}

			var input subagentStartInput
			if err := json.Unmarshal(raw, &input); err != nil {
				debugLog("subagent-start: json parse error: %v", err)
				return outputSubagentStartContext(todoQualityGate)
			}

			agentType := strings.ToLower(input.AgentType)

			if strings.Contains(agentType, "hermes") {
				return handleHermesStart(input)
			}

			// Spec/impl agents may be tempted to re-scan the codebase; nudge them
			// to reuse the Arena knowledge base instead (empty when no Arena exists).
			reminder := arenaScanReminder(input.Cwd)

			// Ares gets the subagent-specific gate: markdown checklist, no Task* tools.
			if strings.Contains(agentType, "ares") {
				return outputSubagentStartContext(aresTaskGate + reminder)
			}

			// Hephaestus writes the spec and is the other agent prone to re-globbing.
			if strings.Contains(agentType, "hephaestus") {
				return outputSubagentStartContext(todoQualityGate + reminder)
			}

			// hooks.json wires `hook subagent-start` only for ares, hephaestus and
			// hermes. Any other agent type reaching here is a wiring change we did
			// not plan for — inject nothing rather than a gate the agent's
			// SubagentStop hook never checks.
			return nil
		},
	}
}

// handleHermesStart creates the hermes-checklist.json and injects tier instructions.
func handleHermesStart(input subagentStartInput) error {
	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	checklistDir, err := findActiveFeatureDir(cwd)
	if err != nil || checklistDir == "" {
		// Fall back to .claude/tmp/
		checklistDir = filepath.Join(cwd, ".claude", "tmp")
		debugLog("hermes-start: no active feature found, using fallback dir: %s", checklistDir)
	}

	if err := os.MkdirAll(checklistDir, 0755); err != nil {
		debugLog("hermes-start: failed to create checklist dir: %v", err)
		return outputSubagentStartContext(todoQualityGate)
	}

	checklistPath := filepath.Join(checklistDir, "hermes-checklist.json")

	tiers := map[string]bool{
		"T1_correct":      false,
		"T2_safe":         false,
		"T3_clear":        false,
		"T4_minimal":      false,
		"T5_consistent":   false,
		"T6_resilient":    false,
		"T7_performant":   false,
		"T8_maintainable": false,
	}
	blockCount := 0

	// A child's async report resumes Hermes and fires this hook again with
	// Hermes's own, unchanged agent id (see hermes.md Step 3b). That resume
	// must not look like a fresh spawn: it kept resetting every tier mark and
	// the block-count guard on the NETZERO 09-17 review, so the guard never
	// tripped and Hermes re-answered `for t in T1..T8; do hermes-list check
	// $t; done` against a blank slate every time. Only a genuinely different
	// agent id — an actual new Hermes spawn — starts the checklist over.
	if existing, ok := readHermesChecklistState(checklistPath); ok && input.AgentID != "" && existing.AgentID == input.AgentID {
		tiers = existing.Tiers
		blockCount = existing.BlockCount
	}

	checklist := map[string]interface{}{
		"agent_id":    input.AgentID,
		"block_count": blockCount,
		"tiers":       tiers,
	}

	checklistData, err := json.MarshalIndent(checklist, "", "  ")
	if err != nil {
		debugLog("hermes-start: failed to marshal checklist: %v", err)
		return outputSubagentStartContext(todoQualityGate)
	}

	if err := os.WriteFile(checklistPath, checklistData, 0644); err != nil {
		debugLog("hermes-start: failed to write checklist: %v", err)
		return outputSubagentStartContext(todoQualityGate)
	}

	debugLog("hermes-start: created checklist at %s", checklistPath)

	additionalContext := fmt.Sprintf(
		"TIER CHECKLIST FILE: %s\n"+
			"After each tier review run (Bash tool, do NOT edit the file directly):\n"+
			"  '%s' hermes-list check T1   # after T1, T2 for T2, … T8 for T8\n"+
			"Run immediately after each tier — not in a batch at the end.\n"+
			"A hook verifies all 8 tiers on stop — incomplete tiers block completion.",
		checklistPath, kratosBinPath(),
	)

	return outputSubagentStartContext(additionalContext)
}

// hermesChecklistState is the persisted shape of hermes-checklist.json that
// handleHermesStart needs to carry across a resume.
type hermesChecklistState struct {
	AgentID    string          `json:"agent_id"`
	BlockCount int             `json:"block_count"`
	Tiers      map[string]bool `json:"tiers"`
}

// readHermesChecklistState reads an existing checklist for the resume check
// above. ok is false on any read or parse error, or when the file has no
// tiers map — handleHermesStart then falls back to a fresh checklist, the
// same fail-open behavior the rest of this file uses for a missing or
// malformed checklist.
func readHermesChecklistState(path string) (hermesChecklistState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return hermesChecklistState{}, false
	}
	var s hermesChecklistState
	if err := json.Unmarshal(data, &s); err != nil || s.Tiers == nil {
		return hermesChecklistState{}, false
	}
	return s, true
}

// findActiveFeatureDir scans .claude/feature/*/status.json and returns the feature folder
// for the first feature where stage 9-review has status pending, in-progress, or ready.
func findActiveFeatureDir(cwd string) (string, error) {
	pattern := filepath.Join(cwd, ".claude", "feature", "*", "status.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	activeStatuses := map[string]bool{
		"pending":     true,
		"in-progress": true,
		"ready":       true,
	}

	for _, statusFile := range matches {
		data, err := os.ReadFile(statusFile)
		if err != nil {
			debugLog("findActiveFeatureDir: failed to read %s: %v", statusFile, err)
			continue
		}

		var statusJSON map[string]interface{}
		if err := json.Unmarshal(data, &statusJSON); err != nil {
			debugLog("findActiveFeatureDir: failed to parse %s: %v", statusFile, err)
			continue
		}

		// Navigate: pipeline["9-review"].status
		pipeline, ok := statusJSON["pipeline"].(map[string]interface{})
		if !ok {
			continue
		}
		reviewStage, ok := pipeline["9-review"].(map[string]interface{})
		if !ok {
			continue
		}
		status, ok := reviewStage["status"].(string)
		if !ok {
			continue
		}

		if activeStatuses[strings.ToLower(status)] {
			return filepath.Dir(statusFile), nil
		}
	}

	return "", nil
}

// outputSubagentStartContext writes the SubagentStart JSON response to stdout.
func outputSubagentStartContext(additionalContext string) error {
	output := subagentStartOutput{
		HookSpecificOutput: subagentStartHookSpecific{
			HookEventName:     "SubagentStart",
			AdditionalContext: additionalContext,
		},
	}
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// gatedAgents are the agents whose SubagentStop has a quality gate that must not be
// bypassable via a malformed payload.
var gatedAgents = []string{"ares", "hephaestus", "hermes", "nemesis", "athena"}

// rawAgentTypeRE pulls the agent_type value out of a payload that failed to parse
// as JSON (the field itself is usually intact; it is the message body that breaks).
var rawAgentTypeRE = regexp.MustCompile(`"agent_type"\s*:\s*"([^"]*)"`)

// gatedAgentREs match gated agent names on word boundaries, so "shares" or
// "compares" in a message body never read as Ares.
var gatedAgentREs = func() []*regexp.Regexp {
	res := make([]*regexp.Regexp, len(gatedAgents))
	for i, a := range gatedAgents {
		res[i] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(a) + `\b`)
	}
	return res
}()

// gatedAgentInRaw reports which gated agent (if any) a raw, unparseable payload
// concerns. It prefers the agent_type field when that survived the corruption;
// otherwise it scans the whole payload on word boundaries — a message body that
// names a gated agent still trips it, deliberately: on a malformed payload we
// fail closed rather than let a gated agent slip through.
func gatedAgentInRaw(raw []byte) string {
	haystack := string(raw)
	if m := rawAgentTypeRE.FindSubmatch(raw); m != nil {
		haystack = string(m[1])
	}
	for i, re := range gatedAgentREs {
		if re.MatchString(haystack) {
			return gatedAgents[i]
		}
	}
	return ""
}

// rawHasStopHookActive detects a stop_hook_active:true marker in a raw payload that failed
// to parse, so the fail-closed branch can't trap an agent in a re-invocation loop.
func rawHasStopHookActive(raw []byte) bool {
	s := strings.ToLower(string(raw))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	return strings.Contains(s, `"stop_hook_active":true`)
}

// testCmdRE matches test-runner invocations across common ecosystems. Word-bounded so
// "attest" or "npmrc test" don't count as evidence.
var testCmdRE = regexp.MustCompile(`(?i)(^|[\s;&|(])(pytest\b|go\s+test\b|cargo\s+test\b|(npm|pnpm)\s+(run\s+)?test\b|yarn\s+test\b|bun\s+test\b|npx\s+(vitest|jest|playwright)\b|vitest\b|jest\b|playwright\s+test\b|dotnet\s+test\b|make\s+test\b|mvnw?\s+(\S+\s+)*test\b|gradlew?\s+(\S+\s+)*test\b|rspec\b|phpunit\b|ctest\b|python3?\s+-m\s+(pytest|unittest)\b|deno\s+test\b|node\s+--test\b|mix\s+test\b)`)

// codeFileExts are the extensions that count as "code was edited" for the verify gate.
// Deliberately excludes .md/.json/.yaml — deliverable and config writes alone must not
// demand a test run.
var codeFileExts = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".go": true, ".rs": true, ".java": true, ".cs": true, ".rb": true,
	".c": true, ".cpp": true, ".h": true, ".hpp": true, ".php": true, ".swift": true,
	".kt": true, ".ex": true, ".exs": true,
}

func isCodeFile(path string) bool {
	return codeFileExts[strings.ToLower(filepath.Ext(path))]
}

// transcriptEntry is the subset of a transcript JSONL line the verify gate needs.
type transcriptEntry struct {
	Type        string `json:"type"`
	IsSidechain bool   `json:"isSidechain"`
	Message     struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// transcriptToolUse is a tool_use block inside an assistant message's content array.
type transcriptToolUse struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Input struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Command      string `json:"command"`
	} `json:"input"`
}

// localCommandPrefixes mark user entries that are harness-injected, not real prompts.
var localCommandPrefixes = []string{
	"<command-name>", "<local-command-stdout>", "<local-command-stderr>", "<local-command-caveat>",
}

// isRealUserPrompt reports whether a transcript entry is a genuine main-session user
// message (tool_result entries carry a content array, not a string; sidechain "user"
// entries belong to a subagent's inner loop).
func isRealUserPrompt(entry transcriptEntry) bool {
	if entry.Type != "user" || entry.IsSidechain {
		return false
	}
	var text string
	if json.Unmarshal(entry.Message.Content, &text) != nil {
		return false
	}
	trimmed := strings.TrimSpace(text)
	for _, p := range localCommandPrefixes {
		if strings.HasPrefix(trimmed, p) {
			return false
		}
	}
	return true
}

// transcriptTestEvidence scans a session transcript and reports whether, since the last
// real user prompt, sidechain (subagent) tool calls edited code files and whether any
// test command ran. SubagentStop only receives the MAIN session transcript, so the
// sidechain filter is what scopes the scan to subagent activity, and resetting at each
// real user prompt scopes it to the current turn's spawn. Unparseable lines are skipped
// (fail open per line); only I/O-level errors are returned.
func transcriptTestEvidence(path string, sidechainOnly bool) (editedCode, ranTests bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return false, false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var entry transcriptEntry
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		if isRealUserPrompt(entry) {
			// New turn: evidence from earlier turns (e.g. a previous agent's test
			// run) must not vouch for this one.
			editedCode, ranTests = false, false
			continue
		}
		if entry.Type != "assistant" || (sidechainOnly && !entry.IsSidechain) {
			continue
		}
		var blocks []transcriptToolUse
		if json.Unmarshal(entry.Message.Content, &blocks) != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type != "tool_use" {
				continue
			}
			switch b.Name {
			case "Write", "Edit", "MultiEdit", "NotebookEdit":
				p := b.Input.FilePath
				if p == "" {
					p = b.Input.NotebookPath
				}
				if isCodeFile(p) {
					editedCode = true
				}
			case "Bash", "PowerShell":
				if testCmdRE.MatchString(b.Input.Command) {
					ranTests = true
				}
			}
		}
	}
	if scanErr := sc.Err(); scanErr != nil {
		return false, false, scanErr
	}
	return editedCode, ranTests, nil
}

// aresVerifyGateFailure runs the fail-then-pass verify gate for Ares: if the transcript
// shows code edits with no test command since the last user prompt, it returns a
// non-empty failure string. Every infra problem (missing path, unreadable file) fails
// OPEN — this gate must never block on anything but genuine missing test evidence.
func aresVerifyGateFailure(input subagentStopInput) string {
	if strings.Contains(strings.ToLower(input.LastAssistantMessage), "tests-not-applicable:") {
		return ""
	}
	path := input.AgentTranscriptPath
	sidechainOnly := false
	if path == "" {
		path = input.TranscriptPath
		sidechainOnly = true
	}
	if path == "" {
		return ""
	}
	editedCode, ranTests, err := transcriptTestEvidence(path, sidechainOnly)
	if err != nil {
		debugLog("ares verify gate: transcript scan failed (fail-open): %v", err)
		return ""
	}
	if editedCode && !ranTests {
		return "code files were edited but no test command was run (run the relevant tests and record fail-then-pass evidence, or state TESTS-NOT-APPLICABLE: <reason> if this change genuinely has no runtime surface)"
	}
	return ""
}

// subagentStopCmd verifies that Ares and Hephaestus produced complete deliverables.
// Prints {} to allow completion or {"decision": "block", "reason": "..."} to block.
func subagentStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "subagent-stop",
		Short: "Handle SubagentStop hook — quality gate for Ares and Hephaestus",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				// Unreadable stdin: no bytes to inspect, so we can't tell which agent
				// this is. Fail closed for gated agents would loop here (we'd never see
				// stop_hook_active either), and this path is an infra error, not a
				// content-reachable bypass — allow.
				return outputSubagentOK()
			}

			var input subagentStopInput
			if err := json.Unmarshal(raw, &input); err != nil {
				// A subagent's message content must not be able to break JSON parsing and
				// thereby skip its quality gate. If the malformed payload concerns a gated
				// agent, fail CLOSED so the gate can't be bypassed. Honor a stop_hook_active
				// marker first so a persistently-malformed payload can't trap the agent in a
				// re-invocation loop.
				if rawHasStopHookActive(raw) {
					return outputSubagentOK()
				}
				if agent := gatedAgentInRaw(raw); agent != "" {
					return outputSubagentBlock(fmt.Sprintf(
						"Malformed SubagentStop payload for %s — failing closed so the quality gate cannot be bypassed. Re-emit a valid final message confirming the task list, files changed, and completion.",
						agent,
					))
				}
				return outputSubagentOK()
			}

			// stop_hook_active=true is a re-invocation after this same hook
			// already blocked once on this stop attempt, not a verified-clean
			// stop. Unconditionally allowing here (the old behavior) let a
			// blocked Ares/Hephaestus/Hermes/Nemesis agent through on its very
			// next try with the deliverable still missing, and it kept every
			// gate's own block-count guard (below) from ever incrementing
			// past 1. The checks below must run the same way regardless of
			// this flag; every content gate now carries its own bounded cap —
			// Hermes via block_count in hermes-checklist.json, the Tier 1
			// "check --verify" gates via MaxRetries in check-state.json, and
			// Ares/Hephaestus/Nemesis via evaluateGateBlock's gateMaxBlocks
			// (added because they previously had no cap at all: an
			// unsatisfiable gate forced continuations indefinitely, since the
			// "ends the turn after 8 consecutive blocks" override is
			// documented only for the plain Stop hook, not SubagentStop) — so
			// none of them can force continuations indefinitely.
			agentType := strings.ToLower(input.AgentType)
			msg := input.LastAssistantMessage
			msgLower := strings.ToLower(msg)

			// Ares (implementation agent) quality checks
			if strings.Contains(agentType, "ares") {
				var failures []string

				// Subagent Ares plans via a markdown checklist (Task* tools are not
				// available to subagents); the SubagentStop hook can only see the
				// final message, so it matches the "Task list:" recap Ares is
				// instructed to print at the end (text TODO still accepted).
				hasTaskList := strings.Contains(msgLower, "task list:") ||
					strings.Contains(msgLower, "todo:") ||
					regexp.MustCompile(`(?i)##\s*(tasks|todo|plan)`).MatchString(msg)
				if !hasTaskList {
					failures = append(failures, "no task list recap was written before starting work")
				}

				mentionsFiles := regexp.MustCompile(`(?i)(created|wrote|implemented|modified|updated).*\.(ts|js|py|go|rs|java|cs|rb|md)`).MatchString(msg)
				if !mentionsFiles {
					failures = append(failures, "no specific files were mentioned as created or modified")
				}

				declaresComplete := strings.Contains(msgLower, "complete") ||
					strings.Contains(msgLower, "done") ||
					strings.Contains(msgLower, "finished") ||
					strings.Contains(msgLower, "implemented")
				if !declaresComplete {
					failures = append(failures, "implementation completion was not confirmed")
				}

				if f := aresLandedGateFailure(input); f != "" {
					failures = append(failures, f)
				}

				if f := aresVerifyGateFailure(input); f != "" {
					failures = append(failures, f)
				}

				statePath := gateStatePath(input.Cwd, "", "ares-stop-state.json")
				if len(failures) == 0 {
					clearGateBlock(statePath)
				} else {
					return evaluateGateBlock("ares", statePath, input.AgentID, fmt.Sprintf(
						"Ares quality gate failed: %s. Write a markdown task checklist (Task* tools are unavailable to subagents), implement all items, end with a 'Task list:' recap naming the files you created or modified, and land the work: commit your files on the current branch and report `Landed: <branch>@<hash>`.",
						strings.Join(failures, "; "),
					))
				}
			}

			// Hephaestus (tech spec agent) quality checks
			if strings.Contains(agentType, "hephaestus") {
				var failures []string

				specSections := []string{"architecture", "data model", "api", "implementation", "schema", "interface"}
				var found []string
				for _, s := range specSections {
					if strings.Contains(msgLower, s) {
						found = append(found, s)
					}
				}
				if len(found) < 2 {
					failures = append(failures, fmt.Sprintf(
						"technical spec appears incomplete (only found sections: %s)",
						func() string {
							if len(found) == 0 {
								return "none"
							}
							return strings.Join(found, ", ")
						}(),
					))
				}

				// Disk check: verify tech-spec-proposal.md or tech-spec.md was written to
				// THIS Hephaestus's feature dir — the one whose pipeline has 4-tech-spec
				// as its current stage — not any feature dir on disk. Checking every
				// feature/* dir let a stale, unrelated feature's tech-spec.md satisfy the
				// gate while the feature actually being worked on had nothing.
				// Only enforce when a pipeline feature is actually on this stage (allows
				// fail-open in pure command mode or when resolution is ambiguous).
				cwd := input.Cwd
				if cwd == "" {
					cwd, _ = os.Getwd()
				}
				featureDir, ferr := findFeatureDirByStage(cwd, "4-tech-spec")
				if ferr != nil {
					debugLog("hephaestus-stop: findFeatureDirByStage error: %v", ferr)
				}
				if featureDir != "" {
					specFound := discoverFileExists(filepath.Join(featureDir, "tech-spec-proposal.md")) ||
						discoverFileExists(filepath.Join(featureDir, "tech-spec.md"))
					if !specFound {
						failures = append(failures, fmt.Sprintf(
							"neither tech-spec-proposal.md nor tech-spec.md was found in %s", featureDir,
						))
					}
				}

				statePath := gateStatePath(cwd, featureDir, "hephaestus-stop-state.json")
				if len(failures) == 0 {
					clearGateBlock(statePath)
				} else {
					return evaluateGateBlock("hephaestus", statePath, input.AgentID, fmt.Sprintf(
						"Hephaestus quality gate failed: %s. A complete spec must cover architecture, data models, API design, and implementation details, written to .claude/feature/<name>/ before completing.",
						strings.Join(failures, "; "),
					))
				}
			}

			// Athena (PRD agent) spec-delta validation gate — runs in addition to the
			// existing "check --verify" Tier 1 file-existence gate (hooks.json wires both).
			if strings.Contains(agentType, "athena") {
				return handleAthenaStop(input)
			}

			// Nemesis (PRD challenge agent) quality checks
			if strings.Contains(agentType, "nemesis") {
				return handleNemesisStop(input)
			}

			// Hermes (code review agent) tier checklist checks
			if strings.Contains(agentType, "hermes") {
				return handleHermesStop(input)
			}

			return outputSubagentOK()
		},
	}
}

// handleAthenaStop verifies that Athena wrote a spec delta and that it passes
// `kratos spec validate` (non-strict — warnings do not block completion). Fails open
// when no feature directory with a prd.md can be found (quick/command mode, or a
// non-CREATE_PRD Athena invocation that never touches prd.md).
func handleAthenaStop(input subagentStopInput) error {
	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	featureDir := findMostRecentFeatureDirWithFile(cwd, "prd.md")
	if featureDir == "" {
		debugLog("athena-stop: no feature dir with prd.md found, failing open")
		return outputSubagentOK()
	}
	feature := filepath.Base(featureDir)

	files, err := listFeatureDeltaFilesIn(cwd, feature)
	if err != nil {
		debugLog("athena-stop: error listing delta files for %q: %v", feature, err)
		return outputSubagentOK()
	}
	if len(files) == 0 {
		return outputSubagentBlock(fmt.Sprintf(
			"Athena quality gate failed: no spec delta found at .claude/feature/%s/spec-delta/. After writing prd.md, assign a capability (existing or new) and write spec-delta/<capability>.md using the spec-delta-template before completing.",
			feature,
		))
	}

	ok, messages, err := specValidateIn(cwd, feature, false)
	if err != nil {
		debugLog("athena-stop: spec validate error for %q: %v", feature, err)
		return outputSubagentOK()
	}
	if !ok {
		return outputSubagentBlock(fmt.Sprintf(
			"Athena quality gate failed: kratos spec validate found errors in the spec delta:\n%s\nFix the delta before completing. Reminder: ADDED vs MODIFIED is relative to the living spec at .claude/.Arena/specs/<capability>/spec.md, not the code: if the capability has no living spec or the requirement isn't recorded there yet, it is ADDED — even for a bug fix to existing behavior.",
			strings.Join(messages, "\n"),
		))
	}

	debugLog("athena-stop: spec delta valid for %q, allowing stop", feature)
	return outputSubagentOK()
}

// findMostRecentFeatureDirWithFile scans .claude/feature/*/<filename> and returns the
// feature directory whose file has the most recent modification time.
func findMostRecentFeatureDirWithFile(cwd, filename string) string {
	pattern := filepath.Join(cwd, ".claude", "feature", "*", filename)
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	best := matches[0]
	bestInfo, err := os.Stat(best)
	if err != nil {
		return filepath.Dir(best)
	}
	for _, m := range matches[1:] {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.ModTime().After(bestInfo.ModTime()) {
			best = m
			bestInfo = info
		}
	}
	return filepath.Dir(best)
}

// challengeHeadingRe matches any markdown heading that contains the word "challenge".
var challengeHeadingRe = regexp.MustCompile(`(?im)^#{1,6}\s+.*challenge`)

// handleNemesisStop verifies prd-challenge.md exists, is non-empty, and contains at least one challenge section.
// Fails open when no feature is currently on stage 2-prd-review (allows completion in
// quick/command mode, or when resolution is ambiguous).
func handleNemesisStop(input subagentStopInput) error {
	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Resolve THIS Nemesis's own feature — the one whose pipeline has
	// 2-prd-review as its current stage — rather than any feature/* dir on
	// disk. Accepting any dir's prd-challenge.md let an old, unrelated
	// feature's valid challenge satisfy the gate for a feature with none.
	featureDir, err := findFeatureDirByStage(cwd, "2-prd-review")
	if err != nil {
		debugLog("nemesis-stop: findFeatureDirByStage error: %v", err)
	}
	if featureDir == "" {
		debugLog("nemesis-stop: no feature on stage 2-prd-review, failing open")
		return outputSubagentOK()
	}

	statePath := gateStatePath(cwd, featureDir, "nemesis-stop-state.json")

	var failure string
	challengePath := filepath.Join(featureDir, "prd-challenge.md")
	if !discoverFileExists(challengePath) {
		failure = fmt.Sprintf(
			"prd-challenge.md not found in %s. Write your PRD challenge to .claude/feature/<name>/prd-challenge.md before completing.",
			featureDir,
		)
	} else if content, rerr := os.ReadFile(challengePath); rerr != nil || len(strings.TrimSpace(string(content))) == 0 {
		failure = "prd-challenge.md exists but is empty. Add at least one challenge section before completing."
	} else if !challengeHeadingRe.Match(content) {
		failure = "prd-challenge.md contains no challenge sections (expected at least one heading containing 'challenge'). Structure your output with explicit challenge headings."
	}

	if failure == "" {
		clearGateBlock(statePath)
		debugLog("nemesis-stop: prd-challenge.md valid, allowing stop")
		return outputSubagentOK()
	}

	return evaluateGateBlock("nemesis", statePath, input.AgentID, "Nemesis quality gate failed: "+failure)
}

// outputSubagentOK allows the subagent to stop: an empty JSON object, exit 0.
func outputSubagentOK() error {
	fmt.Println("{}")
	return nil
}

// outputSubagentBlock blocks the subagent from stopping with the given reason,
// using the only shape Claude Code honors for SubagentStop/Stop.
func outputSubagentBlock(reason string) error {
	data, _ := json.Marshal(subagentStopOutput{Decision: "block", Reason: reason})
	fmt.Println(string(data))
	return nil
}

// gateBlockState is the persisted block-count guard for a SubagentStop content gate that has
// no bounded cap of its own (Hermes has block_count in hermes-checklist.json; the Tier 1
// "check --verify" gates have MaxRetries in check-state.json — see check.go's
// handleRetryLogic). Ares, Hephaestus, and Nemesis's content gates had neither, so an
// unsatisfiable condition forced continuations indefinitely: the "ends the turn after 8
// consecutive blocks" override is documented only for the plain Stop hook, not SubagentStop.
//
// Keyed by agent_id so a fresh spawn (a different agent_id) starts its own count at 0 instead
// of inheriting a previous, unrelated spawn's count — the same cross-spawn carry-over class of
// bug already fixed for check --verify's retry counter and Hermes's block_count.
type gateBlockState struct {
	AgentID    string `json:"agent_id"`
	BlockCount int    `json:"block_count"`
	// Progress is a gate-defined measure of forward motion since the last
	// denial (the handback gate uses the count of finished children). Zero
	// value for every gate that does not set it, so it is safely ignored by
	// the rest of this file's gates.
	Progress int `json:"progress,omitempty"`
}

// gateMaxBlocks bounds every content gate below at the same cap handleHermesStop already uses
// (BlockCount >= 3), so all gated agents behave identically once capped.
const gateMaxBlocks = 3

// gateStatePath resolves where a gate's block-count state persists: inside featureDir when the
// gate already resolved one (Hephaestus, Nemesis — this mirrors the b14cbed feature-scoping fix,
// so the state file can never leak across features either), otherwise cwd's .claude/tmp/, the
// same fallback Hermes's checklist uses.
func gateStatePath(cwd, featureDir, filename string) string {
	if featureDir != "" {
		return filepath.Join(featureDir, filename)
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return filepath.Join(cwd, ".claude", "tmp", filename)
}

// evaluateGateBlock applies the bounded block-count guard for a content gate that just found
// failures: it blocks (persisting an incremented counter) until gateMaxBlocks is reached, then
// allows the stop through with a debug log instead of blocking forever.
func evaluateGateBlock(gateName, statePath, agentID, reason string) error {
	state := readGateBlockState(statePath)
	if agentID != "" && state.AgentID != "" && state.AgentID != agentID {
		// A different spawn than the one that left this count — never carry it over.
		state = gateBlockState{}
	}
	if state.BlockCount >= gateMaxBlocks {
		debugLog("%s-stop: max block attempts reached (%d), allowing stop despite unmet gate", gateName, state.BlockCount)
		return outputSubagentOK()
	}
	state.AgentID = agentID
	state.BlockCount++
	writeGateBlockState(statePath, state)
	return outputSubagentBlock(fmt.Sprintf("%s (attempt %d/%d)", reason, state.BlockCount, gateMaxBlocks))
}

// clearGateBlock resets a gate's block-count state once its checks pass, so a later failure in
// a new stop sequence starts counting from 0 rather than from where an earlier, already-resolved
// sequence left off.
func clearGateBlock(statePath string) {
	_ = os.Remove(statePath)
}

// readGateBlockState reads a gate's persisted block-count state. Any error (missing file,
// unreadable, malformed) fails open to a fresh zero state — state management errors must never
// themselves cause an extra block.
func readGateBlockState(path string) gateBlockState {
	data, err := os.ReadFile(path)
	if err != nil {
		return gateBlockState{}
	}
	var s gateBlockState
	if err := json.Unmarshal(data, &s); err != nil {
		return gateBlockState{}
	}
	return s
}

// writeGateBlockState persists a gate's block-count state. Failures are logged and otherwise
// ignored — state management is best-effort and must never block on its own account.
func writeGateBlockState(path string, state gateBlockState) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		debugLog("gate block state: mkdir failed for %s: %v", path, err)
		return
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		debugLog("gate block state: marshal failed: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		debugLog("gate block state: write failed for %s: %v", path, err)
	}
}

// tierDisplayNames maps tier keys to human-readable names for error messages.
var tierDisplayNames = map[string]string{
	"T1_correct":      "T1 Correct",
	"T2_safe":         "T2 Safe",
	"T3_clear":        "T3 Clear",
	"T4_minimal":      "T4 Minimal",
	"T5_consistent":   "T5 Consistent",
	"T6_resilient":    "T6 Resilient",
	"T7_performant":   "T7 Performant",
	"T8_maintainable": "T8 Maintainable",
}

// tierOrder defines the canonical order for reporting incomplete tiers.
var tierOrder = []string{
	"T1_correct",
	"T2_safe",
	"T3_clear",
	"T4_minimal",
	"T5_consistent",
	"T6_resilient",
	"T7_performant",
	"T8_maintainable",
}

// checkHermesChecklist reads and validates the hermes-checklist.json at the given path.
// Returns (allComplete bool, incompleteTierNames []string).
// Returns (true, nil) when the checklist cannot be read or parsed (fail-open behavior).
func checkHermesChecklist(checklistPath string) (bool, []string) {
	data, err := os.ReadFile(checklistPath)
	if err != nil {
		debugLog("hermes-stop: failed to read checklist %s: %v", checklistPath, err)
		return true, nil
	}

	var checklist struct {
		AgentID string          `json:"agent_id"`
		Tiers   map[string]bool `json:"tiers"`
	}
	if err := json.Unmarshal(data, &checklist); err != nil {
		debugLog("hermes-stop: failed to parse checklist: %v", err)
		return true, nil
	}

	var incomplete []string
	for _, key := range tierOrder {
		if !checklist.Tiers[key] {
			if name, ok := tierDisplayNames[key]; ok {
				incomplete = append(incomplete, name)
			} else {
				incomplete = append(incomplete, key)
			}
		}
	}

	return len(incomplete) == 0, incomplete
}

// handleHermesStop finds and verifies the hermes-checklist.json.
// Fails open (allows stop) if the checklist cannot be found or parsed.
// Applies a max-block guard: after 3 blocked attempts, allows stop with a warning.
func handleHermesStop(input subagentStopInput) error {
	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	checklistPath := findHermesChecklist(cwd)
	if checklistPath == "" {
		debugLog("hermes-stop: checklist not found, failing open")
		return outputSubagentOK()
	}

	data, err := os.ReadFile(checklistPath)
	if err != nil {
		debugLog("hermes-stop: failed to read checklist %s: %v", checklistPath, err)
		return outputSubagentOK()
	}

	var checklist struct {
		AgentID    string          `json:"agent_id"`
		BlockCount int             `json:"block_count"`
		Tiers      map[string]bool `json:"tiers"`
	}
	if err := json.Unmarshal(data, &checklist); err != nil {
		debugLog("hermes-stop: failed to parse checklist: %v", err)
		return outputSubagentOK()
	}

	var incomplete []string
	for _, key := range tierOrder {
		if !checklist.Tiers[key] {
			if name, ok := tierDisplayNames[key]; ok {
				incomplete = append(incomplete, name)
			} else {
				incomplete = append(incomplete, key)
			}
		}
	}

	if len(incomplete) > 0 {
		tierList := strings.Join(incomplete, ", ")
		if checklist.BlockCount >= 3 {
			debugLog("hermes-stop: max block attempts reached (%d), allowing stop with incomplete tiers: %s", checklist.BlockCount, tierList)
			return outputSubagentOK()
		}

		checklist.BlockCount++
		updated, err := json.MarshalIndent(checklist, "", "  ")
		if err != nil {
			debugLog("hermes-stop: failed to marshal updated checklist: %v", err)
		} else if err := os.WriteFile(checklistPath, updated, 0644); err != nil {
			debugLog("hermes-stop: failed to write updated checklist: %v", err)
		}

		return outputSubagentBlock(fmt.Sprintf(
			"Hermes tier checklist incomplete. The following tiers were not reviewed: %s. Review each missing tier (or verify its child report), then run `'%s' hermes-list check <tier>` via the Bash tool for each. (attempt %d/3)",
			tierList,
			kratosBinPath(),
			checklist.BlockCount,
		))
	}

	debugLog("hermes-stop: all 8 tiers complete, allowing stop")
	return outputSubagentOK()
}

// kratosBinPath returns this binary's absolute path for embedding in agent-facing
// instructions. A bare `kratos` is not guaranteed on PATH (the plugin resolves the
// binary via ${CLAUDE_PLUGIN_ROOT}/bin or ~/.kratos/bin), so injected commands must
// carry the full path or the agent's calls silently fail.
func kratosBinPath() string {
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return "kratos"
}

// findHermesChecklist resolves the checklist file for the Hermes agent that is
// stopping now, mirroring handleHermesStart's own resolution exactly:
// findActiveFeatureDir's feature dir when a 9-review is pending/in-progress/
// ready, otherwise .claude/tmp/hermes-checklist.json.
//
// This must mirror the Start-side choice rather than glob every feature dir
// and return "whichever hermes-checklist.json exists" (the old behavior): a
// stale checklist left in an unrelated or already-reviewed feature — however
// recently its mtime happens to be — must never satisfy the review Hermes is
// running right now just because it is the only (or newest) file on disk.
func findHermesChecklist(cwd string) string {
	if dir, err := findActiveFeatureDir(cwd); err != nil {
		debugLog("findHermesChecklist: findActiveFeatureDir error: %v", err)
	} else if dir != "" {
		path := filepath.Join(dir, "hermes-checklist.json")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		debugLog("findHermesChecklist: active feature %s has no checklist yet, falling back to tmp", dir)
	}

	// Fall back to .claude/tmp/ — same fallback handleHermesStart uses when
	// no feature has an active 9-review stage.
	fallback := filepath.Join(cwd, ".claude", "tmp", "hermes-checklist.json")
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}

	return ""
}
