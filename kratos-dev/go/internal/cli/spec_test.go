package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- fixtures ----------

const validSpecShard = `---
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
author: metis
git_hash: abc123
capability: auth
---

# Auth

## Purpose

Handles authentication.

## Requirements

### Requirement: Password Login

The system SHALL allow users to log in with a password.

#### Scenario: valid credentials

- **WHEN** a user submits correct credentials
- **THEN** the system SHALL grant access

### Requirement: Session Timeout

The system SHALL expire sessions after 30 minutes of inactivity.

#### Scenario: idle session

- **WHEN** a session is idle for 30 minutes
- **THEN** the system SHALL expire it
`

const validAddedOnlyDelta = `## ADDED Requirements

### Requirement: MFA Enrollment

The system SHALL allow users to enroll a second authentication factor.

#### Scenario: enroll TOTP

- **WHEN** a user adds a TOTP authenticator
- **THEN** the system SHALL store the shared secret securely
`

// ---------- parse tests ----------

func TestParseRequirements(t *testing.T) {
	// parseRequirements is content-agnostic: it scans for "### Requirement:" headers
	// anywhere in the given text. parseSpecShard restricts it to the Requirements
	// section; calling it directly on the whole shard still finds both requirements.
	reqs := parseRequirements(validSpecShard)
	if len(reqs) != 2 {
		t.Fatalf("parseRequirements on raw shard text: got %d requirements, want 2", len(reqs))
	}

	shard := parseSpecShard(validSpecShard)
	if len(shard.Requirements) != 2 {
		t.Fatalf("expected 2 requirements, got %d", len(shard.Requirements))
	}

	r0 := shard.Requirements[0]
	if r0.Name != "Password Login" {
		t.Errorf("requirement[0].Name = %q, want %q", r0.Name, "Password Login")
	}
	if !r0.HasShall {
		t.Errorf("requirement[0].HasShall = false, want true")
	}
	if r0.Scenarios != 1 {
		t.Errorf("requirement[0].Scenarios = %d, want 1", r0.Scenarios)
	}

	r1 := shard.Requirements[1]
	if r1.Name != "Session Timeout" {
		t.Errorf("requirement[1].Name = %q, want %q", r1.Name, "Session Timeout")
	}
}

func TestParseSpecShardFrontmatterAndPurpose(t *testing.T) {
	shard := parseSpecShard(validSpecShard)
	if shard.Title != "Auth" {
		t.Errorf("Title = %q, want %q", shard.Title, "Auth")
	}
	if shard.Purpose != "Handles authentication." {
		t.Errorf("Purpose = %q, want %q", shard.Purpose, "Handles authentication.")
	}
	if shard.FrontMatter["capability"] != "auth" {
		t.Errorf("FrontMatter[capability] = %q, want %q", shard.FrontMatter["capability"], "auth")
	}
	if shard.FrontMatter["author"] != "metis" {
		t.Errorf("FrontMatter[author] = %q, want %q", shard.FrontMatter["author"], "metis")
	}
}

func TestParseSpecShardEmpty(t *testing.T) {
	shard := parseSpecShard("")
	if len(shard.Requirements) != 0 {
		t.Errorf("expected 0 requirements for empty content, got %d", len(shard.Requirements))
	}
	if shard.Title != "" || shard.Purpose != "" {
		t.Errorf("expected empty Title/Purpose for empty content")
	}
}

func TestParseDeltaAllOperations(t *testing.T) {
	content := `## RENAMED Requirements

- FROM: ` + "`Session Timeout`" + `
- TO: ` + "`Session Expiry`" + `

## REMOVED Requirements

### Requirement: Password Login

No longer needed.

## MODIFIED Requirements

### Requirement: Session Expiry

The system SHALL expire sessions after 15 minutes of inactivity.

#### Scenario: idle session

- **WHEN** a session is idle for 15 minutes
- **THEN** the system SHALL expire it

## ADDED Requirements

### Requirement: SSO Login

The system SHALL allow users to log in via SSO.

#### Scenario: sso redirect

- **WHEN** a user clicks Login with SSO
- **THEN** the system SHALL redirect to the identity provider
`
	d, err := parseDelta(content)
	if err != nil {
		t.Fatalf("parseDelta returned error: %v", err)
	}
	if len(d.Renamed) != 1 || d.Renamed[0].From != "Session Timeout" || d.Renamed[0].To != "Session Expiry" {
		t.Errorf("Renamed = %+v, want one From=Session Timeout To=Session Expiry", d.Renamed)
	}
	if len(d.Removed) != 1 || d.Removed[0].Name != "Password Login" {
		t.Errorf("Removed = %+v, want one Password Login", d.Removed)
	}
	if len(d.Modified) != 1 || d.Modified[0].Name != "Session Expiry" {
		t.Errorf("Modified = %+v, want one Session Expiry", d.Modified)
	}
	if len(d.Added) != 1 || d.Added[0].Name != "SSO Login" {
		t.Errorf("Added = %+v, want one SSO Login", d.Added)
	}
}

func TestParseDeltaNoOperationSection(t *testing.T) {
	_, err := parseDelta("just some text\nwith no headers\n")
	if err == nil {
		t.Fatal("expected error for delta with no operation section, got nil")
	}
}

func TestParseDeltaContentBeforeHeader(t *testing.T) {
	content := "# Some Title\n\n## ADDED Requirements\n\n### Requirement: X\n\nThe system SHALL do X.\n\n#### Scenario: s\n\n- **WHEN** a\n- **THEN** b\n"
	_, err := parseDelta(content)
	if err == nil {
		t.Fatal("expected error for delta with a title before the first operation header, got nil")
	}
}

func TestRenderDeltaDiffLines(t *testing.T) {
	content := `## ADDED Requirements

### Requirement: A

The system SHALL a.

#### Scenario: s

- **WHEN** x
- **THEN** y

## MODIFIED Requirements

### Requirement: B

The system SHALL b.

#### Scenario: s

- **WHEN** x
- **THEN** y

## REMOVED Requirements

### Requirement: C

## RENAMED Requirements

- FROM: ` + "`D`" + `
- TO: ` + "`E`" + `
`
	d, err := parseDelta(content)
	if err != nil {
		t.Fatalf("parseDelta error: %v", err)
	}
	lines := renderDeltaDiffLines(d)
	want := []string{"+ A", "~ B", "- C", "-> D => E"}
	if len(lines) != len(want) {
		t.Fatalf("renderDeltaDiffLines = %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

// ---------- validate tests ----------

func setupCapabilitySpec(t *testing.T, root, capability, content string) {
	t.Helper()
	writeFile(t, specShardPathIn(root, capability), content)
}

func setupFeatureDelta(t *testing.T, root, feature, capability, content string) string {
	t.Helper()
	path := filepath.Join(featureSpecDeltaDirIn(root, feature), capability+".md")
	writeFile(t, path, content)
	return path
}

func TestValidateDeltaFile_Valid(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)
	path := setupFeatureDelta(t, root, "feat-mfa", "auth", validAddedOnlyDelta)

	issues, err := validateDeltaFile(root, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.Severity == "error" {
			t.Errorf("unexpected error issue: %s", iss.Message)
		}
	}
}

func TestValidateDeltaFile_EmptyDelta(t *testing.T) {
	root := t.TempDir()
	path := setupFeatureDelta(t, root, "feat-empty", "auth", "## ADDED Requirements\n")

	issues, err := validateDeltaFile(root, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorContaining(issues, "no requirement entries") {
		t.Errorf("expected 'no requirement entries' error, got %+v", issues)
	}
}

func TestValidateDeltaFile_DuplicateHeader(t *testing.T) {
	root := t.TempDir()
	content := `## ADDED Requirements

### Requirement: X

The system SHALL x.

#### Scenario: s

- **WHEN** a
- **THEN** b

## MODIFIED Requirements

### Requirement: X

The system SHALL x differently.

#### Scenario: s

- **WHEN** a
- **THEN** b
`
	path := setupFeatureDelta(t, root, "feat-dup", "auth", content)
	issues, err := validateDeltaFile(root, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorContaining(issues, "duplicate requirement header") {
		t.Errorf("expected duplicate header error, got %+v", issues)
	}
}

func TestValidateDeltaFile_ModifiedTargetMissing(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)
	content := `## MODIFIED Requirements

### Requirement: Nonexistent Requirement

The system SHALL do something new.

#### Scenario: s

- **WHEN** a
- **THEN** b
`
	path := setupFeatureDelta(t, root, "feat-mod-missing", "auth", content)
	issues, err := validateDeltaFile(root, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorContaining(issues, "MODIFIED target") {
		t.Errorf("expected MODIFIED target error, got %+v", issues)
	}
}

func TestValidateDeltaFile_RemovedTargetMissing(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)
	content := "## REMOVED Requirements\n\n### Requirement: Nonexistent Requirement\n\nGone.\n"
	path := setupFeatureDelta(t, root, "feat-rm-missing", "auth", content)
	issues, err := validateDeltaFile(root, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorContaining(issues, "REMOVED target") {
		t.Errorf("expected REMOVED target error, got %+v", issues)
	}
}

func TestValidateDeltaFile_AddedAlreadyExists(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)
	content := `## ADDED Requirements

### Requirement: Password Login

The system SHALL allow users to log in with a password (dup).

#### Scenario: s

- **WHEN** a
- **THEN** b
`
	path := setupFeatureDelta(t, root, "feat-add-exists", "auth", content)
	issues, err := validateDeltaFile(root, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorContaining(issues, "already exists") {
		t.Errorf("expected ADDED-already-exists error, got %+v", issues)
	}
}

func TestValidateDeltaFile_RenamedFromMissingOrToExists(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)

	fromMissing := "## RENAMED Requirements\n\n- FROM: `Nonexistent`\n- TO: `New Name`\n"
	path1 := setupFeatureDelta(t, root, "feat-ren-from", "auth", fromMissing)
	issues1, err := validateDeltaFile(root, path1, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorContaining(issues1, "RENAMED FROM") {
		t.Errorf("expected RENAMED FROM error, got %+v", issues1)
	}

	toExists := "## RENAMED Requirements\n\n- FROM: `Password Login`\n- TO: `Session Timeout`\n"
	path2 := setupFeatureDelta(t, root, "feat-ren-to", "auth", toExists)
	issues2, err := validateDeltaFile(root, path2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorContaining(issues2, "RENAMED TO") {
		t.Errorf("expected RENAMED TO error, got %+v", issues2)
	}
}

func TestValidateDeltaFile_MissingScenarioAndShall_WarningThenStrictError(t *testing.T) {
	root := t.TempDir()
	content := "## ADDED Requirements\n\n### Requirement: Bare Requirement\n\nThis requirement body has no normative statement and no scenario.\n"
	path := setupFeatureDelta(t, root, "feat-bare", "auth", content)

	issues, err := validateDeltaFile(root, path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasSeverity(issues, "error") {
		t.Errorf("non-strict mode should only warn about missing SHALL/scenario, got errors: %+v", issues)
	}
	if !hasWarningContaining(issues, "no SHALL statement") {
		t.Errorf("expected SHALL warning, got %+v", issues)
	}
	if !hasWarningContaining(issues, "no scenarios") {
		t.Errorf("expected scenario warning, got %+v", issues)
	}

	strictIssues, err := validateDeltaFile(root, path, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasSeverity(strictIssues, "error") {
		t.Errorf("--strict should promote SHALL/scenario warnings to errors, got %+v", strictIssues)
	}
}

func TestSpecValidateIn_NoDeltaFiles(t *testing.T) {
	root := t.TempDir()
	ok, messages, err := specValidateIn(root, "no-such-feature", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false when no delta files exist")
	}
	if len(messages) == 0 {
		t.Error("expected a message explaining no delta files were found")
	}
}

func TestSpecValidateIn_Valid(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)
	setupFeatureDelta(t, root, "feat-mfa", "auth", validAddedOnlyDelta)

	ok, messages, err := specValidateIn(root, "feat-mfa", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Errorf("expected ok=true, got issues: %v", messages)
	}
}

// ---------- archive tests ----------

func TestSpecArchiveIn_MergeOrder(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)

	// RENAMED "Session Timeout" -> "Session Expiry", then MODIFIED targets the new
	// name — this only succeeds if RENAMED is applied before MODIFIED, proving the
	// RENAMED -> REMOVED -> MODIFIED -> ADDED merge order.
	content := `## RENAMED Requirements

- FROM: ` + "`Session Timeout`" + `
- TO: ` + "`Session Expiry`" + `

## REMOVED Requirements

### Requirement: Password Login

Replaced by SSO.

## MODIFIED Requirements

### Requirement: Session Expiry

The system SHALL expire sessions after 15 minutes of inactivity.

#### Scenario: idle session

- **WHEN** a session is idle for 15 minutes
- **THEN** the system SHALL expire it

## ADDED Requirements

### Requirement: SSO Login

The system SHALL allow users to log in via SSO.

#### Scenario: sso redirect

- **WHEN** a user clicks Login with SSO
- **THEN** the system SHALL redirect to the identity provider
`
	deltaPath := setupFeatureDelta(t, root, "feat-auth-revamp", "auth", content)

	summary, err := specArchiveIn(root, "feat-auth-revamp")
	if err != nil {
		t.Fatalf("specArchiveIn returned error: %v", err)
	}
	if !strings.Contains(summary, "renamed 1") || !strings.Contains(summary, "removed 1") ||
		!strings.Contains(summary, "modified 1") || !strings.Contains(summary, "added 1") {
		t.Errorf("summary = %q, want mentions of renamed/removed/modified/added counts", summary)
	}

	merged, err := os.ReadFile(specShardPathIn(root, "auth"))
	if err != nil {
		t.Fatalf("expected merged spec.md to exist: %v", err)
	}
	shard := parseSpecShard(string(merged))

	names := map[string]requirement{}
	for _, r := range shard.Requirements {
		names[r.Name] = r
	}
	if _, ok := names["Password Login"]; ok {
		t.Error("Password Login should have been removed")
	}
	if _, ok := names["Session Timeout"]; ok {
		t.Error("Session Timeout should have been renamed away")
	}
	expiry, ok := names["Session Expiry"]
	if !ok {
		t.Fatal("Session Expiry (renamed target) should exist")
	}
	if !strings.Contains(expiry.Body, "15 minutes") {
		t.Errorf("Session Expiry should carry the MODIFIED body (15 minutes), got: %s", expiry.Body)
	}
	if _, ok := names["SSO Login"]; !ok {
		t.Error("SSO Login should have been added")
	}

	// Delta file must be moved to archived/, not left pending.
	if _, err := os.Stat(deltaPath); !os.IsNotExist(err) {
		t.Errorf("expected original delta file to be moved, still present at %s", deltaPath)
	}
	archivedPath := filepath.Join(filepath.Dir(deltaPath), "archived", "auth.md")
	if _, err := os.Stat(archivedPath); err != nil {
		t.Errorf("expected archived delta at %s: %v", archivedPath, err)
	}
}

func TestSpecArchiveIn_ConflictBlocksMerge(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)

	content := "## MODIFIED Requirements\n\n### Requirement: Nonexistent Requirement\n\nThe system SHALL do something.\n\n#### Scenario: s\n\n- **WHEN** a\n- **THEN** b\n"
	deltaPath := setupFeatureDelta(t, root, "feat-conflict", "auth", content)

	before, _ := os.ReadFile(specShardPathIn(root, "auth"))

	_, err := specArchiveIn(root, "feat-conflict")
	if err == nil {
		t.Fatal("expected specArchiveIn to return a conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "not found in living spec") {
		t.Errorf("error = %v, want it to mention the missing target", err)
	}

	after, _ := os.ReadFile(specShardPathIn(root, "auth"))
	if string(before) != string(after) {
		t.Error("living spec should be unchanged after a blocked archive")
	}
	if _, err := os.Stat(deltaPath); err != nil {
		t.Errorf("delta file should remain pending (not moved) after a blocked archive: %v", err)
	}
}

func TestSpecArchiveIn_MultiCapabilityConflictBlocksAll(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)

	// capA: a valid ADDED-only delta for a brand new capability.
	validContent := "## ADDED Requirements\n\n### Requirement: New Thing\n\nThe system SHALL do a new thing.\n\n#### Scenario: s\n\n- **WHEN** a\n- **THEN** b\n"
	setupFeatureDelta(t, root, "feat-multi", "billing", validContent)

	// capB: targets the existing "auth" capability but with a conflicting MODIFIED.
	conflictContent := "## MODIFIED Requirements\n\n### Requirement: Nonexistent Requirement\n\nThe system SHALL do something.\n\n#### Scenario: s\n\n- **WHEN** a\n- **THEN** b\n"
	setupFeatureDelta(t, root, "feat-multi", "auth", conflictContent)

	_, err := specArchiveIn(root, "feat-multi")
	if err == nil {
		t.Fatal("expected an error due to the auth capability conflict")
	}

	// Because merges are computed for all delta files before anything is written,
	// the billing capability's spec.md must NOT have been created even though its
	// own delta was conflict-free — proving no partial merge across capabilities.
	if _, err := os.Stat(specShardPathIn(root, "billing")); !os.IsNotExist(err) {
		t.Error("billing spec.md should not have been written when a sibling capability conflicted")
	}
}

func TestSpecArchiveIn_IdempotentReArchiveAfterCrash(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)
	setupFeatureDelta(t, root, "feat-mfa", "auth", validAddedOnlyDelta)

	if _, err := specArchiveIn(root, "feat-mfa"); err != nil {
		t.Fatalf("first archive: unexpected error: %v", err)
	}

	// Simulate the crash window this fix targets: the shard write for a capability
	// succeeded but its delta's move to archived/ never completed (or the same
	// delta content is otherwise re-submitted). Recreate the pending delta and
	// re-archive — it must recover cleanly, not hit an "already exists" conflict.
	deltaPath := setupFeatureDelta(t, root, "feat-mfa", "auth", validAddedOnlyDelta)

	summary, err := specArchiveIn(root, "feat-mfa")
	if err != nil {
		t.Fatalf("re-archive after simulated crash should be a clean no-op, got error: %v", err)
	}
	if strings.Contains(summary, "already exists") {
		t.Errorf("summary should not report a conflict, got: %q", summary)
	}

	// The re-submitted delta must have been moved to archived/ again, not left
	// dangling and permanently unarchivable.
	if _, statErr := os.Stat(deltaPath); !os.IsNotExist(statErr) {
		t.Errorf("expected re-archived delta file to be moved, still present at %s", deltaPath)
	}

	// The shard must still contain exactly one "MFA Enrollment" requirement — the
	// re-archive must not have duplicated it.
	merged, err := os.ReadFile(specShardPathIn(root, "auth"))
	if err != nil {
		t.Fatalf("expected merged spec.md to exist: %v", err)
	}
	shard := parseSpecShard(string(merged))
	count := 0
	for _, r := range shard.Requirements {
		if r.Name == "MFA Enrollment" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 MFA Enrollment requirement after re-archive, got %d", count)
	}
}

func TestSpecArchiveIn_AddedDifferentBodyStillConflicts(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth", validSpecShard)
	setupFeatureDelta(t, root, "feat-mfa", "auth", validAddedOnlyDelta)

	if _, err := specArchiveIn(root, "feat-mfa"); err != nil {
		t.Fatalf("first archive: unexpected error: %v", err)
	}

	// A different delta that ADDs the same requirement name but with a different
	// body must still be a hard conflict — idempotency only covers byte-identical
	// re-adds, not arbitrary redefinition.
	differentBody := `## ADDED Requirements

### Requirement: MFA Enrollment

The system SHALL require MFA enrollment within 7 days of account creation.

#### Scenario: grace period expires

- **WHEN** 7 days pass without enrollment
- **THEN** the system SHALL lock the account
`
	setupFeatureDelta(t, root, "feat-mfa2", "auth", differentBody)

	_, err := specArchiveIn(root, "feat-mfa2")
	if err == nil {
		t.Fatal("expected a conflict error for ADDED requirement with a different body")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %v, want it to mention already exists", err)
	}
}

// ---------- helpers ----------

func hasErrorContaining(issues []validateIssue, substr string) bool {
	for _, iss := range issues {
		if iss.Severity == "error" && strings.Contains(iss.Message, substr) {
			return true
		}
	}
	return false
}

func hasWarningContaining(issues []validateIssue, substr string) bool {
	for _, iss := range issues {
		if iss.Severity == "warning" && strings.Contains(iss.Message, substr) {
			return true
		}
	}
	return false
}

func hasSeverity(issues []validateIssue, sev string) bool {
	for _, iss := range issues {
		if iss.Severity == sev {
			return true
		}
	}
	return false
}
