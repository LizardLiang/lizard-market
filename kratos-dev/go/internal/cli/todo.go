package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
)

// getProject returns the current project name from git or directory name
func getProject() string {
	if p := os.Getenv("KRATOS_PROJECT"); p != "" {
		return p
	}
	// Try git repo name
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		return filepath.Base(strings.TrimSpace(string(out)))
	}
	// Fall back to current directory name
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Base(cwd)
	}
	return "default"
}

// TodoCmd returns the 'todo' subcommand
func TodoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Manage personal todo list",
		Long:  "Add, list, complete, and remove personal todo items stored in SQLite",
	}

	cmd.AddCommand(TodoAddCmd())
	cmd.AddCommand(TodoListCmd())
	cmd.AddCommand(TodoDoneCmd())
	cmd.AddCommand(TodoRemoveCmd())

	return cmd
}

// TodoAddCmd adds a new todo
func TodoAddCmd() *cobra.Command {
	var source string

	cmd := &cobra.Command{
		Use:   "add <text>",
		Short: "Add a new todo item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := args[0]
			project := getProject()

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			todo, err := db.AddTodo(conn, text, project, source, nil)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "added",
				"todo":   todo,
			})
		},
	}

	cmd.Flags().StringVar(&source, "source", "user", "Source of the todo (user, ananke, jira)")
	return cmd
}

// TodoListCmd lists todos
func TodoListCmd() *cobra.Command {
	var status string
	var source string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List todo items",
		RunE: func(cmd *cobra.Command, args []string) error {
			project := getProject()

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			todos, err := db.ListTodos(conn, project, status, source)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"project": project,
				"status":  status,
				"todos":   todos,
				"count":   len(todos),
			})
		},
	}

	cmd.Flags().StringVar(&status, "status", "open", "Filter by status: open, done, all")
	cmd.Flags().StringVar(&source, "source", "all", "Filter by source: user, jira, ananke, all")
	return cmd
}

// TodoDoneCmd marks a todo as complete
func TodoDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a todo as complete",
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

			todo, err := db.DoneTodo(conn, id)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "done",
				"todo":   todo,
			})
		},
	}
}

// TodoRemoveCmd removes a todo
func TodoRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a todo item",
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

			if err := db.RemoveTodo(conn, id); err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "removed",
				"id":     id,
			})
		},
	}
}