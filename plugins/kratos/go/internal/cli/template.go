package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// TemplateCmd returns the 'template' command with get/list/copy subcommands.
func TemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Retrieve Kratos agent templates",
	}
	cmd.AddCommand(templateGetCmd())
	cmd.AddCommand(templateListCmd())
	cmd.AddCommand(templateCopyCmd())
	return cmd
}

func findTemplatesDir() (string, error) {
	// 1. Env override (dev / testing)
	if dir := os.Getenv("KRATOS_TEMPLATES_DIR"); dir != "" {
		return dir, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	binDir := filepath.Dir(exe)

	// 2. <binary_dir>/templates/  — ~/.claude/hooks/kratos/ layout
	candidate := filepath.Join(binDir, "templates")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate, nil
	}

	// 3. <binary_dir>/../templates/  — ~/.kratos/bin/ and source-tree layouts
	candidate = filepath.Join(binDir, "..", "templates")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return filepath.Clean(candidate), nil
	}

	return "", fmt.Errorf("templates directory not found — run `kratos install` to set up the templates")
}

func templateGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get <name>",
		Short:        "Print a template to stdout",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !strings.HasSuffix(name, ".md") {
				name += ".md"
			}

			dir, err := findTemplatesDir()
			if err != nil {
				return err
			}

			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return fmt.Errorf("template %q not found in %s", args[0], dir)
			}

			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	return cmd
}

func templateListCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List available templates",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := findTemplatesDir()
			if err != nil {
				return err
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				return fmt.Errorf("failed to read templates directory: %w", err)
			}

			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSuffix(e.Name(), ".md"))
				}
			}
			return nil
		},
	}
}

func templateCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "copy <name> <dest>",
		Short:        "Copy a template to a destination path",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dest := args[1]

			if !strings.HasSuffix(name, ".md") {
				name += ".md"
			}

			dir, err := findTemplatesDir()
			if err != nil {
				return err
			}

			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return fmt.Errorf("template %q not found in %s", args[0], dir)
			}

			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return fmt.Errorf("failed to create destination directory: %w", err)
			}

			if err := os.WriteFile(dest, data, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", dest, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "copied %s → %s\n", args[0], dest)
			return nil
		},
	}
}
