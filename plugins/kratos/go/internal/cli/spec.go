package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Package-level notes on the spec-lifecycle feature:
//
// Living specs are pure markdown at .claude/.Arena/specs/<capability>/spec.md in the
// target project. Spec deltas are pure markdown at
// .claude/feature/<name>/spec-delta/<capability>.md. No database is involved.
//
// Requirement identity is the "### Requirement: <Name>" header text, matched by
// strings.TrimSpace + case-sensitive equality (never fuzzy-matched).
//
// Functions taking a `root` parameter are reusable from both direct CLI invocation
// (root = gitRoot()) and hook handlers (root = the SubagentStop/Start payload's cwd),
// matching the existing dual-usage pattern in check.go and hook.go.

// ---------- Data model ----------

// requirement is a single "### Requirement: <Name>" block, parsed from either a
// living spec shard or a spec delta's ADDED/MODIFIED/REMOVED section.
type requirement struct {
	Name      string // trimmed header text — the durable, cross-feature ID
	Body      string // raw markdown from the header line to the next boundary (exclusive)
	HasShall  bool   // a SHALL statement appears before the first scenario
	Scenarios int     // count of "#### Scenario:" blocks in this requirement
}

// renamePair is one "FROM: ... / TO: ..." entry in a delta's RENAMED Requirements section.
type renamePair struct {
	From string
	To   string
}

// delta is a parsed .claude/feature/<name>/spec-delta/<capability>.md file.
type delta struct {
	Capability string
	Added      []requirement
	Modified   []requirement
	Removed    []requirement
	Renamed    []renamePair
}

// specShard is a parsed .claude/.Arena/specs/<capability>/spec.md living spec.
type specShard struct {
	FrontMatter  map[string]string
	Title        string
	Purpose      string
	Requirements []requirement
}

// ---------- Regexes ----------

var (
	reqHeaderRE      = regexp.MustCompile(`(?m)^###\s+Requirement:\s*(.+?)\s*$`)
	scenarioHeaderRE = regexp.MustCompile(`(?m)^####\s+Scenario:`)
	// headerBoundaryRE matches any "## " or "### " line — used to find where a
	// requirement block ends (the next boundary of either level, or EOF).
	headerBoundaryRE = regexp.MustCompile(`(?m)^#{2,3}\s`)
	opHeaderRE       = regexp.MustCompile(`(?m)^##\s+(ADDED|MODIFIED|REMOVED|RENAMED)\s+Requirements\s*$`)
	shallRE          = regexp.MustCompile(`(?i)\bSHALL\b`)
	renameFromRE     = regexp.MustCompile("(?m)^-\\s*FROM:\\s*`?([^`\r\n]+?)`?\\s*$")
	renameToRE       = regexp.MustCompile("(?m)^-\\s*TO:\\s*`?([^`\r\n]+?)`?\\s*$")
	frontmatterRE    = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n?`)
	titleRE          = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	sectionHeaderRE  = regexp.MustCompile(`(?m)^##\s+(Purpose|Requirements)\s*$`)
	// frReqRE extracts "| FR-001 | Requirement text | ..." rows from the PRD template's
	// requirements tables, for kratos spec backfill (Phase 6).
	frReqRE = regexp.MustCompile(`(?m)^\|\s*(FR-\d+)\s*\|\s*([^|]+?)\s*\|`)
)

// ---------- Parsing: requirement blocks (shared by shards and deltas) ----------

// parseRequirements finds all "### Requirement: <Name>" blocks in content. Each
// block runs from its header line to the next "##"/"###" boundary line, or EOF.
func parseRequirements(content string) []requirement {
	reqMatches := reqHeaderRE.FindAllStringSubmatchIndex(content, -1)
	if len(reqMatches) == 0 {
		return nil
	}

	boundaryMatches := headerBoundaryRE.FindAllStringIndex(content, -1)
	boundaryStarts := make([]int, len(boundaryMatches))
	for i, m := range boundaryMatches {
		boundaryStarts[i] = m[0]
	}

	var reqs []requirement
	for _, m := range reqMatches {
		start := m[0]
		name := strings.TrimSpace(content[m[2]:m[3]])
		end := len(content)
		for _, b := range boundaryStarts {
			if b > start {
				end = b
				break
			}
		}
		block := content[start:end]
		reqs = append(reqs, requirement{
			Name:      name,
			Body:      strings.TrimRight(block, " \t\r\n"),
			HasShall:  hasShallBeforeScenario(block),
			Scenarios: len(scenarioHeaderRE.FindAllString(block, -1)),
		})
	}
	return reqs
}

// hasShallBeforeScenario reports whether a SHALL statement appears anywhere before
// the block's first "#### Scenario:" line (or anywhere in the block if there is none).
func hasShallBeforeScenario(block string) bool {
	text := block
	if idx := scenarioHeaderRE.FindStringIndex(block); idx != nil {
		text = block[:idx[0]]
	}
	return shallRE.MatchString(text)
}

// ---------- Parsing: spec delta ----------

// parseDelta parses a spec-delta/<capability>.md file. Returns an error for
// structural problems (no operation section, or content before the first one) —
// callers treat this as a hard validation failure.
func parseDelta(content string) (*delta, error) {
	d := &delta{}

	opMatches := opHeaderRE.FindAllStringSubmatchIndex(content, -1)
	if len(opMatches) == 0 {
		return d, fmt.Errorf("no operation section found — expected one of: ## ADDED Requirements, ## MODIFIED Requirements, ## REMOVED Requirements, ## RENAMED Requirements")
	}

	firstStart := opMatches[0][0]
	if strings.TrimSpace(content[:firstStart]) != "" {
		return d, fmt.Errorf("delta file must start directly with an operation header — found content before the first ## ADDED/MODIFIED/REMOVED/RENAMED Requirements header")
	}

	for i, m := range opMatches {
		opName := content[m[2]:m[3]]
		sectionStart := m[1]
		sectionEnd := len(content)
		if i+1 < len(opMatches) {
			sectionEnd = opMatches[i+1][0]
		}
		section := content[sectionStart:sectionEnd]

		switch opName {
		case "ADDED":
			d.Added = append(d.Added, parseRequirements(section)...)
		case "MODIFIED":
			d.Modified = append(d.Modified, parseRequirements(section)...)
		case "REMOVED":
			d.Removed = append(d.Removed, parseRequirements(section)...)
		case "RENAMED":
			d.Renamed = append(d.Renamed, parseRenames(section)...)
		}
	}

	return d, nil
}

// parseRenames extracts FROM/TO bullet pairs from a RENAMED Requirements section,
// pairing them positionally in the order they appear.
func parseRenames(section string) []renamePair {
	froms := renameFromRE.FindAllStringSubmatch(section, -1)
	tos := renameToRE.FindAllStringSubmatch(section, -1)
	n := len(froms)
	if len(tos) < n {
		n = len(tos)
	}
	var pairs []renamePair
	for i := 0; i < n; i++ {
		pairs = append(pairs, renamePair{
			From: strings.TrimSpace(froms[i][1]),
			To:   strings.TrimSpace(tos[i][1]),
		})
	}
	return pairs
}

// ---------- Parsing: living spec shard ----------

func parseSpecShard(content string) *specShard {
	s := &specShard{FrontMatter: map[string]string{}}
	body := content

	if m := frontmatterRE.FindStringSubmatchIndex(content); m != nil {
		s.FrontMatter = parseSimpleYAML(content[m[2]:m[3]])
		body = content[m[1]:]
	}

	if m := titleRE.FindStringSubmatch(body); m != nil {
		s.Title = strings.TrimSpace(m[1])
	}

	secMatches := sectionHeaderRE.FindAllStringSubmatchIndex(body, -1)
	for i, m := range secMatches {
		name := body[m[2]:m[3]]
		secStart := m[1]
		secEnd := len(body)
		if i+1 < len(secMatches) {
			secEnd = secMatches[i+1][0]
		}
		section := body[secStart:secEnd]

		switch name {
		case "Purpose":
			s.Purpose = strings.TrimSpace(section)
		case "Requirements":
			s.Requirements = parseRequirements(section)
		}
	}

	return s
}

// parseSimpleYAML parses a flat "key: value" frontmatter block. Not a general YAML
// parser — sufficient for the flat scalar fields used in spec shard frontmatter.
func parseSimpleYAML(fm string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return out
}

// hasRequirement reports whether shard contains a requirement with the exact
// (trimmed, case-sensitive) name.
func (s *specShard) hasRequirement(name string) bool {
	if s == nil {
		return false
	}
	for _, r := range s.Requirements {
		if r.Name == name {
			return true
		}
	}
	return false
}

// render serializes the shard back to a full spec.md file, filling frontmatter
// defaults for any missing fields.
func (s *specShard) render(capability string) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	order := []string{"created", "updated", "author", "git_hash", "capability"}
	written := map[string]bool{}
	for _, k := range order {
		v := s.FrontMatter[k]
		if k == "capability" {
			v = capability
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		written[k] = true
	}
	for k, v := range s.FrontMatter {
		if !written[k] {
			sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	sb.WriteString("---\n\n")

	title := s.Title
	if title == "" {
		title = capabilityTitleFromSlug(capability)
	}
	sb.WriteString("# " + title + "\n\n")

	sb.WriteString("## Purpose\n\n")
	purpose := s.Purpose
	if purpose == "" {
		purpose = "(purpose not yet documented)"
	}
	sb.WriteString(purpose + "\n\n")

	sb.WriteString("## Requirements\n\n")
	var blocks []string
	for _, r := range s.Requirements {
		blocks = append(blocks, strings.TrimRight(r.Body, " \t\r\n"))
	}
	sb.WriteString(strings.Join(blocks, "\n\n"))
	sb.WriteString("\n")

	return sb.String()
}

// capabilityTitleFromSlug turns a kebab-case capability slug into a human title,
// e.g. "spec-lifecycle" -> "Spec Lifecycle".
func capabilityTitleFromSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// renameRequirementHeader rewrites the first line of a requirement block (its
// "### Requirement: <Name>" header) to use newName, preserving the rest of the body.
func renameRequirementHeader(body, newName string) string {
	idx := strings.IndexByte(body, '\n')
	if idx == -1 {
		return "### Requirement: " + newName
	}
	return "### Requirement: " + newName + body[idx:]
}

// ---------- Path helpers (root-parameterized: gitRoot() for CLI, hook cwd for gates) ----------

func specsDirIn(root string) string {
	return filepath.Join(root, ".claude", ".Arena", "specs")
}

func specShardPathIn(root, capability string) string {
	return filepath.Join(specsDirIn(root), capability, "spec.md")
}

func featureSpecDeltaDirIn(root, feature string) string {
	return filepath.Join(root, ".claude", "feature", feature, "spec-delta")
}

// validSpecName rejects names that could escape the intended directory (M-003
// pattern from check.go) — used for both feature and capability name arguments.
func validSpecName(name string) error {
	if !featureNameRE.MatchString(name) {
		return fmt.Errorf("invalid name %q: must contain only alphanumeric characters, hyphens, and underscores", name)
	}
	return nil
}

// listFeatureDeltaFilesIn returns the pending (un-archived) delta files directly
// under spec-delta/ for a feature. Files under spec-delta/archived/ are excluded
// because they are not direct children of the glob.
func listFeatureDeltaFilesIn(root, feature string) ([]string, error) {
	dir := featureSpecDeltaDirIn(root, feature)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func currentGitHash() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ---------- Validate (Phase 4) ----------

type validateIssue struct {
	Severity string // "error" or "warning"
	Message  string
}

// validateDeltaFile runs all validation rules against one delta file.
func validateDeltaFile(root, path string, strict bool) ([]validateIssue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	capability := strings.TrimSuffix(filepath.Base(path), ".md")

	d, err := parseDelta(string(raw))
	if err != nil {
		return []validateIssue{{Severity: "error", Message: err.Error()}}, nil
	}

	var issues []validateIssue

	total := len(d.Added) + len(d.Modified) + len(d.Removed) + len(d.Renamed)
	if total == 0 {
		issues = append(issues, validateIssue{"error", "delta has no requirement entries under any operation section"})
	}

	seen := map[string]string{}
	checkDup := func(name, section string) {
		if prev, ok := seen[name]; ok {
			issues = append(issues, validateIssue{"error", fmt.Sprintf("duplicate requirement header %q (appears in both %s and %s)", name, prev, section)})
			return
		}
		seen[name] = section
	}
	for _, r := range d.Added {
		checkDup(r.Name, "ADDED")
	}
	for _, r := range d.Modified {
		checkDup(r.Name, "MODIFIED")
	}
	for _, r := range d.Removed {
		checkDup(r.Name, "REMOVED")
	}

	var living *specShard
	if raw2, err := os.ReadFile(specShardPathIn(root, capability)); err == nil {
		living = parseSpecShard(string(raw2))
	}

	for _, r := range d.Modified {
		if !living.hasRequirement(r.Name) {
			issues = append(issues, validateIssue{"error", fmt.Sprintf("MODIFIED target %q does not exist in the living spec for capability %q", r.Name, capability)})
		}
	}
	for _, r := range d.Removed {
		if !living.hasRequirement(r.Name) {
			issues = append(issues, validateIssue{"error", fmt.Sprintf("REMOVED target %q does not exist in the living spec for capability %q", r.Name, capability)})
		}
	}
	for _, r := range d.Added {
		if living.hasRequirement(r.Name) {
			issues = append(issues, validateIssue{"error", fmt.Sprintf("ADDED requirement %q already exists in the living spec for capability %q — use MODIFIED instead", r.Name, capability)})
		}
	}
	for _, rn := range d.Renamed {
		if !living.hasRequirement(rn.From) {
			issues = append(issues, validateIssue{"error", fmt.Sprintf("RENAMED FROM %q does not exist in the living spec for capability %q", rn.From, capability)})
		}
		if living.hasRequirement(rn.To) {
			issues = append(issues, validateIssue{"error", fmt.Sprintf("RENAMED TO %q already exists in the living spec for capability %q", rn.To, capability)})
		}
	}

	sev := "warning"
	if strict {
		sev = "error"
	}
	checkShallAndScenario := func(r requirement, section string) {
		if !r.HasShall {
			issues = append(issues, validateIssue{sev, fmt.Sprintf("%s requirement %q has no SHALL statement before its first scenario", section, r.Name)})
		}
		if r.Scenarios == 0 {
			issues = append(issues, validateIssue{sev, fmt.Sprintf("%s requirement %q has no scenarios", section, r.Name)})
		}
	}
	for _, r := range d.Added {
		checkShallAndScenario(r, "ADDED")
	}
	for _, r := range d.Modified {
		checkShallAndScenario(r, "MODIFIED")
	}

	return issues, nil
}

// specValidateIn validates every pending delta file for a feature. Returns
// ok=false if any "error"-severity issue was found.
func specValidateIn(root, feature string, strict bool) (bool, []string, error) {
	if err := validSpecName(feature); err != nil {
		return false, nil, err
	}

	files, err := listFeatureDeltaFilesIn(root, feature)
	if err != nil {
		return false, nil, err
	}
	if len(files) == 0 {
		return false, []string{fmt.Sprintf("no spec delta files found in %s", featureSpecDeltaDirIn(root, feature))}, nil
	}

	ok := true
	var messages []string
	for _, f := range files {
		issues, err := validateDeltaFile(root, f, strict)
		if err != nil {
			return false, nil, err
		}
		for _, iss := range issues {
			messages = append(messages, fmt.Sprintf("[%s] %s: %s", iss.Severity, filepath.Base(f), iss.Message))
			if iss.Severity == "error" {
				ok = false
			}
		}
	}
	return ok, messages, nil
}

// ---------- Archive (Phase 5) ----------

// mergeDeltaIntoShard applies a delta's operations to shard's requirement list in
// the fixed order RENAMED -> REMOVED -> MODIFIED -> ADDED. Returns the rendered
// spec.md content and a one-line change summary, or an error on the first conflict
// (target missing for MODIFIED/REMOVED/RENAMED-FROM, or target already exists for
// ADDED/RENAMED-TO). Does not mutate the caller's shard.
//
// ADDED is idempotent: an ADDED entry whose target already exists with a
// byte-identical body is a no-op, not a conflict — this lets a delta that was
// already merged into the shard (but not yet archived, e.g. after a crash between
// the shard write and the delta move) be re-archived cleanly. An ADDED entry whose
// target exists with a different body is still a hard conflict.
func mergeDeltaIntoShard(shard *specShard, d *delta, capability string) (rendered string, summary string, err error) {
	reqs := append([]requirement(nil), shard.Requirements...)

	findIdx := func(name string) int {
		for i, r := range reqs {
			if r.Name == name {
				return i
			}
		}
		return -1
	}

	var renamed, removed, modified, added int

	for _, rn := range d.Renamed {
		idx := findIdx(rn.From)
		if idx == -1 {
			return "", "", fmt.Errorf("RENAMED FROM %q not found in living spec", rn.From)
		}
		if findIdx(rn.To) != -1 {
			return "", "", fmt.Errorf("RENAMED TO %q already exists in living spec", rn.To)
		}
		reqs[idx].Name = rn.To
		reqs[idx].Body = renameRequirementHeader(reqs[idx].Body, rn.To)
		renamed++
	}

	for _, r := range d.Removed {
		idx := findIdx(r.Name)
		if idx == -1 {
			return "", "", fmt.Errorf("REMOVED target %q not found in living spec", r.Name)
		}
		reqs = append(reqs[:idx], reqs[idx+1:]...)
		removed++
	}

	for _, r := range d.Modified {
		idx := findIdx(r.Name)
		if idx == -1 {
			return "", "", fmt.Errorf("MODIFIED target %q not found in living spec", r.Name)
		}
		reqs[idx] = r
		modified++
	}

	for _, r := range d.Added {
		if idx := findIdx(r.Name); idx != -1 {
			// Idempotent re-archive: if the target already carries this exact body,
			// treat the ADDED op as a no-op instead of a conflict. This lets a delta
			// that was already merged into the shard (but whose own move to
			// archived/ failed, e.g. after a crash) be safely re-archived rather
			// than dead-locking forever on "already exists".
			if reqs[idx].Body == r.Body {
				continue
			}
			return "", "", fmt.Errorf("ADDED requirement %q already exists in living spec", r.Name)
		}
		reqs = append(reqs, r)
		added++
	}

	out := &specShard{
		FrontMatter:  map[string]string{},
		Title:        shard.Title,
		Purpose:      shard.Purpose,
		Requirements: reqs,
	}
	for k, v := range shard.FrontMatter {
		out.FrontMatter[k] = v
	}
	now := time.Now().Format(time.RFC3339)
	if out.FrontMatter["created"] == "" {
		out.FrontMatter["created"] = now
	}
	out.FrontMatter["updated"] = now
	if out.FrontMatter["author"] == "" {
		out.FrontMatter["author"] = "athena"
	}
	out.FrontMatter["git_hash"] = currentGitHash()
	out.FrontMatter["capability"] = capability

	var parts []string
	if renamed > 0 {
		parts = append(parts, fmt.Sprintf("renamed %d", renamed))
	}
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("removed %d", removed))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("modified %d", modified))
	}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("added %d", added))
	}
	summary = strings.Join(parts, ", ")
	if summary == "" {
		summary = "no changes"
	}

	return out.render(capability), summary, nil
}

// archivePlan is the computed (but not yet written) result of merging one delta file.
type archivePlan struct {
	deltaFile  string
	capability string
	shardPath  string
	rendered   string
	summary    string
}

// specArchiveIn merges all of a feature's pending deltas into their living specs.
// All merges are computed first; only if every one is conflict-free are the shard
// files written and the delta files moved to spec-delta/archived/ — so a conflict
// in any one capability blocks the entire archive (no partial merge).
//
// The commit itself is two-phase: every shard's spec.md is written before any delta
// is moved. This shrinks the crash-inconsistency window to just the delta-move step,
// and combined with mergeDeltaIntoShard's idempotent ADDED handling, a re-run after a
// mid-commit failure recovers cleanly instead of leaving an unarchivable delta.
func specArchiveIn(root, feature string) (string, error) {
	if err := validSpecName(feature); err != nil {
		return "", err
	}

	files, err := listFeatureDeltaFilesIn(root, feature)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no spec delta files found for feature %q", feature)
	}

	var plans []archivePlan
	for _, f := range files {
		capability := strings.TrimSuffix(filepath.Base(f), ".md")

		raw, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("cannot read %s: %w", f, err)
		}
		d, err := parseDelta(string(raw))
		if err != nil {
			return "", fmt.Errorf("capability %q: %w", capability, err)
		}

		shardPath := specShardPathIn(root, capability)
		shard := &specShard{FrontMatter: map[string]string{}}
		if b, err := os.ReadFile(shardPath); err == nil {
			shard = parseSpecShard(string(b))
		}

		rendered, summary, err := mergeDeltaIntoShard(shard, d, capability)
		if err != nil {
			return "", fmt.Errorf("capability %q: conflict — %w", capability, err)
		}

		plans = append(plans, archivePlan{
			deltaFile:  f,
			capability: capability,
			shardPath:  shardPath,
			rendered:   rendered,
			summary:    summary,
		})
	}

	// Phase 1: write every capability's spec.md shard first. Doing all shard writes
	// before any delta is moved shrinks the inconsistency window to just the delta
	// move step below — if a shard write fails partway through, no delta file has
	// moved yet, so every pending delta (including the ones whose shards already
	// wrote successfully) is untouched and safe to retry as-is.
	for _, p := range plans {
		if err := os.MkdirAll(filepath.Dir(p.shardPath), 0o755); err != nil {
			return "", fmt.Errorf("cannot create spec dir for %q: %w", p.capability, err)
		}
		if err := os.WriteFile(p.shardPath, []byte(p.rendered), 0o644); err != nil {
			return "", fmt.Errorf("cannot write spec.md for %q: %w", p.capability, err)
		}
	}

	// Phase 2: move every delta file to archived/. If this fails partway through, the
	// still-pending deltas are safe to re-archive on the next run: their requirements
	// were already merged into the shard in phase 1, and mergeDeltaIntoShard's ADDED
	// handling treats an identical re-add as a no-op rather than an "already exists"
	// conflict, so the retry recovers cleanly instead of dead-locking.
	var summaries []string
	for _, p := range plans {
		archivedDir := filepath.Join(filepath.Dir(p.deltaFile), "archived")
		if err := os.MkdirAll(archivedDir, 0o755); err != nil {
			return "", fmt.Errorf("cannot create archived dir: %w", err)
		}
		if err := os.Rename(p.deltaFile, filepath.Join(archivedDir, filepath.Base(p.deltaFile))); err != nil {
			return "", fmt.Errorf("cannot move delta to archived/: %w", err)
		}

		summaries = append(summaries, fmt.Sprintf("%s: %s", p.capability, p.summary))
	}

	return strings.Join(summaries, "\n"), nil
}

// ---------- Backfill (Phase 6) ----------

// backfillCapabilitySlug decides which capability a pre-existing (pre-spec-lifecycle)
// feature's requirements should be grouped under.
//
// KNOWN LIMITATION: backfill has no semantic signal to cluster old features into
// shared capabilities the way Athena does for new ones — it falls back to using the
// feature name itself as the capability slug. Users can manually consolidate
// specs/<capability>/ directories afterward if several backfilled features actually
// share one capability.
func backfillCapabilitySlug(featureName string) string {
	return featureName
}

// extractRequirementsFromPRD pulls "| FR-### | Requirement text | ... |" rows out of
// a PRD following templates/prd-template.md's requirements tables, synthesizing a
// minimal SHALL statement + scenario for each so the result satisfies spec validate's
// structural rules. This is a simple table scan, not a full markdown parser — it is
// tolerant of the standard template layout but may miss non-standard PRD formats.
func extractRequirementsFromPRD(content string) []requirement {
	matches := frReqRE.FindAllStringSubmatch(content, -1)
	var reqs []requirement
	seen := map[string]bool{}
	for _, m := range matches {
		id := m[1]
		title := strings.TrimSpace(m[2])
		if title == "" || strings.HasPrefix(title, "[") || seen[title] {
			continue
		}
		seen[title] = true

		name := title
		if len(name) > 50 {
			name = strings.TrimSpace(name[:50])
		}

		lower := title
		if len(lower) > 0 {
			lower = strings.ToLower(lower[:1]) + lower[1:]
		}

		body := fmt.Sprintf(
			"### Requirement: %s\n\nThe system SHALL %s (backfilled from %s).\n\n#### Scenario: %s\n\n- **WHEN** a user triggers this behavior\n- **THEN** the system SHALL %s",
			name, lower, id, name, lower,
		)
		reqs = append(reqs, requirement{Name: name, Body: body, HasShall: true, Scenarios: 1})
	}
	return reqs
}

// specBackfillIn scans aligned features (prd-alignment verdict == "aligned") in root
// and generates/merges living specs from their prd.md files. Idempotent: re-running
// adds zero new requirements the second time, since requirements are deduplicated by
// name within each capability shard.
func specBackfillIn(root string) (string, error) {
	pattern := filepath.Join(root, ".claude", "feature", "*", "status.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob error: %w", err)
	}

	added := map[string]int{}
	var processed []string

	for _, statusFile := range matches {
		data, err := readStatusJSON(statusFile)
		if err != nil {
			continue
		}
		pipeline, ok := data["pipeline"].(map[string]interface{})
		if !ok {
			continue
		}
		stage8, ok := pipeline["8-prd-alignment"].(map[string]interface{})
		if !ok {
			continue
		}
		verdict, _ := stage8["verdict"].(string)
		if strings.ToLower(verdict) != "aligned" {
			continue
		}

		featureDir := filepath.Dir(statusFile)
		featureName := filepath.Base(featureDir)

		prdRaw, err := os.ReadFile(filepath.Join(featureDir, "prd.md"))
		if err != nil {
			continue
		}

		reqs := extractRequirementsFromPRD(string(prdRaw))
		if len(reqs) == 0 {
			continue
		}

		capability := backfillCapabilitySlug(featureName)
		shardPath := specShardPathIn(root, capability)

		shard := &specShard{FrontMatter: map[string]string{}}
		if b, err := os.ReadFile(shardPath); err == nil {
			shard = parseSpecShard(string(b))
		}

		newCount := 0
		for _, r := range reqs {
			if !shard.hasRequirement(r.Name) {
				shard.Requirements = append(shard.Requirements, r)
				newCount++
			}
		}

		if newCount > 0 {
			now := time.Now().Format(time.RFC3339)
			if shard.FrontMatter["created"] == "" {
				shard.FrontMatter["created"] = now
			}
			shard.FrontMatter["updated"] = now
			if shard.FrontMatter["author"] == "" {
				shard.FrontMatter["author"] = "metis"
			}
			shard.FrontMatter["git_hash"] = currentGitHash()
			shard.FrontMatter["capability"] = capability

			if err := os.MkdirAll(filepath.Dir(shardPath), 0o755); err != nil {
				return "", fmt.Errorf("cannot create spec dir for %q: %w", capability, err)
			}
			if err := os.WriteFile(shardPath, []byte(shard.render(capability)), 0o644); err != nil {
				return "", fmt.Errorf("cannot write spec.md for %q: %w", capability, err)
			}
		}

		added[capability] += newCount
		processed = append(processed, featureName)
	}

	if len(processed) == 0 {
		return "no aligned features with a readable prd.md found — nothing to backfill", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("backfill scanned %d aligned feature(s)\n", len(processed)))
	caps := make([]string, 0, len(added))
	for c := range added {
		caps = append(caps, c)
	}
	sort.Strings(caps)
	for _, c := range caps {
		sb.WriteString(fmt.Sprintf("  %s: +%d requirement(s)\n", c, added[c]))
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// ---------- Cobra commands ----------

// SpecCmd returns the 'spec' command group.
func SpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Manage living behavioral specs and per-feature spec deltas",
		Long:  "Living specs are pure markdown at .claude/.Arena/specs/<capability>/spec.md. No database is involved.",
	}
	cmd.AddCommand(specListCmd())
	cmd.AddCommand(specShowCmd())
	cmd.AddCommand(specDiffCmd())
	cmd.AddCommand(specValidateCmd())
	cmd.AddCommand(specArchiveCmd())
	cmd.AddCommand(specBackfillCmd())
	return cmd
}

func specListCmd() *cobra.Command {
	var capabilities, changes bool
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List living capability shards, or un-archived pending spec deltas",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if capabilities && changes {
				return fmt.Errorf("--capabilities and --changes are mutually exclusive")
			}
			if changes {
				return specListChanges(cmd, gitRoot())
			}
			return specListCapabilities(cmd, gitRoot())
		},
	}
	cmd.Flags().BoolVar(&capabilities, "capabilities", false, "list living capability shards (default)")
	cmd.Flags().BoolVar(&changes, "changes", false, "list un-archived pending spec deltas across all features — the safety-net view")
	return cmd
}

func specListCapabilities(cmd *cobra.Command, root string) error {
	entries, err := os.ReadDir(specsDirIn(root))
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(cmd.OutOrStdout(), "no living specs found — run 'kratos spec backfill' or wait for a feature to archive its first delta")
			return nil
		}
		return err
	}

	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(specsDirIn(root), e.Name(), "spec.md"))
		if err != nil {
			continue
		}
		shard := parseSpecShard(string(raw))
		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %d requirement(s)\n", e.Name(), len(shard.Requirements))
		found = true
	}
	if !found {
		fmt.Fprintln(cmd.OutOrStdout(), "no living specs found")
	}
	return nil
}

func specListChanges(cmd *cobra.Command, root string) error {
	pattern := filepath.Join(root, ".claude", "feature", "*", "spec-delta", "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob error: %w", err)
	}
	if len(matches) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no pending spec deltas")
		return nil
	}
	sort.Strings(matches)
	for _, m := range matches {
		capability := strings.TrimSuffix(filepath.Base(m), ".md")
		feature := filepath.Base(filepath.Dir(filepath.Dir(m)))
		fmt.Fprintf(cmd.OutOrStdout(), "%-24s capability=%-20s %s\n", feature, capability, m)
	}
	return nil
}

func specShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "show <capability>",
		Short:        "Render a living capability spec",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validSpecName(args[0]); err != nil {
				return err
			}
			data, err := os.ReadFile(specShardPathIn(gitRoot(), args[0]))
			if err != nil {
				return fmt.Errorf("no living spec for capability %q: %w", args[0], err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

// renderDeltaDiffLines formats a parsed delta as +/~/-/-> marker lines, independent
// of any output stream — used by specDiffCmd and directly testable.
func renderDeltaDiffLines(d *delta) []string {
	var lines []string
	for _, r := range d.Added {
		lines = append(lines, fmt.Sprintf("+ %s", r.Name))
	}
	for _, r := range d.Modified {
		lines = append(lines, fmt.Sprintf("~ %s", r.Name))
	}
	for _, r := range d.Removed {
		lines = append(lines, fmt.Sprintf("- %s", r.Name))
	}
	for _, rn := range d.Renamed {
		lines = append(lines, fmt.Sprintf("-> %s => %s", rn.From, rn.To))
	}
	return lines
}

func specDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "diff <feature>",
		Short:        "Show a feature's spec delta(s) with +/~/-/-> markers",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validSpecName(args[0]); err != nil {
				return err
			}
			root := gitRoot()
			files, err := listFeatureDeltaFilesIn(root, args[0])
			if err != nil {
				return err
			}
			if len(files) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no spec delta found for feature %q\n", args[0])
				return nil
			}
			for _, f := range files {
				capability := strings.TrimSuffix(filepath.Base(f), ".md")
				fmt.Fprintf(cmd.OutOrStdout(), "capability: %s\n", capability)

				raw, err := os.ReadFile(f)
				if err != nil {
					return err
				}
				d, err := parseDelta(string(raw))
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  (parse error: %v)\n", err)
					continue
				}
				for _, line := range renderDeltaDiffLines(d) {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", line)
				}
			}
			return nil
		},
	}
}

func specValidateCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:          "validate <feature>",
		Short:        "Validate a feature's spec delta(s) against the validation rules",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ok, messages, err := specValidateIn(gitRoot(), args[0], strict)
			if err != nil {
				return err
			}
			for _, m := range messages {
				fmt.Fprintln(cmd.OutOrStdout(), m)
			}
			if !ok {
				return fmt.Errorf("spec validate failed for feature %q", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), "OK")
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "promote warnings (missing SHALL/scenario) to errors")
	return cmd
}

func specArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "archive <feature>",
		Short:        "Merge a feature's spec delta(s) into their living spec(s)",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			summary, err := specArchiveIn(gitRoot(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), summary)
			return nil
		},
	}
}

func specBackfillCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "backfill",
		Short:        "Scan aligned features and generate/merge living specs from their prd.md",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			summary, err := specBackfillIn(gitRoot())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), summary)
			return nil
		},
	}
}
