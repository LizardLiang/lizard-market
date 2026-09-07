package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/models"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// normalizeProjectPath canonicalizes a project path so that storage and lookups
// always match regardless of trailing slashes, slash direction, or Windows path-case.
func normalizeProjectPath(path string) string {
	path = filepath.ToSlash(path)       // C:\foo\bar → C:/foo/bar (no-op on Unix)
	path = strings.TrimRight(path, "/") // strip trailing slashes
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path) // case-insensitive filesystem
	}
	return path
}

// SessionStartCmd returns the 'session start' command
func SessionStartCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "start <project> [feature]",
		Short: "Start a new Kratos session",
		Long: `Start a new Kratos development session for a project.

Optionally specify a feature name to track feature-specific work.

With --session-id (the Claude Code session id every hook receives) the command
is idempotent: an existing row is returned, re-activated if it had ended, so one
Claude Code session maps to exactly one Kratos session and concurrent sessions
in different windows never share or clobber state. Without the flag only one
active session per project is allowed (legacy behaviour).`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := normalizeProjectPath(args[0])
			var featureName *string
			if len(args) == 2 {
				featureName = &args[1]
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if sessionID != "" {
				return startKeyedSession(cmd, conn, sessionID, project, featureName)
			}

			// Legacy path: one active session per project.
			existing, err := db.GetActiveSession(conn, project)
			if err != nil {
				return fmt.Errorf("failed to check active session: %w", err)
			}
			if existing != nil {
				return fmt.Errorf("active session already exists: %s", existing.SessionID)
			}

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

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Use this id (the Claude Code session id) instead of minting one; idempotent")
	return cmd
}

// startKeyedSession implements `session start --session-id`: create the row if
// missing, otherwise return the existing one, re-activating it when it had
// ended (a resumed Claude Code session keeps its id). The JSON output is the
// session plus "created" and "resumed" flags so hooks can tell the cases apart.
func startKeyedSession(cmd *cobra.Command, conn *sql.DB, sessionID, project string, featureName *string) error {
	if existing, err := db.GetSession(conn, sessionID); err == nil {
		resumed := false
		if existing.Status != "active" || existing.EndedAt != nil {
			if err := db.ReactivateSession(conn, sessionID); err != nil {
				return err
			}
			existing.Status = "active"
			existing.EndedAt = nil
			resumed = true
		}
		return encodeSessionWith(cmd, existing, map[string]interface{}{"created": false, "resumed": resumed})
	}

	session := &models.Session{
		SessionID:   sessionID,
		Project:     project,
		FeatureName: featureName,
		StartedAt:   time.Now().UnixMilli(),
		Status:      "active",
	}
	if err := db.CreateSession(conn, session); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return encodeSessionWith(cmd, session, map[string]interface{}{"created": true, "resumed": false})
}

// encodeSessionWith writes the session as JSON with extra top-level fields.
func encodeSessionWith(cmd *cobra.Command, session *models.Session, extra map[string]interface{}) error {
	raw, err := json.Marshal(session)
	if err != nil {
		return err
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	for k, v := range extra {
		out[k] = v
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
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
			project := normalizeProjectPath(args[0])

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			session, err := db.GetActiveSession(conn, project)
			if err != nil {
				return fmt.Errorf("failed to get active session: %w", err)
			}

			result := map[string]interface{}{
				"session": session,
			}
			if session == nil {
				result["message"] = "no active session"
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
