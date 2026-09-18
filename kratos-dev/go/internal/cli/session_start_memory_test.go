package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// memoryFixtureItem mirrors one row of the JSON `memory list` returns.
type memoryFixtureItem struct {
	ID       int    `json:"id"`
	Text     string `json:"text"`
	Category string `json:"category"`
	Project  string `json:"project,omitempty"`
}

// runBuildMemoryReport calls session-start.cjs's buildMemoryReport(data, rules, cwd)
// with fixture JSON through node, the same node-exec pattern
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
		"process.stdout.write(buildMemoryReport(fx.data, fx.rules, fx.cwd) || '');\n"
	out, err := exec.Command(node, "-e", script, string(fixture)).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("node run failed: %v (%s)", err, ee.Stderr)
		}
		t.Fatalf("node run failed: %v", err)
	}
	return string(out)
}

// TestBuildMemoryReportRulesFirstThenCappedScopedFacts pins the tiered
// injection from Fix 2: a project with more scoped facts than MAX_MEMORIES
// used to fill every slot and hide every global preference (2026-09 review,
// KPIM: 16 scoped facts, 0 global preferences ever shown). Rules must show
// first regardless, then at most SCOPED_CAP (4) scoped facts, then global
// preferences fill the rest of MAX_MEMORIES (8).
func TestBuildMemoryReportRulesFirstThenCappedScopedFacts(t *testing.T) {
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
	total := len(memories) + len(rules) // whole-store count, same semantic memory list --limit N reports

	got := runBuildMemoryReport(t, memories, total, rules, here)

	ruleCount := strings.Count(got, "[rule]")
	if ruleCount != 3 {
		t.Errorf("expected 3 rule lines, got %d in:\n%s", ruleCount, got)
	}
	scopedCount := strings.Count(got, "context · this project")
	if scopedCount != 4 {
		t.Errorf("expected scoped facts capped at 4, got %d in:\n%s", scopedCount, got)
	}
	prefCount := strings.Count(got, "[preference]")
	if prefCount != 4 {
		t.Errorf("expected 4 preference lines filling the remaining slots, got %d in:\n%s", prefCount, got)
	}

	firstRule := strings.Index(got, "rule 1")
	firstScoped := strings.Index(got, "scoped fact")
	firstPref := strings.Index(got, "global pref")
	if firstRule < 0 || firstScoped < 0 || firstPref < 0 {
		t.Fatalf("expected all three tiers present:\n%s", got)
	}
	if !(firstRule < firstScoped && firstScoped < firstPref) {
		t.Errorf("expected order rules, then scoped facts, then preferences:\n%s", got)
	}

	// total(25) - shownFacts(8) - shownRules(3) = 14
	if !strings.Contains(got, "+14 more") {
		t.Errorf("expected the '+N more' count to account for both tiers:\n%s", got)
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
	if prefCount != 7 {
		t.Errorf("expected the unused 3 scoped slots to pass to preferences (1+7=8), got %d in:\n%s", prefCount, got)
	}

	// total(11) - shownFacts(8) - shownRules(0) = 3
	if !strings.Contains(got, "+3 more") {
		t.Errorf("expected the '+N more' count to reflect the unshown rows:\n%s", got)
	}
}

// TestFormatMemoriesRetryUsesItsOwnShortTimeout covers Fix 2's own budget
// bug: the unknown-flag retry (an older binary rejecting --with-rules) used
// to reuse the 1500 ms memory-list timeout, pushing the worst-case serial sum
// to 5100 ms — over the 5000 ms SessionStart hook timeout the file's own
// comment states as an invariant. formatMemories takes an injectable capture
// function precisely so this can be asserted without spawning a real binary.
func TestFormatMemoriesRetryUsesItsOwnShortTimeout(t *testing.T) {
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
		"  if (args.includes('--with-rules')) return { out: null, err: 'unknown flag: --with-rules' };\n" +
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
		t.Fatalf("expected 2 capture calls (the first call, then the retry), got %d: %v", len(calls), calls)
	}
	if calls[0].TimeoutMs != 1500 {
		t.Errorf("first call timeout = %d, want 1500", calls[0].TimeoutMs)
	}
	if calls[1].TimeoutMs != 600 {
		t.Errorf("retry call timeout = %d, want its own 600 ms budget, not the 1500 ms memory-list timeout", calls[1].TimeoutMs)
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
	if scopedCount != 8 {
		t.Errorf("expected all 8 slots to go to scoped facts with no global memories at all, got %d in:\n%s", scopedCount, got)
	}
	if strings.Count(got, "[preference]") != 0 {
		t.Errorf("expected no preference lines when none are stored:\n%s", got)
	}
	// total(12) - shownFacts(8) - shownRules(0) = 4
	if !strings.Contains(got, "+4 more") {
		t.Errorf("expected the '+N more' count to reflect the unshown rows:\n%s", got)
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
	if scopedCount != 6 {
		t.Errorf("expected 4 (SCOPED_CAP) + 2 leftover-slot scoped facts = 6, got %d in:\n%s", scopedCount, got)
	}
	prefCount := strings.Count(got, "[preference]")
	if prefCount != 2 {
		t.Errorf("expected both preferences to show, got %d in:\n%s", prefCount, got)
	}
	// total(14) - shownFacts(8) - shownRules(0) = 6
	if !strings.Contains(got, "+6 more") {
		t.Errorf("expected the '+N more' count to reflect the unshown rows:\n%s", got)
	}
}

// TestBuildMemoryReportExcludesOtherProjectsRules covers the "never another
// project's" rule scoping: a rule saved for a different project must not
// leak into this session even though the dedicated rule capture returned it.
func TestBuildMemoryReportExcludesOtherProjectsRules(t *testing.T) {
	here := "/work/here"
	rules := []memoryFixtureItem{
		{ID: 1, Text: "global rule", Category: "rule"},
		{ID: 2, Text: "this project rule", Category: "rule", Project: here},
		{ID: 3, Text: "other project rule", Category: "rule", Project: "/work/elsewhere"},
	}
	got := runBuildMemoryReport(t, nil, 3, rules, here)

	if !strings.Contains(got, "global rule") {
		t.Errorf("expected the global rule to show:\n%s", got)
	}
	if !strings.Contains(got, "this project rule") {
		t.Errorf("expected this project's rule to show:\n%s", got)
	}
	if strings.Contains(got, "other project rule") {
		t.Errorf("another project's rule must never show:\n%s", got)
	}
}
