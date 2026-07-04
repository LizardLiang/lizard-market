package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// taskItem is one entry of pipeline["7-implementation"].tasks.items.
type taskItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// tasksResult is the machine-readable outcome of `pipeline tasks complete`.
type tasksResult struct {
	Feature         string     `json:"feature"`
	CompletedNow    []taskItem `json:"completed_now"`
	AlreadyComplete []string   `json:"already_complete"`
	Remaining       []taskItem `json:"remaining"`
	Total           int        `json:"total"`
	Completed       int        `json:"completed"`
	Pct             int        `json:"pct"`
	Bar             string     `json:"bar"`
	AllComplete     bool       `json:"all_complete"`
	Advanced        bool       `json:"advanced"`
}

// tasksListResult is the machine-readable outcome of `pipeline tasks list`.
type tasksListResult struct {
	Feature   string         `json:"feature"`
	Items     []taskItemFull `json:"items"`
	Total     int            `json:"total"`
	Completed int            `json:"completed"`
	Pct       int            `json:"pct"`
	Bar       string         `json:"bar"`
}

type taskItemFull struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	File   string `json:"file,omitempty"`
	Status string `json:"status"`
}

func pipelineTasksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Manage stage-7 implementation tasks in status.json (User Mode)",
	}
	cmd.AddCommand(pipelineTasksListCmd())
	cmd.AddCommand(pipelineTasksCompleteCmd())
	return cmd
}

func pipelineTasksListCmd() *cobra.Command {
	var feature string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List implementation tasks with progress",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineTasksList(gitRoot(), feature, asJSON)
		},
	}
	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (auto-detected when omitted)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

func pipelineTasksCompleteCmd() *cobra.Command {
	var feature string
	var asJSON, all, noAdvance bool

	cmd := &cobra.Command{
		Use:   "complete <task-id>... | --all",
		Short: "Mark implementation tasks complete (User Mode only)",
		Long: `Mark one or more stage-7 tasks complete in status.json. Validates the whole
batch before writing (unknown ID fails everything, file untouched). Already-complete
IDs are idempotent no-ops. When the last task completes, stage 7 is marked complete
and stage 8 set to ready in the same atomic write (disable with --no-advance).
The literal argument "all" is equivalent to --all.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && strings.EqualFold(args[0], "all") {
				all = true
				args = nil
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("provide task IDs or --all")
			}
			return pipelineTasksComplete(gitRoot(), feature, args, all, noAdvance, asJSON)
		},
	}
	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (auto-detected when omitted)")
	cmd.Flags().BoolVar(&all, "all", false, "Complete every remaining task")
	cmd.Flags().BoolVar(&noAdvance, "no-advance", false, "Do not auto-advance stages when all tasks complete")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

// resolveTaskFeature picks the feature whose stage 7 is active. With an
// explicit name it just loads it. Otherwise exactly one feature with
// 7-implementation in ready/in-progress must exist.
func resolveTaskFeature(root, explicit string) (string, string, map[string]interface{}, error) {
	if explicit != "" {
		return resolveFeatureIn(root, explicit)
	}

	pattern := filepath.Join(root, ".claude", "feature", "*", "status.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", "", nil, fmt.Errorf("glob error: %w", err)
	}

	type candidate struct {
		name, dir string
		data      map[string]interface{}
	}
	var candidates []candidate
	for _, path := range matches {
		data, err := readStatusJSON(path)
		if err != nil {
			continue
		}
		pipeline, _ := data["pipeline"].(map[string]interface{})
		s7, _ := pipeline["7-implementation"].(map[string]interface{})
		s7status, _ := s7["status"].(string)
		if s7status != "ready" && s7status != "in-progress" {
			continue
		}
		name, _ := data["feature"].(string)
		if name == "" {
			name = filepath.Base(filepath.Dir(path))
		}
		candidates = append(candidates, candidate{name, filepath.Dir(path), data})
	}

	switch len(candidates) {
	case 0:
		return "", "", nil, fmt.Errorf("no feature with an active 7-implementation stage — pass --feature")
	case 1:
		return candidates[0].name, candidates[0].dir, candidates[0].data, nil
	}
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.name
	}
	sort.Strings(names)
	return "", "", nil, &ambiguousFeatureError{Candidates: names}
}

// stage7Tasks extracts the task items from stage 7, accepting both container
// shapes seen in the wild: the documented object {total, completed, items[]}
// and a legacy bare array. Returns the items and the stage map.
func stage7Tasks(status map[string]interface{}) ([]map[string]interface{}, map[string]interface{}, error) {
	pipeline, ok := status["pipeline"].(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("invalid pipeline structure in status.json")
	}
	s7, ok := pipeline["7-implementation"].(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("no 7-implementation stage in status.json")
	}

	var rawItems []interface{}
	switch tasks := s7["tasks"].(type) {
	case map[string]interface{}:
		rawItems, _ = tasks["items"].([]interface{})
	case []interface{}:
		rawItems = tasks
	case nil:
		return nil, nil, fmt.Errorf("stage 7 has no tasks — was the feature decomposed for User Mode?")
	default:
		return nil, nil, fmt.Errorf("unrecognized tasks shape in status.json")
	}

	items := make([]map[string]interface{}, 0, len(rawItems))
	for _, raw := range rawItems {
		if m, ok := raw.(map[string]interface{}); ok {
			items = append(items, m)
		}
	}
	if len(items) == 0 {
		return nil, nil, fmt.Errorf("stage 7 task list is empty")
	}
	return items, s7, nil
}

// validateUserMode enforces the /kratos:task-complete preconditions: stage 7
// active and mode == "user". Error text carries the current stage/mode so the
// orchestrator can render the documented error blocks.
func validateUserMode(status map[string]interface{}, s7 map[string]interface{}) error {
	s7status, _ := s7["status"].(string)
	if s7status != "in-progress" && s7status != "ready" {
		current, _ := status["stage"].(string)
		return fmt.Errorf("wrong stage: current stage is %s (7-implementation is %q) — tasks can only be completed while 7-implementation is active", current, s7status)
	}
	mode, _ := s7["mode"].(string)
	if mode != "user" {
		return fmt.Errorf("not in user mode: 7-implementation mode is %q — /kratos:task-complete is only available in User Mode", mode)
	}
	return nil
}

// progressBar renders the documented 20-char block bar: [██████░░░░░░░░░░░░░░] 30% (3/10 tasks)
func progressBar(completed, total int) (pct int, bar string) {
	if total > 0 {
		pct = completed * 100 / total
	}
	filled := 0
	if total > 0 {
		filled = completed * 20 / total
	}
	bar = fmt.Sprintf("[%s%s] %d%% (%d/%d tasks)",
		strings.Repeat("█", filled), strings.Repeat("░", 20-filled), pct, completed, total)
	return pct, bar
}

func itemDone(item map[string]interface{}) bool {
	s, _ := item["status"].(string)
	return s == "complete"
}

func pipelineTasksList(root, feature string, asJSON bool) error {
	name, _, status, err := resolveTaskFeature(root, feature)
	if err != nil {
		return err
	}
	items, _, err := stage7Tasks(status)
	if err != nil {
		return err
	}

	result := tasksListResult{Feature: name, Total: len(items)}
	for _, item := range items {
		id, _ := item["id"].(string)
		itemName, _ := item["name"].(string)
		file, _ := item["file"].(string)
		st, _ := item["status"].(string)
		if st == "" {
			st = "pending"
		}
		if st == "complete" {
			result.Completed++
		}
		result.Items = append(result.Items, taskItemFull{ID: id, Name: itemName, File: file, Status: st})
	}
	result.Pct, result.Bar = progressBar(result.Completed, result.Total)

	if asJSON {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("feature:  %s\n", name)
	fmt.Printf("progress: %s\n", result.Bar)
	fmt.Println("tasks:")
	for _, it := range result.Items {
		mark := " "
		if it.Status == "complete" {
			mark = "x"
		}
		fmt.Printf("  [%s] %-4s %s\n", mark, it.ID, it.Name)
	}
	return nil
}

func pipelineTasksComplete(root, feature string, ids []string, all, noAdvance, asJSON bool) error {
	name, dir, status, err := resolveTaskFeature(root, feature)
	if err != nil {
		return err
	}
	items, s7, err := stage7Tasks(status)
	if err != nil {
		return err
	}
	if err := validateUserMode(status, s7); err != nil {
		return err
	}

	byID := make(map[string]map[string]interface{}, len(items))
	for _, item := range items {
		if id, _ := item["id"].(string); id != "" {
			byID[id] = item
		}
	}

	if all {
		ids = nil
		for _, item := range items {
			if id, _ := item["id"].(string); id != "" {
				ids = append(ids, id)
			}
		}
	}

	// Validate the whole batch before touching anything — a single unknown ID
	// aborts with the file untouched.
	var unknown []string
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		var available []string
		for _, item := range items {
			id, _ := item["id"].(string)
			itemName, _ := item["name"].(string)
			available = append(available, fmt.Sprintf("%s: %s", id, itemName))
		}
		return fmt.Errorf("task(s) not found: %s\navailable tasks:\n  %s",
			strings.Join(unknown, ", "), strings.Join(available, "\n  "))
	}

	ts := now()
	result := tasksResult{Feature: name}
	for _, id := range ids {
		item := byID[id]
		itemName, _ := item["name"].(string)
		if itemDone(item) {
			result.AlreadyComplete = append(result.AlreadyComplete, id)
			continue
		}
		item["status"] = "complete"
		item["completed_at"] = ts
		result.CompletedNow = append(result.CompletedNow, taskItem{ID: id, Name: itemName})
	}

	// Recompute counters and normalize to the object container shape.
	result.Total = len(items)
	rawItems := make([]interface{}, len(items))
	for i, item := range items {
		if itemDone(item) {
			result.Completed++
		} else {
			id, _ := item["id"].(string)
			itemName, _ := item["name"].(string)
			result.Remaining = append(result.Remaining, taskItem{ID: id, Name: itemName})
		}
		rawItems[i] = item
	}
	s7["tasks"] = map[string]interface{}{
		"total":     result.Total,
		"completed": result.Completed,
		"items":     rawItems,
	}
	result.Pct, result.Bar = progressBar(result.Completed, result.Total)
	result.AllComplete = result.Completed == result.Total

	status["updated"] = ts
	if len(result.CompletedNow) > 0 {
		completedIDs := make([]string, len(result.CompletedNow))
		for i, t := range result.CompletedNow {
			completedIDs[i] = t.ID
		}
		history, _ := status["history"].([]interface{})
		status["history"] = append(history, map[string]interface{}{
			"timestamp": ts,
			"stage":     "7-implementation",
			"action":    fmt.Sprintf("tasks completed: %s", strings.Join(completedIDs, ", ")),
		})
	}

	// All done → advance 7→complete and 8→ready inside the same atomic write,
	// with the exact `pipeline update` semantics.
	if result.AllComplete && !noAdvance {
		if err := applyStageUpdate(status, "7-implementation", "complete", "", "", "", "", ts); err != nil {
			return err
		}
		if err := applyStageUpdate(status, "8-prd-alignment", "ready", "", "", "", "", ts); err != nil {
			return err
		}
		result.Advanced = true
	}

	if err := writeStatusJSON(filepath.Join(dir, "status.json"), status); err != nil {
		return err
	}

	if asJSON {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("feature:  %s\n", name)
	fmt.Printf("marked complete: %d (already complete: %d)\n", len(result.CompletedNow), len(result.AlreadyComplete))
	fmt.Printf("progress: %s\n", result.Bar)
	if result.AllComplete {
		if result.Advanced {
			fmt.Println("all tasks complete — stage 7 marked complete, stage 8 set to ready")
		} else {
			fmt.Println("all tasks complete (--no-advance: stages left unchanged)")
		}
	} else {
		fmt.Println("remaining:")
		for _, t := range result.Remaining {
			fmt.Printf("  %-4s %s\n", t.ID, t.Name)
		}
	}
	return nil
}
