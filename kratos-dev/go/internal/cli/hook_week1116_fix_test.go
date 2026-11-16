package cli

import (
	"strings"
	"testing"
)

// Fix round after the Hermes review of the week-2026-11-16 branch. Each table
// row failed before the fix and passes after it.

func TestAresOwnWorktreeFailClosed(t *testing.T) {
	const main = "C:/m"
	const agent = main + "/.claude/worktrees/agent-a1"
	const other = "C:/other/.claude/worktrees/agent-zzz"
	cases := []struct {
		name, cwd, cmd string
		deny           bool
	}{
		{"redirect on the cd", agent, `cd C:/m 2>/dev/null && git reset --hard a`, true},
		{"export GIT_DIR", agent, `export GIT_DIR=C:/m/.git; git reset --hard a`, true},
		{"env GIT_DIR in PowerShell", agent, `$env:GIT_DIR="C:/m/.git"; git reset --hard a`, true},
		{"GIT_WORK_TREE in PowerShell", agent, `$env:GIT_WORK_TREE = "C:/m"; git reset --hard a`, true},
		{"Push-Location out of the worktree", agent, `Push-Location C:/m; git reset --hard a`, true},
		{"subshell cd into the worktree from the main checkout", main, `(cd ` + agent + `) && git reset --hard a`, true},
		{"another agent worktree", main, `cd ` + other + ` && git reset --hard a`, true},
		{"another agent worktree from the own worktree", agent, `cd ` + other + ` && git reset --hard a`, true},
		{"main cwd that cds into an agent worktree", main, `cd ` + agent + ` && git reset --hard a`, true},
		{"git -C into the main checkout, then reset", agent, `git -C C:/m status; git reset --hard a`, true},
		{"second -C leaves the worktree", agent, `git -C ` + agent + ` -C C:/m reset --hard a`, true},
		{"pipe then cd out", agent, `ls | cd C:/m; git reset --hard a`, true},
		{"cd out after a failed branch", agent, `false || cd C:/m; git reset --hard a`, true},
		{"cd through a variable", agent, `cd $M && git reset --hard a`, true},
		{"cd through builtin", agent, `builtin cd C:/m && git reset --hard a`, true},
		{"cd through eval", agent, `eval "cd C:/m"; git reset --hard a`, true},
		{"variable head", agent, `$c C:/m; git reset --hard a`, true},
		{"extra argument on the cd", agent, `cd ` + agent + ` C:/m && git reset --hard a`, true},
		{"dotdot out of the worktree", agent, `cd src && cd ../../../.. && git reset --hard a`, true},
		{"drive-relative path", agent, `cd C:foo && git reset --hard a`, true},
		{"stash in the worktree", agent, `git stash`, true},
		{"clean in the worktree", agent, `git clean -fd`, true},

		{"cwd is the worktree", agent, `git reset --hard abc`, false},
		{"cd to a subdirectory of the same worktree", agent, `cd ` + agent + `/src/x && git checkout -- f`, false},
		{"relative cd inside", agent, `cd src/apps/web && git reset --hard abc`, false},
		{"git -C the same worktree", agent, `git -C ` + agent + `/src reset --hard abc`, false},
		{"cwd deep inside the worktree", agent + "/src/apps", `git restore f.ts`, false},
		{"Git Bash spelling of the same worktree", "/c/m/.claude/worktrees/agent-a1", `cd C:/m/.claude/worktrees/agent-a1/src && git reset --hard abc`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := aresBashGate(tc.cmd, tc.cwd)
			if tc.deny && got.Decision != "deny" {
				t.Errorf("decision %q, want deny", got.Decision)
			}
			if !tc.deny && got.Decision != "" {
				t.Errorf("decision %q (%s), want pass", got.Decision, got.Reason)
			}
		})
	}
}

func TestOdysseusNestedSubstitutionInQuotes(t *testing.T) {
	allow := []string{
		`echo "sha: $(git rev-parse HEAD) ok" >> /tmp/claude/s/notes.txt`,
		`echo "$(date) x" > /dev/null`,
		`echo "a $(git log --oneline -1) b"`,
		"echo \"a `git rev-parse HEAD` b\"",
		`echo "$(cat a | wc -l) lines" && ls`,
		`echo "$(echo "inner $(pwd)") end"`,
	}
	for _, c := range allow {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q, want allow", c, got.Decision)
		}
	}
	deny := []string{
		`echo "$(cat a > b)"`,
		`echo "$(sed -i s/a/b/ f)"`,
		`echo "$(find . -delete)"`,
		`echo "$(git log --output=x)"`,
		`x="$(cat a >b)"`,
		`echo "sha: $(git rev-parse HEAD) ok" >> notes.txt`,
		`echo "$(date) x" > out.txt`,
		"echo \"a `rm x` b\"",
		`echo "$(echo "inner $(rm x)") end"`,
		`echo "$(rm x)"`,
	}
	for _, c := range deny {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
}

func TestMaskQuotedNestedKeepsLength(t *testing.T) {
	for _, c := range []string{
		`echo "sha: $(git rev-parse HEAD) ok" >> f`,
		`echo "$(cat a > b)" 'x $(y)' "z`,
		"echo \"`a`\" \"\\\"$(b)\\\"\"",
		`echo "$(`,
		`"`,
		``,
	} {
		if got := maskQuotedNested(c); len(got) != len(c) {
			t.Errorf("%q: masked length %d, want %d", c, len(got), len(c))
		}
	}
	if got := maskQuotedNested(`echo "a $(cat x > y) b"`); !strings.Contains(got, "cat x > y") {
		t.Errorf("code inside the substitution was masked: %q", got)
	}
	if got := maskQuotedNested(`echo "a ; b"`); strings.Contains(got, ";") {
		t.Errorf("quoted text was not masked: %q", got)
	}
}

func TestTrimPairMismatchFailsClosed(t *testing.T) {
	if _, _, ok := trimPair("abc ", "ab"); ok {
		t.Error("a raw/masked pair of different length must be unusable")
	}
	if kind := shellSegmentWrite(shellSegment{raw: "echo hi", masked: "echo"}, 0, false); kind != shellWriteFound {
		t.Errorf("a misaligned segment must classify as a write, got %v", kind)
	}
	r, m, ok := trimPair("  ab \n", "  xx \n")
	if !ok || r != "ab" || m != "xx" {
		t.Errorf("trimPair = %q %q %v", r, m, ok)
	}
}

func TestOdysseusClassifierPanicDeniesWithShortReason(t *testing.T) {
	old := odysseusShellClassifier
	defer func() { odysseusShellClassifier = old }()
	odysseusShellClassifier = func(string) shellWriteKind { panic("boom") }
	got := odysseusBashDecision("ls")
	if got.Decision != "deny" || got.Reason != odysseusClassifierFailedReason {
		t.Errorf("decision %q reason %q, want a deny with the short reason", got.Decision, got.Reason)
	}
	if strings.Contains(got.Reason, "goroutine") || len(got.Reason) > 200 {
		t.Errorf("reason is not short: %q", got.Reason)
	}
}

func TestOdysseusWrapperFlags(t *testing.T) {
	deny := []string{
		`env -i rm x`, `sudo -u root rm x`, `command -p rm x`, `time -p rm x`, `nice rm x`,
		`sudo -E rm x`, `nice -n 5 rm x`, `ionice -c 3 rm x`, `setsid rm x`,
	}
	for _, c := range deny {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
	for _, c := range []string{`env FOO=1 ls`, `time ls`, `nice ls`, `env FOO=1 BAR=2 git log --oneline -3`, `command -v git`} {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q, want allow", c, got.Decision)
		}
	}
}

func TestOdysseusGitOptionsThatRunCode(t *testing.T) {
	deny := []string{
		`git -c core.pager='rm -rf x' log`,
		`git -c alias.x='!rm y' x`,
		`git -ccore.pager=x log`,
		`git --config-env=core.pager=P log`,
		`git --exec-path=/evil log`,
		`git submodule foreach 'rm y'`,
		`git submodule --quiet foreach ls`,
	}
	for _, c := range deny {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
	for _, c := range []string{`git --no-pager log`, `git -C /repo log --oneline -3`, `git -C /repo --no-pager show HEAD`, `git submodule status`} {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q, want allow", c, got.Decision)
		}
	}
}

func TestOdysseusScriptHeads(t *testing.T) {
	deny := []string{
		`python3.12 x.py`, `python3.9 -c "print(1)"`, `py x.py`, `ruby3.2 x.rb`,
		`xargs --max-args 1 rm x`, `xargs --max-procs 4 rm x`, `xargs -n1 rm x`,
		`./build.sh`, `bash build.sh`, `sh x.sh`, `pwsh x.ps1`, `.\x.ps1`, `../tools/run`, `C:/x/run.bat`,
		`./node_modules/.bin/tsc`,
	}
	for _, c := range deny {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
	for _, c := range []string{
		`python --version`, `python3.12 --version`,
		`/home/u/.kratos/bin/kratos.exe pipeline get --feature x`,
		`./plugins/kratos/bin/kratos pipeline get --feature x`,
		`node /home/u/.claude/plugins/cache/lizard-plugins/kratos/2.127.0/hooks/launch.cjs agent load ares --part body`,
		`xargs --max-args 1 cat`, `/usr/bin/git log`,
	} {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q, want allow", c, got.Decision)
		}
	}
}

func TestOdysseusCheapSuggestions(t *testing.T) {
	deny := []string{
		`export _f=rm; $_f x`,
		`curl -D h.txt http://x`, `curl -c jar http://x`, `curl --trace t.txt http://x`, `curl -o f http://x`,
		`curl -O http://x/f`, `curl --output=f http://x`, `curl --dump-header h http://x`, `curl --cookie-jar j http://x`,
		`iwr http://x -OutF x`, `irm http://x -Meth Post`, `irm http://x -Method:Post`, `Invoke-RestMethod http://x -Method:Delete`,
		`irm http://x -Bod x`, `iwr http://x -CustomMethod PATCH`, `iwr http://x -InF f`,
		`ac out.txt hi`, `spps -Name x`, `epcsv out.csv`,
		`echo x > tmp/claude/x.go`, `echo x > ./tmp/claude/x.go`, `echo x | tee tmp/claude/x.go`,
	}
	for _, c := range deny {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
	for _, c := range []string{
		`curl -s http://localhost:5176/health`, `curl -sS -I http://x`, `irm http://x`, `irm http://x -Method Get`,
		`iwr http://x -Method:Get`, `Get-ChildItem | ForEach-Object { $_ }`, `echo x > /tmp/claude/s/x.txt`,
		`echo x > C:/Users/u/AppData/Local/Temp/claude/a/b/x.txt`,
	} {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q, want allow", c, got.Decision)
		}
	}
}

// A line of thousands of assignments rescanned every later segment per
// assignment: 96 KB took 54 s. The cap turns it into one linear pass and a
// deny.
func TestOdysseusAssignmentCap(t *testing.T) {
	big := strings.Repeat("D=-x;", 20000) + "ls"
	if len(big) < 96000 {
		t.Fatalf("test input is %d bytes, want 96 KB", len(big))
	}
	if got := odysseusBashDecision(big); got.Decision != "deny" {
		t.Errorf("a command of %d assignments: decision %q, want deny", gateMaxAssignments+1, got.Decision)
	}
	over := strings.Repeat("A=1;", gateMaxAssignments+1) + "ls"
	if n := countAssignmentSegments(mustSegs(over)); n != gateMaxAssignments+1 {
		t.Errorf("counted %d assignments, want %d", n, gateMaxAssignments+1)
	}
	if got := odysseusBashDecision(over); got.Decision != "deny" {
		t.Errorf("just over the cap: decision %q, want deny", got.Decision)
	}
	atCap := strings.Repeat("A=1;", gateMaxAssignments) + "ls $A"
	if got := odysseusBashDecision(atCap); got.Decision != "" {
		t.Errorf("at the cap: decision %q (%s), want allow", got.Decision, got.Reason)
	}
}

func mustSegs(c string) []shellSegment {
	segs, _, _ := flattenShellSegments(c)
	return segs
}
