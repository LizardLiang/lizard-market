package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// finalVerdictReq is a deliverable file that must declare a *passing* verdict
// for the feature to be shippable. field names the structured verdict in the
// stage's status.json map (references/status-json-schema.md); altFields are
// older/alternate names that also count.
type finalVerdictReq struct {
	file      string
	field     string
	altFields []string
	passing   []string
	failing   []string
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
// Verdict resolution order (verifyVerdict): the structured field written by
// `pipeline update --verdict` wins (stage 9's two reviewers file into distinct
// fields, so they no longer clobber each other); only when it is absent does
// the gate fall back to reading the deliverable's Verdict section. Reading the
// file first used to fail on documents whose verdict sat above the last 500
// bytes — and taught the orchestrator to append its own "APPROVED" to a
// reviewer's file (2026-09 transcript review).
var finalGateReqs = []finalStageReq{
	{stage: "1-prd", files: []string{"prd.md", "decisions.md"}},
	{stage: "2-prd-review", verdicts: []finalVerdictReq{
		{file: "prd-challenge.md", field: "nemesis_verdict", altFields: []string{"verdict"}, passing: []string{"approved"}, failing: []string{"revisions", "rejected"}},
	}},
	{stage: "4-tech-spec", files: []string{"tech-spec.md"}},
	{stage: "5-spec-review-sa", verdicts: []finalVerdictReq{
		{file: "spec-review-sa.md", field: "verdict", passing: []string{"sound"}, failing: []string{"concerns", "unsound"}},
	}},
	{stage: "6-test-plan", files: []string{"test-plan.md"}},
	{stage: "7-implementation", files: []string{"implementation-notes.md"}},
	{stage: "8-prd-alignment", verdicts: []finalVerdictReq{
		{file: "prd-alignment.md", field: "alignment_verdict", passing: []string{"aligned"}, failing: []string{"gaps", "misaligned"}},
	}},
	{stage: "9-review", verdicts: []finalVerdictReq{
		{file: "code-review.md", field: "code_review_verdict", passing: []string{"approved"}, failing: []string{"changes-required", "changes required", "changes-requested"}},
		{file: "risk-analysis.md", field: "risk_verdict", passing: []string{"clear", "caution"}, failing: []string{"blocked"}},
	}},
}

// verdictWordRE builds a case-insensitive, word-boundary regex for a verdict
// term. Word boundaries are essential: they stop "sound" from matching inside
// "unsound" and "aligned" from matching inside "misaligned", which a plain
// substring check would get wrong.
func verdictWordRE(term string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
}

// verdictHeadingRE finds a "Verdict" heading (any level, optional trailing
// text) so the scan can start where the reviewer actually declared the result.
var verdictHeadingRE = regexp.MustCompile(`(?im)^#{1,6}\s*verdict\b.*$`)

// verdictRegion returns the part of a deliverable that carries the verdict:
// from the LAST "Verdict" heading to the end of the file, or, when no such
// heading exists, the final 500 bytes (verdict sections live at the end of
// agent documents).
func verdictRegion(content []byte) string {
	locs := verdictHeadingRE.FindAllIndex(content, -1)
	if len(locs) > 0 {
		return string(content[locs[len(locs)-1][0]:])
	}
	if len(content) > 500 {
		return string(content[len(content)-500:])
	}
	return string(content)
}

// lastNonEmptyLine returns the last line of text with content, for messages.
func lastNonEmptyLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			if len(l) > 120 {
				l = l[:120] + "…"
			}
			return l
		}
	}
	return ""
}

// verifyPassingVerdict reads a deliverable's verdict region and returns nil
// only when it declares a passing verdict and NO failing verdict. A present
// failing verdict always fails, even if a passing keyword also appears
// (conservative: the ship gate errs toward blocking, never toward shipping a
// failed review).
func verifyPassingVerdict(featureDir string, req finalVerdictReq) error {
	path := filepath.Join(featureDir, req.file)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w", req.file, err)
	}
	text := verdictRegion(content)

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
	return fmt.Errorf("%s does not declare a passing verdict in its Verdict section (expected one of: %s; last line found: %q). Do not edit the reviewer's file — re-spawn the reviewer or file the verdict with `pipeline update --verdict`",
		req.file, strings.Join(req.passing, ", "), lastNonEmptyLine(text))
}

// structuredVerdict returns the normalized verdict recorded in the stage map
// under req.field (or an alternate field), and whether one was present.
func structuredVerdict(stageMap map[string]interface{}, req finalVerdictReq) (string, bool) {
	if stageMap == nil {
		return "", false
	}
	for _, key := range append([]string{req.field}, req.altFields...) {
		if key == "" {
			continue
		}
		if v, ok := stageMap[key].(string); ok {
			v = strings.ToLower(strings.TrimSpace(v))
			if canon, ok := verdictSynonyms[v]; ok {
				v = canon
			}
			if v != "" {
				return v, true
			}
		}
	}
	return "", false
}

// verifyVerdict is the gate's verdict check: the deliverable must exist, and
// the structured verdict in status.json decides when present; otherwise the
// file's Verdict section is read.
func verifyVerdict(featureDir string, req finalVerdictReq, stageMap map[string]interface{}) error {
	if err := verifyFileExists(featureDir, req.file); err != nil {
		return err
	}
	if v, ok := structuredVerdict(stageMap, req); ok {
		for _, f := range req.failing {
			if v == strings.ToLower(f) {
				return fmt.Errorf("%s: status.json %s = %q (failing)", req.file, req.field, v)
			}
		}
		for _, p := range req.passing {
			if v == strings.ToLower(p) {
				return nil
			}
		}
		return fmt.Errorf("%s: status.json %s = %q is not a passing verdict (expected one of: %s)",
			req.file, req.field, v, strings.Join(req.passing, ", "))
	}
	return verifyPassingVerdict(featureDir, req)
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
// it is used to honor stages explicitly marked "skipped" and to read the
// structured verdicts.
func evaluateFinalGate(featureDir string, status map[string]interface{}) []string {
	var failures []string
	pipeline, _ := status["pipeline"].(map[string]interface{})

	for _, req := range finalGateReqs {
		if stageSkipped(pipeline, req.stage) {
			continue
		}
		var stageMap map[string]interface{}
		if pipeline != nil {
			stageMap, _ = pipeline[req.stage].(map[string]interface{})
		}
		for _, f := range req.files {
			if err := verifyFileExists(featureDir, f); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %s", req.stage, err.Error()))
			}
		}
		for _, v := range req.verdicts {
			if err := verifyVerdict(featureDir, v, stageMap); err != nil {
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
	var final, landed bool
	var feature, hash string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Consolidated ship-gate verification",
		Long: "Verify that every pipeline stage produced its deliverable and declared a passing verdict.\n" +
			"Run 'kratos verify --final --feature <name>' before declaring VICTORY. Exits non-zero if any check fails.\n\n" +
			"'kratos verify --landed [--hash <commit>]' checks that implementation work is committed: the named\n" +
			"commit exists and no tracked source file (outside .claude/) is left modified. Exits non-zero otherwise.",
		// A failed ship gate is an expected outcome, not CLI misuse: don't dump
		// usage, and let main.go print the single error line.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if landed {
				cwd, _ := os.Getwd()
				return runVerifyLanded(cmd, cwd, hash)
			}
			if final {
				return runVerifyFinal(feature)
			}
			return cmd.Help()
		},
	}

	cmd.Flags().BoolVar(&final, "final", false, "Run the consolidated all-stages-passed ship gate")
	cmd.Flags().StringVar(&feature, "feature", "", "Feature name (required with --final)")
	cmd.Flags().BoolVar(&landed, "landed", false, "Check that implementation work is committed (clean tracked tree outside .claude/)")
	cmd.Flags().StringVar(&hash, "hash", "", "Commit reported by Ares (Landed: <branch>@<hash>); must exist in the repository")
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

// landedIgnorePrefixes are paths whose dirtiness never counts against a landed
// check: pipeline bookkeeping and Kratos state, not implementation work.
var landedIgnorePrefixes = []string{".claude/", ".kratos/"}

// evaluateLanded returns the reasons implementation work in repoDir is not
// landed: the reported commit is missing, or tracked files outside the
// bookkeeping dirs are still modified/untracked. An empty slice means landed.
// Any git infrastructure problem (no git, not a repo) is returned as a single
// reason so callers see why the check could not run.
func evaluateLanded(repoDir, hash string) []string {
	git, err := exec.LookPath("git")
	if err != nil {
		return []string{"git is not on PATH; cannot verify landing"}
	}
	if out, err := exec.Command(git, "-C", repoDir, "rev-parse", "--is-inside-work-tree").CombinedOutput(); err != nil || !strings.Contains(string(out), "true") {
		return []string{fmt.Sprintf("%s is not a git work tree; nothing can land here (use LANDED-NOT-APPLICABLE: not a git repository)", repoDir)}
	}

	var failures []string
	if hash != "" {
		if out, err := exec.Command(git, "-C", repoDir, "cat-file", "-e", hash+"^{commit}").CombinedOutput(); err != nil {
			failures = append(failures, fmt.Sprintf("commit %s not found in the repository (%s)", hash, strings.TrimSpace(string(out))))
		}
	}

	out, err := exec.Command(git, "-C", repoDir, "status", "--porcelain", "--untracked-files=all").CombinedOutput()
	if err != nil {
		return append(failures, fmt.Sprintf("git status failed: %s", strings.TrimSpace(string(out))))
	}
	var dirty []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		if i := strings.Index(p, " -> "); i >= 0 { // renames: keep the destination
			p = p[i+4:]
		}
		p = strings.Trim(p, `"`)
		ignore := false
		for _, pre := range landedIgnorePrefixes {
			if strings.HasPrefix(p, pre) {
				ignore = true
				break
			}
		}
		if !ignore {
			dirty = append(dirty, p)
		}
	}
	if len(dirty) > 0 {
		shown := dirty
		if len(shown) > 10 {
			shown = append(shown[:10], fmt.Sprintf("… %d more", len(dirty)-10))
		}
		failures = append(failures, fmt.Sprintf("%d uncommitted path(s) outside .claude/: %s", len(dirty), strings.Join(shown, ", ")))
	}
	return failures
}

// runVerifyLanded prints VERIFIED/BLOCKED for the landed check and returns a
// non-nil error (→ non-zero exit) when work is not landed.
func runVerifyLanded(cmd *cobra.Command, repoDir, hash string) error {
	failures := evaluateLanded(repoDir, hash)
	if len(failures) == 0 {
		if hash != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "VERIFIED: landed — commit %s exists and the tracked tree outside .claude/ is clean\n", hash)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "VERIFIED: landed — the tracked tree outside .claude/ is clean")
		}
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "BLOCKED: work is not landed — %d failure(s):\n", len(failures))
	for _, f := range failures {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s\n", f)
	}
	return fmt.Errorf("work is not landed (%d failure(s))", len(failures))
}
