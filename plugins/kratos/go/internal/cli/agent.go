package cli

import (
	"embed"
	"fmt"
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

			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "", "Execution mode: 'command' appends command-mode suffix if one exists")
	return cmd
}
