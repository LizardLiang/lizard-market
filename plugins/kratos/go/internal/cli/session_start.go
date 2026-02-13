package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/db"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// SessionStartCmd returns the 'session start' command
func SessionStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <project> [feature]",
		Short: "Start a new Kratos session",
		Long: `Start a new Kratos development session for a project.

Optionally specify a feature name to track feature-specific work.
Only one active session per project is allowed.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			var featureName *string
			if len(args) == 2 {
				featureName = &args[1]
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			// Check for existing active session
			existing, err := db.GetActiveSession(conn, project)
			if err != nil {
				return fmt.Errorf("failed to check active session: %w", err)
			}
			if existing != nil {
				return fmt.Errorf("active session already exists: %s", existing.SessionID)
			}

			// Create new session
			session := &models.Session{
				SessionID:          uuid.New().String(),
				Project:            project,
				FeatureName:        featureName,
				StartedAt:          time.Now().UnixMilli(),
				Status:             "active",
				TotalSteps:         0,
				TotalAgentsSpawned: 0,
			}

			if err := db.CreateSession(conn, session); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(session)
		},
	}
}

// SessionActiveCmd returns the 'session active' command
func SessionActiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "active <project>",
		Short: "Show active session for project",
		Long: `Display the currently active session for a project.

Returns the session details if one is active, otherwise returns null.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			session, err := db.GetActiveSession(conn, project)
			if err != nil {
				return fmt.Errorf("failed to get active session: %w", err)
			}

			result := map[string]interface{}{}
			if session == nil {
				result["message"] = "no active session"
				result["session"] = nil
			} else {
				result["session"] = session
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}

// SessionEndCmd returns the 'session end' command
func SessionEndCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "end <session_id> [summary]",
		Short: "End a session",
		Long: `End a Kratos session and mark it as completed.

Optionally provide a summary of the work accomplished.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			summary := ""
			if len(args) == 2 {
				summary = args[1]
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.EndSession(conn, sessionID, summary); err != nil {
				return fmt.Errorf("failed to end session: %w", err)
			}

			// Return updated session
			session, err := db.GetSession(conn, sessionID)
			if err != nil {
				return fmt.Errorf("failed to get session: %w", err)
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(session)
		},
	}
}
