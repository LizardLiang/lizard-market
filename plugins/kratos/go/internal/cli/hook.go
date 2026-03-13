package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

// Patterns to strip before keyword matching (prevent false positives)
var stripPatterns = []*regexp.Regexp{
	regexp.MustCompile("(?s)```.*?```"),                // fenced code blocks
	regexp.MustCompile("`[^`]+`"),                      // inline code
	regexp.MustCompile(`<[^>]+>[^<]*</[^>]+>`),         // XML tags with content
	regexp.MustCompile(`https?://\S+`),                 // URLs
	regexp.MustCompile(`(?:^|\s)[/\\]\S+`),             // file paths
	regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`), // system reminders
}

// subagentStopInput is the JSON Claude Code sends for SubagentStop
type subagentStopInput struct {
	AgentType            string `json:"agent_type"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

// subagentStopOutput is returned to allow or block subagent completion
type subagentStopOutput struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

// preToolUseInput is the JSON Claude Code sends for PreToolUse
type preToolUseInput struct {
	ToolName  string             `json:"tool_name"`
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
	return cmd
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

	var input hookInput
	if err := json.Unmarshal(raw, &input); err != nil {
		debugLog("json parse error: %v", err)
		return outputPassthrough()
	}

	prompt := input.Prompt
	if prompt == "" {
		return outputPassthrough()
	}

	// Sanitize: strip code blocks, URLs, paths, system reminders
	cleaned := sanitizePrompt(prompt)

	// Match keywords (case-insensitive, word-boundary)
	matched := matchKeywords(cleaned)

	if len(matched) == 0 {
		return outputPassthrough()
	}

	debugLog("matched keywords: %v", matched)

	// Build injection context
	context := buildInjectionContext(matched)

	output := hookOutput{
		Continue: true,
		HookSpecificOutput: &hookSpecificOutput{
			HookEventName:     "UserPromptSubmit",
			AdditionalContext: context,
		},
	}

	return outputJSON(output)
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

func outputPassthrough() error {
	output := hookOutput{
		Continue: true,
	}
	return outputJSON(output)
}

func outputJSON(output hookOutput) error {
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// subagentStartCmd injects a mandatory TODO-first instruction into Ares and Hephaestus agents.
// Stdout text is automatically added to the subagent's starting context by Claude Code.
func subagentStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "subagent-start",
		Short: "Handle SubagentStart hook — inject TODO-first quality gate",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(`
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
`)
			return nil
		},
	}
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
				return outputSubagentOK()
			}

			var input subagentStopInput
			if err := json.Unmarshal(raw, &input); err != nil {
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

				hasTodoList := strings.Contains(msgLower, "todo:") ||
					strings.Contains(msgLower, "task list:") ||
					regexp.MustCompile(`(?i)##\s*(tasks|todo|plan)`).MatchString(msg)
				if !hasTodoList {
					failures = append(failures, "no TODO list was written before starting work")
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

				if len(failures) > 0 {
					return outputSubagentBlock(fmt.Sprintf(
						"Ares quality gate failed: %s. Write a TODO list, implement all items, and confirm which files were created.",
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
			}

			return outputSubagentOK()
		},
	}
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
