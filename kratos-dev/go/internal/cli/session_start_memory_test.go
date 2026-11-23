package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// memoryFixtureItem mirrors one row of the JSON `memory list` returns.
type memoryFixtureItem struct {
	ID       int    `json:"id"`
	Text     string `json:"text"`
	Category string `json:"category"`
	Project  string `json:"project,omitempty"`
}

// runBuildMemoryReport calls session-start.cjs's buildMemoryReport(data, ruleTotal, cwd)
// (ruleTotal is len(rules)) with fixture JSON through node, the same node-exec pattern
// TestGateProjectFileMatchesJS uses for tool-use.cjs. It returns the report
// string (possibly empty).
func runBuildMemoryReport(t *testing.T, memories []memoryFixtureItem, total int, rules []memoryFixtureItem, cwd string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	hook, err := filepath.Abs(filepath.Join(hooksDirPath(), "session-start.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := json.Marshal(map[string]interface{}{
		"data":  map[string]interface{}{"memories": memories, "total": total},
		"rules": rules,
		"cwd":   cwd,
	})
	if err != nil {
		t.Fatal(err)
	}
	script := "const { buildMemoryReport } = require(" + strconv.Quote(filepath.ToSlash(hook)) + ");\n" +
		"const fx = JSON.parse(process.argv[1]);\n" +
		"process.stdout.write(buildMemoryReport(fx.data, (fx.rules || []).length, fx.cwd) || '');\n"
	out, err := exec.Command(node, "-e", script, string(fixture)).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("node run failed: %v (%s)", err, ee.Stderr)
		}
		t.Fatalf("node run failed: %v", err)
	}
	return string(out)
}

// TestBuildMemoryReportCappedScopedFactsAndNoRules pins the tiered injection
// from Fix 2: a project with more scoped facts than MAX_MEMORIES used to fill
// every slot and hide every global preference (2026-09 review, KPIM: 16 scoped
// facts, 0 global preferences ever shown). At most SCOPED_CAP (4) scoped facts
// show, then global preferences fill the rest of MAX_MEMORIES (12). Standing
// rules are printed by the `--part rules` hook, so the main report carries none
// and its "+N more" count leaves them out.
func TestBuildMemoryReportCappedScopedFactsAndNoRules(t *testing.T) {
	here := "/work/kpim"
	var memories []memoryFixtureItem
	for i := 1; i <= 12; i++ {
		memories = append(memories, memoryFixtureItem{ID: i, Text: fmt.Sprintf("scoped fact %d", i), Category: "context", Project: here})
	}
	for i := 1; i <= 10; i++ {
		memories = append(memories, memoryFixtureItem{ID: 100 + i, Text: fmt.Sprintf("global pref %d", i), Category: "preference"})
	}
	var rules []memoryFixtureItem
	for i := 1; i <= 3; i++ {
		rules = append(rules, memoryFixtureItem{ID: 200 + i, Text: fmt.Sprintf("rule %d", i), Category: "rule"})
	}
	// the window also carries a rule row, as a real `memory list` does
	memories = append(memories, rules[0])
	total := 22 + len(rules) // whole-store count, same semantic memory list --limit N reports

	got := runBuildMemoryReport(t, memories, total, rules, here)

	if n := strings.Count(got, "[rule]"); n != 0 {
		t.Errorf("the main report must not print rules, got %d [rule] lines in:\n%s", n, got)
	}
	if strings.Contains(got, "rule 1") {
		t.Errorf("the main report must not print a rule row from the window:\n%s", got)
	}
	if n := strings.Count(got, "context · this project"); n != 4 {
		t.Errorf("expected scoped facts capped at 4, got %d in:\n%s", n, got)
	}
	if n := strings.Count(got, "[preference]"); n != 8 {
		t.Errorf("expected 8 preference lines filling the remaining slots, got %d in:\n%s", n, got)
	}
	// total(25) - ruleTotal(3) - shownFacts(12) = 10
	if !strings.Contains(got, "+10 more") {
		t.Errorf("expected the '+N more' count to cover facts only:\n%s", got)
	}
}

// TestBuildMemoryReportUnusedScopedSlotsGoToPreferences covers the other
// direction: a project with only one scoped fact must not waste the other
// three SCOPED_CAP slots — they pass to global preferences instead.
func TestBuildMemoryReportUnusedScopedSlotsGoToPreferences(t *testing.T) {
	here := "/work/small-project"
	memories := []memoryFixtureItem{
		{ID: 1, Text: "the one scoped fact", Category: "context", Project: here},
	}
	for i := 1; i <= 10; i++ {
		memories = append(memories, memoryFixtureItem{ID: 100 + i, Text: fmt.Sprintf("global pref %d", i), Category: "preference"})
	}
	total := len(memories)

	got := runBuildMemoryReport(t, memories, total, nil, here)

	if strings.Count(got, "[rule]") != 0 {
		t.Errorf("expected no rule lines when none are stored:\n%s", got)
	}
	scopedCount := strings.Count(got, "context · this project")
	if scopedCount != 1 {
		t.Errorf("expected exactly the 1 available scoped fact, got %d in:\n%s", scopedCount, got)
	}
	prefCount := strings.Count(got, "[preference]")
	if prefCount != 10 {
		t.Errorf("expected the unused 3 scoped slots to pass to preferences (1+10=11, all 10 available preferences shown), got %d in:\n%s", prefCount, got)
	}

	// total(11) - shownFacts(11) - shownRules(0) = 0: every row shown, no "more" line.
	if strings.Contains(got, "more —") {
		t.Errorf("expected no '+N more' line once every row is shown:\n%s", got)
	}
}

// TestFormatMemoriesCalls pins the two capture calls the main part makes and
// their timeouts: the fact window at 1500 ms, then the rule count at its own
// short 600 ms budget, so the worst-case serial sum stays under the 10000 ms
// SessionStart hook limit. Neither call asks for --with-rules any more. The
// capture function is injectable so this runs without a real binary.
func TestFormatMemoriesCalls(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	hook, err := filepath.Abs(filepath.Join(hooksDirPath(), "session-start.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	script := "const { formatMemories } = require(" + strconv.Quote(filepath.ToSlash(hook)) + ");\n" +
		"const calls = [];\n" +
		"function stubCapture(args, timeoutMs) {\n" +
		"  calls.push({ args, timeoutMs });\n" +
		"  return { out: JSON.stringify({ memories: [], total: 0 }), err: '' };\n" +
		"}\n" +
		"formatMemories('/work/x', stubCapture);\n" +
		"process.stdout.write(JSON.stringify(calls));\n"
	out, err := exec.Command(node, "-e", script).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("node run failed: %v (%s)", err, ee.Stderr)
		}
		t.Fatalf("node run failed: %v", err)
	}

	var calls []struct {
		Args      []string `json:"args"`
		TimeoutMs int      `json:"timeoutMs"`
	}
	if err := json.Unmarshal(out, &calls); err != nil {
		t.Fatalf("cannot parse node output %q: %v", out, err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 capture calls (the fact window, then the rule count), got %d: %v", len(calls), calls)
	}
	if calls[0].TimeoutMs != 1500 || calls[1].TimeoutMs != 600 {
		t.Errorf("timeouts = %d, %d; want 1500 then 600", calls[0].TimeoutMs, calls[1].TimeoutMs)
	}
	for _, c := range calls {
		if strings.Contains(strings.Join(c.Args, " "), "--with-rules") {
			t.Errorf("call %v must not ask for --with-rules", c.Args)
		}
	}
}

// TestBuildMemoryReportAllUnusedSlotsGoToScopedFacts covers the direction
// Fix 2's own test left unpinned: with NO global memories at all (not even a
// few), every one of the MAX_MEMORIES slots must return to scoped facts
// rather than sit empty. TestBuildMemoryReportUnusedScopedSlotsGoToPreferences
// passes identically on the pre-fix code (10 global preferences already fill
// every leftover slot on their own), so it does not exercise this path.
func TestBuildMemoryReportAllUnusedSlotsGoToScopedFacts(t *testing.T) {
	here := "/work/big-project"
	var memories []memoryFixtureItem
	for i := 1; i <= 12; i++ {
		memories = append(memories, memoryFixtureItem{ID: i, Text: fmt.Sprintf("scoped fact %d", i), Category: "context", Project: here})
	}
	total := len(memories)

	got := runBuildMemoryReport(t, memories, total, nil, here)

	scopedCount := strings.Count(got, "context · this project")
	if scopedCount != 12 {
		t.Errorf("expected all 12 slots to go to scoped facts with no global memories at all, got %d in:\n%s", scopedCount, got)
	}
	if strings.Count(got, "[preference]") != 0 {
		t.Errorf("expected no preference lines when none are stored:\n%s", got)
	}
	// total(12) - shownFacts(12) - shownRules(0) = 0: every row shown, no "more" line.
	if strings.Contains(got, "more —") {
		t.Errorf("expected no '+N more' line once every row is shown:\n%s", got)
	}
}

// TestBuildMemoryReportPartialGlobalsLeaveRestToScopedFacts covers the
// partial case between the two above: a FEW global preferences (not enough
// to fill every leftover slot on their own) must not stop the remaining
// slots from returning to scoped facts.
func TestBuildMemoryReportPartialGlobalsLeaveRestToScopedFacts(t *testing.T) {
	here := "/work/big-project"
	var memories []memoryFixtureItem
	for i := 1; i <= 12; i++ {
		memories = append(memories, memoryFixtureItem{ID: i, Text: fmt.Sprintf("scoped fact %d", i), Category: "context", Project: here})
	}
	memories = append(memories,
		memoryFixtureItem{ID: 101, Text: "global pref 1", Category: "preference"},
		memoryFixtureItem{ID: 102, Text: "global pref 2", Category: "preference"},
	)
	total := len(memories)

	got := runBuildMemoryReport(t, memories, total, nil, here)

	scopedCount := strings.Count(got, "context · this project")
	if scopedCount != 10 {
		t.Errorf("expected 4 (SCOPED_CAP) + 6 leftover-slot scoped facts = 10, got %d in:\n%s", scopedCount, got)
	}
	prefCount := strings.Count(got, "[preference]")
	if prefCount != 2 {
		t.Errorf("expected both preferences to show, got %d in:\n%s", prefCount, got)
	}
	// total(14) - shownFacts(12) - shownRules(0) = 2
	if !strings.Contains(got, "+2 more") {
		t.Errorf("expected the '+N more' count to reflect the unshown rows:\n%s", got)
	}
}

// TestBuildMemoryReportGlobalPrefRankBeatsRecency pins the E2 ranking rule: a
// global preference outranks a weak-spot regardless of recency. `memories`
// arrives newest-first, so a preference appended after 15 weak-spots is the
// OLDEST global fact in the list — under plain recency slicing it would be
// the first one cut once the global facts exceed MAX_MEMORIES (12). The
// category-rank sort moves it to the front instead, so it always shows, at
// the cost of the newest weak-spots beyond the cap.
func TestBuildMemoryReportGlobalPrefRankBeatsRecency(t *testing.T) {
	here := "/work/rank-project"
	var memories []memoryFixtureItem
	for i := 1; i <= 15; i++ {
		memories = append(memories, memoryFixtureItem{ID: i, Text: fmt.Sprintf("weak spot %d", i), Category: "weak-spot"})
	}
	memories = append(memories, memoryFixtureItem{ID: 100, Text: "the oldest preference", Category: "preference"})
	total := len(memories)

	got := runBuildMemoryReport(t, memories, total, nil, here)

	if !strings.Contains(got, "the oldest preference") {
		t.Fatalf("expected the oldest preference to still show by category rank, not get cut by recency:\n%s", got)
	}
	prefIdx := strings.Index(got, "the oldest preference")
	firstSpotIdx := strings.Index(got, "weak spot 1 ")
	if firstSpotIdx < 0 || prefIdx > firstSpotIdx {
		t.Errorf("expected the preference to sort ahead of every weak-spot despite being the oldest global fact:\n%s", got)
	}
	prefCount := strings.Count(got, "[preference]")
	if prefCount != 1 {
		t.Errorf("expected exactly 1 preference line, got %d in:\n%s", prefCount, got)
	}
	spotCount := strings.Count(got, "[weak-spot]")
	if spotCount != 11 {
		t.Errorf("expected 11 weak-spot lines (12 global slots minus the 1 preference), got %d in:\n%s", spotCount, got)
	}
}

// runRulesReport calls session-start.cjs's buildRulesReport(rules, cwd, budget)
// through node. budget 0 means the default.
func runRulesReport(t *testing.T, rules []memoryFixtureItem, cwd string, budget int) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	hook, err := filepath.Abs(filepath.Join(hooksDirPath(), "session-start.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := json.Marshal(map[string]interface{}{"rules": rules, "cwd": cwd, "budget": budget})
	if err != nil {
		t.Fatal(err)
	}
	script := "const { buildRulesReport } = require(" + strconv.Quote(filepath.ToSlash(hook)) + ");\n" +
		"const fx = JSON.parse(process.argv[1]);\n" +
		"process.stdout.write(buildRulesReport(fx.rules, fx.cwd, fx.budget || undefined) || '');\n"
	out, err := exec.Command(node, "-e", script, string(fixture)).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("node run failed: %v (%s)", err, ee.Stderr)
		}
		t.Fatalf("node run failed: %v", err)
	}
	return string(out)
}

// 44 rules apply to the GHGRS project on 2026-11-23 (global plus its own);
// the old tier showed the newest 6. All 44 show now, this project's first,
// and another project's rule never does.
func TestBuildRulesReportShowsEveryEligibleRule(t *testing.T) {
	here := "/work/ghgrs"
	var rules []memoryFixtureItem
	for i := 1; i <= 30; i++ {
		rules = append(rules, memoryFixtureItem{ID: i, Text: fmt.Sprintf("global rule %d", i), Category: "rule"})
	}
	for i := 1; i <= 14; i++ {
		rules = append(rules, memoryFixtureItem{ID: 100 + i, Text: fmt.Sprintf("project rule %d", i), Category: "rule", Project: here})
	}
	for i := 1; i <= 5; i++ {
		rules = append(rules, memoryFixtureItem{ID: 200 + i, Text: fmt.Sprintf("other project rule %d", i), Category: "rule", Project: "/work/elsewhere"})
	}

	got := runRulesReport(t, rules, here, 0)

	if n := strings.Count(got, "[rule]"); n != 44 {
		t.Errorf("expected all 44 eligible rules, got %d in:\n%s", n, got)
	}
	if strings.Contains(got, "other project rule") {
		t.Errorf("another project's rule must never show:\n%s", got)
	}
	if !strings.HasPrefix(got, "## Standing rules\n- project rule 1 [rule]\n") {
		t.Errorf("expected the header, then this project's rules first:\n%s", got)
	}
	if strings.Index(got, "project rule 14 [rule]") > strings.Index(got, "global rule 1 [rule]") {
		t.Errorf("expected every project rule before the first global rule:\n%s", got)
	}
	if strings.Contains(got, "not shown") {
		t.Errorf("nothing was cut, so no truncation line:\n%s", got)
	}
}

func TestBuildRulesReportTruncatesAtBudgetWithVisibleLine(t *testing.T) {
	var rules []memoryFixtureItem
	for i := 1; i <= 200; i++ {
		rules = append(rules, memoryFixtureItem{ID: i, Text: fmt.Sprintf("global rule number %03d %s", i, strings.Repeat("x", 80)), Category: "rule"})
	}

	got := runRulesReport(t, rules, "/work/here", 0)

	if n := utf8.RuneCountInString(got) + 1; n > 9500 {
		t.Errorf("output is %d characters with its newline, over the 9500 budget", n)
	}
	shown := strings.Count(got, "[rule]")
	if shown == 0 || shown >= 200 {
		t.Fatalf("expected a cut list, got %d rules shown", shown)
	}
	lines := strings.Split(got, "\n")
	want := fmt.Sprintf("(+%d rules not shown — `kratos memory list --category rule`)", 200-shown)
	if lines[len(lines)-1] != want {
		t.Errorf("last line = %q, want %q", lines[len(lines)-1], want)
	}
	if !strings.HasPrefix(lines[len(lines)-2], "- global rule number ") || !strings.HasSuffix(lines[len(lines)-2], " [rule]") {
		t.Errorf("a rule line must be cut whole, not mid-line: %q", lines[len(lines)-2])
	}
}

// seedRuleMemory adds one memory row through the plugin binary into home.
func seedRuleMemory(t *testing.T, bin, home, text, project string) {
	t.Helper()
	args := []string{"memory", "add", text, "--category", "rule"}
	if project != "" {
		args = append(args, "--project", project)
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("memory add: %v\n%s", err, out)
	}
}

// The --part rules hook prints the rules on every source, compact included,
// and the main part prints no [rule] line, so no rule shows twice.
func TestSessionStartRulesPartOnEverySource(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	root, err := filepath.Abs(filepath.Join(hooksDirPath(), ".."))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin", "kratos")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skip("plugin binary not built")
	}
	home := t.TempDir()
	cwd := t.TempDir()
	seedRuleMemory(t, bin, home, "global rule alpha", "")
	seedRuleMemory(t, bin, home, "global rule beta", "")
	seedRuleMemory(t, bin, home, "this project rule gamma", cwd)
	seedRuleMemory(t, bin, home, "other project rule delta", filepath.Join(cwd, "elsewhere"))

	run := func(source string, args ...string) string {
		payload, _ := json.Marshal(map[string]any{"session_id": "sess-r", "source": source, "cwd": cwd})
		cmd := exec.Command(node, append([]string{filepath.Join(root, "hooks", "session-start.cjs")}, args...)...)
		cmd.Stdin = strings.NewReader(string(payload))
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "CLAUDE_PLUGIN_ROOT="+root)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("session-start.cjs %v (%s): %v", args, source, err)
		}
		return string(out)
	}

	for _, source := range []string{"startup", "resume", "clear", "compact"} {
		got := run(source, "--part", "rules")
		if n := strings.Count(got, "[rule]"); n != 3 {
			t.Errorf("%s: expected 3 rules, got %d in:\n%s", source, n, got)
		}
		if !strings.HasPrefix(got, "## Standing rules\n- this project rule gamma [rule]\n") {
			t.Errorf("%s: expected this project's rule first:\n%s", source, got)
		}
		if strings.Contains(got, "other project rule") {
			t.Errorf("%s: another project's rule showed:\n%s", source, got)
		}
		if utf8.RuneCountInString(got) >= hookOutputCap {
			t.Errorf("%s: rules output is over the hook cap", source)
		}
		if main := run(source); strings.Contains(main, "[rule]") || strings.Contains(main, "Standing rules") {
			t.Errorf("%s: the main part must print no rule line:\n%s", source, main)
		}
	}
}

// runSessionStart drives session-start.cjs with a scratch HOME whose ledger
// for session "sess-x" names inline_god "iris", and returns its stdout. hookArgs
// are passed to the script (for example "--part", "extras"). It skips when the
// plugin-local binary has not been built.
func runSessionStart(t *testing.T, source string, hookArgs ...string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	root, err := filepath.Abs(filepath.Join(hooksDirPath(), ".."))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin", "kratos")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skip("plugin binary not built")
	}
	home := t.TempDir()
	ledger := filepath.Join(home, ".kratos", "sessions", "sess-x.json")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger, []byte(`{"session_id":"sess-x","inline_god":"iris"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"session_id": "sess-x", "source": source, "cwd": t.TempDir()})
	cmd := exec.Command(node, append([]string{filepath.Join(root, "hooks", "session-start.cjs")}, hookArgs...)...)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "CLAUDE_PLUGIN_ROOT="+root)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hook failed: %v", err)
	}
	return string(out)
}

const godExtrasMarker = "# Agent Protocol (injected)"

func TestSessionStartCompactReinjectsInlineGodExtras(t *testing.T) {
	out := runSessionStart(t, "compact", "--part", "extras")
	if !strings.Contains(out, godExtrasMarker) {
		t.Fatalf("compact extras output lacks the iris extras:\n%s", out)
	}
	if n := utf8.RuneCountInString(out); n >= hookOutputCap {
		t.Fatalf("extras output is %d characters, over the %d hook cap", n, hookOutputCap)
	}
	if main := runSessionStart(t, "compact"); strings.Contains(main, godExtrasMarker) {
		t.Fatalf("main mode carries the extras, which belong to the --part extras hook:\n%s", main)
	}
}

func TestSessionStartOtherSourcesSkipInlineGodExtras(t *testing.T) {
	for _, source := range []string{"startup", "resume", "clear"} {
		for _, args := range [][]string{nil, {"--part", "extras"}} {
			if out := runSessionStart(t, source, args...); strings.Contains(out, godExtrasMarker) {
				t.Fatalf("%s output %v carries the extras:\n%s", source, args, out)
			}
		}
	}
}
