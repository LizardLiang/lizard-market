package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/db"
)

// RecallCmd returns the 'recall' command for recalling past sessions
func RecallCmd() *cobra.Command {
	var global bool
	var incomplete bool
	var limit int

	cmd := &cobra.Command{
		Use:   "recall [project]",
		Short: "Recall previous Kratos sessions",
		Long: `Recall information about previous Kratos sessions.

Examples:
  kratos recall /path/to/project          # Get last session for project
  kratos recall --global                  # Get recent sessions across all projects
  kratos recall /path/to/project --incomplete  # Get incomplete features`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			result := make(map[string]interface{})

			// Global recall - recent sessions across all projects
			if global {
				sessions, err := db.GetRecentSessionsGlobal(conn, limit)
				if err != nil {
					return fmt.Errorf("failed to get recent sessions: %w", err)
				}
				result["recent_sessions"] = sessions
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			// Project-specific recall
			if len(args) == 0 {
				return fmt.Errorf("project path required when --global not specified")
			}
			project := args[0]

			// Get incomplete features
			if incomplete {
				incompleteFeatures, err := db.GetIncompleteFeatures(conn, project)
				if err != nil {
					return fmt.Errorf("failed to get incomplete features: %w", err)
				}
				result["incomplete_features"] = incompleteFeatures
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			// Get last session
			lastSession, err := db.GetLastSessionForProject(conn, project)
			if err != nil {
				return fmt.Errorf("failed to get last session: %w", err)
			}
			result["last_session"] = lastSession

			// If there's a last session, also check for incomplete features
			if lastSession != nil {
				incompleteFeatures, err := db.GetIncompleteFeatures(conn, project)
				if err == nil && len(incompleteFeatures) > 0 {
					result["incomplete_features"] = incompleteFeatures
				}
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Show recent sessions across all projects")
	cmd.Flags().BoolVar(&incomplete, "incomplete", false, "Show only incomplete features")
	cmd.Flags().IntVar(&limit, "limit", 5, "Number of recent sessions to show (for --global)")

	return cmd
}
