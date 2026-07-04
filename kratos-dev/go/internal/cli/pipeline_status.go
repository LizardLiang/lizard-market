package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// stageRow is one pipeline stage in a feature report.
type stageRow struct {
	Key      string            `json:"key"`
	Number   int               `json:"number"`
	Status   string            `json:"status"`
	Assignee string            `json:"assignee,omitempty"`
	Document string            `json:"document,omitempty"`
	Verdicts map[string]string `json:"verdicts,omitempty"`
}

// featureReport is the computed dashboard state for one feature. The markdown
// layer renders theming (emoji, boxes) from these numbers.
type featureReport struct {
	Feature     string     `json:"feature"`
	Description string     `json:"description,omitempty"`
	Priority    string     `json:"priority,omitempty"`
	Created     string     `json:"created,omitempty"`
	Updated     string     `json:"updated,omitempty"`
	Stage       string     `json:"stage"`
	StageNumber int        `json:"stage_number"`
	TotalStages int        `json:"total_stages"`
	Completed   int        `json:"completed"`
	Total       int        `json:"total"`
	ProgressPct int        `json:"progress_pct"`
	Health      string     `json:"health"` // blocked | conflict | stale | healthy
	Conflicts   []string   `json:"conflicts,omitempty"`
	Verified    bool       `json:"verified"`
	Structural  bool       `json:"structurally_complete"`
	Stages      []stageRow `json:"stages"`
	Next        nextResult `json:"next"`
}

// statusReport is the full `pipeline status` output.
type statusReport struct {
	Features []featureReport `json:"features"`
	PlanOnly []string        `json:"plan_only,omitempty"`
}

func pipelineStatusCmd() *cobra.Command {
	var asJSON bool
	var staleDays int

	cmd := &cobra.Command{
		Use:   "status [feature]",
		Short: "Dashboard report for one or all features",
		Long: `Computes the status dashboard the /kratos:status command renders: stage N of 9,
completion %, per-stage verdicts, health (blocked > conflict > stale > healthy),
conflict detection, and the next action per the stage-transition table. Feature
dirs without a status.json are listed separately as plan_only.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feature := ""
			if len(args) == 1 {
				feature = args[0]
			}
			return pipelineStatusRun(gitRoot(), feature, staleDays, asJSON, time.Now())
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit machine-readable JSON")
	cmd.Flags().IntVar(&staleDays, "stale-days", 7, "Days without updates before a feature counts as stale")
	return cmd
}

func pipelineStatusRun(root, feature string, staleDays int, asJSON bool, nowT time.Time) error {
	report := statusReport{}

	if feature != "" {
		name, dir, status, err := resolveFeatureIn(root, feature)
		if err != nil {
			return err
		}
		report.Features = append(report.Features, buildFeatureReport(name, dir, status, staleDays, nowT))
	} else {
		featureRoot := filepath.Join(root, ".claude", "feature")
		entries, _ := os.ReadDir(featureRoot)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(featureRoot, entry.Name())
			status, err := readStatusJSON(filepath.Join(dir, "status.json"))
			if err != nil {
				// No parseable status.json → plan-only folder (e.g. a
				// spec-delta/ authored by Odysseus on the quick path).
				report.PlanOnly = append(report.PlanOnly, entry.Name())
				continue
			}
			name, _ := status["feature"].(string)
			if name == "" {
				name = entry.Name()
			}
			report.Features = append(report.Features, buildFeatureReport(name, dir, status, staleDays, nowT))
		}
		sort.Slice(report.Features, func(i, j int) bool {
			return report.Features[i].Updated > report.Features[j].Updated
		})
	}

	if asJSON {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	printStatusReport(report)
	return nil
}

// buildFeatureReport computes the dashboard fields for one feature. Pure over
// the feature dir + parsed status.json + clock, so tests drive it directly.
func buildFeatureReport(name, featureDir string, status map[string]interface{}, staleDays int, nowT time.Time) featureReport {
	r := featureReport{
		Feature:     name,
		TotalStages: len(stageDisplayOrder),
	}
	r.Description, _ = status["description"].(string)
	r.Priority, _ = status["priority"].(string)
	r.Created, _ = status["created"].(string)
	r.Updated, _ = status["updated"].(string)
	r.Stage, _ = status["stage"].(string)
	r.StageNumber = stageNumber(r.Stage)

	r.Completed, r.Total = featureProgress(status)
	if r.Total > 0 {
		r.ProgressPct = r.Completed * 100 / r.Total
	}
	r.Structural = isFeatureComplete(status)
	if r.Structural {
		r.Verified = isFeatureVerified(featureDir, status)
	}

	pipeline, _ := status["pipeline"].(map[string]interface{})
	for _, key := range stageDisplayOrder {
		row := stageRow{Key: key, Number: stageNumber(key), Status: "missing"}
		if stage, ok := pipeline[key].(map[string]interface{}); ok {
			row.Status, _ = stage["status"].(string)
			row.Assignee, _ = stage["assignee"].(string)
			row.Document, _ = stage["document"].(string)
			verdicts := map[string]string{}
			for _, field := range []string{"verdict", "nemesis_verdict", "alignment_verdict", "code_review_verdict", "risk_verdict"} {
				if v, _ := stage[field].(string); v != "" {
					verdicts[field] = v
				}
			}
			if len(verdicts) > 0 {
				row.Verdicts = verdicts
			}
		}
		r.Stages = append(r.Stages, row)
	}

	r.Conflicts = detectConflicts(featureDir, pipeline)
	r.Next = computeNext(name, featureDir, status)

	switch {
	case r.Next.Action == "blocked":
		r.Health = "blocked"
	case len(r.Conflicts) > 0:
		r.Health = "conflict"
	case isStale(r.Updated, staleDays, nowT) && !r.Structural:
		r.Health = "stale"
	default:
		r.Health = "healthy"
	}
	return r
}

// detectConflicts applies the rules from references/status-json-schema.md:
// tech spec written against an older PRD (based_on_prd_version), plus a
// file-mtime check when both documents exist on disk.
func detectConflicts(featureDir string, pipeline map[string]interface{}) []string {
	var conflicts []string

	prd, _ := pipeline["1-prd"].(map[string]interface{})
	spec, _ := pipeline["4-tech-spec"].(map[string]interface{})
	if prd != nil && spec != nil {
		prdCompleted, _ := prd["completed"].(string)
		basedOn, _ := spec["based_on_prd_version"].(string)
		if basedOn != "" && prdCompleted != "" && basedOn < prdCompleted {
			conflicts = append(conflicts, fmt.Sprintf("tech-spec.md based on PRD version %s but prd.md completed %s — tech spec may be outdated", basedOn, prdCompleted))
		}
	}

	prdInfo, prdErr := os.Stat(filepath.Join(featureDir, "prd.md"))
	specInfo, specErr := os.Stat(filepath.Join(featureDir, "tech-spec.md"))
	if prdErr == nil && specErr == nil && prdInfo.ModTime().After(specInfo.ModTime()) {
		conflicts = append(conflicts, "prd.md modified after tech-spec.md — tech spec may be outdated")
	}
	return conflicts
}

func isStale(updated string, staleDays int, nowT time.Time) bool {
	if updated == "" || staleDays <= 0 {
		return false
	}
	t, err := time.Parse(time.RFC3339, updated)
	if err != nil {
		return false
	}
	return nowT.Sub(t) > time.Duration(staleDays)*24*time.Hour
}

func printStatusReport(report statusReport) {
	if len(report.Features) == 0 && len(report.PlanOnly) == 0 {
		fmt.Println("no features found — run 'kratos pipeline init' to start one")
		return
	}

	for i, f := range report.Features {
		if i > 0 {
			fmt.Println()
		}
		doneLabel := ""
		switch {
		case f.Structural && f.Verified:
			doneLabel = "  [done]"
		case f.Structural && !f.Verified:
			doneLabel = "  [complete — verification FAILED, not shippable]"
		}
		fmt.Printf("feature:  %s%s\n", f.Feature, doneLabel)
		if f.Priority != "" {
			fmt.Printf("priority: %s\n", f.Priority)
		}
		fmt.Printf("stage:    %s (%d of %d)\n", f.Stage, f.StageNumber, f.TotalStages)
		fmt.Printf("progress: %d/%d stages complete (%d%%)\n", f.Completed, f.Total, f.ProgressPct)
		fmt.Printf("health:   %s\n", f.Health)
		for _, c := range f.Conflicts {
			fmt.Printf("conflict: %s\n", c)
		}
		fmt.Println("pipeline:")
		for _, s := range f.Stages {
			fmt.Printf("  %-22s %-14s %s\n", s.Key, s.Status, discoverStatusSymbol(s.Status))
		}
		fmt.Printf("next:     action=%s", f.Next.Action)
		if f.Next.Next != nil && f.Next.Next.Stage != "" {
			fmt.Printf(" stage=%s procedure=%s", f.Next.Next.Stage, f.Next.Next.Procedure)
		}
		fmt.Println()
	}

	if len(report.PlanOnly) > 0 {
		fmt.Println("\nplan-only folders (no status.json — pending spec delta):")
		for _, p := range report.PlanOnly {
			fmt.Printf("  %s\n", p)
		}
	}
}
