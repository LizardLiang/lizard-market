// Package cli implements all kratos CLI subcommands.
package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/gencmd"
	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/protocol"
)

// Agents are embedded at build time from go/internal/cli/agents/.
// This directory is a maintained copy of plugins/kratos/agents/ — keep in sync when editing agents.
//
//go:embed agents/*.md
var agentsFS embed.FS

// Command-mode suffix files are embedded from go/internal/cli/command-mode-suffix/.
// This directory is a maintained copy of plugins/kratos/command-mode-suffix/ — keep in sync.
//
//go:embed command-mode-suffix/*.md
var commandSuffixFS embed.FS

// The shared agent protocol is embedded from go/internal/cli/references/.
// This is a maintained copy of plugins/kratos/references/agent-protocol.md
// (synced by make sync-assets). Per-agent sections are composed from it at
// runtime so agents never need to read agent-protocol.md themselves.
//
//go:embed references/agent-protocol.md
var protocolFS embed.FS

// AgentCmd returns the 'agent' command with load and protocol subcommands.
func AgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Load Kratos agent definitions",
	}
	cmd.AddCommand(agentLoadCmd())
	cmd.AddCommand(agentProtocolCmd())
	return cmd
}

// composeProtocolFor returns the composed protocol block for an agent (its
// protocol_sections frontmatter list applied to the embedded
// agent-protocol.md), "" if the agent lists no sections, or an error for an
// unknown agent or invalid slug list (a build bug — content is embedded).
func composeProtocolFor(name string) (string, error) {
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	body, err := agentsFS.ReadFile("agents/" + name)
	if err != nil {
		return "", fmt.Errorf("agent %q not found", strings.TrimSuffix(name, ".md"))
	}
	fm, err := gencmd.ParseFrontmatter(name, string(body))
	if err != nil {
		return "", err
	}
	slugs := protocol.ParseList(fm.Fields["protocol_sections"])
	if len(slugs) == 0 {
		return "", nil
	}
	raw, err := protocolFS.ReadFile("references/agent-protocol.md")
	if err != nil {
		return "", fmt.Errorf("embedded agent-protocol.md missing: %w", err)
	}
	doc, err := protocol.Parse(string(raw))
	if err != nil {
		return "", err
	}
	return protocol.Compose(doc, slugs)
}

// Slices of an agent definition that `agent load --part` prints. Claude Code
// inlines at most 30,000 characters of a !`cmd` line in a slash command (the
// Bash tool's limit); a whole god — body + protocol + lessons — is 31–34 KB,
// so one loader line reached the model as a 2 KB <persisted-output> preview
// in 10 of 11 Iris launches (2026-09 review). Launchers now run two lines,
// one per part; agent_inline_budget_test.go keeps each under the limit.
const (
	loadPartAll    = ""       // legacy: body + extras in one stream
	loadPartBody   = "body"   // agents/<god>.md verbatim
	loadPartExtras = "extras" // lessons, protocol block, command-mode suffix
)

// partSeparator joins the pieces inside Extras. Body and Extras are joined
// by "\n---\n\n" in the legacy full output (bodies end with a newline).
const partSeparator = "\n\n---\n\n"

// agentLoadParts is an agent definition split for inline injection.
type agentLoadParts struct {
	Body   string // agents/<god>.md, unmodified
	Extras string // lessons FIRST, then protocol, then suffix; "" when all empty
}

// composeAgentLoad builds both parts for name ("iris" or "iris.md"). lessons
// is the already-rendered lessons block ("" for none) so callers without a
// store — tests and the inline-budget lint — can pass a synthetic one. Extras
// puts the lessons first so a partial read still sees the user's corrections
// (they sat unread at the tail of a persisted file all of 2026-09).
func composeAgentLoad(name, mode, lessons string) (agentLoadParts, error) {
	file := name
	if !strings.HasSuffix(file, ".md") {
		file += ".md"
	}
	body, err := agentsFS.ReadFile("agents/" + file)
	if err != nil {
		return agentLoadParts{}, fmt.Errorf("agent %q not found", strings.TrimSuffix(name, ".md"))
	}

	// Inject the composed protocol block for every load (command mode or
	// not) — agents opt in via protocol_sections frontmatter.
	block, err := composeProtocolFor(file)
	if err != nil {
		return agentLoadParts{}, err
	}

	var pieces []string
	for _, p := range []string{lessons, block} {
		if p = strings.TrimRight(p, "\n"); p != "" {
			pieces = append(pieces, p)
		}
	}
	if mode == "command" {
		if suffix, err := commandSuffixFS.ReadFile("command-mode-suffix/" + file); err == nil {
			if s := strings.TrimRight(string(suffix), "\n"); s != "" {
				pieces = append(pieces, s)
			}
		}
	}

	parts := agentLoadParts{Body: string(body)}
	if len(pieces) > 0 {
		parts.Extras = strings.Join(pieces, partSeparator) + "\n"
	}
	return parts, nil
}

// joinAgentLoad renders the legacy single-stream output (no --part).
func joinAgentLoad(p agentLoadParts) string {
	if p.Extras == "" {
		return p.Body
	}
	return p.Body + "\n---\n\n" + p.Extras
}

func agentLoadCmd() *cobra.Command {
	var mode string
	var part string
	var resolve bool
	var rootFlag string

	cmd := &cobra.Command{
		Use:          "load <name>",
		Short:        "Print an agent definition to stdout, optionally with a command-mode suffix",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch part {
			case loadPartAll, loadPartBody, loadPartExtras:
			default:
				return fmt.Errorf("invalid --part %q (want body or extras)", part)
			}
			god := strings.TrimSuffix(args[0], ".md")

			// Stored lessons from past user corrections — inline gods have no
			// SubagentStart hook to inject them, so they ride along here. The
			// body part never opens the store.
			lessons := ""
			if part != loadPartBody {
				lessons = lessonsBlockFor(god)
			}

			parts, err := composeAgentLoad(god, mode, lessons)
			if err != nil {
				return err
			}

			var out string
			switch part {
			case loadPartBody:
				out = parts.Body
			case loadPartExtras:
				out = parts.Extras
			default:
				out = joinAgentLoad(parts)
			}

			if resolve {
				out = resolveTokens(out, rootFlag)
			}

			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "", "Execution mode: 'command' appends command-mode suffix if one exists")
	cmd.Flags().StringVar(&part, "part", "", "Print one slice only: 'body' (agents/<god>.md) or 'extras' (lessons, protocol, command-mode suffix). Default prints both. Each slice stays under Claude Code's 30,000-char inline limit for !`cmd` lines")
	cmd.Flags().BoolVar(&resolve, "resolve", false, "Substitute <KRATOS_ROOT> and <kratos-bin> tokens with discovered absolute paths")
	cmd.Flags().StringVar(&rootFlag, "root", "", "Explicit plugin root for <KRATOS_ROOT> substitution (overrides discovery); only used with --resolve")
	return cmd
}

// agentProtocolCmd prints just the composed protocol block for an agent —
// consumed by hooks/path-inject.cjs to inject it into spawned subagents at
// SubagentStart. Empty protocol_sections prints nothing (exit 0).
func agentProtocolCmd() *cobra.Command {
	var resolve bool
	var rootFlag string

	cmd := &cobra.Command{
		Use:          "protocol <name>",
		Short:        "Print an agent's composed protocol sections to stdout",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			block, err := composeProtocolFor(args[0])
			if err != nil {
				return err
			}
			if block == "" {
				return nil
			}
			if resolve {
				block = resolveTokens(block, rootFlag)
			}
			fmt.Fprintln(cmd.OutOrStdout(), block)
			return nil
		},
	}

	cmd.Flags().BoolVar(&resolve, "resolve", false, "Substitute <KRATOS_ROOT> and <kratos-bin> tokens with discovered absolute paths")
	cmd.Flags().StringVar(&rootFlag, "root", "", "Explicit plugin root for <KRATOS_ROOT> substitution (overrides discovery); only used with --resolve")
	return cmd
}

// resolveTokens replaces every <KRATOS_ROOT> and <kratos-bin> token in body
// with discovered absolute paths, normalized to forward slashes. <KRATOS_ROOT>
// is left unmodified if no root can be discovered (never guess); <kratos-bin>
// is left unmodified if os.Executable() fails.
func resolveTokens(body, rootFlag string) string {
	if root := discoverRoot(rootFlag); root != "" {
		body = strings.ReplaceAll(body, "<KRATOS_ROOT>", root)
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		body = strings.ReplaceAll(body, "<kratos-bin>", toSlash(exe))
	}
	return body
}

// toSlash normalizes path separators to forward slashes on every host OS.
// filepath.ToSlash is a no-op on Linux, but Windows-style paths can arrive
// via --root or CLAUDE_PLUGIN_ROOT regardless of where the binary runs.
func toSlash(p string) string {
	return strings.ReplaceAll(p, `\`, "/")
}

// discoverRoot resolves the plugin root directory for <KRATOS_ROOT>
// substitution, or "" if undiscoverable. Precedence: explicit rootFlag >
// CLAUDE_PLUGIN_ROOT env > the executable's grandparent directory (only if
// that directory contains an agents/ subdirectory, guarding against
// misidentifying an unrelated install layout).
func discoverRoot(rootFlag string) string {
	if rootFlag != "" {
		return toSlash(rootFlag)
	}
	if env := os.Getenv("CLAUDE_PLUGIN_ROOT"); env != "" {
		return toSlash(env)
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		candidate := filepath.Dir(filepath.Dir(exe))
		if info, statErr := os.Stat(filepath.Join(candidate, "agents")); statErr == nil && info.IsDir() {
			return toSlash(candidate)
		}
	}
	return ""
}
