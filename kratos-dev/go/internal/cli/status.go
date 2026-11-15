package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// StatusCmd returns the 'status' command
func StatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Kratos installation status",
		Long:  "Shows the memory database, the binary, and any legacy global hooks left by old installs (hooks ship with the plugin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkStatus(cmd.Root().Version)
		},
	}
}

func checkStatus(version string) error {
	fmt.Println("Kratos Installation Status")
	fmt.Println("===========================")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	settingsFile := filepath.Join(home, ".claude", "settings.json")
	dbPath := filepath.Join(home, ".kratos", "memory.db")

	// Hooks ship with the plugin (hooks/hooks.json). The only hook state worth
	// reporting is a legacy global install, which double-fires every hook next
	// to the plugin's own (2026-09 review: 63 of 63 sessions).
	fmt.Println("Hooks: provided by the plugin (hooks/hooks.json)")
	legacy := hasLegacyHooks(settingsFile)
	if legacy {
		fmt.Println("  ⚠ Legacy global hooks found in ~/.claude/settings.json — run 'kratos install'")
	}

	dbExists := false
	var dbSize int64
	if stat, err := os.Stat(dbPath); err == nil {
		dbExists = true
		dbSize = stat.Size()
	}
	fmt.Printf("Memory database: %s\n", statusString(dbExists))
	if dbExists {
		fmt.Printf("  Size: %.1f KB\n", float64(dbSize)/1024)
	}

	if exe, err := os.Executable(); err == nil {
		fmt.Printf("Kratos binary: %s (%s)\n", exe, version)
	}

	fmt.Println("\n===========================")
	switch {
	case legacy:
		fmt.Println("Status: ⚠ LEGACY HOOKS — run 'kratos install'")
	case !dbExists:
		fmt.Println("Status: ❌ DATABASE MISSING — run 'kratos init'")
	default:
		fmt.Println("Status: ✅ FULLY OPERATIONAL")
	}

	return nil
}

func statusString(ok bool) string {
	if ok {
		return "✅ INSTALLED"
	}
	return "❌ NOT INSTALLED"
}
