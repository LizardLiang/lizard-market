package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/cli"
)

var version = "2.0.0-go"

func main() {
	rootCmd := &cobra.Command{
		Use:     "kratos",
		Short:   "Kratos Memory System - Fast SQLite journey tracking",
		Version: version,
	}

	rootCmd.AddCommand(cli.InitCmd())
	rootCmd.AddCommand(cli.SessionCmd())
	rootCmd.AddCommand(cli.QueryCmd())
	rootCmd.AddCommand(cli.RecallCmd())
	rootCmd.AddCommand(cli.StepCmd())
	rootCmd.AddCommand(cli.InstallCmd())
	rootCmd.AddCommand(cli.UninstallCmd())
	rootCmd.AddCommand(cli.StatusCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
