package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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

// HookCmd returns the 'hook' command group
func HookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Hook handlers for Claude Code events",
	}

	cmd.AddCommand(promptSubmitCmd())
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
