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
	regexp.MustCompile(`(?:^|\s)[/\\]\S+`),                           // file paths
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
	AgentType            string `json:"agent_type"`
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

// subagentStopOutput is returned to allow or block subagent completion
type subagentStopOutput struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

// preToolUseInput is the JSON Claude Code sends for PreToolUse
type preToolUseInput struct {
	ToolName  string              `json:"tool_name"`
	ToolInput preToolUseToolInput `json:"tool_input"`
}

type preToolUseToolInput struct {
	Command string `json:"command"`
}

// preToolUseOutput is the hookSpecificOutput response for PreToolUse
type preToolUseOutput struct {
	HookSpecificOutput preToolUseHookSpecific `json:"hookSpecificOutput"`
}

type preToolUseHookSpecific struct {
	HookEventName      string            `json:"hookEventName"`
	PermissionDecision string            `json:"permissionDecision"`
	UpdatedInput       map[string]string `json:"updatedInput,omitempty"`
	AdditionalContext  string            `json:"additionalContext,omitempty"`
}

// npmWordBoundary matches the word "npm" with word boundaries
var npmWordBoundary = regexp.MustCompile(`\bnpm\b`)

// HookCmd returns the 'hook' command group
func HookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Hook handlers for Claude Code events",
	}

	cmd.AddCommand(promptSubmitCmd())
	cmd.AddCommand(subagentStartCmd())
	cmd.AddCommand(subagentStopCmd())
	cmd.AddCommand(fixPMCmd())
	cmd.AddCommand(specDeltaCheckCmd())
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

func specDeltaCheckIn(root string, raw []byte, stdout io.Writer) error {
	var input postToolUseInput
	if err := json.Unmarshal(raw, &input); err != nil {
		debugLog("spec-delta-check: json parse error: %v", err)
		return nil
	}

	m := specDeltaPathRE.FindStringSubmatch(input.ToolInput.FilePath)
	if m == nil {
		return nil
	}
	feature := m[1]

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

	// Sanitize: strip code blocks, URLs, paths, system reminders
	cleaned := sanitizePrompt(prompt)

	// Match keywords (case-insensitive, word-boundary)
	matched := matchKeywords(cleaned)

	var keywordContext string
	if len(matched) > 0 {
		debugLog("matched keywords: %v", matched)
		keywordContext = buildInjectionContext(matched)
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
╔══════════════════════════════════════════════════════════════╗
║  KRATOS QUALITY GATE — MANDATORY BEFORE ANY TOOL CALL        ║
╠══════════════════════════════════════════════════════════════╣
║  1. Write your complete numbered TODO list FIRST             ║
║     Format:                                                  ║
║       TODO:                                                  ║
║       1. [ ] Task description                                ║
║       2. [ ] Task description                                ║
║       ...                                                    ║
║  2. Work through each item in order                          ║
║  3. Mark each item [x] as you complete it                    ║
║  4. Do NOT call any tool before your TODO list is written    ║
╚══════════════════════════════════════════════════════════════╝

Output terse: drop articles/filler/pleasantries. Pattern: [status][what][result][next]. Fragments OK. Technical terms exact.
`

// aresTaskGate is injected for Ares specifically. Ares has the Task* tools, so its
// planning step is the TaskCreate tool rather than a text TODO list. The closing
// "Task list:" recap is what the SubagentStop gate matches on (it can only see the
// final message text, not tool calls), so the recap keeps the gate meaningful.
const aresTaskGate = `
╔══════════════════════════════════════════════════════════════╗
║  KRATOS QUALITY GATE — CREATE YOUR TASK LIST FIRST          ║
╠══════════════════════════════════════════════════════════════╣
║  1. Call TaskCreate once per job BEFORE any other tool       ║
║     — one task per file/module, not one vague "implement"    ║
║     (small missions ≤2 files: one umbrella task is enough)   ║
║  2. TaskUpdate a task in_progress when you start it          ║
║  3. TaskUpdate completed ONLY when truly done (tests green)  ║
║  4. TaskCreate any new work that surfaces mid-mission        ║
║  5. End with a "Task list:" recap of every task + status     ║
╚══════════════════════════════════════════════════════════════╝

Output terse: drop articles/filler/pleasantries. Pattern: [status][what][result][next]. Fragments OK. Technical terms exact.
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

			// Ares plans via the TaskCreate tool, not a text TODO list.
			if strings.Contains(agentType, "ares") {
				return outputSubagentStartContext(aresTaskGate + reminder)
			}

			// Hephaestus writes the spec and is the other agent prone to re-globbing.
			if strings.Contains(agentType, "hephaestus") {
				return outputSubagentStartContext(todoQualityGate + reminder)
			}

			// For all other agents (athena, daedalus, etc.) — inject text TODO quality gate
			return outputSubagentStartContext(todoQualityGate)
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

	checklist := map[string]interface{}{
		"agent_id": input.AgentID,
		"tiers": map[string]bool{
			"T1_correct":      false,
			"T2_safe":         false,
			"T3_clear":        false,
			"T4_minimal":      false,
			"T5_consistent":   false,
			"T6_resilient":    false,
			"T7_performant":   false,
			"T8_maintainable": false,
		},
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
			"A hook verifies all 8 tiers on stop — incomplete tiers block completion.\n\n"+
			"Output terse: drop articles/filler/pleasantries. Pattern: [status][what][result][next]. Fragments OK. Technical terms exact.",
		checklistPath, kratosBinPath(),
	)

	return outputSubagentStartContext(additionalContext)
}

// findActiveFeatureDir scans .claude/feature/*/status.json and returns the feature folder
// for the first feature where stage 8-review has status pending, in-progress, or ready.
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

// gatedAgentInRaw reports which gated agent (if any) a raw, unparseable payload appears
// to concern. It scans the whole payload, so a message body that merely mentions a gated
// agent name also trips it — that is intentional: on a malformed payload we prefer to
// fail closed rather than risk letting a gated agent slip through.
func gatedAgentInRaw(raw []byte) string {
	s := strings.ToLower(string(raw))
	for _, a := range gatedAgents {
		if strings.Contains(s, a) {
			return a
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
// Returns {"ok": true} to allow completion or {"ok": false, "reason": "..."} to block.
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

			// Prevent infinite loops
			if input.StopHookActive {
				return outputSubagentOK()
			}

			agentType := strings.ToLower(input.AgentType)
			msg := input.LastAssistantMessage
			msgLower := strings.ToLower(msg)

			// Ares (implementation agent) quality checks
			if strings.Contains(agentType, "ares") {
				var failures []string

				// Ares plans via TaskCreate; the SubagentStop hook can only see the
				// final message, not tool calls, so it matches the "Task list:" recap
				// Ares is instructed to print at the end (text TODO still accepted).
				hasTaskList := strings.Contains(msgLower, "task list:") ||
					strings.Contains(msgLower, "todo:") ||
					regexp.MustCompile(`(?i)##\s*(tasks|todo|plan)`).MatchString(msg)
				if !hasTaskList {
					failures = append(failures, "no task list (TaskCreate recap) was written before starting work")
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

				if f := aresVerifyGateFailure(input); f != "" {
					failures = append(failures, f)
				}

				if len(failures) > 0 {
					return outputSubagentBlock(fmt.Sprintf(
						"Ares quality gate failed: %s. Create tasks via TaskCreate, implement all items, and end with a 'Task list:' recap naming the files you created or modified.",
						strings.Join(failures, "; "),
					))
				}
			}

			// Hephaestus (tech spec agent) quality checks
			if strings.Contains(agentType, "hephaestus") {
				specSections := []string{"architecture", "data model", "api", "implementation", "schema", "interface"}
				var found []string
				for _, s := range specSections {
					if strings.Contains(msgLower, s) {
						found = append(found, s)
					}
				}
				if len(found) < 2 {
					return outputSubagentBlock(fmt.Sprintf(
						"Hephaestus quality gate failed: technical spec appears incomplete (only found sections: %s). A complete spec must cover architecture, data models, API design, and implementation details.",
						func() string {
							if len(found) == 0 {
								return "none"
							}
							return strings.Join(found, ", ")
						}(),
					))
				}

				// Disk check: verify tech-spec-proposal.md or tech-spec.md was written to a feature dir.
				// Only enforce when a pipeline feature dir exists (allows fail-open in pure command mode).
				cwd := input.Cwd
				if cwd == "" {
					cwd, _ = os.Getwd()
				}
				dirs, _ := filepath.Glob(filepath.Join(cwd, ".claude", "feature", "*"))
				if len(dirs) > 0 {
					specFound := false
					for _, dir := range dirs {
						if discoverFileExists(filepath.Join(dir, "tech-spec-proposal.md")) ||
							discoverFileExists(filepath.Join(dir, "tech-spec.md")) {
							specFound = true
							break
						}
					}
					if !specFound {
						return outputSubagentBlock(
							"Hephaestus quality gate failed: neither tech-spec-proposal.md nor tech-spec.md was found in any feature directory. Write the output to .claude/feature/<name>/ before completing.",
						)
					}
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
// Fails open when no feature directory exists (allows completion in quick/command mode).
func handleNemesisStop(input subagentStopInput) error {
	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	dirs, _ := filepath.Glob(filepath.Join(cwd, ".claude", "feature", "*"))
	if len(dirs) == 0 {
		debugLog("nemesis-stop: no feature dirs found, failing open")
		return outputSubagentOK()
	}

	var challengePath string
	for _, dir := range dirs {
		p := filepath.Join(dir, "prd-challenge.md")
		if discoverFileExists(p) {
			challengePath = p
			break
		}
	}

	if challengePath == "" {
		return outputSubagentBlock(
			"Nemesis quality gate failed: prd-challenge.md not found in any feature directory. Write your PRD challenge to .claude/feature/<name>/prd-challenge.md before completing.",
		)
	}

	content, err := os.ReadFile(challengePath)
	if err != nil || len(strings.TrimSpace(string(content))) == 0 {
		return outputSubagentBlock(
			"Nemesis quality gate failed: prd-challenge.md exists but is empty. Add at least one challenge section before completing.",
		)
	}

	if !challengeHeadingRe.Match(content) {
		return outputSubagentBlock(
			"Nemesis quality gate failed: prd-challenge.md contains no challenge sections (expected at least one heading containing 'challenge'). Structure your output with explicit challenge headings.",
		)
	}

	debugLog("nemesis-stop: prd-challenge.md valid, allowing stop")
	return outputSubagentOK()
}

func outputSubagentOK() error {
	data, _ := json.Marshal(subagentStopOutput{OK: true})
	fmt.Println(string(data))
	return nil
}

func outputSubagentBlock(reason string) error {
	data, _ := json.Marshal(subagentStopOutput{OK: false, Reason: reason})
	fmt.Println(string(data))
	return nil
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

// findHermesChecklist scans .claude/feature/*/hermes-checklist.json and returns
// the most recently modified one. Falls back to .claude/tmp/hermes-checklist.json.
func findHermesChecklist(cwd string) string {
	pattern := filepath.Join(cwd, ".claude", "feature", "*", "hermes-checklist.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		debugLog("findHermesChecklist: glob error: %v", err)
	}

	if len(matches) == 1 {
		return matches[0]
	}

	if len(matches) > 1 {
		// Return the most recently modified file
		best := matches[0]
		bestInfo, err := os.Stat(best)
		if err != nil {
			return best
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
		return best
	}

	// Fall back to .claude/tmp/
	fallback := filepath.Join(cwd, ".claude", "tmp", "hermes-checklist.json")
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}

	return ""
}

// fixPMCmd intercepts Bash commands using npm and rewrites them to the correct
// package manager detected from lockfiles in the project root.
func fixPMCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fix-pm",
		Short: "Handle PreToolUse Bash hook — auto-correct npm to the project's package manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil
			}

			var input preToolUseInput
			if err := json.Unmarshal(raw, &input); err != nil {
				return nil
			}

			command := input.ToolInput.Command

			// Only act if npm is used
			if !npmWordBoundary.MatchString(command) {
				return nil
			}

			// Detect package manager from lockfiles
			cwd := os.Getenv("CLAUDE_PROJECT_DIR")
			if cwd == "" {
				cwd, _ = os.Getwd()
			}

			pm, lockfile := detectPackageManager(cwd)
			if pm == "" {
				return nil // no alternative PM found, let npm through
			}

			fixed := npmWordBoundary.ReplaceAllString(command, pm)

			output := preToolUseOutput{
				HookSpecificOutput: preToolUseHookSpecific{
					HookEventName:      "PreToolUse",
					PermissionDecision: "allow",
					UpdatedInput:       map[string]string{"command": fixed},
					AdditionalContext:  fmt.Sprintf("[Kratos] Auto-corrected: npm → %s (detected %s in project root). Use %s for all package operations in this project.", pm, lockfile, pm),
				},
			}

			data, err := json.Marshal(output)
			if err != nil {
				return nil
			}
			fmt.Println(string(data))
			return nil
		},
	}
}

// detectPackageManager checks lockfiles in cwd to determine the package manager.
// Priority: bun.lockb > yarn.lock > pnpm-lock.yaml
func detectPackageManager(cwd string) (pm string, lockfile string) {
	checks := []struct {
		file string
		pm   string
	}{
		{"bun.lockb", "bun"},
		{"yarn.lock", "yarn"},
		{"pnpm-lock.yaml", "pnpm"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(cwd, c.file)); err == nil {
			return c.pm, c.file
		}
	}
	return "", ""
}
