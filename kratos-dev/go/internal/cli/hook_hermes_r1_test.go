package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	out, err := exec.Command(git, "-C", dir, "init", "-q").CombinedOutput()
	require.NoError(t, err, "%s", out)
}

func planAt(root string) string { return root + "/.claude/.Arena/tactical-plans/x.md" }

func TestGateContainmentRootIgnoresKratosTmp(t *testing.T) {
	root, sub := subdirRepo(t)
	require.NoError(t, os.MkdirAll(sub+"/.claude/tmp", 0o755))
	assert.Equal(t, filepath.Clean(root), filepath.Clean(gateContainmentRoot(sub)))
	res := editGateDecision(odysseusWriteAt(sub, planAt(root)), nil)
	assert.Equal(t, "", res.Decision, res.Reason)
}

func TestGateContainmentRootNestedRepoWithoutClaude(t *testing.T) {
	t.Run("git work tree", func(t *testing.T) {
		workspace := filepath.ToSlash(t.TempDir())
		require.NoError(t, os.MkdirAll(workspace+"/.claude/.Arena", 0o755))
		repo := workspace + "/repo"
		require.NoError(t, os.MkdirAll(repo+"/src", 0o755))
		gitInit(t, repo)
		got := gateContainmentRoot(repo + "/src")
		assert.Equal(t, filepath.Clean(repo), filepath.Clean(got))
		assert.Equal(t, "deny", editGateDecision(odysseusWriteAt(repo+"/src", planAt(workspace)), nil).Decision)
	})
	t.Run("git boundary without git metadata", func(t *testing.T) {
		ws := filepath.ToSlash(t.TempDir())
		require.NoError(t, os.MkdirAll(ws+"/.claude/.Arena", 0o755))
		require.NoError(t, os.MkdirAll(ws+"/r/.git", 0o755))
		require.NoError(t, os.MkdirAll(ws+"/r/src", 0o755))
		got := gateContainmentRoot(ws + "/r/src")
		assert.NotEqual(t, filepath.Clean(ws), filepath.Clean(got), "the walk must stop at the .git entry")
	})
}

func TestGateContainmentRootGitTopLevel(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	sub := root + "/a/b"
	require.NoError(t, os.MkdirAll(sub, 0o755))
	gitInit(t, root)
	assert.Equal(t, filepath.Clean(root), filepath.Clean(gateContainmentRoot(sub)))
	assert.Equal(t, "", editGateDecision(odysseusWriteAt(sub, planAt(root)), nil).Decision)
}

func TestGateContainmentRootBackslashCwd(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("backslash separators are a Windows path form")
	}
	root, sub := subdirRepo(t)
	bs := strings.ReplaceAll(sub, "/", `\`)
	assert.Equal(t, "", editGateDecision(odysseusWriteAt(bs, strings.ReplaceAll(planAt(root), "/", `\`)), nil).Decision)
	assert.Equal(t, "", editGateDecision(odysseusWriteAt(bs, planAt(root)), nil).Decision)
}

// A cwd in the 8.3 short form and a plan path in the same spelling must agree
// with the root git reports, which is the long form.
func TestGateContainmentRootKeepsCwdSpelling(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	sub := filepath.Join(root, "a")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	assert.Equal(t, "", editGateDecision(odysseusWriteAt(sub, planAt(filepath.ToSlash(root))), nil).Decision)
}

func TestGateContainmentRootStopsAtHome(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	require.NoError(t, os.MkdirAll(home+"/.claude/.Arena", 0o755))
	sub := filepath.ToSlash(home) + "/proj/x"
	require.NoError(t, os.MkdirAll(sub, 0o755))
	assert.Equal(t, filepath.Clean(sub), filepath.Clean(gateContainmentRoot(sub)), "home is never a root")
}

func TestSubagentTranscriptPath(t *testing.T) {
	want := filepath.Join("C:/x/proj", "sess", "subagents", "agent-a1.jsonl")
	assert.Equal(t, want, subagentTranscriptPath("C:/x/proj/sess.jsonl", "sess", "a1"))
	assert.Equal(t, want, subagentTranscriptPath(`C:\x\proj\sess.jsonl`, "sess", "a1"))
	assert.Equal(t, "", subagentTranscriptPath("", "sess", "a1"))
	assert.Equal(t, "", subagentTranscriptPath("C:/x/p.jsonl", "", "a1"))
	assert.Equal(t, "", subagentTranscriptPath("C:/x/p.jsonl", "sess", ""))
}

func TestFoldHeaderListsReachPerItem(t *testing.T) {
	got := foldHeaderLists("Reach:\n- not applicable — test file only\n- hook.go handleX — reached by Y → fix\nLanded: x")
	assert.Equal(t, "Reach: not applicable — test file only\nReach: hook.go handleX — reached by Y → fix\nLanded: x", got)
	got = foldHeaderLists("Files created/modified:\n- a.go\n- b.go\nLanded: x")
	assert.Equal(t, "Files created/modified: a.go b.go\nLanded: x", got)
}

func TestFoldHeaderListsStopsAtWaiverWithoutColon(t *testing.T) {
	got := foldHeaderLists("Reach:\n- a\nEVIDENCE-SKIPPED for the docs item\nTESTS-NOT-APPLICABLE docs only")
	assert.Equal(t, "Reach: a\nEVIDENCE-SKIPPED for the docs item\nTESTS-NOT-APPLICABLE docs only", got)
}

func TestFoldHeaderListsCapsAtMaxFoldedLines(t *testing.T) {
	var items []string
	for i := 0; i < maxFoldedLines+3; i++ {
		items = append(items, "- item"+string(rune('a'+i)))
	}
	got := foldHeaderLists("Files created/modified:\n" + strings.Join(items, "\n"))
	lines := strings.Split(got, "\n")
	require.Len(t, lines, 4, "the header and its folded lines are one line; the rest stay as written")
	assert.NotContains(t, lines[0], "itemi")
	assert.Equal(t, "- itemi", lines[1])
}

func TestAresReachMixedListRequiresChild(t *testing.T) {
	report := foldBase + "Files created/modified: src/handler.go\n" + foldTail +
		"Reach:\n- not applicable — test file only\n- hook.go handleX — reached by Y → fix\nTESTS-NOT-APPLICABLE: fixture\n"
	in := reachInput(t, "aresMixed", userPromptLine("x"))
	assert.Contains(t, reachFailureText(aresReportFailures(report, in)), "child agent")

	allNA := foldBase + "Files created/modified: src/handler.go\n" + foldTail +
		"Reach: not applicable — no code changed\nTESTS-NOT-APPLICABLE: fixture\n"
	assert.NotContains(t, reachFailureText(aresReportFailures(allNA, in)), "child agent")
}

func TestAresVerifyGateTestsNotApplicableColonless(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "t.jsonl")
	edit := `{"type":"assistant","isSidechain":true,"message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"a.go"}}]}}` + "\n"
	require.NoError(t, os.WriteFile(tr, []byte(edit), 0o644))
	in := subagentStopInput{AgentTranscriptPath: tr}
	assert.NotEmpty(t, aresVerifyGateFailure("done", in))
	assert.Empty(t, aresVerifyGateFailure("TESTS-NOT-APPLICABLE: docs", in))
	assert.Empty(t, aresVerifyGateFailure("done.\n- TESTS-NOT-APPLICABLE for this mission", in))
	assert.NotEmpty(t, aresVerifyGateFailure("I did not use TESTS-NOT-APPLICABLE here", in))
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func TestEvaluateLandedDiffRelativeFromSubdirectory(t *testing.T) {
	dir, hash := gitRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("b\n"), 0o644))
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "sub")
	gitIn(t, dir, "config", "diff.relative", "true")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644))

	failures := evaluateLanded(filepath.Join(dir, "sub"), hash)
	require.Len(t, failures, 1, "a change outside the subdirectory must still block")
	assert.Contains(t, failures[0], "a.txt")
}

func TestGateContainmentRootGitRepoAtHomeIsNotARoot(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	gitInit(t, home)
	sub := filepath.ToSlash(home) + "/proj"
	require.NoError(t, os.MkdirAll(sub, 0o755))
	assert.Equal(t, filepath.Clean(sub), filepath.Clean(gateContainmentRoot(sub)))
}
