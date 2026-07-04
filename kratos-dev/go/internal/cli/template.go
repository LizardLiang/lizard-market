package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Templates are embedded at build time from go/internal/cli/templates/.
// This directory is a maintained copy of plugins/kratos/templates/ — keep in sync when editing templates.
//
//go:embed templates/*.md
var templatesFS embed.FS

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

func templateGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "get <name>",
		Short:        "Print a template to stdout",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !strings.HasSuffix(name, ".md") {
				name += ".md"
			}
			data, err := templatesFS.ReadFile("templates/" + name)
			if err != nil {
				return fmt.Errorf("template %q not found", args[0])
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func templateListCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List available templates",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := templatesFS.ReadDir("templates")
			if err != nil {
				return fmt.Errorf("failed to read embedded templates: %w", err)
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
			data, err := templatesFS.ReadFile("templates/" + name)
			if err != nil {
				return fmt.Errorf("template %q not found", args[0])
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
