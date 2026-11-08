package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The four compound inspection commands the Odysseus gate denied in the week
// of 2026-11-01 (binary v2.121.1 / v2.124.0), verbatim from the transcripts.
// Each opened with a quoted `VAR="…"` assignment, or carried one mid-line, and
// each passed once the planner retyped it without the variable.
func TestOdysseusReadOnlyWeek1102(t *testing.T) {
	legit := []string{
		// 2026-11-05 12:35
		`F="D:/Programing/Works/TheVegan/leweigen-erp-v2/.claude/.Arena/tactical-plans/2026-11-05-2078-manual-confirm-before-ecount-stock-count.html"; grep -c "<pre" "$F"; grep -c 'class="flow"\|class="branches"\|\.arrow::after\|核准並送出' "$F"; grep -n "^footer{" "$F"`,
		// 2026-11-05 12:42
		`C:/Users/shotu/.kratos/bin/kratos.exe template get ares-plan-mission 2>&1 | head -60; grep -n "Remove the code that only existed\|Remove post_opening\|nightly no longer calls post" D:/Programing/Works/TheVegan/leweigen-erp-v2/.claude/.Arena/tactical-plans/2026-11-05-plan.md | head -69`,
		// 2026-11-06 06:04
		`ls D:/Programing/Works/TheVegan/leweigen-erp-v2/.claude/.Arena/specs/ D:/Programing/Works/TheVegan/leweigen-erp-v2/.claude/.Arena/flows/ | head -30; W=D:/Programing/Works/TheVegan/leweigen-erp-v2/.claude/worktrees/wt-ecount-create-bg; grep -n "class WarehouseRefreshState" -A25 $W/backend/app/models/warehouse_mapping.py | head -80`,
		// 2026-11-07 05:01
		`R="D:/Programing/Works/TheVegan/leweigen-erp-v2"; grep -l "status: draft" $R/.claude/.Arena/tactical-plans/*.md 2>/dev/null; grep -liE "組合商品|combo|PROD_TYPE|品項類型" $R/.claude/.Arena/tactical-plans/*.md 2>/dev/null | head; ls $R/.claude/.Arena/flows/ $R/.claude/.Arena/specs/ 2>/dev/null`,
		// single-quoted value, Windows backslashes, a reference inside quotes
		`F='C:\repo\plan.md'; wc -l "$F"; head -5 "$F"`,
		// a flag spelled like the variable name is not a reuse of the name
		`R=D:/repo; grep -R "x" $R/src | head; grep -F "y" $R/src | head`,
		// PowerShell pipeline readers without a script block
		`Get-ChildItem D:/repo -Filter *.md | Select-Object -First 5 Name, Length`,
		`Get-Content D:/repo/a.log | Measure-Object -Line`,
		`Get-ChildItem D:/repo | Sort-Object LastWriteTime | Format-Table Name -AutoSize`,
		// opening the plan page
		`Start-Process D:/repo/.claude/.Arena/tactical-plans/2026-11-07-x.html`,
		`Start-Process -FilePath "D:/repo/.claude/.Arena/tactical-plans/2026-11-07-x.html"`,
		`cmd /c start "" "D:\repo\.claude\.Arena\tactical-plans\2026-11-07-x.html"`,
		`xdg-open .claude/.Arena/tactical-plans/x.html`,
	}
	short := func(s string, n int) string {
		if len(s) > n {
			return s[:n]
		}
		return s
	}
	for _, cmd := range legit {
		t.Run(short(cmd, 24), func(t *testing.T) {
			if got := odysseusBashDecision(cmd); got.Decision != "" {
				t.Errorf("%q: decision = %q (%s), want none", cmd, got.Decision, got.Reason)
			}
		})
	}
	deny := []string{
		// the remote shell is still not inspection
		`wsl ssh sippos-server "docker exec leweigen-erp-v2-db-1 psql -U leweigen -d leweigen -Atc 'select 1'"`,
		`wsl.exe -e bash -lc 'ssh -o ConnectTimeout=15 sippos-server "docker ps"'`,
		// a value that names a mutating program, at the head and in a flag
		`K=rm; $K -rf build`,
		`K="rm -rf"; $K build`,
		`F=-i; sed "$F" s/a/b/ x.txt`,
		// an unused assignment, and one followed by a pipe, classify as before
		`PATH=/x; ls`,
		`K=ls | $K`,
		// a later segment that can reassign the variable
		`K=ls; read K; $K`,
		// script blocks can run anything
		`Get-ChildItem D:/repo | ForEach-Object { npm run build }`,
		`Get-ChildItem D:/repo | Select-Object @{n='x';e={ Remove-Item $_ }}`,
		// open heads with anything but the plan page
		`Start-Process notepad`,
		`Start-Process D:/repo/dist/index.html`,
		`cmd /c start "" https://example.com`,
		`open -a Terminal`,
		`Start-Process D:/repo/.claude/.Arena/tactical-plans/x.html D:/repo/.claude/.Arena/tactical-plans/y.html`,
	}
	for _, cmd := range deny {
		t.Run("deny/"+short(cmd, 16), func(t *testing.T) {
			if got := odysseusBashDecision(cmd); got.Decision != "deny" {
				t.Errorf("%q: decision = %q, want deny", cmd, got.Decision)
			}
		})
	}
}

// The plan's HTML page is allowed beside its .md plan and nowhere else.
func TestOdysseusPlanPageWrite(t *testing.T) {
	root, sub := subdirRepo(t)
	plans := root + "/.claude/.Arena/tactical-plans"
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plans+"/2026-11-07-x.md", []byte("---\nstatus: ready\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, file, want string
	}{
		{"page beside its plan allowed", plans + "/2026-11-07-x.html", ""},
		{"page from a subdirectory allowed", plans + "/2026-11-07-x.html", ""},
		{"page with no plan denied", plans + "/2026-11-07-y.html", "deny"},
		{"html outside the plan root denied", root + "/docs/plan.html", "deny"},
		{"html in a plan subfolder denied", plans + "/sub/2026-11-07-x.html", "deny"},
		{"other extension beside the plan denied", plans + "/2026-11-07-x.pdf", "deny"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cwd := root
			if strings.Contains(tc.name, "subdirectory") {
				cwd = sub
			}
			if res := editGateDecision(odysseusWriteAt(cwd, tc.file), nil); res.Decision != tc.want {
				t.Fatalf("decision = %q (%s), want %q", res.Decision, res.Reason, tc.want)
			}
		})
	}
}

// From a linked worktree the planner may write the plan, the page, the flow
// shard and the spec delta into the main checkout — where `kratos spec
// validate` reads them — and still into the worktree's own state dir.
func TestOdysseusMainCheckoutFromWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	base := t.TempDir()
	main := filepath.Join(base, "main")
	if err := os.MkdirAll(filepath.Join(main, ".claude", ".Arena", "tactical-plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, main, "init", "-q")
	if err := os.WriteFile(filepath.Join(main, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, main, "add", "a.txt")
	gitRun(t, main, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")
	wt := filepath.Join(base, "wt")
	gitRun(t, main, "worktree", "add", "-q", "-b", "feat", wt)
	if err := os.MkdirAll(filepath.Join(wt, ".claude", ".Arena"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainS, wtS := filepath.ToSlash(main), filepath.ToSlash(wt)
	if err := os.WriteFile(filepath.Join(main, ".claude", ".Arena", "tactical-plans", "p.md"), []byte("---\nstatus: draft\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := gateMainWorktreeRoot(wt); normalizeLedgerPath(got) != normalizeLedgerPath(mainS) {
		t.Fatalf("gateMainWorktreeRoot(%s) = %q, want %q", wt, got, mainS)
	}

	allowed := []string{
		mainS + "/.claude/.Arena/tactical-plans/p.md",
		mainS + "/.claude/.Arena/tactical-plans/p.html",
		mainS + "/.claude/.Arena/flows/sync.md",
		mainS + "/.claude/feature/p/spec-delta/sync.md",
		wtS + "/.claude/.Arena/tactical-plans/q.md",
	}
	for _, f := range allowed {
		if res := editGateDecision(odysseusWriteAt(wt, f), nil); res.Decision != "" {
			t.Errorf("%s from worktree: decision = %q (%s), want none", f, res.Decision, res.Reason)
		}
	}
	for _, f := range []string{mainS + "/a.txt", wtS + "/src/x.go", mainS + "/.claude/feature/p/prd.md"} {
		if res := editGateDecision(odysseusWriteAt(wt, f), nil); res.Decision != "deny" {
			t.Errorf("%s from worktree: decision = %q, want deny", f, res.Decision)
		}
	}

	t.Run("main root is cached in the ledger", func(t *testing.T) {
		ledger := map[string]any{"session_id": "s", "cwd": wtS, "inline_god": "odysseus"}
		calls := 0
		saved := gateMainRootLookup
		gateMainRootLookup = func(dir string) string { calls++; return saved(dir) }
		defer func() { gateMainRootLookup = saved }()
		in := preToolUseInput{SessionID: "s", ToolName: "Write", Cwd: wtS, ToolInput: preToolUseToolInput{FilePath: mainS + "/.claude/.Arena/tactical-plans/p.md"}}
		for i := 0; i < 3; i++ {
			if res := editGateDecision(in, ledger); res.Decision != "" {
				t.Fatalf("decision = %q (%s)", res.Decision, res.Reason)
			}
		}
		if calls != 1 {
			t.Fatalf("main root lookup ran %d times, want 1", calls)
		}
		if normalizeLedgerPath(ledgerString(ledger, ledgerKeyMainRoot)) != normalizeLedgerPath(mainS) {
			t.Fatalf("ledger main_root = %q", ledgerString(ledger, ledgerKeyMainRoot))
		}
	})
}

// Assignments at any position are expanded; a value that would hide a flag is
// not, and the assignment segment stays put when nothing used it.
func TestSubstituteAssignments(t *testing.T) {
	cases := []struct {
		cmd  string
		want []string
	}{
		{`K=/x/kratos.exe; $K slug x`, []string{" /x/kratos.exe slug x"}},
		{`F="/r/p.md"; wc -l "$F"`, []string{` wc -l "/r/p.md"`}},
		{`ls a; W=/wt; grep -n x $W/f | head`, []string{"ls a", " grep -n x /wt/f ", " head"}},
		{`A=1; B=2; echo $A $B`, []string{" echo 1 2"}},
		{`PATH=/x; ls`, []string{"PATH=/x", " ls"}},
		{`F=-i; sed "$F" s/a/b/ x`, []string{"F=-i", ` sed "$F" s/a/b/ x`}},
		{`K=ls | $K`, []string{"K=ls ", " $K"}},
	}
	for _, tc := range cases {
		segs := substituteAssignments(tc.cmd, splitShellSegments(tc.cmd))
		var got []string
		for _, s := range segs {
			got = append(got, s.raw)
		}
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%q: segments = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

// `kratos template get plan-page-template` serves the HTML skeleton; a bare
// name still resolves to the .md template first.
func TestTemplateGetHTML(t *testing.T) {
	for name, prefix := range map[string]string{
		"plan-page-template":      "<!doctype html>",
		"plan-page-template.html": "<!doctype html>",
		"tactical-plan-template":  "#",
	} {
		var out bytes.Buffer
		cmd := templateGetCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{name})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.HasPrefix(strings.TrimSpace(out.String()), prefix) {
			t.Errorf("%s: output starts %q, want %q", name, out.String()[:min(40, out.Len())], prefix)
		}
	}
	var listed bytes.Buffer
	list := templateListCmd()
	list.SetOut(&listed)
	if err := list.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed.String(), "plan-page-template.html") {
		t.Errorf("template list does not name the page template:\n%s", listed.String())
	}
}
