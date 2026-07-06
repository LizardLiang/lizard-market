package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/spf13/cobra"
)

// RoutineCmd returns the 'routine' subcommand
func RoutineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routine",
		Short: "Manage recurring personal routines",
		Long:  "Add, list, complete, and remove recurring routines (daily, weekly, monthly) stored in SQLite",
	}

	cmd.AddCommand(RoutineAddCmd())
	cmd.AddCommand(RoutineListCmd())
	cmd.AddCommand(RoutineDoneCmd())
	cmd.AddCommand(RoutineRemoveCmd())

	return cmd
}

// routineNow resolves "today" for due computation: the profile timezone key
// (when a valid IANA name) overrides system local time, silently.
func routineNow(conn *sql.DB) time.Time {
	entry, err := db.GetProfile(conn, "timezone")
	if err != nil {
		return time.Now()
	}
	loc, err := time.LoadLocation(entry.Value)
	if err != nil {
		return time.Now()
	}
	return time.Now().In(loc)
}

// RoutineAddCmd adds a new routine
func RoutineAddCmd() *cobra.Command {
	var cadence string

	cmd := &cobra.Command{
		Use:   "add <text>",
		Short: "Add a recurring routine",
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

			routine, err := db.AddRoutine(conn, args[0], cadence)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status":  "added",
				"routine": routine,
			})
		},
	}

	cmd.Flags().StringVar(&cadence, "cadence", "daily", "Cadence: daily, weekly:<day[,day...]>, monthly:<1-28>")
	return cmd
}

// RoutineListCmd lists routines with due_today computed
func RoutineListCmd() *cobra.Command {
	var dueOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List routines (due_today computed against the profile timezone or system local)",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			now := routineNow(conn)
			routines, err := db.ListRoutines(conn, now, dueOnly)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"date":     now.Format("2006-01-02"),
				"routines": routines,
				"count":    len(routines),
			})
		},
	}

	cmd.Flags().BoolVar(&dueOnly, "due", false, "Only show routines due today")
	return cmd
}

// RoutineDoneCmd marks a routine as done
func RoutineDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a routine as done for today",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			routine, err := db.DoneRoutine(conn, id)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status":  "done",
				"routine": routine,
			})
		},
	}
}

// RoutineRemoveCmd removes a routine
func RoutineRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a routine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			if err := db.RemoveRoutine(conn, id); err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "removed",
				"id":     id,
			})
		},
	}
}
