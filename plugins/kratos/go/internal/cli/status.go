package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// StatusCmd returns the 'status' command
func StatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Kratos installation status",
		Long:  "Shows whether hooks are installed and configured",
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkStatus()
		},
	}
}

func checkStatus() error {
	fmt.Println("Kratos Installation Status")
	fmt.Println("===========================")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	claudeDir := filepath.Join(home, ".claude")
	hooksDir := filepath.Join(claudeDir, "hooks", "kratos")
	settingsFile := filepath.Join(claudeDir, "settings.json")
	dbPath := filepath.Join(home, ".kratos", "memory.db")

	// Check hooks directory
	hooksInstalled := false
	var hookFiles []string
	if stat, err := os.Stat(hooksDir); err == nil && stat.IsDir() {
		hooksInstalled = true
		entries, _ := os.ReadDir(hooksDir)
		for _, entry := range entries {
			hookFiles = append(hookFiles, entry.Name())
		}
	}

	fmt.Printf("Hooks directory: %s\n", statusString(hooksInstalled))
	if hooksInstalled {
		fmt.Printf("  Location: %s\n", hooksDir)
		fmt.Printf("  Files: %s\n", strings.Join(hookFiles, ", "))
	}

	// Check settings.json
	hasKratosHooks := false
	if data, err := os.ReadFile(settingsFile); err == nil {
		var settings map[string]interface{}
		if json.Unmarshal(data, &settings) == nil {
			if hooks, ok := settings["hooks"].(map[string]interface{}); ok {
				_, hasStart := hooks["SessionStart"]
				_, hasPost := hooks["PostToolUse"]
				_, hasStop := hooks["Stop"]
				hasKratosHooks = hasStart || hasPost || hasStop
			}
		}
	}

	fmt.Printf("Settings.json: %s\n", statusString(hasKratosHooks))

	// Check database
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

	// Check kratos binary
	kratosInHooks := false
	kratosBinaryPath := filepath.Join(hooksDir, "kratos")
	if _, err := os.Stat(kratosBinaryPath); err == nil {
		kratosInHooks = true
	}

	fmt.Printf("Kratos binary: %s\n", statusString(kratosInHooks))
	if kratosInHooks {
		fmt.Printf("  Location: %s\n", kratosBinaryPath)
	}

	// Overall status
	fmt.Println("\n===========================")
	if hooksInstalled && hasKratosHooks && kratosInHooks {
		fmt.Println("Status: ✅ FULLY OPERATIONAL")
	} else if hooksInstalled && hasKratosHooks {
		fmt.Println("Status: ⚠ INSTALLED (Binary missing)")
		fmt.Println("\nRun 'kratos install' to reinstall.")
	} else {
		fmt.Println("Status: ❌ NOT INSTALLED")
		fmt.Println("\nRun 'kratos install' to install.")
	}

	return nil
}

func statusString(ok bool) string {
	if ok {
		return "✅ INSTALLED"
	}
	return "❌ NOT INSTALLED"
}
