package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// HermesListCmd is the parent command for Hermes tier-checklist operations.
func HermesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hermes-list",
		Short: "Manage the Hermes review tier checklist",
	}
	cmd.AddCommand(hermesListCheckCmd())
	cmd.AddCommand(hermesListShowCmd())
	return cmd
}

// tierShortForms maps short aliases (T1–T8) to canonical tier keys.
var tierShortForms = map[string]string{
	"T1": "T1_correct",
	"T2": "T2_safe",
	"T3": "T3_clear",
	"T4": "T4_minimal",
	"T5": "T5_consistent",
	"T6": "T6_resilient",
	"T7": "T7_performant",
	"T8": "T8_maintainable",
}

func hermesListCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <tier>",
		Short: "Mark a Hermes review tier complete in hermes-checklist.json",
		Long:  "Accepts short form (T1–T8) or full key (T1_correct–T8_maintainable).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tier := args[0]
			// Accept short forms like T1, T2 … T8.
			if full, ok := tierShortForms[strings.ToUpper(tier)]; ok {
				tier = full
			}
			if _, ok := tierDisplayNames[tier]; !ok {
				return fmt.Errorf("invalid tier %q (valid short forms: T1–T8, or full keys: %s)", args[0], strings.Join(tierOrder, ", "))
			}

			cwd, _ := os.Getwd()
			path := findHermesChecklist(cwd)
			if path == "" {
				return fmt.Errorf("hermes-checklist.json not found under %s; SubagentStart hook may not have run", cwd)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}

			var doc map[string]interface{}
			if err := json.Unmarshal(data, &doc); err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}

			tiers, _ := doc["tiers"].(map[string]interface{})
			if tiers == nil {
				tiers = map[string]interface{}{}
			}
			tiers[tier] = true
			doc["tiers"] = tiers

			updated, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return err
			}

			tmp := path + ".tmp"
			if err := os.WriteFile(tmp, updated, 0644); err != nil {
				return err
			}
			if err := os.Rename(tmp, path); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s marked complete: %s\n", tier, path)
			return nil
		},
	}
}

func hermesListShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the current hermes-checklist.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, _ := os.Getwd()
			path := findHermesChecklist(cwd)
			if path == "" {
				return fmt.Errorf("hermes-checklist.json not found under %s", cwd)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}
