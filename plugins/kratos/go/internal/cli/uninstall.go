package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// UninstallCmd returns the 'uninstall' command
func UninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Kratos hooks",
		Long:  "Removes hook files and settings (preserves database)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return uninstallHooks()
		},
	}
}

func uninstallHooks() error {
	fmt.Println("Kratos Hook Uninstaller")
	fmt.Println("=======================")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	claudeDir := filepath.Join(home, ".claude")
	hooksDir := filepath.Join(claudeDir, "hooks", "kratos")
	settingsFile := filepath.Join(claudeDir, "settings.json")

	// Remove hooks from settings.json
	fmt.Println("Updating settings.json...")
	if err := removeHooksFromSettings(settingsFile); err != nil {
		fmt.Printf("  ⚠ Failed to update settings: %v\n", err)
	} else {
		fmt.Println("  ✓ Removed kratos hooks from settings")
	}

	// Remove hook files
	fmt.Println("\nRemoving hook files...")
	if _, err := os.Stat(hooksDir); err == nil {
		if err := os.RemoveAll(hooksDir); err != nil {
			return fmt.Errorf("failed to remove hooks directory: %w", err)
		}
		fmt.Printf("  ✓ Removed %s\n", hooksDir)
	} else {
		fmt.Println("  ℹ Hook directory not found")
	}

	// Summary
	fmt.Println("\n=======================")
	fmt.Println("Uninstallation complete!")
	fmt.Printf("\nNote: Memory database preserved at %s\n", filepath.Join(home, ".kratos", "memory.db"))
	fmt.Println("To delete all data, manually remove the ~/.kratos directory.")

	return nil
}

func removeHooksFromSettings(settingsFile string) error {
	// Read existing settings
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	// Remove kratos hooks
	if hooks, ok := settings["hooks"].(map[string]interface{}); ok {
		delete(hooks, "SessionStart")
		delete(hooks, "PostToolUse")
		delete(hooks, "Stop")

		// Remove empty hooks object
		if len(hooks) == 0 {
			delete(settings, "hooks")
		}
	}

	// Write settings
	data, err = json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsFile, data, 0644)
}
