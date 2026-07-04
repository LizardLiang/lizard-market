package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// agentSpec names an agent and its default model (commands/main.md agent
// table). Eco/power mode overrides stay in the markdown layer.
type agentSpec struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// stageAgents and stageDocs mirror the Stage/Agent/Document table in
// commands/main.md. Stage 9 runs two agents in parallel.
var stageAgents = map[string][]agentSpec{
	"1-prd":            {{"athena", "opus"}},
	"2-prd-review":     {{"nemesis", "opus"}},
	"3-decomposition":  {{"daedalus", "sonnet"}},
	"4-tech-spec":      {{"hephaestus", "opus"}},
	"5-spec-review-sa": {{"apollo", "opus"}},
	"6-test-plan":      {{"artemis", "sonnet"}},
	"7-implementation": {{"ares", "sonnet"}},
	"8-prd-alignment":  {{"hera", "sonnet"}},
	"9-review":         {{"hermes", "opus"}, {"cassandra", "sonnet"}},
}

var stageDocs = map[string][]string{
	"1-prd":            {"prd.md"},
	"2-prd-review":     {"prd-challenge.md"},
	"3-decomposition":  {"decomposition.md"},
	"4-tech-spec":      {"tech-spec.md"},
	"5-spec-review-sa": {"spec-review-sa.md"},
	"6-test-plan":      {"test-plan.md"},
	"7-implementation": {"implementation-notes.md"},
	"8-prd-alignment":  {"prd-alignment.md"},
	"9-review":         {"code-review.md", "risk-analysis.md"},
}

// nextStage describes what the orchestrator should run next.
type nextStage struct {
	Stage     string      `json:"stage,omitempty"`
	Agents    []agentSpec `json:"agents,omitempty"`
	Documents []string    `json:"documents,omitempty"`
	// Procedure tokens the markdown maps to orchestration docs:
	// spawn, spawn-parallel, gap-analysis, complexity-check, hephaestus-gate,
	// pre-implementation, spec-archive-offer, ship-gate, recovery.
	Procedure string `json:"procedure"`
}

type gateResult struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures,omitempty"`
}

// nextResult is the full machine-readable answer of `pipeline next`.
type nextResult struct {
	Feature       string            `json:"feature,omitempty"`
	CurrentStage  string            `json:"current_stage,omitempty"`
	CurrentStatus string            `json:"current_status,omitempty"`
	Verdicts      map[string]string `json:"verdicts,omitempty"`
	Gate          *gateResult       `json:"gate,omitempty"`
	Next          *nextStage        `json:"next,omitempty"`
	// Action: next | wait-user-tasks | blocked | ship-gate | complete |
	// no-feature | ambiguous
	Action     string   `json:"action"`
	Candidates []string `json:"candidates,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

func pipelineNextCmd() *cobra.Command {
	var feature string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Compute the next pipeline action from status.json",
		Long: `Walks the stage-transition state machine (commands/main.md Stage Transition
Logic) over a feature's status.json: current stage, verdict routing, gate checks
(deliverable files, verdicts), and what agent(s) to spawn next. The command only
reports — feature selection among candidates, optional-stage opt-in, and model
overrides (eco/power modes) stay with the orchestrator.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineNext(gitRoot(), feature, asJSON)
		},
	}
	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (auto-detected when omitted)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

func pipelineNext(root, feature string, asJSON bool) error {
	name, dir, status, err := resolveFeatureIn(root, feature)

	var result nextResult
	switch {
	case err == nil:
		result = computeNext(name, dir, status)
	case err == errNoFeature:
		result = nextResult{Action: "no-feature", Reason: err.Error()}
	default:
		var ambiguous *ambiguousFeatureError
		if ok := asAmbiguous(err, &ambiguous); ok {
			result = nextResult{Action: "ambiguous", Candidates: ambiguous.Candidates, Reason: err.Error()}
		} else {
			return err
		}
	}

	if asJSON {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	printNextResult(result)
	return nil
}

// asAmbiguous is a minimal errors.As for *ambiguousFeatureError (avoids pulling
// in errors just for one type assertion through our own return paths).
func asAmbiguous(err error, target **ambiguousFeatureError) bool {
	a, ok := err.(*ambiguousFeatureError)
	if ok {
		*target = a
	}
	return ok
}

// stageVerdict reads the normalized verdict(s) recorded on a stage.
func stageVerdict(stage map[string]interface{}, fields ...string) string {
	for _, f := range fields {
		if v, _ := stage[f].(string); v != "" {
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	return ""
}

// collectVerdicts gathers every review verdict for the report.
func collectVerdicts(pipeline map[string]interface{}) map[string]string {
	verdicts := map[string]string{}
	read := func(stageKey string, outKey string, fields ...string) {
		if stage, ok := pipeline[stageKey].(map[string]interface{}); ok {
			if v := stageVerdict(stage, fields...); v != "" {
				verdicts[outKey] = v
			}
		}
	}
	read("2-prd-review", "nemesis_verdict", "nemesis_verdict", "verdict")
	read("5-spec-review-sa", "verdict", "verdict")
	read("8-prd-alignment", "alignment_verdict", "alignment_verdict", "verdict")
	read("9-review", "code_review_verdict", "code_review_verdict")
	read("9-review", "risk_verdict", "risk_verdict")
	return verdicts
}

// checkDocs verifies a stage's deliverable files exist in the feature dir.
// Stage 7 accepts implementation-notes.md OR a non-empty tasks/ dir.
func checkDocs(featureDir, stageKey string) []string {
	var failures []string
	if stageKey == "7-implementation" {
		if verifyFileExists(featureDir, "implementation-notes.md") == nil {
			return nil
		}
		if tasks, err := filepath.Glob(filepath.Join(featureDir, "tasks", "*.md")); err == nil && len(tasks) > 0 {
			return nil
		}
		return []string{"missing deliverable: implementation-notes.md (or tasks/*.md)"}
	}
	for _, doc := range stageDocs[stageKey] {
		if err := verifyFileExists(featureDir, doc); err != nil {
			failures = append(failures, fmt.Sprintf("missing deliverable: %s", doc))
		}
	}
	return failures
}

// gateFor evaluates the transition gate leaving fromStage: its deliverables
// must exist, plus any extra prerequisites for the target.
func gateFor(featureDir, fromStage string, extraDocs ...string) *gateResult {
	failures := checkDocs(featureDir, fromStage)
	for _, doc := range extraDocs {
		if err := verifyFileExists(featureDir, doc); err != nil {
			failures = append(failures, fmt.Sprintf("missing prerequisite: %s", doc))
		}
	}
	return &gateResult{Passed: len(failures) == 0, Failures: failures}
}

func advanceTo(stageKey, procedure string) *nextStage {
	return &nextStage{
		Stage:     stageKey,
		Agents:    stageAgents[stageKey],
		Documents: stageDocs[stageKey],
		Procedure: procedure,
	}
}

// computeNext walks the transition table for one feature. Pure over the
// feature dir + parsed status.json, so tests drive it directly.
func computeNext(name, featureDir string, status map[string]interface{}) nextResult {
	pipeline, ok := status["pipeline"].(map[string]interface{})
	if !ok {
		return nextResult{Feature: name, Action: "blocked", Reason: "invalid pipeline structure in status.json",
			Next: &nextStage{Procedure: "recovery"}}
	}

	currentKey, _ := status["stage"].(string)
	current, _ := pipeline[currentKey].(map[string]interface{})
	currentStatus, _ := current["status"].(string)

	result := nextResult{
		Feature:       name,
		CurrentStage:  currentKey,
		CurrentStatus: currentStatus,
		Verdicts:      collectVerdicts(pipeline),
		Action:        "next",
	}

	// Feature fully done → complete (verified) or ship-gate (needs the gate run).
	if isFeatureComplete(status) {
		if isFeatureVerified(featureDir, status) {
			result.Action = "complete"
			result.Reason = "all stages complete and verified"
			return result
		}
		result.Action = "ship-gate"
		result.Next = &nextStage{Procedure: "ship-gate"}
		result.Reason = "all stages complete — run 'kratos verify --final' before declaring victory"
		return result
	}

	if current == nil {
		result.Action = "blocked"
		result.Reason = fmt.Sprintf("current stage %q not found in pipeline", currentKey)
		result.Next = &nextStage{Procedure: "recovery"}
		return result
	}

	// Stage still running → resume it (stage 7 User Mode waits on the user).
	if currentStatus == "in-progress" || currentStatus == "ready" {
		if currentKey == "7-implementation" {
			if mode, _ := current["mode"].(string); mode == "user" {
				result.Action = "wait-user-tasks"
				result.Reason = "User Mode — complete tasks, then /kratos:task-complete all"
				return result
			}
		}
		procedure := "spawn"
		switch currentKey {
		case "1-prd":
			// "Continue" at stage 1 with no prd.md runs the full gap-analysis
			// flow, not a bare Athena spawn (main.md Step 3 note).
			if verifyFileExists(featureDir, "prd.md") != nil {
				procedure = "gap-analysis"
			}
		case "4-tech-spec":
			procedure = "hephaestus-gate"
		}
		result.Next = advanceTo(currentKey, procedure)
		result.Gate = &gateResult{Passed: true}
		result.Reason = fmt.Sprintf("stage %s is %s — resume it", currentKey, currentStatus)
		return result
	}

	if currentStatus != "complete" && currentStatus != "skipped" {
		result.Action = "blocked"
		result.Reason = fmt.Sprintf("stage %s is %q — resolve it before advancing (see pipeline/recovery.md)", currentKey, currentStatus)
		result.Next = &nextStage{Procedure: "recovery"}
		return result
	}

	// Current stage is complete/skipped → route by the transition table.
	verdict := ""
	switch currentKey {
	case "1-prd":
		result.Next = advanceTo("2-prd-review", "spawn")
		result.Gate = gateFor(featureDir, "1-prd")

	case "2-prd-review":
		verdict = stageVerdict(current, "nemesis_verdict", "verdict")
		switch verdict {
		case "approved":
			// Complexity check → optional stage 3 → optional discuss → stage 4.
			result.Next = advanceTo("4-tech-spec", "complexity-check")
			result.Gate = gateFor(featureDir, "2-prd-review")
		case "revisions":
			result.Next = advanceTo("1-prd", "spawn")
			result.Gate = &gateResult{Passed: true}
			result.Reason = "Nemesis requested revisions — revise PRD and re-review"
		case "rejected":
			result.Action = "blocked"
			result.Reason = "Nemesis rejected the PRD — fundamental issue, escalate to user"
		default:
			result.Action = "blocked"
			result.Reason = fmt.Sprintf("2-prd-review complete but verdict is %q — see pipeline/recovery.md", verdict)
			result.Next = &nextStage{Procedure: "recovery"}
		}

	case "3-decomposition":
		result.Next = advanceTo("4-tech-spec", "hephaestus-gate")
		result.Gate = gateFor(featureDir, "2-prd-review")

	case "4-tech-spec":
		result.Next = advanceTo("5-spec-review-sa", "spawn")
		result.Gate = gateFor(featureDir, "4-tech-spec")

	case "5-spec-review-sa":
		verdict = stageVerdict(current, "verdict")
		switch verdict {
		case "sound":
			result.Next = advanceTo("6-test-plan", "spawn")
			result.Gate = gateFor(featureDir, "5-spec-review-sa")
		case "concerns", "unsound":
			result.Next = advanceTo("4-tech-spec", "spawn")
			result.Gate = &gateResult{Passed: true}
			result.Reason = fmt.Sprintf("Apollo verdict %q — Hephaestus must revise the spec", verdict)
		default:
			result.Action = "blocked"
			result.Reason = fmt.Sprintf("5-spec-review-sa complete but verdict is %q — see pipeline/recovery.md", verdict)
			result.Next = &nextStage{Procedure: "recovery"}
		}

	case "6-test-plan":
		result.Next = advanceTo("7-implementation", "pre-implementation")
		result.Gate = gateFor(featureDir, "6-test-plan", "tech-spec.md")

	case "7-implementation":
		result.Next = advanceTo("8-prd-alignment", "spawn")
		result.Gate = gateFor(featureDir, "7-implementation", "prd.md")

	case "8-prd-alignment":
		verdict = stageVerdict(current, "alignment_verdict", "verdict")
		switch verdict {
		case "aligned":
			result.Next = advanceTo("9-review", "spec-archive-offer")
			result.Gate = gateFor(featureDir, "8-prd-alignment")
		case "gaps":
			result.Next = advanceTo("7-implementation", "spawn")
			result.Gate = &gateResult{Passed: true}
			result.Reason = "Hera found gaps — Ares must add missing coverage / remove scope creep"
		case "misaligned":
			result.Action = "blocked"
			result.Reason = "Hera verdict misaligned — fundamental scope issue, escalate to user"
		default:
			result.Action = "blocked"
			result.Reason = fmt.Sprintf("8-prd-alignment complete but verdict is %q — see pipeline/recovery.md", verdict)
			result.Next = &nextStage{Procedure: "recovery"}
		}

	case "9-review":
		review := stageVerdict(current, "code_review_verdict")
		risk := stageVerdict(current, "risk_verdict")
		switch {
		case review == "approved" && (risk == "clear" || risk == "caution"):
			result.Action = "ship-gate"
			result.Next = &nextStage{Procedure: "ship-gate"}
			result.Gate = gateFor(featureDir, "9-review")
			result.Reason = "reviews passed — run 'kratos verify --final' before declaring victory"
		case review == "approved" && risk == "blocked":
			result.Action = "blocked"
			result.Reason = "Cassandra risk verdict is blocked (critical) — fix risks, re-run stage 9"
		case review == "changes-required":
			result.Next = advanceTo("7-implementation", "spawn")
			result.Gate = &gateResult{Passed: true}
			result.Reason = "Hermes requires changes — Ares must address the review"
		default:
			result.Action = "blocked"
			result.Reason = fmt.Sprintf("9-review complete but verdicts are review=%q risk=%q — see pipeline/recovery.md", review, risk)
			result.Next = &nextStage{Procedure: "recovery"}
		}

	default:
		result.Action = "blocked"
		result.Reason = fmt.Sprintf("unknown stage %q — see pipeline/recovery.md", currentKey)
		result.Next = &nextStage{Procedure: "recovery"}
	}

	if result.Action == "next" && result.Gate != nil && !result.Gate.Passed {
		result.Action = "blocked"
		result.Reason = fmt.Sprintf("gate failed leaving %s: %s", currentKey, strings.Join(result.Gate.Failures, "; "))
	}
	return result
}

func printNextResult(r nextResult) {
	switch r.Action {
	case "no-feature":
		fmt.Println("no incomplete feature found — run 'kratos pipeline init' to start one")
		return
	case "ambiguous":
		fmt.Println("multiple incomplete features — pass --feature to pick one:")
		for _, c := range r.Candidates {
			fmt.Printf("  %s\n", c)
		}
		return
	}

	fmt.Printf("feature:  %s\n", r.Feature)
	fmt.Printf("current:  %s (%s)\n", r.CurrentStage, r.CurrentStatus)
	for k, v := range r.Verdicts {
		fmt.Printf("verdict:  %s = %s\n", k, v)
	}
	fmt.Printf("action:   %s\n", r.Action)
	if r.Next != nil {
		if r.Next.Stage != "" {
			fmt.Printf("next:     %s (procedure: %s)\n", r.Next.Stage, r.Next.Procedure)
			for _, a := range r.Next.Agents {
				fmt.Printf("agent:    %s (model: %s)\n", a.Name, a.Model)
			}
		} else {
			fmt.Printf("next:     procedure %s\n", r.Next.Procedure)
		}
	}
	if r.Gate != nil && !r.Gate.Passed {
		fmt.Println("gate:     FAILED")
		for _, f := range r.Gate.Failures {
			fmt.Printf("  - %s\n", f)
		}
	}
	if r.Reason != "" {
		fmt.Printf("reason:   %s\n", r.Reason)
	}
}
