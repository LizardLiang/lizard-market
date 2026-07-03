package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// finalVerdictReq is a deliverable file that must declare a *passing* verdict
// for the feature to be shippable.
type finalVerdictReq struct {
	file    string
	passing []string
	failing []string
}

// finalStageReq is one stage's ship-gate requirement: its deliverable files
// must exist (non-empty) and any verdict files must declare a passing verdict.
type finalStageReq struct {
	stage    string // pipeline key, used to honor a "skipped" status
	files    []string
	verdicts []finalVerdictReq
}

// finalGateReqs is the consolidated "all verification passed" gate evaluated by
// `kratos verify --final`. It is the mechanical meaning of VICTORY.
//
// Verdicts are read from the deliverable *files* (each reviewer always writes
// its own file), NOT from the status.json "verdict" field — stage 9's two
// reviewers (Hermes, Cassandra) both write that single field via
// `pipeline update --verdict`, clobbering each other, so it is unreliable.
var finalGateReqs = []finalStageReq{
	{stage: "1-prd", files: []string{"prd.md", "decisions.md"}},
	{stage: "2-prd-review", verdicts: []finalVerdictReq{
		{file: "prd-challenge.md", passing: []string{"approved"}, failing: []string{"revisions", "rejected"}},
	}},
	{stage: "4-tech-spec", files: []string{"tech-spec.md"}},
	{stage: "5-spec-review-sa", verdicts: []finalVerdictReq{
		{file: "spec-review-sa.md", passing: []string{"sound"}, failing: []string{"concerns", "unsound"}},
	}},
	{stage: "6-test-plan", files: []string{"test-plan.md"}},
	{stage: "7-implementation", files: []string{"implementation-notes.md"}},
	{stage: "8-prd-alignment", verdicts: []finalVerdictReq{
		{file: "prd-alignment.md", passing: []string{"aligned"}, failing: []string{"gaps", "misaligned"}},
	}},
	{stage: "9-review", verdicts: []finalVerdictReq{
		{file: "code-review.md", passing: []string{"approved"}, failing: []string{"changes-required", "changes required", "changes-requested"}},
		{file: "risk-analysis.md", passing: []string{"clear", "caution"}, failing: []string{"blocked"}},
	}},
}

// verdictWordRE builds a case-insensitive, word-boundary regex for a verdict
// term. Word boundaries are essential: they stop "sound" from matching inside
// "unsound" and "aligned" from matching inside "misaligned", which a plain
// substring check would get wrong.
func verdictWordRE(term string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
}

// verifyPassingVerdict reads the tail of a deliverable file and returns nil only
// when it declares a passing verdict and NO failing verdict. A present failing
// verdict always fails, even if a passing keyword also appears (conservative:
// the ship gate errs toward blocking, never toward shipping a failed review).
//
// Only the last 500 bytes are scanned, matching verifyVerdictPresent (H-001):
// verdict sections live at the end of agent documents. If the verdict is out of
// range the gate reports "no passing verdict" and blocks — the safe direction.
func verifyPassingVerdict(featureDir string, req finalVerdictReq) error {
	path := filepath.Join(featureDir, req.file)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w", req.file, err)
	}
	tail := content
	if len(tail) > 500 {
		tail = tail[len(tail)-500:]
	}
	text := string(tail)

	for _, f := range req.failing {
		if verdictWordRE(f).MatchString(text) {
			return fmt.Errorf("%s declares a failing verdict (%q)", req.file, f)
		}
	}
	for _, p := range req.passing {
		if verdictWordRE(p).MatchString(text) {
			return nil
		}
	}
	return fmt.Errorf("%s does not declare a passing verdict (expected one of: %s)",
		req.file, strings.Join(req.passing, ", "))
}

// stageSkipped reports whether a stage is explicitly marked "skipped" in the
// pipeline map, so an intentionally-skipped optional stage doesn't block the gate.
func stageSkipped(pipeline map[string]interface{}, stage string) bool {
	if pipeline == nil {
		return false
	}
	s, ok := pipeline[stage].(map[string]interface{})
	if !ok {
		return false
	}
	status, _ := s["status"].(string)
	return strings.ToLower(status) == "skipped"
}

// evaluateFinalGate returns the list of ship-gate failures for a feature. An
// empty slice means every verification passed. status may be nil; when provided
// it is used to honor stages explicitly marked "skipped".
func evaluateFinalGate(featureDir string, status map[string]interface{}) []string {
	var failures []string
	pipeline, _ := status["pipeline"].(map[string]interface{})

	for _, req := range finalGateReqs {
		if stageSkipped(pipeline, req.stage) {
			continue
		}
		for _, f := range req.files {
			if err := verifyFileExists(featureDir, f); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %s", req.stage, err.Error()))
			}
		}
		for _, v := range req.verdicts {
			if err := verifyPassingVerdict(featureDir, v); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %s", req.stage, err.Error()))
			}
		}
	}
	return failures
}

// isFeatureVerified reports whether a feature passed every ship-gate
// verification (deliverables present + passing verdicts). Used to gate the
// "[done]" label so a structurally-complete-but-failed feature never shows done.
func isFeatureVerified(featureDir string, status map[string]interface{}) bool {
	return len(evaluateFinalGate(featureDir, status)) == 0
}

// arenaScanReminder returns a one-line "don't re-scan" nudge for the spec/impl
// agents (Hephaestus, Apollo, Ares) when the project already has an Arena
// knowledge base to read instead of globbing the codebase again. It returns ""
// when no Arena exists (nothing to reuse), so the reminder only appears when
// acting on it is actually possible.
func arenaScanReminder(cwd string) string {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if _, err := os.Stat(filepath.Join(cwd, ".claude", ".Arena", "index.md")); err != nil {
		return ""
	}
	return "\n\nCONTEXT REUSE: This project has an Arena knowledge base (.claude/.Arena/). " +
		"Upstream stages already scanned the codebase — read Arena (index.md → architecture/, conventions/) " +
		"and the stage summaries via 'kratos pipeline get --compact' instead of re-globbing broadly. " +
		"A broad codebase re-scan duplicates upstream work and wastes tokens."
}

// VerifyCmd returns the 'verify' command: the consolidated ship gate.
func VerifyCmd() *cobra.Command {
	var final bool
	var feature string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Consolidated ship-gate verification",
		Long: "Verify that every pipeline stage produced its deliverable and declared a passing verdict.\n" +
			"Run 'kratos verify --final --feature <name>' before declaring VICTORY. Exits non-zero if any check fails.",
		// A failed ship gate is an expected outcome, not CLI misuse: don't dump
		// usage, and let main.go print the single error line.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if final {
				return runVerifyFinal(feature)
			}
			return cmd.Help()
		},
	}

	cmd.Flags().BoolVar(&final, "final", false, "Run the consolidated all-stages-passed ship gate")
	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required with --final)")
	return cmd
}

// runVerifyFinal evaluates the ship gate for a feature and returns a non-nil
// error (→ non-zero exit) if any stage is missing a deliverable or a passing
// verdict. It fails closed: a missing/unreadable status.json blocks shipping.
func runVerifyFinal(feature string) error {
	if feature == "" {
		return fmt.Errorf("--feature is required with --final")
	}
	if !featureNameRE.MatchString(feature) {
		return fmt.Errorf("invalid feature name %q: must contain only alphanumeric characters, hyphens, and underscores", feature)
	}

	path := statusPath(feature)
	status, err := readStatusJSON(path)
	if err != nil {
		// Fail closed: without status.json we cannot confirm anything passed.
		return fmt.Errorf("cannot verify feature %q: %w", feature, err)
	}
	featureDir := filepath.Dir(path)

	failures := evaluateFinalGate(featureDir, status)
	if len(failures) == 0 {
		fmt.Printf("VERIFIED: %s — all stages passed, safe to ship\n", feature)
		return nil
	}

	fmt.Printf("BLOCKED: %s — %d verification failure(s):\n", feature, len(failures))
	for _, f := range failures {
		fmt.Printf("  ✗ %s\n", f)
	}
	return fmt.Errorf("feature %q failed the ship gate (%d failure(s))", feature, len(failures))
}
