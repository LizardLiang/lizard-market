package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSessionEndReportsDraftPlans covers the durability reminder for abandoned
// /kratos:plan sessions.
//
// Odysseus creates no session, no feature dir and no status.json, so a plan-only
// session that dies mid-clarification is invisible to every other recall surface.
// The draft plan file is the only trace, and this reporter is what surfaces it.
//
// HOME/USERPROFILE are redirected at a temp dir so `~/.kratos/active-session.json`
// is guaranteed absent — the hook then takes its "no active session" branch and
// never touches the real memory DB.
func TestSessionEndReportsDraftPlans(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; session-end is a .cjs hook")
	}
	hook, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks", "session-end.cjs"))
	if err != nil {
		t.Fatalf("resolve hook path: %v", err)
	}

	work := t.TempDir()
	home := t.TempDir()
	planDir := filepath.Join(work, ".claude", ".Arena", "tactical-plans")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(planDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	write("2026-07-28-abandoned.md", `---
status: draft
started: 2026-07-28T10:00:00Z
---

> **DRAFT — clarification loop in progress. NOT ready for Ares.**

# Tactical Plan: Abandoned Thing

## Locked Decisions
- **masking approach** — Q: How should the non-target region be dimmed? → **A: overlay, not destructive edit**
- **scope** — Q: Multi-bill images only? → **A: yes**

## Decision Tree
Task: Abandoned Thing
`)

	write("2026-07-27-finished.md", `---
status: ready
started: 2026-07-27T09:00:00Z
completed: 2026-07-27T09:40:00Z
---

# Tactical Plan: Finished Thing

## Locked Decisions
- **thing** — Q: A or B? → **A: B**
`)

	write("legacy-no-frontmatter.md", "# Tactical Plan: Legacy\n\nWritten before drafts existed.\n")

	cmd := exec.Command(node, hook)
	cmd.Dir = work
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"USERPROFILE="+home,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	got := string(out)

	if !strings.Contains(got, "1 unfinished plan draft(s)") {
		t.Errorf("expected exactly one draft reported, got:\n%s", got)
	}
	if !strings.Contains(got, "2026-07-28-abandoned.md") {
		t.Errorf("draft plan not named in output:\n%s", got)
	}
	if !strings.Contains(got, "2 locked decisions") {
		t.Errorf("locked-decision count missing or wrong:\n%s", got)
	}
	if strings.Contains(got, "2026-07-27-finished.md") {
		t.Errorf("status: ready plan must not be reported as a draft:\n%s", got)
	}
	if strings.Contains(got, "legacy-no-frontmatter.md") {
		t.Errorf("frontmatter-less plan must not be reported as a draft:\n%s", got)
	}
	if !strings.Contains(got, "/kratos:plan") {
		t.Errorf("resume instruction missing:\n%s", got)
	}
}

// TestSessionEndSilentWithoutDrafts keeps the reminder from becoming noise on
// every single session stop.
func TestSessionEndSilentWithoutDrafts(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; session-end is a .cjs hook")
	}
	hook, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "plugins", "kratos", "hooks", "session-end.cjs"))
	if err != nil {
		t.Fatalf("resolve hook path: %v", err)
	}

	work := t.TempDir()
	home := t.TempDir()

	cmd := exec.Command(node, hook)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}

	if strings.Contains(string(out), "unfinished plan draft") {
		t.Errorf("reported drafts with no tactical-plans dir present:\n%s", out)
	}
}
