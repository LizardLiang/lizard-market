package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// defaultStateDir is the project directory that holds Kratos state when no
// setting names another one.
const defaultStateDir = ".claude"

var (
	stateDirOnce  sync.Once
	stateDirValue = defaultStateDir
)

// validStateDirName reports whether v is one path segment that starts with a dot.
func validStateDirName(v string) bool {
	if len(v) < 2 || v[0] != '.' {
		return false
	}
	if v == ".." || strings.ContainsAny(v, `/\`) || strings.ContainsRune(v, 0) {
		return false
	}
	return true
}

// stateDirName returns the project directory name for Kratos state.
// Precedence: env KRATOS_STATE_DIR, then ~/.kratos/config.json "state_dir",
// then ".claude". An invalid value prints one warning and falls back. The
// result is computed once per process.
func stateDirName() string {
	stateDirOnce.Do(func() {
		stateDirValue = computeStateDir(os.Stderr)
	})
	return stateDirValue
}

func computeStateDir(warn *os.File) string {
	if v := os.Getenv("KRATOS_STATE_DIR"); v != "" {
		if validStateDirName(v) {
			return v
		}
		fmt.Fprintf(warn, "kratos: ignoring invalid KRATOS_STATE_DIR %q (need one segment starting with '.')\n", v)
		return defaultStateDir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return defaultStateDir
	}
	data, err := os.ReadFile(filepath.Join(home, ".kratos", "config.json"))
	if err != nil {
		return defaultStateDir
	}
	var cfg struct {
		StateDir string `json:"state_dir"`
	}
	if json.Unmarshal(data, &cfg) != nil || cfg.StateDir == "" {
		return defaultStateDir
	}
	if validStateDirName(cfg.StateDir) {
		return cfg.StateDir
	}
	fmt.Fprintf(warn, "kratos: ignoring invalid state_dir %q in ~/.kratos/config.json (need one segment starting with '.')\n", cfg.StateDir)
	return defaultStateDir
}

// resetStateDirForTest clears the per-process cache.
func resetStateDirForTest() {
	stateDirOnce = sync.Once{}
	stateDirValue = defaultStateDir
}

// featureRoot is <root>/<state>/feature.
func featureRoot(root string) string { return filepath.Join(root, stateDirName(), "feature") }

// arenaRoot is <root>/<state>/.Arena.
func arenaRoot(root string) string { return filepath.Join(root, stateDirName(), ".Arena") }

// tmpRoot is <root>/<state>/tmp.
func tmpRoot(root string) string { return filepath.Join(root, stateDirName(), "tmp") }

// stateRel returns the slash-form relative prefix "<state>/<sub>".
func stateRel(sub string) string { return stateDirName() + "/" + sub }

// stateQuoted is the state directory name escaped for use inside a regexp.
func stateQuoted() string { return regexp.QuoteMeta(stateDirName()) }

// rewriteStateText replaces the four Kratos-owned .claude prefixes in text
// with the resolved state directory. It is a no-op for the default.
func rewriteStateText(s string) string {
	name := stateDirName()
	if name == defaultStateDir {
		return s
	}
	for _, sub := range []string{"feature", ".Arena", ".arena", "tmp"} {
		s = strings.ReplaceAll(s, defaultStateDir+"/"+sub, name+"/"+sub)
	}
	return s
}
