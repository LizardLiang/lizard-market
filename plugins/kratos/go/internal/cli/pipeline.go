package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// stageNumberToKey maps the numeric stage index to its full pipeline key.
// --stage accepts only numbers 1-9.
var stageNumberToKey = map[string]string{
	"1": "1-prd",
	"2": "2-prd-review",
	"3": "3-decomposition",
	"4": "4-tech-spec",
	"5": "5-spec-review-sa",
	"6": "6-test-plan",
	"7": "7-implementation",
	"8": "8-prd-alignment",
	"9": "9-review",
}

// PipelineCmd returns the 'pipeline' command group
func PipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage feature pipeline status.json",
		Long:  "Commands for initializing and updating feature pipeline status with real timestamps",
	}

	cmd.AddCommand(pipelineInitCmd())
	cmd.AddCommand(pipelineUpdateCmd())
	cmd.AddCommand(pipelineGetCmd())
	cmd.AddCommand(pipelineSetPendingCmd())
	cmd.AddCommand(pipelineDiscoverCmd())

	return cmd
}

// NowCmd returns the 'now' command that prints the current ISO8601 timestamp.
// Agents use this to get a precise timestamp for manual status.json edits.
func NowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "now",
		Short: "Print the current ISO8601 timestamp (RFC3339)",
		Long:  "Prints the current time as an RFC3339 / ISO8601 timestamp. Use this whenever you need to write a timestamp into status.json manually.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(now())
		},
	}
}

// gitRoot returns the git repository root directory, falling back to cwd
func gitRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return strings.TrimSpace(string(out))
}

// statusPath resolves the status.json path for a feature
func statusPath(feature string) string {
	return filepath.Join(gitRoot(), ".claude", "feature", feature, "status.json")
}

// readStatusJSON reads and parses a status.json file
func readStatusJSON(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var status map[string]interface{}
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return status, nil
}

// writeStatusJSON atomically writes a status.json file
func writeStatusJSON(path string, status map[string]interface{}) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal JSON: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("cannot write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("cannot rename temp file: %w", err)
	}
	return nil
}

// now returns the current time in RFC3339 format
func now() string {
	return time.Now().Format(time.RFC3339)
}

// --- pipeline init ---

func pipelineInitCmd() *cobra.Command {
	var feature, description, priority string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new feature pipeline status.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineInit(feature, description, priority)
		},
	}

	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Feature description (required)")
	cmd.Flags().StringVar(&priority, "priority", "P2", "Priority: P0, P1, P2, P3")
	cmd.MarkFlagRequired("feature")
	cmd.MarkFlagRequired("description")

	return cmd
}

func pipelineInit(feature, description, priority string) error {
	path := statusPath(feature)

	// Check if already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("status.json already exists at %s", path)
	}

	ts := now()

	status := map[string]interface{}{
		"feature":     feature,
		"description": description,
		"priority":    priority,
		"created":     ts,
		"updated":     ts,
		"stage":       "1-prd",
		"pipeline": map[string]interface{}{
			"1-prd": map[string]interface{}{
				"status":    "in-progress",
				"assignee":  "athena",
				"started":   ts,
				"completed": nil,
				"document":  "prd.md",
			},
			"2-prd-review": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "nemesis",
				"started":   nil,
				"completed": nil,
				"document":  "prd-challenge.md",
				"gate": map[string]interface{}{
					"requires":  []string{"1-prd"},
					"condition": "prd.status === 'complete'",
				},
			},
			"3-decomposition": map[string]interface{}{
				"status":    "skipped",
				"assignee":  "daedalus",
				"started":   nil,
				"completed": nil,
				"document":  "decomposition.md",
				"optional":  true,
				"gate": map[string]interface{}{
					"requires":  []string{"2-prd-review"},
					"condition": "prd-review.verdict === 'approved' AND user opts in",
				},
			},
			"4-tech-spec": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "hephaestus",
				"started":   nil,
				"completed": nil,
				"document":  "tech-spec.md",
				"gate": map[string]interface{}{
					"requires":  []string{"2-prd-review"},
					"condition": "prd-review.verdict === 'approved'",
				},
			},
			"5-spec-review-sa": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "apollo",
				"started":   nil,
				"completed": nil,
				"document":  "spec-review-sa.md",
				"gate": map[string]interface{}{
					"requires":  []string{"4-tech-spec"},
					"condition": "tech-spec.status === 'complete'",
				},
			},
			"6-test-plan": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "artemis",
				"started":   nil,
				"completed": nil,
				"document":  "test-plan.md",
				"gate": map[string]interface{}{
					"requires":  []string{"5-spec-review-sa"},
					"condition": "review passed",
				},
			},
			"7-implementation": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "ares",
				"started":   nil,
				"completed": nil,
				"document":  "implementation-notes.md",
				"mode":      nil,
				"tasks":     nil,
				"gate": map[string]interface{}{
					"requires":  []string{"6-test-plan"},
					"condition": "test-plan exists",
				},
			},
			"8-prd-alignment": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "hera",
				"started":   nil,
				"completed": nil,
				"document":  "prd-alignment.md",
				"gate": map[string]interface{}{
					"requires":  []string{"7-implementation"},
					"condition": "implementation complete",
				},
			},
			"9-review": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "hermes",
				"started":   nil,
				"completed": nil,
				"document":  "code-review.md",
				"gate": map[string]interface{}{
					"requires":  []string{"8-prd-alignment"},
					"condition": "prd-alignment verdict === 'aligned'",
				},
			},
		},
		"documents": map[string]interface{}{},
		"history":   []interface{}{},
	}

	if err := writeStatusJSON(path, status); err != nil {
		return err
	}

	// Output result as JSON
	out, _ := json.MarshalIndent(status, "", "  ")
	fmt.Println(string(out))
	return nil
}

// --- pipeline update ---

func pipelineUpdateCmd() *cobra.Command {
	var feature, stage, status, mode, verdict, document, summary string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a pipeline stage status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineUpdate(feature, stage, status, mode, verdict, document, summary)
		},
	}

	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required)")
	cmd.Flags().StringVar(&stage, "stage", "", "Pipeline stage number 1-9 (required)")
	cmd.Flags().StringVar(&status, "status", "", "New status: in-progress, complete, blocked, ready, skipped (required)")
	cmd.Flags().StringVar(&mode, "mode", "", "Implementation mode: ares or user (stage 8 only)")
	cmd.Flags().StringVar(&verdict, "verdict", "", "Review verdict: approved, revisions, sound, concerns, unsound, changes-requested, rejected")
	cmd.Flags().StringVar(&document, "document", "", "Document path to record")
	cmd.Flags().StringVar(&summary, "summary", "", "2-3 sentence summary for downstream agents")
	cmd.MarkFlagRequired("feature")
	cmd.MarkFlagRequired("stage")
	cmd.MarkFlagRequired("status")

	return cmd
}

func pipelineUpdate(feature, stage, newStatus, mode, verdict, document, summary string) error {
	resolved, ok := stageNumberToKey[stage]
	if !ok {
		return fmt.Errorf("invalid stage %q: must be a number 1-9", stage)
	}
	stage = resolved

	path := statusPath(feature)

	statusJSON, err := readStatusJSON(path)
	if err != nil {
		return err
	}

	ts := now()

	// Get pipeline map
	pipeline, ok := statusJSON["pipeline"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid pipeline structure in status.json")
	}

	// Get stage map
	stageData, ok := pipeline[stage]
	if !ok {
		return fmt.Errorf("unknown stage: %s", stage)
	}
	stageMap, ok := stageData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid stage data for: %s", stage)
	}

	// Update status
	oldStatus, _ := stageMap["status"].(string)
	stageMap["status"] = newStatus

	// Auto-set timestamps
	if newStatus == "in-progress" && oldStatus != "in-progress" {
		stageMap["started"] = ts
	}
	if newStatus == "complete" {
		stageMap["completed"] = ts
		if stageMap["started"] == nil {
			stageMap["started"] = ts
		}
	}

	// Optional fields
	if mode != "" {
		stageMap["mode"] = mode
	}
	if verdict != "" {
		stageMap["verdict"] = verdict
	}
	if document != "" {
		stageMap["document"] = document
	}
	if summary != "" {
		stageMap["summary"] = summary
	}

	// Update top-level fields
	statusJSON["updated"] = ts
	if newStatus == "in-progress" || newStatus == "complete" {
		statusJSON["stage"] = stage
	}

	// Record in history
	historyEntry := map[string]interface{}{
		"timestamp": ts,
		"stage":     stage,
		"action":    fmt.Sprintf("status changed from '%s' to '%s'", oldStatus, newStatus),
	}
	if verdict != "" {
		historyEntry["verdict"] = verdict
	}

	history, _ := statusJSON["history"].([]interface{})
	statusJSON["history"] = append(history, historyEntry)

	// Record document in documents map
	if document != "" {
		docs, _ := statusJSON["documents"].(map[string]interface{})
		if docs == nil {
			docs = map[string]interface{}{}
		}
		docs[stage] = document
		statusJSON["documents"] = docs
	}

	if err := writeStatusJSON(path, statusJSON); err != nil {
		return err
	}

	// Output result
	out, _ := json.MarshalIndent(statusJSON, "", "  ")
	fmt.Println(string(out))
	return nil
}

// --- pipeline get ---

func pipelineGetCmd() *cobra.Command {
	var feature string
	var compact bool

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get current pipeline status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineGet(feature, compact)
		},
	}

	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required)")
	cmd.Flags().BoolVar(&compact, "compact", false, "Omit the audit-only history[] and per-stage check_failures[] to save tokens (agents should prefer this)")
	cmd.MarkFlagRequired("feature")

	return cmd
}

func pipelineGet(feature string, compact bool) error {
	path := statusPath(feature)

	statusJSON, err := readStatusJSON(path)
	if err != nil {
		return err
	}

	if compact {
		statusJSON = compactStatus(statusJSON)
	}

	out, _ := json.MarshalIndent(statusJSON, "", "  ")
	fmt.Println(string(out))
	return nil
}

// compactStatus returns a projection of status.json for downstream agents that
// need current state, not the audit trail. It drops the monotonically-growing
// top-level history[] and each stage's append-only check_failures[] — the two
// fields that make `pipeline get` output balloon over a feature's life. All
// decision-relevant fields (status, verdicts, summary, document, mode, timing)
// are preserved. The underlying status.json file is never modified.
func compactStatus(status map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(status))
	for k, v := range status {
		if k == "history" {
			continue
		}
		out[k] = v
	}

	pipeline, ok := status["pipeline"].(map[string]interface{})
	if !ok {
		return out
	}
	trimmedPipeline := make(map[string]interface{}, len(pipeline))
	for stageKey, stageVal := range pipeline {
		stageMap, ok := stageVal.(map[string]interface{})
		if !ok {
			trimmedPipeline[stageKey] = stageVal
			continue
		}
		trimmedStage := make(map[string]interface{}, len(stageMap))
		for k, v := range stageMap {
			if k == "check_failures" {
				continue
			}
			trimmedStage[k] = v
		}
		trimmedPipeline[stageKey] = trimmedStage
	}
	out["pipeline"] = trimmedPipeline
	return out
}

// pipelineSetPendingCmd sets pending_stage in status.json so that kratos check --init
// knows which stage Athena is about to run — before Athena has updated "stage" itself.
func pipelineSetPendingCmd() *cobra.Command {
	var feature, stage string

	cmd := &cobra.Command{
		Use:   "set-pending",
		Short: "Set pending_stage before spawning a multi-stage agent (e.g. Athena)",
		Long: `Set the pending_stage field in status.json before spawning an agent that runs at
multiple stages (like Athena). The kratos check --init hook reads this field first so it
can inject the correct deliverable expectations at SubagentStart time.

Clear it by passing --stage "" after the agent completes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineSetPending(feature, stage)
		},
	}

	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required)")
	cmd.Flags().StringVar(&stage, "stage", "", "Stage to set as pending (e.g. 2-prd-review); empty string clears it")
	cmd.MarkFlagRequired("feature")

	return cmd
}

// --- pipeline discover ---

// stageDisplayOrder defines the canonical display order for pipeline stages.
var stageDisplayOrder = []string{
	"1-prd", "2-prd-review", "3-decomposition", "4-tech-spec",
	"5-spec-review-sa", "6-test-plan", "7-implementation",
	"8-prd-alignment", "9-review",
}

// nonOptionalStages lists the stages that count toward feature completeness.
// 3-decomposition is optional and excluded.
var nonOptionalStages = []string{
	"1-prd", "2-prd-review", "4-tech-spec", "5-spec-review-sa",
	"6-test-plan", "7-implementation", "8-prd-alignment", "9-review",
}

func pipelineDiscoverCmd() *cobra.Command {
	var all, verify bool

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "List features and their pipeline status",
		Long: `List features with their pipeline stage status.

By default, shows only incomplete features (at least one non-optional stage not complete).
Use --all to include features where all stages are complete.
Use --verify to find the stage-7-ready feature and verify its prerequisites (Ares workflow).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && verify {
				return fmt.Errorf("--all and --verify are mutually exclusive")
			}
			if verify {
				return pipelineDiscoverVerify()
			}
			return pipelineDiscoverList(all)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Include completed features")
	cmd.Flags().BoolVar(&verify, "verify", false, "Find stage-7-ready feature and verify prerequisites (Ares workflow)")
	return cmd
}

// pipelineDiscoverList lists features sorted by updated desc.
// When all is false, only incomplete features are shown.
func pipelineDiscoverList(all bool) error {
	root := gitRoot()
	pattern := filepath.Join(root, ".claude", "feature", "*", "status.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob error: %w", err)
	}

	type entry struct {
		feature    string
		updated    string
		structural bool
		verified   bool
		data       map[string]interface{}
	}

	var entries []entry
	for _, path := range matches {
		data, err := readStatusJSON(path)
		if err != nil {
			continue
		}
		feature, _ := data["feature"].(string)
		updated, _ := data["updated"].(string)
		// A feature is only "done" when every stage is complete AND every
		// review passed. Verdict verification (file-based) runs only once the
		// structural check passes, so a failed review keeps the feature visible.
		structural := isFeatureComplete(data)
		verified := false
		if structural {
			verified = isFeatureVerified(filepath.Dir(path), data)
		}
		if !all && structural && verified {
			continue
		}
		entries = append(entries, entry{feature, updated, structural, verified, data})
	}

	if len(entries) == 0 {
		if all {
			fmt.Println("no features found — run 'kratos pipeline init' to start one")
		} else {
			fmt.Println("no incomplete features found — run 'kratos pipeline init' to start one")
		}
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].updated > entries[j].updated
	})

	for i, e := range entries {
		if i > 0 {
			fmt.Println()
		}
		printFeatureStatus(e.feature, e.structural, e.verified, e.data)
	}
	return nil
}

// pipelineDiscoverVerify finds the stage-7-ready feature and verifies prerequisites.
// This is the original discover behavior, preserved for backward compatibility (Ares).
func pipelineDiscoverVerify() error {
	root := gitRoot()
	pattern := filepath.Join(root, ".claude", "feature", "*", "status.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob error: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no features found — run 'kratos pipeline init' first")
	}

	type candidate struct {
		feature string
		updated string
		data    map[string]interface{}
	}

	var ready []candidate
	var allFeatures []string

	for _, path := range matches {
		data, err := readStatusJSON(path)
		if err != nil {
			continue
		}

		feature, _ := data["feature"].(string)
		stage, _ := data["stage"].(string)
		allFeatures = append(allFeatures, fmt.Sprintf("  %s (stage: %s)", feature, stage))

		pipeline, ok := data["pipeline"].(map[string]interface{})
		if !ok {
			continue
		}
		stage7, ok := pipeline["7-implementation"].(map[string]interface{})
		if !ok {
			continue
		}
		s7status, _ := stage7["status"].(string)
		if s7status != "ready" && s7status != "in-progress" {
			continue
		}

		updated, _ := data["updated"].(string)
		ready = append(ready, candidate{feature: feature, updated: updated, data: data})
	}

	if len(ready) == 0 {
		fmt.Fprintf(os.Stderr, "no feature ready for stage 7-implementation\n\navailable features:\n")
		for _, line := range allFeatures {
			fmt.Fprintln(os.Stderr, line)
		}
		return fmt.Errorf("no feature at stage 7")
	}

	if len(ready) > 1 {
		latest := ready[0]
		for _, c := range ready[1:] {
			if c.updated > latest.updated {
				latest = c
			}
		}
		fmt.Fprintf(os.Stderr, "warning: %d features ready for stage 7, using most recently updated: %s\n", len(ready), latest.feature)
		ready = []candidate{latest}
	}

	c := ready[0]
	featureDir := filepath.Join(root, ".claude", "feature", c.feature)
	pipeline, _ := c.data["pipeline"].(map[string]interface{})

	stage6status := "missing"
	stage6ok := false
	if s6, ok := pipeline["6-test-plan"].(map[string]interface{}); ok {
		stage6status, _ = s6["status"].(string)
		stage6ok = stage6status == "complete"
	}

	stage7, _ := pipeline["7-implementation"].(map[string]interface{})
	stage7status, _ := stage7["status"].(string)
	stage7ok := stage7status == "ready" || stage7status == "in-progress"

	techSpecPath := filepath.Join(featureDir, "tech-spec.md")
	testPlanPath := filepath.Join(featureDir, "test-plan.md")
	techSpecOk := discoverFileExists(techSpecPath)
	testPlanOk := discoverFileExists(testPlanPath)

	fmt.Printf("feature:          %s\n", c.feature)
	fmt.Printf("6-test-plan:      %s %s\n", stage6status, discoverMark(stage6ok))
	fmt.Printf("7-implementation: %s %s\n", stage7status, discoverMark(stage7ok))
	fmt.Printf("tech-spec.md:     %s\n", discoverPresence(techSpecOk, techSpecPath))
	fmt.Printf("test-plan.md:     %s\n", discoverPresence(testPlanOk, testPlanPath))

	if !stage6ok || !techSpecOk || !testPlanOk {
		return fmt.Errorf("prerequisites not satisfied")
	}
	return nil
}

// isFeatureComplete returns true when all non-optional stages are complete.
func isFeatureComplete(data map[string]interface{}) bool {
	pipeline, ok := data["pipeline"].(map[string]interface{})
	if !ok {
		return false
	}
	for _, key := range nonOptionalStages {
		stage, ok := pipeline[key].(map[string]interface{})
		if !ok {
			return false
		}
		status, _ := stage["status"].(string)
		if status != "complete" {
			return false
		}
	}
	return true
}

// featureProgress returns (completedCount, totalNonOptional).
func featureProgress(data map[string]interface{}) (int, int) {
	pipeline, _ := data["pipeline"].(map[string]interface{})
	total := len(nonOptionalStages)
	done := 0
	for _, key := range nonOptionalStages {
		stage, ok := pipeline[key].(map[string]interface{})
		if !ok {
			continue
		}
		if s, _ := stage["status"].(string); s == "complete" {
			done++
		}
	}
	return done, total
}

// discoverStatusSymbol maps a pipeline stage status to its display symbol.
func discoverStatusSymbol(status string) string {
	switch status {
	case "complete":
		return "✓"
	case "in-progress":
		return "⋯"
	case "skipped":
		return "-"
	default:
		return "✗"
	}
}

// printFeatureStatus writes one feature block to stdout. structural means every
// non-optional stage is complete; verified means every review passed the ship
// gate. A feature that is structurally complete but failed a review is flagged
// so it is never mistaken for shippable.
func printFeatureStatus(feature string, structural, verified bool, data map[string]interface{}) {
	doneLabel := ""
	switch {
	case structural && verified:
		doneLabel = "  [done]"
	case structural && !verified:
		doneLabel = "  [complete — verification FAILED, not shippable]"
	}
	fmt.Printf("feature:  %s%s\n", feature, doneLabel)

	currentStage, _ := data["stage"].(string)
	pipeline, _ := data["pipeline"].(map[string]interface{})

	currentStatus := ""
	if stage, ok := pipeline[currentStage].(map[string]interface{}); ok {
		currentStatus, _ = stage["status"].(string)
	}
	fmt.Printf("stage:    %-20s (%s)\n", currentStage, currentStatus)

	done, total := featureProgress(data)
	fmt.Printf("progress: %d/%d stages complete\n", done, total)

	fmt.Println("pipeline:")
	for _, key := range stageDisplayOrder {
		stage, ok := pipeline[key].(map[string]interface{})
		status := "missing"
		if ok {
			status, _ = stage["status"].(string)
		}
		symbol := discoverStatusSymbol(status)
		fmt.Printf("  %-22s %-14s %s\n", key, status, symbol)
	}
}

func discoverFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func discoverMark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

func discoverPresence(ok bool, path string) string {
	if ok {
		return fmt.Sprintf("present ✓  (%s)", path)
	}
	return fmt.Sprintf("missing ✗  (%s)", path)
}

// --- pipeline set-pending ---

func pipelineSetPending(feature, stage string) error {
	if stage != "" {
		resolved, ok := stageNumberToKey[stage]
		if !ok {
			return fmt.Errorf("invalid stage %q: must be a number 1-9", stage)
		}
		stage = resolved
	}

	path := statusPath(feature)

	statusJSON, err := readStatusJSON(path)
	if err != nil {
		return err
	}

	if stage == "" {
		delete(statusJSON, "pending_stage")
	} else {
		statusJSON["pending_stage"] = stage
	}
	statusJSON["updated"] = time.Now().Format(time.RFC3339)

	if err := writeStatusJSON(path, statusJSON); err != nil {
		return err
	}

	if stage == "" {
		fmt.Printf(`{"feature":%q,"pending_stage_cleared":true}`+"\n", feature)
	} else {
		fmt.Printf(`{"feature":%q,"pending_stage":%q}`+"\n", feature, stage)
	}
	return nil
}
