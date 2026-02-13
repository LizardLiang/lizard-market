package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/db"
)

// InitCmd returns the 'init' command for initializing the Kratos database
func InitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the Kratos memory database",
		Long: `Initialize the Kratos memory database with the required schema.

This command creates the database file at ~/.kratos/memory.db (or $KRATOS_MEMORY_DB)
and sets up all tables, indexes, and full-text search triggers.

This command is idempotent - it can be safely run multiple times.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("initialization failed: %w", err)
			}

			result := map[string]string{"status": "initialized"}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}
