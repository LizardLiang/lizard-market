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

// A git whose diff subcommand fails: the reason names git's stderr.
func TestEvaluateLandedDiffFailureCarriesStderr(t *testing.T) {
	dir, _ := gitRepo(t)
	realGit, err := exec.LookPath("git")
	require.NoError(t, err)
	shim := t.TempDir()
	if runtime.GOOS == "windows" {
		body := "@echo off\r\n" +
			"echo %* | findstr /C:\" diff \" >nul && (echo boom-on-diff 1>&2 & exit /b 3)\r\n" +
			"\"" + realGit + "\" %*\r\n"
		require.NoError(t, os.WriteFile(filepath.Join(shim, "git.cmd"), []byte(body), 0o755))
	} else {
		body := "#!/bin/sh\n" +
			"case \" $* \" in *\" diff \"*) echo boom-on-diff >&2; exit 3;; esac\n" +
			"exec " + realGit + " \"$@\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(shim, "git"), []byte(body), 0o755))
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	failures := evaluateLanded(dir, "")
	require.NotEmpty(t, failures)
	joined := strings.Join(failures, "|")
	assert.Contains(t, joined, "git diff failed")
	assert.Contains(t, joined, "boom-on-diff", "git's stderr belongs in the message")
}
