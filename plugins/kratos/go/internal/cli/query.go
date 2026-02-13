package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/db"
)

// QueryCmd returns the 'query' command for querying Kratos data
func QueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query Kratos memory database",
		Long: `Query sessions, steps, and other data from the Kratos memory system.

Supports filtering by status, project, and full-text search.`,
	}

	// Add subcommands
	cmd.AddCommand(QuerySessionsCmd())
	cmd.AddCommand(QueryStepsCmd())
	cmd.AddCommand(QuerySearchCmd())
	cmd.AddCommand(QueryCountCmd())

	return cmd
}

// QuerySessionsCmd returns the 'query sessions' command
func QuerySessionsCmd() *cobra.Command {
	var limit int
	var status string
	var project string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Query sessions",
		Long: `Query sessions with optional filters.

Can filter by status, project, or get recent sessions with a limit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			var sessions interface{}

			// Determine which query to use based on flags
			if status != "" {
				sessions, err = db.GetSessionsByStatus(conn, status)
			} else if project != "" {
				sessions, err = db.GetSessionsByProject(conn, project)
			} else {
				sessions, err = db.GetRecentSessions(conn, limit)
			}

			if err != nil {
				return fmt.Errorf("failed to query sessions: %w", err)
			}

			result := map[string]interface{}{
				"sessions": sessions,
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Number of recent sessions to return")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (active, completed, abandoned)")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project path")

	return cmd
}

// QueryStepsCmd returns the 'query steps' command
func QueryStepsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "steps <session_id>",
		Short: "Query steps for a session",
		Long: `Get the timeline of all steps for a specific session.

Returns steps in chronological order.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			steps, err := db.GetSessionTimeline(conn, sessionID)
			if err != nil {
				return fmt.Errorf("failed to query steps: %w", err)
			}

			result := map[string]interface{}{
				"session_id": sessionID,
				"steps":      steps,
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}

// QuerySearchCmd returns the 'query search' command
func QuerySearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <term>",
		Short: "Full-text search across sessions",
		Long: `Search sessions by project, feature name, or summary.

Uses full-text search to find matching sessions.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			searchTerm := args[0]

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			sessions, err := db.SearchSessions(conn, searchTerm)
			if err != nil {
				return fmt.Errorf("failed to search sessions: %w", err)
			}

			result := map[string]interface{}{
				"query":    searchTerm,
				"sessions": sessions,
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}

// QueryCountCmd returns the 'query count' command
func QueryCountCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "count",
		Short: "Get total session count",
		Long:  `Returns the total number of sessions in the database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			count, err := db.GetSessionCount(conn)
			if err != nil {
				return fmt.Errorf("failed to get session count: %w", err)
			}

			result := map[string]interface{}{
				"count": count,
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}
