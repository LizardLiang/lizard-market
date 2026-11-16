package cli

import (
	"strings"
	"testing"
)

// Replays of the gate denials from the week of 2026-11-09 to 11-16 (this
// machine's transcripts), with user paths made neutral. Each table holds the
// real command that was denied and the outcome the new rule must give.

func TestOdysseusKnownWritesWeek1116(t *testing.T) {
	allow := []struct{ name, cmd string }{
		{"loop over dirs with command substitution in echo", `cd /repo && ls .claude/.Arena/tactical-plans/ 2>/dev/null | tail -5; grep -l 'status: draft' .claude/.Arena/tactical-plans/*.md 2>/dev/null; ls .claude/.Arena/flows/ 2>/dev/null; echo ---; for d in electron/main src/renderer; do echo "$d: $(git ls-files $d | grep -vE '__tests__|\.test\.' | grep -E '\.(ts|tsx|mjs)$' | xargs cat 2>/dev/null | wc -l)"; done; echo ---; wc -l src/a.ts src/b.ts`},
		{"git -C with a variable repo", `R=/c/work/repo; git -C $R show --stat --format="%h %ad %s" --date=short ca91b45 | head -12; git -C $R branch --contains ca91b45 | head -5; git -C $R log --oneline -1 feature/x; git -C $R branch | head -8`},
		{"file captured with substitution then grep", `f=$(ls m/Migrations/*AddCol.cs 2>/dev/null || find modules -name "*AddCol.cs" | head -1); grep -n -E "AddColumn|name:|table:" $f | head -30; echo ---; git log --oneline -15 | cat`},
		{"head -c with arithmetic", `cd /repo/proto; grep -obF "function bb(" a.html; o=$(grep -obF "function bb(" a.html | cut -d: -f1); head -c $((o+700)) a.html | tail -c 700; echo; echo "== report"; head -c 3039300 a.html | tail -c 1700`},
		{"grep with a backtick inside the quotes", "ls submit; grep -n 'useCtx\\|isLocked' submit/*.tsx | head; grep -rn \"navigate(\\`/annual/\\|to={\\`/annual/\" ../ --include=*.tsx | grep -v test | head"},
		{"for loop with grep and sed", `cd /repo/src && for f in A.cs B.cs C/D.cs; do echo "== $f"; grep -E "public [A-Za-z<>?]+ [A-Za-z]+ \{" $f | sed 's/^ *//' | tr '\n' ' '; echo; done; ls ../Models 2>/dev/null | head`},
		{"for loop over ls output with sed on the match", `ls -t .claude/.Arena/tactical-plans/ 2>/dev/null | head -5; for f in $(ls -t .claude/.Arena/tactical-plans/*.md 2>/dev/null | head -5); do grep -m1 '^status:' "$f" | sed "s|^|$f: |"; done; ls .claude/.Arena/specs/ 2>/dev/null | head -30`},
		{"directory captured then listed and read", `ls .claude/feature/ | tail -5; f=$(ls -d .claude/feature/*/spec-delta 2>/dev/null | tail -1); ls $f; head -40 $f/*.md | head -50`},
		{"rg capture then grep", `cd /repo; f=$(rg -l "class BillParser" src); echo $f; grep -n "Normalize\|Confidence\." $f | head -40`},
		{"verify landed with git reads", `/home/u/.kratos/bin/kratos.exe verify --landed --hash b248ba0; git log --oneline 693d16e..HEAD; git show --stat 42c6d96 | head -30; git log --oneline -3 -- src/apps/web/src/*ocument*`},
		{"verify landed then status and log", `cd /repo; /home/u/.kratos/bin/kratos.exe verify --landed --hash 308e7c3; git status --short | head; git log --oneline -4`},
		{"verify landed in two worktrees", `cd /repo-wt/fix-a && git log --oneline b0286f4..HEAD && git status --short && git diff --stat b0286f4..HEAD && /home/u/.kratos/bin/kratos.exe verify --landed --hash f6b2a76 2>&1 | tail -5; cd ../fix-b && git log --oneline -1 && git status --short`},
		{"grep chain across a loop result", `cd /repo/src; grep -rn "CancelPendingAsync" --include=*.cs modules | grep -v Tests | head -5; f=$(grep -rln "class .*CancelStore\b\|: ICancelStore" --include=*.cs modules | head -1); echo $f; grep -n "WorkType\|Status" $f | head -30; sed -n 1,60p modules/Handler.cs`},
		{"read the browser tool output", `T=/tmp/claude/x/tasks; tail -15 $T/a.output | cut -c1-300; echo ====; tail -30 $T/b.output | grep -a -E 'Login|ERR|WRN' | cut -c1-300 | tail -10`},
		{"browser read verbs with a scratchpad screenshot", `S="C:/Users/u/AppData/Local/Temp/claude/abc/def/scratchpad"; agent-browser --session proto snapshot; agent-browser --session proto eval "document.title"; agent-browser --session proto screenshot --full "$S/list.png" >/dev/null; agent-browser --session proto get text main`},
		{"PowerShell resolves chrome and opens the plan page", `$c = @("$env:ProgramFiles\Google\Chrome\Application\chrome.exe", "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe", "$env:LOCALAPPDATA\Google\Chrome\Application\chrome.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1; if ($c) { Start-Process $c "file:///C:/Users/u/proj/.claude/.Arena/tactical-plans/2026-11-09-inv08.html"; "opened with $c" } else { "chrome not found" }`},
		{"kratos open on the plan page", `/home/u/.kratos/bin/kratos.exe open /repo/.claude/.Arena/tactical-plans/2026-11-16-x.html`},
		{"kratos open on the plan page, Windows path", `"C:/Users/u/.kratos/bin/kratos.exe" open C:/repo/.claude/.Arena/tactical-plans/2026-11-16-x.html`},
	}
	for _, tc := range allow {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			if got := odysseusBashDecision(tc.cmd); got.Decision != "" {
				t.Errorf("decision %q (%s), want allow", got.Decision, got.Reason)
			}
		})
	}

	deny := []struct{ name, cmd string }{
		{"cherry-pick", `git status --short | head -5; git cherry-pick 0a32165 dee89ab 2>&1 | tail -2; git log --oneline -3`},
		{"curl POST", `tail -15 /tmp/claude/x/a.output | cut -c1-300; echo ====; curl -s -X POST http://localhost:5176/api/Login -H 'Content-Type: application/json' -d '{"u":"a"}' -w '\nHTTP %{http_code}\n' | head -c 300`},
		{"Stop-Process then Start-Process pwsh", `Get-Process -Id 36736 -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue; $cmd = "Set-Location 'C:\repo'; node ui.mjs login"; $p = Start-Process pwsh -ArgumentList '-NoExit','-Command',$cmd -PassThru; "PID $($p.Id)"`},
		{"python -c", `cd /tmp/claude/x && PYTHONIOENCODING=utf-8 python -I -c "
import json,sys
d=json.load(open(r'C:\x\board.txt',encoding='utf-8'))
print(list(d.keys()))
"`},
		{"python -c after a grep chain", `cd /tmp/claude/x/scratchpad && grep -rhoa "https://host[^ \"']*" *.md *.log 2>/dev/null | sort -u | head; PYTHONIOENCODING=utf-8 python -c "import re,sys;print(1)"`},
		{"python script with output redirect", `cd /tmp/claude/x/scratchpad && python -u open_cloak.py "https://host/Prototype/" > cloak-run.log 2>&1`},
		{"python from a heredoc", "python - <<'EOF'\np='.claude/.Arena/flows/x.md'\ns=open(p,encoding='utf-8').read()\nEOF"},
		{"python -c with a here-doc style script", `python -I -c "
import json,sys
sys.stdout.reconfigure(encoding='utf-8')
t=[x for x in d['tables'] if x['name']=='EmReport'][0]
print(t['id'])
"`},
		{"browser press, fill and click through a variable", `A="agent-browser --session proto"; $A press Escape >/dev/null; $A eval "document.title" >/dev/null; $A wait 400 >/dev/null; $A fill "input[placeholder='q']" "x" >/dev/null; $A click "button"`},
		{"browser press", `agent-browser --session proto press Escape >/dev/null; agent-browser --session proto eval "1"; agent-browser --session proto wait 800 >/dev/null`},
		{"rm in a loop body", `for f in a b c; do rm -f $f; done`},
		{"rm in a command substitution", `echo $(rm -rf build)`},
		{"mv after an and-chain", `ls && mv a b`},
		{"redirect to a project file", `echo hi > notes.txt`},
		{"npm run", `cd web && npm run build`},
		{"go test", `go test ./...`},
		{"sed in place", `sed -i s/a/b/ f.txt`},
		{"git push", `git push origin main`},
		{"git checkout", `git checkout -b x`},
		{"Out-File", `Get-ChildItem | Out-File list.txt`},
		{"Remove-Item in a script block", `Get-ChildItem . | ForEach-Object { Remove-Item $_ }`},
		{"curl with data", `curl -d x http://localhost`},
		{"node -e", `node -e "console.log(1)"`},
		{"bash -c", `bash -c "ls"`},
		{"dotnet build", `dotnet build`},
		{"kratos pipeline update", `/home/u/.kratos/bin/kratos.exe pipeline update --feature x --stage 7 --status complete`},
		{"kratos open outside the plan root", `/home/u/.kratos/bin/kratos.exe open /repo/dist/index.html`},
		{"Start-Process a non-plan page", `Start-Process C:/repo/dist/index.html`},
	}
	for _, tc := range deny {
		t.Run("deny/"+tc.name, func(t *testing.T) {
			if got := odysseusBashDecision(tc.cmd); got.Decision != "deny" {
				t.Errorf("decision %q, want deny", got.Decision)
			}
		})
	}
}

// A redirect or heredoc into a plan, flow or spec-delta file is a write the
// planner has a tool for; the deny text says which.
func TestOdysseusDocRedirectDenyNamesWriteOrEdit(t *testing.T) {
	cmds := []string{
		"cat >> .claude/.Arena/flows/annual-report.md <<'EOF'\n\n## State\n\n| a | b |\nEOF",
		"cat >> .claude/.Arena/flows/kpim.md <<'EOF'\n## Refill updates the row\n- text\nEOF",
		"mkdir -p .claude/feature/2026-11-12-x/spec-delta && cat > .claude/feature/2026-11-12-x/spec-delta/doc.md <<'EOF'\n## ADDED Requirements\nEOF",
		`echo x > .claude/.Arena/tactical-plans/p.md`,
	}
	for _, c := range cmds {
		got := odysseusBashDecision(c)
		if got.Decision != "deny" || !strings.Contains(got.Reason, "Write or Edit") {
			t.Errorf("%q: decision %q reason %q, want deny naming the Write or Edit tool", c, got.Decision, got.Reason)
		}
	}
	if got := odysseusBashDecision(`rm -rf build`); got.Decision != "deny" || strings.Contains(got.Reason, "Write or Edit tool for this file") {
		t.Errorf("a plain write must keep the generic reason, got %q", got.Reason)
	}
}

func TestOdysseusScratchpadRedirectsStayAllowed(t *testing.T) {
	for _, c := range []string{
		`ls > /dev/null`,
		`ls 2>&1 | head`,
		`echo x > /c/Users/u/AppData/Local/Temp/claude/abc/def/scratchpad/out.txt`,
		`ls > $null`,
	} {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q, want allow", c, got.Decision)
		}
	}
}

func TestAresOwnWorktreeGitWeek1116(t *testing.T) {
	const main = "C:/Users/u/work/proj"
	const agent = main + "/.claude/worktrees/agent-a0c9cedb67ca35904"
	cases := []struct {
		name, cwd, cmd string
		deny           bool
	}{
		{"reset hard after cd to the agent worktree", main, `cd ` + agent + ` && git reset --hard 3923a3b -q && git log -1 --format=%h && ls src/`, false},
		{"reset hard with the agent worktree as cwd", agent, `git cat-file -t 3923a3b && git reset --hard 3923a3b && git log -1 --oneline`, false},
		{"reset keep with the agent worktree as cwd", agent, `git reset --keep 3923a3b; git log -1 --format=%h`, false},
		{"checkout files after a quoted cd into a subdirectory", main, `cd "` + agent + `/src/apps/web"; git checkout -- src/pages/Home.tsx test/home.test.tsx 2>&1; git status --porcelain | head`, false},
		{"reset hard through git -C", main, `git -C ` + agent + ` reset --hard abc123`, false},
		{"restore in the agent worktree", agent, `git restore src/a.ts`, false},
		{"relative cd stays inside the agent worktree", agent, `cd src/apps/web && git reset --hard abc123`, false},

		{"reset hard in the main checkout", main, `git reset --hard abc123`, true},
		{"reset hard with no cwd", "", `git reset --hard abc123`, true},
		{"checkout files in the main checkout", main, `git checkout -- src/a.ts`, true},
		{"restore in the main checkout", main, `git restore src/a.ts`, true},
		{"clean in the agent worktree", agent, `git clean -n src/apps/web`, true},
		{"clean after cd into a merge worktree", main, `cd ` + main + `-wt/merge-dctt; git clean -n src/apps/web; rm -f x.js`, true},
		{"stash in the agent worktree", agent, `git stash`, true},
		{"detach in a scratchpad worktree", main, `cd C:/Users/u/AppData/Local/Temp/claude/abc/def/scratchpad/wt && git checkout -q --detach 1187e88 2>&1 | tail -1`, true},
		{"detach in the agent worktree", agent, `git checkout -q --detach 1187e88`, true},
		{"switch branch in the agent worktree", agent, `git checkout main`, true},
		{"git -C out of the agent worktree", agent, `git -C ` + main + ` reset --hard abc123`, true},
		{"cd out of the agent worktree", agent, `cd ` + main + ` && git reset --hard abc123`, true},
		{"dotdot out of the agent worktree", agent, `cd ../../.. && git reset --hard abc123`, true},
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

// Four hand-backs this week were denied for the punctuation after the waiver
// token, not for a missing waiver.
func TestEvidenceWaiverPunctuationWeek1116(t *testing.T) {
	pass := []string{
		"Evidence: EVIDENCE-SKIPPED for the style-only items (2, 5, 8)",
		"The suite ran green, 193 passed. RED half: EVIDENCE-SKIPPED, the new APIs did not exist before",
		"Evidence:\n- Item 1: EVIDENCE-SKIPPED, a removal of behavior with its tests.",
		"Evidence: EVIDENCE-SKIPPED - docs only",
		"Evidence: EVIDENCE-SKIPPED — config rename",
		"Evidence: EVIDENCE-SKIPPED: reason",
		"TESTS-NOT-APPLICABLE, docs only",
		"TESTS-NOT-APPLICABLE for the comment change",
		"TESTS-NOT-APPLICABLE — comment change",
		"TESTS-NOT-APPLICABLE - comment change",
		"TESTS-NOT-APPLICABLE: comment change",
	}
	for _, r := range pass {
		if !evidenceWaiverRE.MatchString(r) {
			t.Errorf("%q: evidence waiver not recognized", r)
		}
	}
	if !testsNotApplicableRE.MatchString("TESTS-NOT-APPLICABLE, docs only") {
		t.Error("testsNotApplicableRE misses the comma form")
	}
	fail := []string{
		"Evidence: tests pass.",
		"Tests written: 4. All passing.",
		"the evidence was skipped and tests are not applicable",
		"see EVIDENCE-SKIPPEDX",
	}
	for _, r := range fail {
		if evidenceWaiverRE.MatchString(r) {
			t.Errorf("%q: recognized as a waiver", r)
		}
	}
}

func TestHandbackCapIs10000Week1116(t *testing.T) {
	if _, ok := handbackSizeFailure(strings.Repeat("x", 10000), "ares"); ok {
		t.Error("a 10,000-char Ares report must pass")
	}
	if _, ok := handbackSizeFailure(strings.Repeat("x", 10001), "ares"); !ok {
		t.Error("a 10,001-char Ares report must fail")
	}
	if _, ok := handbackSizeFailure(strings.Repeat("x", 9500), "hermes"); ok {
		t.Error("a 9,500-char Hermes report must pass")
	}
}

// Fresh-context review of the first cut: each form below got past the gate.
func TestOdysseusKnownWritesReachReview(t *testing.T) {
	deny := []string{
		"cat <<EOF\n$(rm -rf x)\nEOF",
		"cat <<EOF\n`rm x`\nEOF",
		"echo $((1<<n))\nrm file",
		`echo ${x:-$(rm y)}`,
		`echo x > nul.txt`,
		`echo x >/dev/null.bak`,
		`echo x > /tmp/claude/../../etc/x`,
		`Out-File -FilePath:out.txt`,
		`Set-Content -Path:a.txt -Value:x`,
		`Export-Csv out.csv`,
		`xcopy a b`,
		`npx rimraf build`,
		`export PATH=/evil:$PATH`,
		`git update-index --assume-unchanged f`,
		`Start-Process https://evil/x/.claude/.Arena/tactical-plans/a.html`,
	}
	for _, c := range deny {
		if got := odysseusBashDecision(c); got.Decision != "deny" {
			t.Errorf("%q: decision %q, want deny", c, got.Decision)
		}
	}
	for _, c := range []string{"echo $((1<<3))", "cat <<'EOF'\n$(rm x)\nEOF"} {
		if got := odysseusBashDecision(c); got.Decision != "" {
			t.Errorf("%q: decision %q, want allow", c, got.Decision)
		}
	}
}

func TestAresOwnWorktreeReachReview(t *testing.T) {
	const main = "C:/Users/u/work/proj"
	const agent = main + "/.claude/worktrees/agent-a1"
	reset := "git re" + "set --hard"
	for _, c := range []string{
		"git --git-dir=.git " + reset[4:],
		"git --work-tree=" + main + " " + reset[4:],
		"cd $MAIN && " + reset,
		"cd ~ && " + reset,
		"cd - && " + reset,
		"cd && " + reset,
		"pushd " + main + " && " + reset,
		"Set-Location " + main + "; " + reset,
		"(cd " + main + " && " + reset + ")",
		"git -C $MAIN " + reset[4:],
		"GIT_DIR=" + main + "/.git " + reset,
	} {
		if got := aresBashGate(c, agent); got.Decision != "deny" {
			t.Errorf("%q from the agent worktree: decision %q, want deny", c, got.Decision)
		}
	}
	for _, c := range []string{"git --no-pager " + reset[4:], "git --git-dir=.git " + reset[4:], "git -c core.x=1 " + reset[4:]} {
		if got := aresBashGate(c, main); got.Decision != "deny" {
			t.Errorf("%q from the main checkout: decision %q, want deny", c, got.Decision)
		}
	}
	if got := aresBashGate("git -c core.x=1 "+reset[4:]+" abc", agent); got.Decision != "" {
		t.Errorf("git -c reset inside the agent worktree: decision %q, want pass", got.Decision)
	}
}
