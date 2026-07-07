package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// spec export renders living spec shards (.claude/.Arena/specs/<capability>/spec.md)
// into a single self-contained HTML document (inline CSS/JS, no external resources)
// or a concatenated Markdown document. Pending spec deltas are never included —
// only archived, living content is exported. See spec_export_render.go for the
// hand-rolled markdown-to-HTML renderer and the go:embed HTML shell.

// specExportShard pairs a capability slug with its parsed living spec shard, in the
// sorted order they should appear in an export.
type specExportShard struct {
	Capability string
	Shard      *specShard
}

// exportTimestampFormat is the "generated at" timestamp layout shared by the
// HTML and Markdown export footers/headers.
const exportTimestampFormat = "2006-01-02 15:04:05 UTC"

var (
	slugNonWordRE = regexp.MustCompile(`[^a-z0-9\s-]`)
	slugSpaceRE   = regexp.MustCompile(`\s+`)
)

// normalizeNewlines converts CRLF to LF so downstream line-based parsing never
// has to special-case a trailing '\r' (spec.go's own regexes tolerate \r?\n; the
// renderer instead normalizes once up front).
func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// slugifyHeading turns arbitrary heading text into a GitHub-style anchor slug:
// lowercase, punctuation stripped, whitespace collapsed to hyphens. This is
// deliberately separate from slug.go's slugify (which backs `kratos slug` and
// collapses any run of non-alphanumeric characters, including hyphens, into
// one dash): markdown export's TOC links must resolve against whatever GFM
// renderer displays the file (GitHub, VS Code preview, etc.), which slugifies
// headings by stripping punctuation without collapsing repeated hyphens.
// Reusing slugify here would produce anchors that don't match the renderer's
// own heading IDs.
func slugifyHeading(s string) string {
	s = strings.ToLower(s)
	s = slugNonWordRE.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return slugSpaceRE.ReplaceAllString(s, "-")
}

// shardTitle returns shard's own H1 title, falling back to a human-readable
// title derived from the capability slug when the shard has none.
func shardTitle(shard *specShard, capability string) string {
	title := shard.Title
	if strings.TrimSpace(title) == "" {
		title = capabilityTitleFromSlug(capability)
	}
	return title
}

// specsExportDirIn is the default export destination, mirroring specsDirIn's
// root-parameterized path helper convention.
func specsExportDirIn(root string) string {
	return filepath.Join(root, ".claude", ".Arena", "specs-export")
}

// loadSpecShardsIn reads every living capability shard under specsDirIn(root),
// sorted by capability slug. A missing specs directory is not an error — it is
// the empty-state case, reported to the caller as a nil, nil-error slice.
//
// A capability directory without a readable spec.md is skipped rather than
// failing the whole export (mirrors specListCapabilities' tolerance for a
// stray/incomplete directory), but it is not silently dropped: an export is
// meant to be a complete, shareable archive, so every skipped capability is
// reported as a stderr warning (same pattern as pipelineDiscoverVerify's
// ambiguous-candidate warning in pipeline.go) — this never changes the
// returned shards, the error, or the caller's exit code.
func loadSpecShardsIn(root string) ([]specExportShard, error) {
	dir := specsDirIn(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var skipped []string
	shards := make([]specExportShard, 0, len(names))
	for _, name := range names {
		raw, readErr := os.ReadFile(specShardPathIn(root, name))
		if readErr != nil {
			skipped = append(skipped, name)
			continue
		}
		shards = append(shards, specExportShard{Capability: name, Shard: parseSpecShard(string(raw))})
	}
	if len(skipped) > 0 {
		fmt.Fprintf(os.Stderr, "warning: spec export skipped %d unreadable capability shard(s): %s\n", len(skipped), strings.Join(skipped, ", "))
	}
	return shards, nil
}

// selectExportShards narrows allShards to what one export call should render:
// every shard when capability is empty, or exactly the one matching shard when
// it isn't. Returns an error if capability fails the path-safety check shared
// with the other spec subcommands, or names a capability with no living spec —
// in the latter case, the error lists whatever capabilities do exist.
func selectExportShards(allShards []specExportShard, capability string) ([]specExportShard, error) {
	if capability == "" {
		return allShards, nil
	}
	if err := validSpecName(capability); err != nil {
		return nil, err
	}
	for _, s := range allShards {
		if s.Capability == capability {
			return []specExportShard{s}, nil
		}
	}
	if len(allShards) == 0 {
		return nil, fmt.Errorf("no living spec for capability %q — no living specs exist yet", capability)
	}
	names := make([]string, 0, len(allShards))
	for _, s := range allShards {
		names = append(names, s.Capability)
	}
	return nil, fmt.Errorf("no living spec for capability %q — available: %s", capability, strings.Join(names, ", "))
}

// resolveExportPath returns the file an export should be written to: outOverride
// verbatim when the caller gave one, otherwise the default
// .claude/.Arena/specs-export/ location — "specs.<format>" for a full export,
// "<capability>.<format>" for a single-capability one.
func resolveExportPath(root, outOverride, capability, format string) string {
	if outOverride != "" {
		return outOverride
	}
	filename := "specs." + format
	if capability != "" {
		filename = capability + "." + format
	}
	return filepath.Join(specsExportDirIn(root), filename)
}

// specExportIn renders living specs (all capabilities, or one named capability) in
// the given format ("html" or "md") and writes the result to outOverride, or the
// default .claude/.Arena/specs-export/ location when outOverride is empty.
//
// Returns the written file's absolute path and wrote=true on success. When there
// are no living specs to export, it returns wrote=false with a nil error (the
// empty-state case) instead of writing anything.
func specExportIn(root, capability, format, outOverride string) (path string, wrote bool, err error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "html"
	}
	if format != "html" && format != "md" {
		return "", false, fmt.Errorf("invalid --format %q: must be html or md", format)
	}

	allShards, err := loadSpecShardsIn(root)
	if err != nil {
		return "", false, err
	}

	shards, err := selectExportShards(allShards, capability)
	if err != nil {
		return "", false, err
	}
	if len(shards) == 0 {
		return "", false, nil
	}

	var rendered string
	switch format {
	case "html":
		rendered, err = renderSpecExportHTML(shards)
		if err != nil {
			return "", false, err
		}
	case "md":
		rendered = renderSpecExportMarkdown(shards)
	}

	outPath := resolveExportPath(root, outOverride, capability, format)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", false, fmt.Errorf("cannot create export directory %s: %w", filepath.Dir(outPath), err)
	}
	if err := os.WriteFile(outPath, []byte(rendered), 0o644); err != nil {
		return "", false, fmt.Errorf("cannot write export file %s: %w", outPath, err)
	}

	abs, absErr := filepath.Abs(outPath)
	if absErr != nil {
		return outPath, true, nil
	}
	return abs, true, nil
}

// renderSpecExportMarkdown concatenates every shard's rendered content (frontmatter
// stripped) into one document, preceded by a generated table of contents with
// anchor links. Reuses specShard.render for byte-identical fidelity with the
// living spec's own canonical serialization.
func renderSpecExportMarkdown(shards []specExportShard) string {
	var toc strings.Builder
	var body strings.Builder

	toc.WriteString("# Living Specs Export\n\n")
	toc.WriteString(fmt.Sprintf("_Generated %s_\n\n", time.Now().UTC().Format(exportTimestampFormat)))
	toc.WriteString("## Table of Contents\n\n")

	for _, s := range shards {
		title := shardTitle(s.Shard, s.Capability)
		toc.WriteString(fmt.Sprintf("- [%s](#%s)\n", title, slugifyHeading(title)))
		for _, r := range s.Shard.Requirements {
			toc.WriteString(fmt.Sprintf("  - [%s](#%s)\n", r.Name, slugifyHeading("Requirement: "+r.Name)))
		}

		rendered := frontmatterRE.ReplaceAllString(s.Shard.render(s.Capability), "")
		body.WriteString(strings.TrimLeft(rendered, "\n"))
		body.WriteString("\n\n---\n\n")
	}

	full := toc.String() + "\n" + strings.TrimRight(body.String(), "\n") + "\n"
	return normalizeNewlines(full)
}

// ---------- Cobra command ----------

func specExportCmd() *cobra.Command {
	var format, out string
	cmd := &cobra.Command{
		Use:   "export [capability]",
		Short: "Export living specs to a self-contained HTML or Markdown document",
		Long: "Renders .claude/.Arena/specs/<capability>/spec.md shards into a single self-contained " +
			"HTML document (inline CSS/JS, no external resources) or a concatenated Markdown document. " +
			"Pending spec deltas are never included — only archived, living content is exported.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			capability := ""
			if len(args) == 1 {
				capability = args[0]
			}
			path, wrote, err := specExportIn(gitRoot(), capability, format, out)
			if err != nil {
				return err
			}
			if !wrote {
				fmt.Fprintln(cmd.OutOrStdout(), "no living specs found — the Arena holds no record yet; nothing to export")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "html", "export format: html or md")
	cmd.Flags().StringVar(&out, "out", "", "write the export to this path instead of the default .claude/.Arena/specs-export/ location")
	return cmd
}
