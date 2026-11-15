package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleDiff = `diff --git a/jobs.py b/jobs.py
index 1111111..2222222 100644
--- a/jobs.py
+++ b/jobs.py
@@ -10,0 +11,3 @@ def run():
+    # deliberately NOT calling refresh — would overwrite Total
+    x = "known issue in a string literal"
+    y = 1  # for now
@@ -30,0 +34,4 @@ def handler():
+    """Shopee leg.
+
+    Deliberately NOT fetch_orders_and_info: it would overwrite Total.
+    """
@@ -50,2 +58,1 @@ def old():
-    # removed: workaround comment goes away
-    z = 2
+    z = 3
diff --git a/notes.md b/notes.md
--- a/notes.md
+++ b/notes.md
@@ -1,0 +2,1 @@
+- workaround: documented debt, not code
diff --git a/app.js b/app.js
--- a/app.js
+++ b/app.js
@@ -5,0 +6,3 @@
+  /* hack: retry twice */
+  const s = ` + "`" + `a = """not a docstring""" ` + "`" + `;
+  // FIXME later
`

func TestAddedCommentLines(t *testing.T) {
	got := addedCommentLines(sampleDiff)
	var flat []string
	for _, l := range got {
		flat = append(flat, fmt.Sprintf("%s:%d:%s", l.File, l.Line, l.Text))
	}
	assert.Equal(t, []string{
		"jobs.py:11:# deliberately NOT calling refresh — would overwrite Total",
		`jobs.py:34:"""Shopee leg.`,
		"jobs.py:35:",
		"jobs.py:36:Deliberately NOT fetch_orders_and_info: it would overwrite Total.",
		`jobs.py:37:"""`,
		"app.js:6:/* hack: retry twice */",
		"app.js:8:// FIXME later",
	}, flat)
}

func TestCaveatCommentRE(t *testing.T) {
	for _, s := range []string{
		"# deliberately NOT calling refresh",
		"// Intentionally don't reorder these",
		"# workaround for pyodbc",
		"# known issue: totals drift",
		"# would overwrite Total",
		"# keep this for now",
		"// FIXME",
		"# WARNING: do not call twice",
		"# must not be called before register_job",
	} {
		assert.True(t, caveatCommentRE.MatchString(s), s)
	}
	for _, s := range []string{
		"# Reads the checked orders once and passes the list to every leg.",
		"# TODO #123 add momo leg",
		"# This function is not thread-safe by design; callers hold the lock.",
		"# assumes the caller validated member_id",
		"// Returns the forward index of the item.",
	} {
		assert.False(t, caveatCommentRE.MatchString(s), s)
	}
}

func commitFile(t *testing.T, dir, name, content, msg string) string {
	t.Helper()
	git, err := exec.LookPath("git")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	run := func(args ...string) string {
		cmd := exec.Command(git, append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	run("add", name)
	run("commit", "-q", "-m", msg)
	return run("rev-parse", "--short", "HEAD")
}

func TestAresCommentGateFailure(t *testing.T) {
	dir, _ := gitRepo(t)
	report := func(hash string) string {
		return "ARES COMPLETE\n\nTask list:\n1. [x] fix — done\nFiles created/modified: jobs.py\nLanded: main@" + hash + "\nNot run: none\n"
	}

	caveat := commitFile(t, dir, "jobs.py", "def run():\n    # deliberately NOT calling fetch_orders_and_info\n    return 1\n", "caveat")
	f := aresCommentGateFailure(report(caveat), dir)
	assert.Contains(t, f, "design caveat")
	assert.Contains(t, f, "jobs.py:2")
	assert.Contains(t, f, "deliberately NOT calling fetch_orders_and_info")
	assert.Contains(t, f, caveat)

	assert.Empty(t, aresCommentGateFailure(report(caveat)+"CAVEAT-COMMENT-KEPT: jobs.py:2 — vendor API quirk\n", dir), "waiver")

	clean := commitFile(t, dir, "jobs.py", "def run():\n    # Reads the checked orders once and passes the list along.\n    return 2\n", "clean")
	assert.Empty(t, aresCommentGateFailure(report(clean), dir), "clean comment")

	literal := commitFile(t, dir, "jobs.py", "def run():\n    msg = 'deliberately not a comment'\n    return 3\n", "literal")
	assert.Empty(t, aresCommentGateFailure(report(literal), dir), "string literal is code")

	assert.Empty(t, aresCommentGateFailure(report("deadbeef"), dir), "unknown hash fails open")
	assert.Empty(t, aresCommentGateFailure(report(caveat), t.TempDir()), "non-repo fails open")
	assert.Empty(t, aresCommentGateFailure(report(caveat), ""), "no cwd fails open")
	assert.Empty(t, aresCommentGateFailure("ARES COMPLETE\nLANDED-NOT-APPLICABLE: docs only\n", dir), "no hash")
}

func TestAresReportFailuresRunsCommentGate(t *testing.T) {
	dir, _ := gitRepo(t)
	caveat := commitFile(t, dir, "svc.go", "package svc\n\n// workaround: the job re-reads the flag per platform\nfunc Run() {}\n", "caveat")
	report := "ARES COMPLETE\n\nImplemented the fix.\n\nTask list:\n1. [x] fix — done\nFiles created/modified: svc.go\nLanded: main@" + caveat + "\nNot run: none\nTESTS-NOT-APPLICABLE: comment-only change\n"
	failures := aresReportFailures(report, subagentStopInput{AgentType: "ares", Cwd: dir})
	joined := strings.Join(failures, "\n")
	assert.Contains(t, joined, "design caveat")
	assert.Contains(t, joined, "svc.go:3")
}
