package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// InstallCmd returns the 'install' command
func InstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install Kratos hooks globally",
		Long:  "Copies hook files to ~/.claude/hooks/kratos/ and updates settings.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return installHooks()
		},
	}
}

func installHooks() error {
	fmt.Println("Kratos Hook Installer (Go)")
	fmt.Println("===========================")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	claudeDir := filepath.Join(home, ".claude")
	hooksDir := filepath.Join(claudeDir, "hooks", "kratos")
	settingsFile := filepath.Join(claudeDir, "settings.json")

	// Get kratos executable path
	kratosExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get hooks source directory (relative to executable)
	// Assuming kratos binary is in plugins/kratos/bin/
	binDir := filepath.Dir(kratosExe)
	pluginRoot := filepath.Dir(binDir)
	hooksSource := filepath.Join(pluginRoot, "hooks")

	// Create directories
	fmt.Println("Creating directories...")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}
	fmt.Printf("  ✓ %s\n", hooksDir)

	// Copy hook files
	fmt.Println("\nCopying hook files...")
	hookFiles := []string{
		"session-start.cjs",
		"session-end.cjs",
		"tool-use.cjs",
	}

	for _, file := range hookFiles {
		src := filepath.Join(hooksSource, file)
		dst := filepath.Join(hooksDir, file)

		if err := copyFile(src, dst); err != nil {
			fmt.Printf("  ⚠ %s (skipped: %v)\n", file, err)
		} else {
			fmt.Printf("  ✓ %s\n", file)
		}
	}

	// Copy kratos binary
	fmt.Println("\nCopying kratos binary...")
	kratosDst := filepath.Join(hooksDir, "kratos")
	if err := copyFile(kratosExe, kratosDst); err != nil {
		return fmt.Errorf("failed to copy kratos binary: %w", err)
	}
	// Make it executable
	if err := os.Chmod(kratosDst, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}
	fmt.Printf("  ✓ kratos binary\n")

	// Update settings.json
	fmt.Println("\nUpdating settings.json...")
	if err := updateSettings(settingsFile, hooksDir); err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	fmt.Printf("  ✓ Updated ~/.claude/settings.json\n")

	// Summary
	fmt.Println("\n===========================")
	fmt.Println("Installation complete!")
	fmt.Printf("\nHooks installed to: %s\n", hooksDir)
	fmt.Printf("Memory database: %s\n", filepath.Join(home, ".kratos", "memory.db"))
	fmt.Println("\nKratos will now track your sessions automatically.")
	fmt.Println("Use 'kratos recall' to see your last session context.")

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func updateSettings(settingsFile, hooksDir string) error {
	// Read existing settings
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsFile); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return err
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Ensure hooks object exists
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		hooks = make(map[string]interface{})
		settings["hooks"] = hooks
	}

	// Generate hook config
	hookPath := filepath.ToSlash(hooksDir) // Normalize for JSON

	// SessionStart hook
	hooks["SessionStart"] = []map[string]interface{}{
		{
			"matcher": "",
			"hooks": []map[string]interface{}{
				{
					"type":    "command",
					"command": fmt.Sprintf("node \"%s/session-start.cjs\"", hookPath),
					"timeout": 5000,
				},
			},
		},
	}

	// PostToolUse hook
	hooks["PostToolUse"] = []map[string]interface{}{
		{
			"matcher": "Task|Write|Edit",
			"hooks": []map[string]interface{}{
				{
					"type":    "command",
					"command": fmt.Sprintf("node \"%s/tool-use.cjs\"", hookPath),
					"timeout": 5000,
				},
			},
		},
	}

	// Stop hook
	hooks["Stop"] = []map[string]interface{}{
		{
			"matcher": "",
			"hooks": []map[string]interface{}{
				{
					"type":    "command",
					"command": fmt.Sprintf("node \"%s/session-end.cjs\"", hookPath),
					"timeout": 10000,
				},
			},
		},
	}

	// Write settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsFile, data, 0644)
}
