package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get current pipeline status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineGet(feature)
		},
	}

	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required)")
	cmd.MarkFlagRequired("feature")

	return cmd
}

func pipelineGet(feature string) error {
	path := statusPath(feature)

	statusJSON, err := readStatusJSON(path)
	if err != nil {
		return err
	}

	out, _ := json.MarshalIndent(statusJSON, "", "  ")
	fmt.Println(string(out))
	return nil
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

func pipelineDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Find the active feature ready for stage 7 (implementation) and verify prerequisites",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineDiscover()
		},
	}
}

func pipelineDiscover() error {
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
	var all []string

	for _, path := range matches {
		data, err := readStatusJSON(path)
		if err != nil {
			continue
		}

		feature, _ := data["feature"].(string)
		stage, _ := data["stage"].(string)
		all = append(all, fmt.Sprintf("  %s (stage: %s)", feature, stage))

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
		for _, line := range all {
			fmt.Fprintln(os.Stderr, line)
		}
		return fmt.Errorf("no feature at stage 7")
	}

	// If multiple, pick most recently updated
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

	// Check stage 6
	stage6status := "missing"
	stage6ok := false
	if s6, ok := pipeline["6-test-plan"].(map[string]interface{}); ok {
		stage6status, _ = s6["status"].(string)
		stage6ok = stage6status == "complete"
	}

	// Check stage 7
	stage7, _ := pipeline["7-implementation"].(map[string]interface{})
	stage7status, _ := stage7["status"].(string)
	stage7ok := stage7status == "ready" || stage7status == "in-progress"

	// Check prerequisite documents
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
