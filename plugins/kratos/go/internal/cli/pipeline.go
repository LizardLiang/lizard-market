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

	return cmd
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
				"assignee":  "pm-expert",
				"started":   ts,
				"completed": nil,
				"document":  "prd.md",
			},
			"2-prd-review": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "pm-expert",
				"started":   nil,
				"completed": nil,
				"document":  "prd-review.md",
				"gate": map[string]interface{}{
					"requires":  []string{"1-prd"},
					"condition": "prd.status === 'approved'",
				},
			},
			"2.5-decomposition": map[string]interface{}{
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
			"3-tech-spec": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "tech-spec",
				"started":   nil,
				"completed": nil,
				"document":  "tech-spec.md",
				"gate": map[string]interface{}{
					"requires":  []string{"2-prd-review"},
					"condition": "prd-review.verdict === 'approved'",
				},
			},
			"4-spec-review-pm": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "pm-expert",
				"started":   nil,
				"completed": nil,
				"document":  "spec-review-pm.md",
				"gate": map[string]interface{}{
					"requires":  []string{"3-tech-spec"},
					"condition": "tech-spec.status === 'complete'",
				},
			},
			"5-spec-review-sa": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "sa-expert",
				"started":   nil,
				"completed": nil,
				"document":  "spec-review-sa.md",
				"gate": map[string]interface{}{
					"requires":  []string{"3-tech-spec"},
					"condition": "tech-spec.status === 'complete'",
				},
			},
			"6-test-plan": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "qa-expert",
				"started":   nil,
				"completed": nil,
				"document":  "test-plan.md",
				"gate": map[string]interface{}{
					"requires":  []string{"4-spec-review-pm", "5-spec-review-sa"},
					"condition": "both reviews passed",
				},
			},
			"7-implementation": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "implementer",
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
			"8-code-review": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "code-review",
				"started":   nil,
				"completed": nil,
				"document":  "code-review.md",
				"gate": map[string]interface{}{
					"requires":  []string{"7-implementation"},
					"condition": "implementation complete",
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
	var feature, stage, status, mode, verdict, document string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a pipeline stage status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineUpdate(feature, stage, status, mode, verdict, document)
		},
	}

	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required)")
	cmd.Flags().StringVar(&stage, "stage", "", "Pipeline stage, e.g. 1-prd (required)")
	cmd.Flags().StringVar(&status, "status", "", "New status: in-progress, complete, blocked, ready, skipped (required)")
	cmd.Flags().StringVar(&mode, "mode", "", "Implementation mode: ares or user (stage 7 only)")
	cmd.Flags().StringVar(&verdict, "verdict", "", "Review verdict: approved, revisions, sound, concerns, unsound, changes-requested, rejected")
	cmd.Flags().StringVar(&document, "document", "", "Document path to record")
	cmd.MarkFlagRequired("feature")
	cmd.MarkFlagRequired("stage")
	cmd.MarkFlagRequired("status")

	return cmd
}

func pipelineUpdate(feature, stage, newStatus, mode, verdict, document string) error {
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
