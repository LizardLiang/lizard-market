package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// checkDebugLog writes a message to stderr with [kratos-check] prefix.
// Suppressed unless KRATOS_DEBUG=1 is set (L-002).
func checkDebugLog(format string, args ...any) {
	if os.Getenv("KRATOS_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "[kratos-check] "+format+"\n", args...)
}

// featureNameRE restricts --feature values to safe names, preventing path traversal (M-003).
var featureNameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// stageCheck defines the Tier 1 verification requirements for a pipeline stage.
type stageCheck struct {
	Tier          int
	Files         []string            // files that must exist and be non-empty
	Verdicts      map[string][]string // file -> accepted verdict strings (lowercase)
	MaxRetries    int
	Optional      bool                  // true for stages 3, 4 (skip if status == "skipped")
	AgentDispatch map[string]stageCheck // for stage 9-review: agent_type -> sub-check
}

// stageChecks is the declarative stage-to-deliverable mapping for all Tier 1 stages.
// Optional pre-pipeline research is excluded (produces Arena files, not feature-scoped deliverables)
// Stage 4-tech-spec: excluded from Tier 1 (dual-hook retry counter interaction risk)
// Stage 6-implementation: excluded from Tier 1 (Tier 2 deferred; existing Ares gate remains)
var stageChecks = map[string]stageCheck{
	"1-prd": {
		Tier:       1,
		Files:      []string{"prd.md", "decisions.md"},
		MaxRetries: 2,
	},
	"2-prd-review": {
		Tier:  1,
		Files: []string{"prd-challenge.md"},
		Verdicts: map[string][]string{
			"prd-challenge.md": {"approved", "revisions", "rejected"},
		},
		MaxRetries: 2,
	},
	"3-decomposition": {
		Tier:       1,
		Files:      []string{"decomposition.md"},
		MaxRetries: 2,
		Optional:   true,
	},
	// 4-tech-spec excluded from Tier 1 -- see tech-spec Section 3 "Hephaestus Exclusion"
	"5-spec-review-sa": {
		Tier:  1,
		Files: []string{"spec-review-sa.md"},
		Verdicts: map[string][]string{
			"spec-review-sa.md": {"sound", "concerns", "unsound"},
		},
		MaxRetries: 2,
	},
	"6-test-plan": {
		Tier:       1,
		Files:      []string{"test-plan.md"},
		MaxRetries: 2,
	},
	// 7-implementation excluded from Tier 1 (Tier 2 deferred; existing Ares gate remains)
	"8-prd-alignment": {
		Tier:  1,
		Files: []string{"prd-alignment.md"},
		Verdicts: map[string][]string{
			"prd-alignment.md": {"aligned", "gaps", "misaligned"},
		},
		MaxRetries: 2,
	},
	"9-review": {
		Tier: 1,
		AgentDispatch: map[string]stageCheck{
			"kratos:cassandra": {
				Tier:       1,
				Files:      []string{"risk-analysis.md"},
				MaxRetries: 2,
			},
			// kratos:hermes: no Tier 1 check -- existing handleHermesStop remains
		},
		MaxRetries: 2,
	},
}

// CheckCmd returns the 'check' subcommand.
func CheckCmd() *cobra.Command {
	var initFlag, verifyFlag bool
	var stage, feature string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Stage-aware agent verification",
		Long:  "Validate that agents produce expected deliverables. Use --init on SubagentStart and --verify on SubagentStop.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if initFlag {
				return handleCheckInit(stage, feature)
			}
			if verifyFlag {
				return handleCheckVerify(stage, feature)
			}
			return cmd.Help()
		},
	}

	cmd.Flags().BoolVar(&initFlag, "init", false, "Initialize verification context (SubagentStart)")
	cmd.Flags().BoolVar(&verifyFlag, "verify", false, "Run verification checks (SubagentStop)")
	cmd.Flags().StringVar(&stage, "stage", "", "Pipeline stage (e.g., 1-prd, 4-tech-spec)")
	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (optional, auto-discovers if omitted)")

	return cmd
}

// handleCheckInit processes a SubagentStart event: injects verification context into the agent.
func handleCheckInit(stage, feature string) error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		checkDebugLog("init: stdin read error: %v", err)
		return outputSubagentStartContext("")
	}

	var input subagentStartInput
	if err := json.Unmarshal(raw, &input); err != nil {
		checkDebugLog("init: json parse error: %v", err)
		return outputSubagentStartContext("")
	}

	// Auto-detect stage from status.json when --stage not provided (Athena multi-stage case)
	if stage == "" {
		cwd := input.Cwd
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		detected, err := detectCurrentStage(cwd, feature)
		if err != nil || detected == "" {
			checkDebugLog("init: could not auto-detect stage: %v", err)
			return outputSubagentStartContext("")
		}
		stage = detected
	}

	check, ok := stageChecks[stage]
	if !ok {
		checkDebugLog("init: unknown stage %q, returning empty context", stage)
		return outputSubagentStartContext("")
	}

	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// For stage 9-review, dispatch by agent_type so each agent sees only its own deliverables
	if len(check.AgentDispatch) > 0 {
		agentType := strings.ToLower(input.AgentType)
		if sub, ok := check.AgentDispatch[agentType]; ok {
			ctx := buildInitContext(stage, sub)
			return outputSubagentStartContext(ctx)
		}
		// Unknown agent type at stage 9-review — fail open
		checkDebugLog("init: stage 9-review unknown agent_type %q, returning empty context", input.AgentType)
		return outputSubagentStartContext("")
	}

	// Build the additional context listing expected deliverables
	ctx := buildInitContext(stage, check)
	// Apollo (stage 5) reviews architecture and may re-scan; nudge it to reuse Arena.
	if stage == "5-spec-review-sa" {
		ctx += arenaScanReminder(cwd)
	}
	return outputSubagentStartContext(ctx)
}

// buildInitContext constructs the additionalContext string for --init responses.
// check must already be the resolved check for this specific agent (AgentDispatch handled upstream).
func buildInitContext(stage string, check stageCheck) string {
	var sb strings.Builder
	sb.WriteString("VERIFICATION GATE ACTIVE\n")
	sb.WriteString(fmt.Sprintf("Stage: %s\n", stage))

	sb.WriteString("Expected deliverables:\n")
	for _, f := range check.Files {
		sb.WriteString(fmt.Sprintf("  - %s (must exist, non-empty)\n", f))
	}
	if len(check.Verdicts) > 0 {
		for f, verdicts := range check.Verdicts {
			sb.WriteString(fmt.Sprintf("  - %s must contain a verdict (one of: %s)\n",
				f, strings.Join(verdicts, ", ")))
		}
	}

	sb.WriteString("You will be blocked from completing if these files are missing.")
	return sb.String()
}

// handleCheckVerify processes a SubagentStop event: validates deliverables.
func handleCheckVerify(stage, feature string) error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		checkDebugLog("verify: stdin read error: %v", err)
		return outputSubagentOK()
	}

	var input subagentStopInput
	if err := json.Unmarshal(raw, &input); err != nil {
		checkDebugLog("verify: json parse error: %v", err)
		return outputSubagentOK()
	}

	// Guard: prevent infinite loops
	if input.StopHookActive {
		checkDebugLog("verify: stop_hook_active=true, returning OK immediately")
		return outputSubagentOK()
	}

	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Auto-detect stage from status.json when --stage not provided (Athena multi-stage case)
	if stage == "" {
		detected, err := detectCurrentStage(cwd, feature)
		if err != nil || detected == "" {
			checkDebugLog("verify: could not auto-detect stage: %v", err)
			return outputSubagentOK()
		}
		stage = detected
	}

	check, ok := stageChecks[stage]
	if !ok {
		checkDebugLog("verify: unknown stage %q, failing open", stage)
		return outputSubagentOK()
	}

	featureDir, err := resolveFeatureDir(cwd, feature, stage)
	if err != nil || featureDir == "" {
		checkDebugLog("verify: could not resolve feature dir for stage %q: %v", stage, err)
		return outputSubagentOK()
	}

	// Optional stage: check if it was skipped
	if check.Optional {
		skipped, err := isStageSkipped(featureDir, stage)
		if err != nil {
			checkDebugLog("verify: error checking if stage %q is skipped: %v", stage, err)
			// Fail open
		} else if skipped {
			checkDebugLog("verify: stage %q is skipped, returning OK", stage)
			return outputSubagentOK()
		}
	}

	// Stage 9-review: dispatch by agent_type
	if len(check.AgentDispatch) > 0 {
		agentType := strings.ToLower(input.AgentType)
		subCheck, found := check.AgentDispatch[agentType]
		if !found {
			// Unknown agent type at stage 9-review — fail open (includes kratos:hermes)
			checkDebugLog("verify: stage 9-review unknown agent_type %q, failing open", agentType)
			return outputSubagentOK()
		}
		// Run the sub-check for the dispatched agent
		failedChecks := runStageChecks(featureDir, subCheck)
		return handleRetryLogic(featureDir, stage, subCheck, failedChecks)
	}

	// Standard check: run file existence and verdict checks
	failedChecks := runStageChecks(featureDir, check)
	return handleRetryLogic(featureDir, stage, check, failedChecks)
}

// handleRetryLogic manages the retry counter and blocking behavior.
func handleRetryLogic(featureDir, stage string, check stageCheck, failedChecks []string) error {
	if len(failedChecks) == 0 {
		// All checks pass — reset retry counter
		if err := resetRetry(featureDir, stage); err != nil {
			checkDebugLog("verify: failed to reset retry counter for %q: %v", stage, err)
		}
		return outputSubagentOK()
	}

	// Checks failed — manage retry counter
	count, err := incrementRetry(featureDir, stage)
	if err != nil {
		checkDebugLog("verify: failed to increment retry counter for %q: %v", stage, err)
		// Fail open on state management errors
		return outputSubagentOK()
	}

	if count > check.MaxRetries {
		// Max retries exhausted — record failure and allow through
		checkDebugLog("verify: max retries exhausted for stage %q, recording failure and allowing stop", stage)
		if err := recordCheckFailure(featureDir, stage, check.Tier, failedChecks); err != nil {
			checkDebugLog("verify: failed to record check failure: %v", err)
		}
		if err := resetRetry(featureDir, stage); err != nil {
			checkDebugLog("verify: failed to reset retry counter after exhaustion: %v", err)
		}
		return outputSubagentOK()
	}

	// Block agent with failure reason
	reason := formatFailureReason(stage, failedChecks, count, check.MaxRetries)
	return outputSubagentBlock(reason)
}

// formatFailureReason builds the human-readable block reason string.
func formatFailureReason(stage string, failedChecks []string, attempt, maxRetries int) string {
	details := strings.Join(failedChecks, "; ")
	return fmt.Sprintf("Tier 1 verification failed for stage %s: %s (attempt %d/%d)",
		stage, details, attempt, maxRetries)
}

// verifyFileExists checks that a file exists and has non-zero size.
func verifyFileExists(featureDir, filename string) error {
	path := filepath.Join(featureDir, filename)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found in %s", filename, featureDir)
		}
		return fmt.Errorf("%s cannot be accessed: %w", filename, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s exists but is empty (0 bytes)", filename)
	}
	return nil
}

// verifyVerdictPresent checks that a file contains at least one accepted verdict (case-insensitive).
// Only the last 500 bytes are searched: verdict sections appear at the end of agent documents,
// so this avoids false positives from incidental keywords in quoted blocks or historical notes (H-001).
func verifyVerdictPresent(featureDir, filename string, verdicts []string) error {
	path := filepath.Join(featureDir, filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w", filename, err)
	}
	tail := content
	if len(tail) > 500 {
		tail = tail[len(tail)-500:]
	}
	lower := strings.ToLower(string(tail))
	for _, v := range verdicts {
		if strings.Contains(lower, v) {
			return nil
		}
	}
	return fmt.Errorf("%s does not contain a required verdict (expected one of: %s)",
		filename, strings.Join(verdicts, ", "))
}

// runStageChecks runs all file and verdict checks for a stage, returning failure messages.
func runStageChecks(featureDir string, check stageCheck) []string {
	var failures []string

	for _, f := range check.Files {
		if err := verifyFileExists(featureDir, f); err != nil {
			failures = append(failures, err.Error())
		}
	}

	for f, verdicts := range check.Verdicts {
		// Only check verdict if file exists (file check already reports missing)
		path := filepath.Join(featureDir, f)
		if _, err := os.Stat(path); err == nil {
			if err := verifyVerdictPresent(featureDir, f, verdicts); err != nil {
				failures = append(failures, err.Error())
			}
		}
	}

	return failures
}

// findFeatureDirByStage scans .claude/feature/*/status.json and returns the feature directory
// whose status.json matches the target stage (top-level "stage" field or pipeline stage status).
// If multiple features match, returns the one with the most recent "updated" timestamp.
func findFeatureDirByStage(cwd string, targetStage string) (string, error) {
	pattern := filepath.Join(cwd, ".claude", "feature", "*", "status.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob error: %w", err)
	}

	type candidate struct {
		dir     string
		updated string
	}
	var candidates []candidate

	for _, statusFile := range matches {
		data, err := os.ReadFile(statusFile)
		if err != nil {
			checkDebugLog("findFeatureDirByStage: failed to read %s: %v", statusFile, err)
			continue
		}

		var status map[string]interface{}
		if err := json.Unmarshal(data, &status); err != nil {
			checkDebugLog("findFeatureDirByStage: failed to parse %s: %v", statusFile, err)
			continue
		}

		matched := false

		// Match top-level "stage" field
		if stage, ok := status["stage"].(string); ok && stage == targetStage {
			matched = true
		}

		// Also check if the target stage has status "in-progress" or "ready"
		if !matched {
			pipeline, ok := status["pipeline"].(map[string]interface{})
			if ok {
				stageData, ok := pipeline[targetStage].(map[string]interface{})
				if ok {
					stageStatus, _ := stageData["status"].(string)
					if stageStatus == "in-progress" || stageStatus == "ready" {
						matched = true
					}
				}
			}
		}

		if matched {
			updated, _ := status["updated"].(string)
			candidates = append(candidates, candidate{
				dir:     filepath.Dir(statusFile),
				updated: updated,
			})
		}
	}

	if len(candidates) == 0 {
		return "", nil
	}

	if len(candidates) == 1 {
		return candidates[0].dir, nil
	}

	// Multiple candidates — return the most recently updated one
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.updated > best.updated {
			best = c
		}
	}
	return best.dir, nil
}

// detectCurrentStage reads status.json to find the stage for Athena auto-detection.
// It checks "pending_stage" first (set by the orchestrator before spawning Athena) so
// --init gets the right deliverables even before the agent has updated "stage" itself.
// When multiple features are active, the most recently updated one wins (M-001: consistent
// with findFeatureDirByStage ordering so stage and directory resolution always agree).
func detectCurrentStage(cwd, feature string) (string, error) {
	readStageAndUpdated := func(data []byte) (stage, updated string) {
		var status map[string]interface{}
		if err := json.Unmarshal(data, &status); err != nil {
			return "", ""
		}
		// pending_stage wins: the orchestrator sets this before spawning
		if s, ok := status["pending_stage"].(string); ok && s != "" {
			stage = s
		} else {
			stage, _ = status["stage"].(string)
		}
		updated, _ = status["updated"].(string)
		return stage, updated
	}

	if feature != "" {
		statusFile := filepath.Join(cwd, ".claude", "feature", feature, "status.json")
		data, err := os.ReadFile(statusFile)
		if err != nil {
			return "", fmt.Errorf("cannot read status.json for feature %q: %w", feature, err)
		}
		stage, _ := readStageAndUpdated(data)
		return stage, nil
	}

	// Scan all features — return the stage from the most recently updated one.
	pattern := filepath.Join(cwd, ".claude", "feature", "*", "status.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob error: %w", err)
	}

	bestStage := ""
	bestUpdated := ""
	for _, statusFile := range matches {
		data, err := os.ReadFile(statusFile)
		if err != nil {
			continue
		}
		stage, updated := readStageAndUpdated(data)
		if stage == "" {
			continue
		}
		if bestStage == "" || updated > bestUpdated {
			bestStage = stage
			bestUpdated = updated
		}
	}

	return bestStage, nil
}

// resolveFeatureDir returns the feature directory path from --feature flag or auto-discovery.
// featureFlag is validated against featureNameRE before path construction to prevent traversal (M-003).
func resolveFeatureDir(cwd, featureFlag, stage string) (string, error) {
	if featureFlag != "" {
		if !featureNameRE.MatchString(featureFlag) {
			return "", fmt.Errorf("invalid feature name %q: must contain only alphanumeric characters, hyphens, and underscores", featureFlag)
		}
		dir := filepath.Join(cwd, ".claude", "feature", featureFlag)
		info, err := os.Stat(dir)
		if err != nil {
			return "", fmt.Errorf("feature directory %q not found: %w", dir, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%q is not a directory", dir)
		}
		return dir, nil
	}

	return findFeatureDirByStage(cwd, stage)
}

// isStageSkipped reads status.json and checks if the given stage has status "skipped".
func isStageSkipped(featureDir, stage string) (bool, error) {
	statusFile := filepath.Join(featureDir, "status.json")
	status, err := readStatusJSON(statusFile)
	if err != nil {
		return false, err
	}

	pipeline, ok := status["pipeline"].(map[string]interface{})
	if !ok {
		return false, nil
	}
	stageData, ok := pipeline[stage].(map[string]interface{})
	if !ok {
		return false, nil
	}
	stageStatus, _ := stageData["status"].(string)
	return strings.ToLower(stageStatus) == "skipped", nil
}

// readCheckState reads check-state.json from the feature directory.
// Returns an empty map if the file does not exist.
func readCheckState(featureDir string) (map[string]int, error) {
	path := filepath.Join(featureDir, "check-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int{}, nil
		}
		return nil, fmt.Errorf("cannot read check-state.json: %w", err)
	}

	var state map[string]int
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("cannot parse check-state.json: %w", err)
	}
	return state, nil
}

// writeCheckState atomically writes check-state.json to the feature directory.
func writeCheckState(featureDir string, state map[string]int) error {
	path := filepath.Join(featureDir, "check-state.json")

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal check-state.json: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		return fmt.Errorf("cannot create feature dir: %w", err)
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

// incrementRetry increments the retry counter for a stage and returns the new count.
// Stage 11 concurrency assumption (M-002): kratos:hermes has no Tier 1 check and returns OK
// immediately (check.go AgentDispatch: hermes entry absent), so it never calls incrementRetry.
// The only stage-11 agent that reaches this path is kratos:cassandra, making the concurrent
// read-modify-write race between two stage-11 agents a non-issue in practice. If future Tier 2/3
// checks are added for hermes, introduce an advisory file lock before the read-modify-write here.
func incrementRetry(featureDir, stage string) (int, error) {
	state, err := readCheckState(featureDir)
	if err != nil {
		return 0, err
	}
	state[stage]++
	if err := writeCheckState(featureDir, state); err != nil {
		return 0, err
	}
	return state[stage], nil
}

// resetRetry removes the retry counter for a stage (sets to 0 / removes key).
func resetRetry(featureDir, stage string) error {
	state, err := readCheckState(featureDir)
	if err != nil {
		return err
	}
	delete(state, stage)
	return writeCheckState(featureDir, state)
}

// recordCheckFailure appends a check_failures entry to the stage object in status.json.
func recordCheckFailure(featureDir, stage string, tier int, failedChecks []string) error {
	statusFile := filepath.Join(featureDir, "status.json")
	status, err := readStatusJSON(statusFile)
	if err != nil {
		return fmt.Errorf("cannot read status.json: %w", err)
	}

	// Ensure pipeline map exists
	pipeline, ok := status["pipeline"].(map[string]interface{})
	if !ok {
		pipeline = map[string]interface{}{}
		status["pipeline"] = pipeline
	}

	// Ensure stage entry exists
	stageData, ok := pipeline[stage].(map[string]interface{})
	if !ok {
		stageData = map[string]interface{}{}
		pipeline[stage] = stageData
	}

	// Build failure entry
	failure := map[string]interface{}{
		"timestamp":         time.Now().Format(time.RFC3339),
		"tier":              tier,
		"checks_failed":     failedChecks,
		"retries_exhausted": true,
	}

	// Append to check_failures array
	existing, _ := stageData["check_failures"].([]interface{})
	existing = append(existing, failure)
	stageData["check_failures"] = existing

	return writeStatusJSON(statusFile, status)
}
