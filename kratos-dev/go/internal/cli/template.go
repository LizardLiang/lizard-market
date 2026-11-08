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
//go:embed templates/*.md templates/*.html
var templatesFS embed.FS

// templateFile resolves a template name to its embedded path: an explicit
// `.md`/`.html` name is taken as is; a bare name tries `.md`, then `.html`
// (the plan page skeleton, plan-page-template.html, is the one HTML template).
func templateFile(name string) (string, error) {
	candidates := []string{name}
	if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".html") {
		candidates = []string{name + ".md", name + ".html"}
	}
	for _, c := range candidates {
		if _, err := templatesFS.ReadFile("templates/" + c); err == nil {
			return "templates/" + c, nil
		}
	}
	return "", fmt.Errorf("template %q not found", name)
}

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
			file, err := templateFile(args[0])
			if err != nil {
				return err
			}
			data, _ := templatesFS.ReadFile(file)
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
				switch {
				case e.IsDir():
				case strings.HasSuffix(e.Name(), ".md"):
					fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSuffix(e.Name(), ".md"))
				case strings.HasSuffix(e.Name(), ".html"):
					fmt.Fprintln(cmd.OutOrStdout(), e.Name())
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
			dest := args[1]
			file, err := templateFile(args[0])
			if err != nil {
				return err
			}
			data, _ := templatesFS.ReadFile(file)
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
