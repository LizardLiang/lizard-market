package cli

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/spf13/cobra"
)

// profileValueMaxLen is the maximum allowed length for a profile value.
// Larger than the memory cap — goals and work-hours descriptions need room.
const profileValueMaxLen = 500

// profileKeyPattern constrains keys to stable snake_case slots.
var profileKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// ProfileCmd returns the 'profile' subcommand
func ProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage the structured user profile",
		Long:  "Set, get, list, and remove structured user facts (timezone, work_hours, goals, current_focus, ...) stored in SQLite",
	}

	cmd.AddCommand(ProfileSetCmd())
	cmd.AddCommand(ProfileGetCmd())
	cmd.AddCommand(ProfileListCmd())
	cmd.AddCommand(ProfileRemoveCmd())

	return cmd
}

// ProfileSetCmd sets (upserts) a profile key
func ProfileSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a profile key (overwrites existing value)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			if !profileKeyPattern.MatchString(key) {
				return fmt.Errorf("invalid profile key %q (use snake_case: lowercase letters, digits, underscores)", key)
			}
			if len(value) > profileValueMaxLen {
				return fmt.Errorf("profile value exceeds %d characters (got %d)", profileValueMaxLen, len(value))
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			entry, err := db.SetProfile(conn, key, value)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status":  "set",
				"profile": entry,
			})
		},
	}
}

// ProfileGetCmd gets a single profile key
func ProfileGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a profile key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			entry, err := db.GetProfile(conn, args[0])
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"profile": entry,
			})
		},
	}
}

// ProfileListCmd lists all profile entries
func ProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profile entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			entries, err := db.ListProfile(conn)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"profile": entries,
				"count":   len(entries),
			})
		},
	}
}

// ProfileRemoveCmd removes a profile key
func ProfileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <key>",
		Short: "Remove a profile key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			if err := db.RemoveProfile(conn, args[0]); err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "removed",
				"key":    args[0],
			})
		},
	}
}
