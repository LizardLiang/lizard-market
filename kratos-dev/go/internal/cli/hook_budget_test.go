package cli

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// runFitToBudget calls hooks/hook-budget.cjs fitToBudget through node and
// returns its output. Inputs travel as JSON on stdin, so line endings pass
// through unchanged.
func runFitToBudget(t *testing.T, head []string, body, root string, budget int) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	module := filepath.ToSlash(filepath.Join(hooksDirPath(), "hook-budget.cjs"))
	script := `let raw='';process.stdin.on('data',c=>raw+=c);process.stdin.on('end',()=>{` +
		`const i=JSON.parse(raw);` +
		`process.stdout.write(require(process.argv[1]).fitToBudget(i.head,i.body,i.root,i.budget));});`
	in, err := json.Marshal(map[string]any{"head": head, "body": body, "root": root, "budget": budget})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "-e", script, module)
	cmd.Stdin = strings.NewReader(string(in))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("fitToBudget failed: %v", err)
	}
	return string(out)
}

var hookBudgetSectionRE = regexp.MustCompile(`(?m)^## (.+?)\r?$`)

func TestFitToBudgetDropPath(t *testing.T) {
	root := "C:/Users/lizard_liang/.claude/plugins/cache/lizard-plugins/kratos/2.121.0"
	bin := "C:/Users/lizard_liang/.kratos/bin/kratos.exe"
	block, err := composeProtocolFor("iris")
	if err != nil {
		t.Fatal(err)
	}
	block = strings.ReplaceAll(strings.ReplaceAll(block, "<KRATOS_ROOT>", root), "<kratos-bin>", bin)
	head := []string{
		"**Kratos binary:** `" + bin + "` — use it wherever instructions show `<kratos-bin>`.",
		"**Kratos plugin root:** `" + root + "` — use it wherever instructions show `<KRATOS_ROOT>`.",
	}
	titles := hookBudgetSectionRE.FindAllStringSubmatch(block, -1)
	if len(titles) < 5 {
		t.Fatalf("expected the protocol block to carry several sections, found %d", len(titles))
	}

	for name, form := range map[string]string{"LF": block, "CRLF": strings.ReplaceAll(block, "\n", "\r\n")} {
		t.Run(name, func(t *testing.T) {
			const budget = 3000
			out := runFitToBudget(t, head, form, root, budget)
			if n := utf8.RuneCountInString(out); n > budget {
				t.Errorf("output is %d characters, budget %d", n, budget)
			}
			if !strings.Contains(out, "## Output Format") {
				t.Error("Output Format section was dropped")
			}
			if !strings.Contains(out, "# Agent Protocol (injected)") {
				t.Error("protocol header was dropped")
			}
			notice := "Protocol sections cut by the hook cap:"
			at := strings.Index(out, notice)
			if at < 0 {
				t.Fatal("no drop notice in the output")
			}
			noticeLine := out[at:]
			dropped := 0
			for _, m := range titles {
				title := strings.TrimSpace(m[1])
				if strings.Contains(out[:at], "## "+title) {
					continue
				}
				dropped++
				if !strings.Contains(noticeLine, title) {
					t.Errorf("section %q is gone but the notice does not name it", title)
				}
			}
			if dropped == 0 {
				t.Error("no section was dropped at the 3000-character budget")
			}

			full := runFitToBudget(t, head, form, root, hookOutputBudget)
			if strings.Contains(full, notice) {
				t.Error("notice printed although the block fits the 9,500-character budget")
			}
			for _, m := range titles {
				if !strings.Contains(full, "## "+strings.TrimSpace(m[1])) {
					t.Errorf("section %q dropped at the 9,500-character budget", m[1])
				}
			}
		})
	}
}

// hookOutputBudget mirrors HOOK_OUTPUT_BUDGET in hooks/hook-budget.cjs.
const hookOutputBudget = 9500
