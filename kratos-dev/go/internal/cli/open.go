package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// openLauncher starts a program without waiting for it. Tests replace it so no
// browser starts.
var openLauncher = func(prog string, args ...string) error {
	return exec.Command(prog, args...).Start()
}

// openLookup finds a program by name or install path.
var openLookup = func(name string) (string, bool) {
	if filepath.IsAbs(name) {
		if info, err := os.Stat(name); err == nil && !info.IsDir() {
			return name, true
		}
		return "", false
	}
	p, err := exec.LookPath(name)
	return p, err == nil
}

// openBrowserCandidates lists the browsers to try, in order: Chrome, then Edge.
func openBrowserCandidates(goos string, env func(string) string) [][]string {
	switch goos {
	case "windows":
		var chrome, edge []string
		for _, root := range []string{env("ProgramFiles"), env("ProgramFiles(x86)"), env("LOCALAPPDATA")} {
			if root == "" {
				continue
			}
			chrome = append(chrome, filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"))
			edge = append(edge, filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"))
		}
		return [][]string{append(chrome, "chrome"), append(edge, "msedge")}
	case "darwin":
		return [][]string{{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}, {"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"}}
	}
	return [][]string{{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"}, {"microsoft-edge", "microsoft-edge-stable", "msedge"}}
}

// openFileURL turns a path into a file:/// URL.
func openFileURL(abs string) string {
	p := filepath.ToSlash(abs)
	if strings.HasPrefix(p, "/") {
		return "file://" + p
	}
	return "file:///" + p
}

// openTarget opens path and returns the name of the program it started. An
// .html file goes to Chrome, then Edge, then the OS default handler.
func openTarget(path, goos string, env func(string) string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	if strings.EqualFold(filepath.Ext(abs), ".html") {
		url := openFileURL(abs)
		for _, names := range openBrowserCandidates(goos, env) {
			for _, name := range names {
				if prog, ok := openLookup(name); ok {
					if err := openLauncher(prog, url); err != nil {
						return "", err
					}
					return prog, nil
				}
			}
		}
	}
	switch goos {
	case "windows":
		return "cmd /c start", openLauncher("cmd", "/c", "start", "", abs)
	case "darwin":
		return "open", openLauncher("open", abs)
	}
	return "xdg-open", openLauncher("xdg-open", abs)
}

// OpenCmd returns `kratos open <path>`: show a file to the user. It exists so
// the plan page opens in a browser, not in the editor that owns .html on the
// user's machine.
func OpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <path>",
		Short: "Open a file for the user (.html goes to Chrome, then Edge, then the OS default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prog, err := openTarget(args[0], runtime.GOOS, os.Getenv)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "opened with %s\n", prog)
			return nil
		},
	}
}
