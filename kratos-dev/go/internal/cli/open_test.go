package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenTargetPicksChromeThenEdgeThenDefault(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "plan.html")
	if err := os.WriteFile(page, []byte("<p>x</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	var started []string
	oldLaunch, oldLookup := openLauncher, openLookup
	defer func() { openLauncher, openLookup = oldLaunch, oldLookup }()
	openLauncher = func(prog string, args ...string) error {
		started = append([]string{prog}, args...)
		return nil
	}
	env := func(k string) string {
		if k == "ProgramFiles" {
			return `C:\Program Files`
		}
		return ""
	}
	cases := []struct {
		name   string
		have   string
		wantIn string
	}{
		{"chrome first", "chrome.exe", "chrome.exe"},
		{"edge when no chrome", "msedge.exe", "msedge.exe"},
		{"os default when neither", "", "cmd /c start"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			started = nil
			openLookup = func(name string) (string, bool) {
				if tc.have != "" && strings.HasSuffix(name, tc.have) {
					return name, true
				}
				return "", false
			}
			prog, err := openTarget(page, "windows", env)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(prog, tc.wantIn) {
				t.Errorf("program = %q, want it to contain %q", prog, tc.wantIn)
			}
			if tc.have != "" && (len(started) != 2 || !strings.HasPrefix(started[1], "file://")) {
				t.Errorf("launch args = %v, want a file:// URL", started)
			}
		})
	}
}

func TestOpenTargetMissingFile(t *testing.T) {
	if _, err := openTarget(filepath.Join(t.TempDir(), "none.html"), "linux", os.Getenv); err == nil {
		t.Error("missing file opened without error")
	}
}
