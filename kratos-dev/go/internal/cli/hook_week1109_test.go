package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Week of 2026-11-09 transcript review: four gate fixes.

func TestAresHandbackCap8000(t *testing.T) {
	if aresHandbackMaxChars != 8000 || hermesHandbackMaxChars != 8000 {
		t.Fatalf("caps = %d/%d, want 8000/8000", aresHandbackMaxChars, hermesHandbackMaxChars)
	}
	if _, ok := handbackSizeFailure(strings.Repeat("字", 5500), "ares"); ok {
		t.Fatal("5,500-rune Ares report must pass")
	}
	if _, ok := handbackSizeFailure(strings.Repeat("字", 9000), "ares"); !ok {
		t.Fatal("9,000-rune Ares report must fail")
	}
	if _, ok := handbackSizeFailure(strings.Repeat("x", 8001), "hermes"); !ok {
		t.Fatal("Hermes stays capped at 8,000")
	}
}

func TestAresLandedGateRepoSuffix(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	cwd, _ := gitRepo(t)
	other, _ := gitRepo(t)
	if l, err := filepath.EvalSymlinks(other); err == nil {
		other = l
	}
	gitIn := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", other}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.WriteFile(filepath.Join(other, "b.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn("add", "b.txt")
	gitIn("commit", "-q", "-m", "second")
	hash := gitIn("rev-parse", "--short", "HEAD")
	cases := []struct {
		name, report string
		wantFail     bool
	}{
		{"repo suffix in other repo", "Landed: feat/x@" + hash + " (repo " + other + ")", false},
		{"repo suffix forward slashes", "Landed: feat/x@" + hash + " (repo " + filepath.ToSlash(other) + ")", false},
		{"in form", "Landed: feat/x@" + hash + " in " + other, false},
		{"bare form absent from cwd", "Landed: feat/x@" + hash, true},
		{"suffix path lacks the commit", "Landed: feat/x@" + hash + " (repo " + cwd + ")", true},
		{"waiver still works", "LANDED-NOT-APPLICABLE: no edits", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := aresLandedGateFailure(tc.report, cwd)
			if (got != "") != tc.wantFail {
				t.Fatalf("failure = %q, wantFail %v", got, tc.wantFail)
			}
		})
	}
}

func TestTestCmdREBun(t *testing.T) {
	yes := []string{"bun run test", "bun run test src/lib/x", "E2E_WORKERS=1 bun run test src/components", "bun test", "bunx playwright test", "cd x && bun run test"}
	no := []string{"bun run build", "bun run testing", "bun add test-utils"}
	for _, c := range yes {
		if !testCmdRE.MatchString(c) {
			t.Errorf("%q should match", c)
		}
	}
	for _, c := range no {
		if testCmdRE.MatchString(c) {
			t.Errorf("%q should not match", c)
		}
	}
}

func TestOdysseusReadOnlyWeek1109(t *testing.T) {
	allow := []string{
		`f=$(grep X dir); sed -n 1,5p "$f"`,
		`f=$(grep -rl X dir | head -1); sed -n 1,5p "$f"; grep -n y "$f"`,
		"f=`ls`; head -3 \"$f\"",
		`d=$(ls); echo $d`,
		`fd -e tsx . src`,
		`awk '$1>10' file`,
		`sed 's/a/b/' file`,
		`sed -n 1,5p file`,
		`git stash list`,
		`git stash show -p`,
	}
	deny := []string{
		`f=$(grep X); sed -i s/a/b/ "$f"`,
		`f=$(rm -rf x); echo $f`,
		`f=$(ls); $f`,
		`f=$(ls); find . $f`,
		`f=$(ls); sed -n $f x`,
		`f=$(ls); read f; echo $f`,
		`f=$(ls) | echo $f`,
		`cat >> x.md`,
		`cat > x.md`,
		`git switch -c y`,
		`git checkout -b y`,
		`mkdir d && cat > d/x`,
		`git fetch`,
		`fd -x rm {}`,
		`fd --exec rm`,
		`fd -X rm`,
		`awk -i inplace '{print}' f`,
		`awk '{print > "x"}' f`,
		`sed -i s/a/b/ f`,
		`git stash`,
		`git stash drop`,
		`python -c "print(1)"`,
	}
	for _, c := range allow {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q (%s), want allow", c, got.Decision, got.Reason)
		}
	}
	for _, c := range deny {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
}

func TestWeek1109ReachFixes(t *testing.T) {
	for _, c := range []string{`f=$(grep X d); sed "$f"`, `f=$(grep X d); sed -n -e "$f"`, `f=$(grep X d); sed -n -f "$f"`, `f=$(ls); file -C $f`} {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
	cwd, _ := gitRepo(t)
	for _, r := range []string{"Landed: main@abc1234 in 3 files", "Landed: main@abc1234 in the repo", "Landed: main@abc1234 (repo /nonexistent/x)", "Landed: main@abc1234 (repo ../x)"} {
		if aresLandedGateFailure(r, cwd) == "" {
			t.Errorf("%q passed, want failure", r)
		}
	}
}
