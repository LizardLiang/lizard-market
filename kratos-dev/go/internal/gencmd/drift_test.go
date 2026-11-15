package gencmd

import (
	"runtime"
	"testing"
)

// TestNoDrift guards against the same drift class the codegen exists to
// prevent: plugins/kratos/commands/<god>.md and skills/auto/SKILL.md's
// god-derived regions must always match what plugins/kratos/agents/*.md
// frontmatter (plus kratos-dev/codegen/partials/) would generate. Run this
// via `go test ./...` (already CI-gated) after any change to agents/*.md,
// commands/*.md, SKILL.md, or the partials — regenerate with
// `cd kratos-dev/go && make gen` if it fails.
func TestNoDrift(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to resolve this test file's path")
	}

	repoRoot, err := FindRepoRoot(thisFile)
	if err != nil {
		t.Fatalf("locating repo root: %v", err)
	}

	plan, err := BuildPlan(repoRoot)
	if err != nil {
		t.Fatalf("building plan: %v", err)
	}

	drift, err := plan.Check()
	if err != nil {
		t.Fatalf("checking drift: %v", err)
	}
	if len(drift) == 0 {
		return
	}

	t.Errorf("%d generated file(s) drifted from plugins/kratos/agents/*.md frontmatter; run `cd kratos-dev/go && make gen` and review the diff:", len(drift))
	for _, d := range drift {
		t.Errorf("  %s: %s", d.Reason, d.Path)
	}
}
