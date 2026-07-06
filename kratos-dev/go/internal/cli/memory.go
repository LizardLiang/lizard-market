package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/spf13/cobra"
)

// memoryTextMaxLen is the maximum allowed length for a single memory's text.
const memoryTextMaxLen = 200

// MemoryCmd returns the 'memory' subcommand
func MemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage the persistent user memory model",
		Long:  "Add, list, and remove durable user facts (preferences, habits, weak spots) stored in SQLite",
	}

	cmd.AddCommand(MemoryAddCmd())
	cmd.AddCommand(MemoryListCmd())
	cmd.AddCommand(MemoryRemoveCmd())

	return cmd
}

// MemoryAddCmd adds a new user memory
func MemoryAddCmd() *cobra.Command {
	var category string

	cmd := &cobra.Command{
		Use:   "add <text>",
		Short: "Add a new user memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := args[0]
			if len(text) > memoryTextMaxLen {
				return fmt.Errorf("memory text exceeds %d characters (got %d)", memoryTextMaxLen, len(text))
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			memory, err := db.AddMemory(conn, text, category)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "added",
				"memory": memory,
			})
		},
	}

	cmd.Flags().StringVar(&category, "category", "context", "Category: preference, habit, weak-spot, context")
	return cmd
}

// MemoryListCmd lists user memories
func MemoryListCmd() *cobra.Command {
	var category string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List user memories",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			memories, err := db.ListMemories(conn, category)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"category": category,
				"memories": memories,
				"count":    len(memories),
			})
		},
	}

	cmd.Flags().StringVar(&category, "category", "all", "Filter by category: preference, habit, weak-spot, context, all")
	return cmd
}

// MemoryRemoveCmd removes a user memory
func MemoryRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a user memory",
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

			if err := db.RemoveMemory(conn, id); err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "removed",
				"id":     id,
			})
		},
	}
}
