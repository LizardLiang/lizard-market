package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// specRoot resolves to the main checkout from inside a linked worktree, so a
// delta written in the main dir and code landed in the worktree share one
// .claude/.Arena (2026-10-25 16:28 manual cp/mv dance).
func TestSpecRootResolvesMainWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	main := t.TempDir()
	gitRun(t, main, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(main, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, main, "add", "a.txt")
	gitRun(t, main, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")
	wt := filepath.Join(t.TempDir(), "feature-wt")
	gitRun(t, main, "worktree", "add", "-q", "-b", "feat/x", wt)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(wt); err != nil {
		t.Fatal(err)
	}

	got := gitMainWorktreeRoot()
	want, _ := filepath.EvalSymlinks(main)
	gotEval, _ := filepath.EvalSymlinks(got)
	if !strings.EqualFold(filepath.ToSlash(gotEval), filepath.ToSlash(want)) {
		t.Fatalf("gitMainWorktreeRoot() from worktree = %q, want main checkout %q", got, main)
	}
	if r := specRoot(); !strings.EqualFold(filepath.ToSlash(mustEval(t, r)), filepath.ToSlash(want)) {
		t.Fatalf("specRoot() = %q, want %q", r, main)
	}

	t.Run("--arena-root overrides", func(t *testing.T) {
		old := specArenaRoot
		specArenaRoot = "D:/elsewhere"
		t.Cleanup(func() { specArenaRoot = old })
		if specRoot() != "D:/elsewhere" {
			t.Fatalf("specRoot() = %q", specRoot())
		}
	})

	t.Run("main checkout resolves to itself", func(t *testing.T) {
		if err := os.Chdir(main); err != nil {
			t.Fatal(err)
		}
		if r := gitMainWorktreeRoot(); !strings.EqualFold(filepath.ToSlash(mustEval(t, r)), filepath.ToSlash(want)) {
			t.Fatalf("gitMainWorktreeRoot() from main = %q, want %q", r, main)
		}
	})
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	e, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return e
}
