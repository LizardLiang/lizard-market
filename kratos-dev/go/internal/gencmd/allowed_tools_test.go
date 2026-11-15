package gencmd

import (
	"strings"
	"testing"
)

// Every launcher must pre-approve the Bash commands its !`…` lines run: a
// failed permission check on dynamic injection aborts the whole command, and
// the persona never loads.
func TestLauncherAllowedTools(t *testing.T) {
	cases := []struct {
		partial string
		want    string
	}{
		{"", "Bash(echo:*), Bash(node:*)"},
		{"Read, Write", "Read, Write, Bash(echo:*), Bash(node:*)"},
		{"Read, Bash, Task", "Read, Bash, Task"},
		{"Bash(node:*), Read", "Bash(node:*), Read, Bash(echo:*)"},
		{" Read ,, Read ", "Read, Bash(echo:*), Bash(node:*)"},
	}
	for _, c := range cases {
		if got := LauncherAllowedTools(c.partial); got != c.want {
			t.Errorf("LauncherAllowedTools(%q) = %q, want %q", c.partial, got, c.want)
		}
	}
}

// The rendered launcher uses the documented inline injection form and carries
// the fallback instruction for the case where the loader still did not run.
func TestRenderCommand_InlineInjectionAndFallback(t *testing.T) {
	a := &Agent{Name: "iris", Description: "Personal secretary"}
	out := RenderCommand(a, nil, false)

	for _, want := range []string{
		"allowed-tools: Bash(echo:*), Bash(node:*)\n",
		"!`echo \"KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}\"`",
		"!`node \"${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs\" agent load iris --resolve --part body`",
		"!`node \"${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs\" agent load iris --resolve --part extras`",
		"If no `# Iris -` agent definition appears above",
		"execute `node \"${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs\" agent load iris --resolve --part body` and then `node \"${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs\" agent load iris --resolve --part extras`",
		"If the definition above is a `<persisted-output>` preview",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("launcher missing %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "!") && !strings.HasPrefix(line, "!`") {
			t.Errorf("bare ! line is not dynamic injection: %q", line)
		}
	}
}
