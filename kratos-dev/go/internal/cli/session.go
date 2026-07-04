package cli

import (
	"github.com/spf13/cobra"
)

// SessionCmd returns the 'session' command for managing Kratos sessions
func SessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage Kratos sessions",
		Long: `Manage Kratos development sessions.

Sessions track your work journeys through features, tracking steps,
agent spawns, decisions, and file changes.`,
	}

	// Add subcommands
	cmd.AddCommand(SessionStartCmd())
	cmd.AddCommand(SessionActiveCmd())
	cmd.AddCommand(SessionEndCmd())

	return cmd
}
