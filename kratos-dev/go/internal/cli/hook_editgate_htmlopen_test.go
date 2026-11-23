package cli

import (
	"strings"
	"testing"
)

// The commands below are the real opens of the week of 2026-11-16 (deny) and
// the correct forms Iris ran beside them (pass).
func TestEditGateHTMLOpen(t *testing.T) {
	deny := []string{
		`Start-Process 'C:\Users\lizard_liang\work\CCA\EPA-CCO_GHGRS2609\.claude\.Arena\tactical-plans\2026-11-16-inv01-07-annual-history-list-backend.html'`,
		`Start-Process "C:\Users\lizard_liang\work\GSS_VITAL-NETZERO2604\.claude\.Arena\tactical-plans\2026-11-19-favorite-inventory-tasks-pulled-from-kpim.html"`,
		`powershell.exe -NoProfile -Command "Start-Process 'C:\Users\x\.claude\.Arena\specs\2026-11-18-data-fixes.html'" && echo opened`,
		`Start-Process "C:\Users\LIZARD~1\AppData\Local\Temp\claude\x\scratchpad\unc-spec\INV08-09-uncertainty-spec.html"`,
		`Invoke-Item x.html`,
		`ii C:\x\y.htm`,
		`cmd //c start "" x.html`,
		`cmd /c start "" "D:\repo\plan.html"`,
		`rtk explorer.exe x.htm`,
		`xdg-open .claude/.Arena/tactical-plans/x.html`,
		`Start-Process -FilePath "D:/repo/x.html"`,
		`saps file:///C:/x/y.html`,
		`start x.html`,
	}
	pass := []string{
		`Start-Process msedge "file:///C:/Users/lizard_liang/work/CCA/EPA-CCO_GHGRS2609/.claude/.Arena/specs/2026-11-18-data-fixes.html"`,
		`Start-Process msedge -ArgumentList '--new-window','file:///C:/x/y.html'`,
		`$u = ([Uri]"C:\x\index.html").AbsoluteUri; Start-Process msedge -ArgumentList @("--new-window", $u); $u`,
		`Start-Process pwsh -ArgumentList '-NoExit','-File','C:\x\run-seed.ps1'`,
		`Start-Process "C:\Program Files\PowerShell\7\pwsh.exe" -ArgumentList @('-NoExit','-File','"C:\x\run-seed.ps1"')`,
		`Start-Process "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" -ArgumentList "file:///C:/x/y.html"`,
		`Start-Process -FilePath chrome -ArgumentList x.html`,
		`cmd /c start msedge x.html`,
		`kratos open C:\x\plan.html`,
		`C:/Users/u/.kratos/bin/kratos.exe open C:\x\plan.html`,
		`grep -n "Start-Process x.html" notes.md`,
		`Start-Process notepad notes.txt`,
		`Start-Process https://example.com/index.html`,
		`explorer.exe C:\x\folder`,
		`powershell -Command "Start-Process msedge file:///C:/x.html"`,
	}
	for _, tool := range []string{"Bash", "PowerShell"} {
		for _, cmd := range deny {
			t.Run(tool+"/deny/"+cmd[:min(len(cmd), 30)], func(t *testing.T) {
				res := editGateDecision(preToolUseInput{ToolName: tool, ToolInput: preToolUseToolInput{Command: cmd}}, nil)
				if res.Decision != "deny" || !strings.HasPrefix(res.Reason, "HTML opens in VS Code on this machine. Run ") || !strings.Contains(res.Reason, " open ") {
					t.Errorf("%q: got %q %q, want the HTML deny", cmd, res.Decision, res.Reason)
				}
			})
		}
		for _, cmd := range pass {
			t.Run(tool+"/pass/"+cmd[:min(len(cmd), 30)], func(t *testing.T) {
				res := editGateDecision(preToolUseInput{ToolName: tool, ToolInput: preToolUseToolInput{Command: cmd}}, nil)
				if res.Decision != "" {
					t.Errorf("%q: got %q %q, want no decision", cmd, res.Decision, res.Reason)
				}
			})
		}
	}
}

func TestEditGateHTMLOpenReasonNamesPath(t *testing.T) {
	res := editGateDecision(preToolUseInput{ToolName: "PowerShell", ToolInput: preToolUseToolInput{Command: `Start-Process "C:\x\a b.html"`}}, nil)
	want := ` open C:\x\a b.html.`
	if res.Decision != "deny" || !strings.HasSuffix(res.Reason, want) {
		t.Fatalf("got %q %q, want a deny ending %q", res.Decision, res.Reason, want)
	}
}

func TestEditGateHTMLOpenAppliesToSpawnedAgents(t *testing.T) {
	res := editGateDecision(preToolUseInput{ToolName: "Bash", AgentType: "kratos:ares", AgentID: "a1", SessionID: "s1", ToolInput: preToolUseToolInput{Command: `Start-Process x.html`}}, nil)
	if res.Decision != "deny" || !strings.Contains(res.Reason, "VS Code") {
		t.Fatalf("spawned agent: got %q %q", res.Decision, res.Reason)
	}
}
