package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// canonicalStageKeys are the pipeline keys the CLI understands. A status.json
// carrying any other numbered key predates the 2026-07 renumbering and can no
// longer be advanced by `pipeline next`.
var canonicalStageKeys = map[string]bool{
	"0-research": true, "1-prd": true, "2-prd-review": true, "3-decomposition": true,
	"4-tech-spec": true, "5-spec-review-sa": true, "6-test-plan": true,
	"7-implementation": true, "8-prd-alignment": true, "9-review": true,
}

// featureArchiveDirName is where `pipeline gc` moves retired feature folders.
// Glob-based discovery (`.claude/feature/*/status.json`) never looks one level
// deeper, so archived features stop appearing in `pipeline next` candidates.
const featureArchiveDirName = "_archive"

type gcEntry struct {
	Feature string `json:"feature"`
	Reason  string `json:"reason"`
}

type gcResult struct {
	Root     string    `json:"root"`
	DryRun   bool      `json:"dry_run"`
	Archived []gcEntry `json:"archived"`
	Kept     []string  `json:"kept"`
	Skipped  []gcEntry `json:"skipped"`
}

// legacyStageKeys returns the non-canonical numbered keys in a pipeline map.
func legacyStageKeys(pipeline map[string]interface{}) []string {
	var legacy []string
	for k := range pipeline {
		if len(k) > 0 && k[0] >= '0' && k[0] <= '9' && !canonicalStageKeys[k] {
			legacy = append(legacy, k)
		}
	}
	sort.Strings(legacy)
	return legacy
}

// gcReason decides whether a feature folder should be archived: legacy stage
// keys, or no activity (status.json mtime) for more than maxAge. Folders with
// a pending spec delta are never archived — the delta must be archived or
// dropped first, or the living spec loses it.
func gcReason(root, name, dir string, maxAge time.Duration, now time.Time) (archive bool, reason string, skip string) {
	if pending, _ := listFeatureDeltaFilesIn(root, name); len(pending) > 0 {
		return false, "", fmt.Sprintf("%d pending spec delta(s) — archive them first (kratos spec archive %s)", len(pending), name)
	}
	statusFile := filepath.Join(dir, "status.json")
	info, err := os.Stat(statusFile)
	if err != nil {
		// Plan-only folder (no status.json): retire it by age of the folder itself.
		if dinfo, derr := os.Stat(dir); derr == nil && now.Sub(dinfo.ModTime()) > maxAge {
			return true, fmt.Sprintf("plan-only folder untouched for %d days", int(now.Sub(dinfo.ModTime()).Hours()/24)), ""
		}
		return false, "", ""
	}
	if status, rerr := readStatusJSON(statusFile); rerr == nil {
		if pipeline, ok := status["pipeline"].(map[string]interface{}); ok {
			if legacy := legacyStageKeys(pipeline); len(legacy) > 0 {
				return true, "legacy stage keys: " + strings.Join(legacy, ", "), ""
			}
		}
	}
	if age := now.Sub(info.ModTime()); age > maxAge {
		return true, fmt.Sprintf("no activity for %d days", int(age.Hours()/24)), ""
	}
	return false, "", ""
}

// pipelineGCIn evaluates every feature folder under root/.claude/feature and,
// unless dryRun, moves the retirable ones to .claude/feature/_archive/<name>.
func pipelineGCIn(root string, maxAge time.Duration, dryRun bool, now time.Time) (gcResult, error) {
	res := gcResult{Root: root, DryRun: dryRun}
	featureRoot := filepath.Join(root, ".claude", "feature")
	entries, err := os.ReadDir(featureRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, fmt.Errorf("cannot read %s: %w", featureRoot, err)
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(featureRoot, e.Name())
		archive, reason, skip := gcReason(root, e.Name(), dir, maxAge, now)
		switch {
		case skip != "":
			res.Skipped = append(res.Skipped, gcEntry{Feature: e.Name(), Reason: skip})
		case archive:
			if !dryRun {
				dest := filepath.Join(featureRoot, featureArchiveDirName, e.Name())
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return res, err
				}
				if err := os.Rename(dir, dest); err != nil {
					return res, fmt.Errorf("cannot archive %s: %w", e.Name(), err)
				}
			}
			res.Archived = append(res.Archived, gcEntry{Feature: e.Name(), Reason: reason})
		default:
			res.Kept = append(res.Kept, e.Name())
		}
	}
	return res, nil
}

// pipelineGCCmd is `kratos pipeline gc`: retire feature folders that only add
// noise — legacy stage keys the CLI cannot advance, or no activity for --days.
// LizMeter alone had 23 feature folders, many with 10-/11- stage keys, so every
// `pipeline next` asked "which feature?" (2026-09 review).
func pipelineGCCmd() *cobra.Command {
	var days int
	var dryRun, asJSON bool
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Archive stale or legacy feature folders to .claude/feature/_archive/",
		Long: `Move feature folders that the pipeline can no longer use out of the way:
folders whose status.json carries pre-renumbering stage keys (10-prd-alignment,
11-review, 8-code-review, …) and folders with no activity for --days days.
Folders with a pending spec delta are skipped until the delta is archived.
Archived folders are hidden from pipeline next/status; move them back by hand
to revive one.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := gitRoot()
			res, err := pipelineGCIn(root, time.Duration(days)*24*time.Hour, dryRun, time.Now())
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			verb := "archived"
			if dryRun {
				verb = "would archive"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pipeline gc: %s %d, kept %d, skipped %d\n", verb, len(res.Archived), len(res.Kept), len(res.Skipped))
			for _, a := range res.Archived {
				fmt.Fprintf(cmd.OutOrStdout(), "  → %s (%s)\n", a.Feature, a.Reason)
			}
			for _, s := range res.Skipped {
				fmt.Fprintf(cmd.OutOrStdout(), "  ! %s: %s\n", s.Feature, s.Reason)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Archive folders with no status.json activity for this many days")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report what would be archived without moving anything")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	return cmd
}
