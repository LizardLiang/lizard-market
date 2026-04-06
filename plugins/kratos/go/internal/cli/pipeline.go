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
	cmd.AddCommand(pipelineSetPendingCmd())
	cmd.AddCommand(pipelineSetTasksCmd())
	cmd.AddCommand(pipelineTaskDoneCmd())

	return cmd
}

// NowCmd returns the 'now' command that prints the current ISO8601 timestamp.
func NowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "now",
		Short: "Print the current ISO8601 timestamp (RFC3339)",
		Long:  "Prints the current time as an RFC3339 / ISO8601 timestamp.",
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
			"4-discuss": map[string]interface{}{
				"status":    "skipped",
				"assignee":  "themis",
				"started":   nil,
				"completed": nil,
				"document":  "context.md",
				"optional":  true,
				"gate": map[string]interface{}{
					"requires":  []string{"2-prd-review"},
					"condition": "prd-review.verdict === 'approved' AND user opts in",
				},
			},
			"5-tech-spec": map[string]interface{}{
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
			"7-spec-review-sa": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "apollo",
				"started":   nil,
				"completed": nil,
				"document":  "spec-review-sa.md",
				"gate": map[string]interface{}{
					"requires":  []string{"5-tech-spec"},
					"condition": "tech-spec.status === 'complete'",
				},
			},
			"8-test-plan": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "artemis",
				"started":   nil,
				"completed": nil,
				"document":  "test-plan.md",
				"gate": map[string]interface{}{
					"requires":  []string{"7-spec-review-sa"},
					"condition": "both reviews passed",
				},
			},
			"9-implementation": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "ares",
				"started":   nil,
				"completed": nil,
				"document":  "implementation-notes.md",
				"mode":      nil,
				"tasks":     nil,
				"gate": map[string]interface{}{
					"requires":  []string{"8-test-plan"},
					"condition": "test-plan exists",
				},
			},
			"10-prd-alignment": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "hera",
				"started":   nil,
				"completed": nil,
				"document":  "prd-alignment.md",
				"gate": map[string]interface{}{
					"requires":  []string{"9-implementation"},
					"condition": "implementation complete",
				},
			},
			"11-review": map[string]interface{}{
				"status":    "blocked",
				"assignee":  "hermes",
				"started":   nil,
				"completed": nil,
				"document":  "code-review.md",
				"gate": map[string]interface{}{
					"requires":  []string{"10-prd-alignment"},
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
	cmd.Flags().StringVar(&stage, "stage", "", "Pipeline stage, e.g. 1-prd (required)")
	cmd.Flags().StringVar(&status, "status", "", "New status: in-progress, complete, blocked, ready, skipped (required)")
	cmd.Flags().StringVar(&mode, "mode", "", "Implementation mode: ares or user (stage 9 only)")
	cmd.Flags().StringVar(&verdict, "verdict", "", "Review verdict: approved, revisions, sound, concerns, unsound, changes-requested, rejected")
	cmd.Flags().StringVar(&document, "document", "", "Document path to record")
	cmd.Flags().StringVar(&summary, "summary", "", "2-3 sentence summary of work done (written to stage summary field)")
	cmd.MarkFlagRequired("feature")
	cmd.MarkFlagRequired("stage")
	cmd.MarkFlagRequired("status")

	return cmd
}

func pipelineUpdate(feature, stage, newStatus, mode, verdict, document, summary string) error {
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

func pipelineSetPending(feature, stage string) error {
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

// --- pipeline set-tasks ---

func pipelineSetTasksCmd() *cobra.Command {
	var feature, tasksJSON string

	cmd := &cobra.Command{
		Use:   "set-tasks",
		Short: "Initialize the tasks array for stage 9 (User Mode)",
		Long: `Set the tasks array on the 9-implementation stage. Call this after
'pipeline update --stage 9-implementation --status in-progress --mode user'
to register the task list. Pass tasks as a JSON array of objects with
id, name, and file fields.

Example:
  kratos pipeline set-tasks --feature my-feature \
    --tasks '[{"id":"01","name":"Create model","file":"01-create-model.md"},{"id":"02","name":"Add migrations","file":"02-migrations.md"}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineSetTasks(feature, tasksJSON)
		},
	}

	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required)")
	cmd.Flags().StringVar(&tasksJSON, "tasks", "", "JSON array of task objects: [{\"id\":\"01\",\"name\":\"...\",\"file\":\"01-name.md\"}] (required)")
	cmd.MarkFlagRequired("feature")
	cmd.MarkFlagRequired("tasks")

	return cmd
}

func pipelineSetTasks(feature, tasksJSON string) error {
	path := statusPath(feature)

	statusData, err := readStatusJSON(path)
	if err != nil {
		return err
	}

	// Parse tasks JSON
	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(tasksJSON), &items); err != nil {
		return fmt.Errorf("invalid tasks JSON: %w", err)
	}

	// Add status: pending to each item if missing
	for _, item := range items {
		if _, ok := item["status"]; !ok {
			item["status"] = "pending"
		}
	}

	// Navigate to stage
	pipeline, ok := statusData["pipeline"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid pipeline structure in status.json")
	}
	stageData, ok := pipeline["9-implementation"]
	if !ok {
		return fmt.Errorf("stage 9-implementation not found in pipeline")
	}
	stageMap, ok := stageData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid stage data for 9-implementation")
	}

	// Build tasks object
	itemsInterface := make([]interface{}, len(items))
	for i, item := range items {
		itemsInterface[i] = item
	}
	stageMap["tasks"] = map[string]interface{}{
		"total":     len(items),
		"completed": 0,
		"items":     itemsInterface,
	}

	statusData["updated"] = now()

	if err := writeStatusJSON(path, statusData); err != nil {
		return err
	}

	out, _ := json.MarshalIndent(statusData, "", "  ")
	fmt.Println(string(out))
	return nil
}

// --- pipeline task-done ---

func pipelineTaskDoneCmd() *cobra.Command {
	var feature, taskID string

	cmd := &cobra.Command{
		Use:   "task-done",
		Short: "Mark a User Mode implementation task as complete",
		Long: `Mark a single task as complete in the 9-implementation tasks array.
Updates the item's status to 'complete' and increments the completed counter.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineTaskDone(feature, taskID)
		},
	}

	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required)")
	cmd.Flags().StringVar(&taskID, "task-id", "", "Task ID to mark complete, e.g. 01 (required)")
	cmd.MarkFlagRequired("feature")
	cmd.MarkFlagRequired("task-id")

	return cmd
}

func pipelineTaskDone(feature, taskID string) error {
	path := statusPath(feature)

	statusData, err := readStatusJSON(path)
	if err != nil {
		return err
	}

	pipeline, ok := statusData["pipeline"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid pipeline structure in status.json")
	}
	stageData, ok := pipeline["9-implementation"]
	if !ok {
		return fmt.Errorf("stage 9-implementation not found")
	}
	stageMap, ok := stageData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid stage data for 9-implementation")
	}

	tasksData, ok := stageMap["tasks"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no tasks array found on 9-implementation — run 'pipeline set-tasks' first")
	}

	items, ok := tasksData["items"].([]interface{})
	if !ok {
		return fmt.Errorf("tasks.items is not an array")
	}

	found := false
	completed := 0
	for _, itemRaw := range items {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := item["id"].(string)
		if id == taskID {
			item["status"] = "complete"
			found = true
		}
		if s, _ := item["status"].(string); s == "complete" {
			completed++
		}
	}

	if !found {
		return fmt.Errorf("task ID %q not found in tasks array", taskID)
	}

	tasksData["completed"] = completed
	statusData["updated"] = now()

	if err := writeStatusJSON(path, statusData); err != nil {
		return err
	}

	total, _ := tasksData["total"].(int)
	if total == 0 {
		// total may be float64 from JSON round-trip
		if tf, ok := tasksData["total"].(float64); ok {
			total = int(tf)
		}
	}

	fmt.Printf(`{"feature":%q,"task_id":%q,"status":"complete","completed":%d,"total":%d}`+"\n",
		feature, taskID, completed, total)
	return nil
}
