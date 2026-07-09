// Package cli implements all kratos CLI subcommands.
package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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

// Per-agent protocol slices are embedded from go/internal/cli/agent-protocol-slices/.
// Source lives in plugins/kratos/references/agent-protocol-slices/ — keep in sync.
// Each slice contains only the agent-protocol.md sections relevant to that agent in
// command mode, pre-embedded so agents don't need to read agent-protocol.md at runtime.
//
//go:embed agent-protocol-slices/*.md
var protocolSlicesFS embed.FS

// AgentCmd returns the 'agent' command with a load subcommand.
func AgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Load Kratos agent definitions",
	}
	cmd.AddCommand(agentLoadCmd())
	return cmd
}

func agentLoadCmd() *cobra.Command {
	var mode string
	var resolve bool
	var rootFlag string

	cmd := &cobra.Command{
		Use:          "load <name>",
		Short:        "Print an agent definition to stdout, optionally with a command-mode suffix",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !strings.HasSuffix(name, ".md") {
				name += ".md"
			}

			body, err := agentsFS.ReadFile("agents/" + name)
			if err != nil {
				return fmt.Errorf("agent %q not found", args[0])
			}

			out := string(body)

			if mode == "command" {
				// Inject per-agent protocol slice between body and suffix.
				// Agents opt in by adding a file to agent-protocol-slices/.
				slice, err := protocolSlicesFS.ReadFile("agent-protocol-slices/" + name)
				if err == nil {
					out += "\n---\n\n" + string(slice)
				}

				suffix, err := commandSuffixFS.ReadFile("command-mode-suffix/" + name)
				if err == nil {
					out += "\n---\n\n" + string(suffix)
				}
			}

			if resolve {
				out = resolveTokens(out, rootFlag)
			}

			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "", "Execution mode: 'command' appends command-mode suffix if one exists")
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
