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
		Long:  "Removes legacy hook files and settings entries (preserves database)",
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

	// Remove kratos entries from settings.json — only ours; other tools' hooks
	// on the same events stay (the old code deleted whole event lists).
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

// removeHooksFromSettings strips every kratos-authored entry: the legacy
// ~/.claude/hooks/kratos/ commands and the permission rules the old installer
// added for the plugin cache and the ~/.kratos binary.
func removeHooksFromSettings(settingsFile string) error {
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	removeLegacyHookEntries(settings)

	if perms, ok := settings["permissions"].(map[string]interface{}); ok {
		if allowList, ok := perms["allow"].([]interface{}); ok {
			filtered := make([]interface{}, 0, len(allowList))
			for _, rule := range allowList {
				s, _ := rule.(string)
				isKratos := s == "Read(~/.claude/plugins/cache/lizard-plugins/kratos/**)" ||
					s == "Bash(~/.kratos/bin/kratos:*)"
				if !isKratos {
					filtered = append(filtered, rule)
				}
			}
			if len(filtered) == 0 {
				delete(perms, "allow")
			} else {
				perms["allow"] = filtered
			}
			if len(perms) == 0 {
				delete(settings, "permissions")
			}
		}
	}

	return writeSettings(settingsFile, settings)
}
