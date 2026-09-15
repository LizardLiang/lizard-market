package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// legacyHookMarker identifies hook commands and permission rules written by the
// pre-plugin installer, which copied session-start/session-end/tool-use into
// ~/.claude/hooks/kratos/ and registered them in ~/.claude/settings.json.
const legacyHookMarker = "hooks/kratos/"

// InstallCmd returns the 'install' command.
//
// Hooks ship inside the plugin (hooks/hooks.json) and are registered by Claude
// Code when the plugin is enabled, so there is nothing to install any more. The
// old copies never updated and ran alongside the plugin hooks in every session:
// `recall --project` (a flag that no longer exists) errored at every
// SessionStart, `session start` was refused as "active session already exists"
// because the plugin hook had already registered the session, and the legacy
// Stop hook printed "Kratos: Session ended" after every turn (2026-09 transcript
// review). `install` is therefore a migration: it removes those entries and
// files and leaves the plugin's own hooks in place.
func InstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Remove legacy Kratos hooks from ~/.claude (hooks now ship with the plugin)",
		Long: `Hooks are registered by the plugin's hooks/hooks.json, so nothing needs
installing. This command migrates an older setup: it removes the
~/.claude/hooks/kratos/ copies and their settings.json entries, which
otherwise run twice per event alongside the plugin hooks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateLegacyHooks(cmd)
		},
	}
}

func migrateLegacyHooks(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	claudeDir := filepath.Join(home, ".claude")
	hooksDir := filepath.Join(claudeDir, "hooks", "kratos")
	settingsFile := filepath.Join(claudeDir, "settings.json")

	fmt.Fprintln(out, "Kratos hooks ship with the plugin (hooks/hooks.json); checking for legacy copies.")

	removed, err := removeLegacyHooksFromSettingsFile(settingsFile)
	switch {
	case err != nil && os.IsNotExist(err):
		fmt.Fprintln(out, "  ℹ no ~/.claude/settings.json — nothing to migrate")
	case err != nil:
		return fmt.Errorf("failed to update %s: %w", settingsFile, err)
	case removed == 0:
		fmt.Fprintln(out, "  ✓ settings.json has no legacy kratos hook entries")
	default:
		fmt.Fprintf(out, "  ✓ removed %d legacy hook/permission entries from settings.json\n", removed)
	}

	if _, err := os.Stat(hooksDir); err == nil {
		if err := os.RemoveAll(hooksDir); err != nil {
			return fmt.Errorf("failed to remove %s: %w", hooksDir, err)
		}
		fmt.Fprintf(out, "  ✓ removed %s\n", hooksDir)
	} else {
		fmt.Fprintln(out, "  ✓ no legacy hook directory")
	}

	fmt.Fprintln(out, "\nDone. Restart Claude Code sessions so only the plugin hooks run.")
	return nil
}

// removeLegacyHooksFromSettingsFile rewrites settingsFile without the legacy
// kratos hook entries and permission rules. Returns how many entries were
// removed; the file is left untouched when that is zero.
func removeLegacyHooksFromSettingsFile(settingsFile string) (int, error) {
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return 0, err
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return 0, err
	}
	removed := removeLegacyHookEntries(settings)
	if removed == 0 {
		return 0, nil
	}
	return removed, writeSettings(settingsFile, settings)
}

// removeLegacyHookEntries drops, in place, every hook command and permission
// rule that points at ~/.claude/hooks/kratos/. Hooks registered for the same
// event by anything else are preserved. Returns the number of entries removed.
func removeLegacyHookEntries(settings map[string]interface{}) int {
	removed := 0

	if hooks, ok := settings["hooks"].(map[string]interface{}); ok {
		for event, groupsRaw := range hooks {
			groups, ok := groupsRaw.([]interface{})
			if !ok {
				continue
			}
			keptGroups := make([]interface{}, 0, len(groups))
			for _, groupRaw := range groups {
				group, ok := groupRaw.(map[string]interface{})
				if !ok {
					keptGroups = append(keptGroups, groupRaw)
					continue
				}
				entries, ok := group["hooks"].([]interface{})
				if !ok {
					keptGroups = append(keptGroups, groupRaw)
					continue
				}
				keptEntries := make([]interface{}, 0, len(entries))
				for _, entryRaw := range entries {
					entry, ok := entryRaw.(map[string]interface{})
					command, _ := entry["command"].(string)
					if ok && isLegacyKratosCommand(command) {
						removed++
						continue
					}
					keptEntries = append(keptEntries, entryRaw)
				}
				if len(keptEntries) == 0 {
					continue
				}
				group["hooks"] = keptEntries
				keptGroups = append(keptGroups, group)
			}
			if len(keptGroups) == 0 {
				delete(hooks, event)
			} else {
				hooks[event] = keptGroups
			}
		}
		if len(hooks) == 0 {
			delete(settings, "hooks")
		}
	}

	if perms, ok := settings["permissions"].(map[string]interface{}); ok {
		if allowList, ok := perms["allow"].([]interface{}); ok {
			filtered := make([]interface{}, 0, len(allowList))
			for _, rule := range allowList {
				if s, _ := rule.(string); isLegacyKratosCommand(s) {
					removed++
					continue
				}
				filtered = append(filtered, rule)
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

	return removed
}

// isLegacyKratosCommand matches both slash and backslash spellings of the
// legacy path; filepath.ToSlash is a no-op on Linux, so the replacement is
// explicit (the CI runner is Linux, the settings files come from Windows).
func isLegacyKratosCommand(s string) bool {
	return strings.Contains(strings.ReplaceAll(s, "\\", "/"), legacyHookMarker)
}

// writeSettings writes settings.json indented and without HTML escaping, so
// matcher regexes such as "Write|Edit" and paths survive the round trip.
func writeSettings(settingsFile string, settings map[string]interface{}) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(settings); err != nil {
		return err
	}
	return os.WriteFile(settingsFile, buf.Bytes(), 0644)
}
