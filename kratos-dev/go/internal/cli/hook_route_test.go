package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func promptOut(t *testing.T, prompt string) string {
	t.Helper()
	raw, err := json.Marshal(hookInput{Prompt: prompt, SessionID: "route-test", Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	out := promptSubmitIn(raw)
	if out.HookSpecificOutput == nil {
		return ""
	}
	return out.HookSpecificOutput.AdditionalContext
}

// Regression for the 2026-08-31 fire on a /kratos:iris turn: the expanded
// launcher body ("!`echo "KRATOS_ROOT=…"` … agent load iris --resolve …
// Request: add this to todo and then have ares on it directly") carries no
// "/kratos:" prefix, so the god name in the arguments triggered a second
// routing through kratos:auto.
func TestPromptSubmit_SkipsExpandedLauncherBody(t *testing.T) {
	body := "!`echo \"KRATOS_ROOT=C:/Users/x/.claude/plugins/cache/lizard-plugins/kratos/2.105.0\"`\n\n" +
		"!`node \"C:/Users/x/.claude/plugins/cache/lizard-plugins/kratos/2.105.0/hooks/launch.cjs\" agent load iris --resolve`\n\n" +
		"---\n\nYou ARE Iris for this turn. Adopt the persona…\n\nRequest: add this to todo and then have ares on it directly"
	if got := promptOut(t, body); got != "" {
		t.Errorf("expanded launcher body must pass through untouched, got:\n%s", got)
	}
	if got := promptOut(t, "KRATOS_ROOT=C:/x\nplease have hermes review"); got != "" {
		t.Errorf("a KRATOS_ROOT echo marks a launcher body, got:\n%s", got)
	}
}

// TestIsExpandedLauncherBody pins the three examples the function's own comment
// uses, including the one it used to get wrong: an unanchored "contains" test
// classified "what does hooks/launch.cjs do?" as a launcher body, and a real
// user turn misread as a body keeps the previous turn's spent edit budget and
// its stand-down flag.
func TestIsExpandedLauncherBody(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   bool
	}{
		{
			name:   "the KRATOS_ROOT echo opens a real launcher body",
			prompt: "!`echo \"KRATOS_ROOT=C:/Users/x/.claude/plugins/cache/lizard-plugins/kratos/2.109.0\"`\n\n!`node \"C:/Users/x/.../hooks/launch.cjs\" agent load iris --resolve`\n\n---\n\nYou ARE Iris for this turn.",
			want:   true,
		},
		{
			name:   "the launch.cjs load line opens a real launcher body",
			prompt: "!`node \"C:/Users/x/.claude/plugins/cache/lizard-plugins/kratos/2.109.0/hooks/launch.cjs\" agent load odysseus --resolve --part body`",
			want:   true,
		},
		{
			name:   "a user question about launch.cjs is a real user turn",
			prompt: "what does hooks/launch.cjs do?",
			want:   false,
		},
		{
			name:   "a bare KRATOS_ROOT assignment still opens a body",
			prompt: "KRATOS_ROOT=C:/x\nplease have hermes review",
			want:   true,
		},
		{
			name:   "a bare agent load line opens a body",
			prompt: "agent load iris --resolve",
			want:   true,
		},
		{
			name:   "a user quoting the load line mid-sentence is a real turn",
			prompt: "why does the launcher run `agent load iris --resolve` twice?",
			want:   false,
		},
		{
			name:   "a user pasting a review comment about KRATOS_ROOT is a real turn",
			prompt: "the docs say plugin paths are written as KRATOS_ROOT=... — is that still true?",
			want:   false,
		},
		{
			name:   "leading blank lines do not hide the marker",
			prompt: "\n\n   \n!`echo \"KRATOS_ROOT=C:/x\"`",
			want:   true,
		},
		{
			name:   "empty prompt",
			prompt: "",
			want:   false,
		},
	}
	for _, tc := range cases {
		if got := isExpandedLauncherBody(tc.prompt); got != tc.want {
			t.Errorf("%s: isExpandedLauncherBody(%q) = %v, want %v", tc.name, tc.prompt, got, tc.want)
		}
	}
}

// "pass this to ares" already names the god: the hook now hands the model a
// one-line route instead of forcing a kratos:auto load that ends at the same
// god (4 of 5 fires in the review did exactly that).
func TestPromptSubmit_DirectRouteHintForAddressedGod(t *testing.T) {
	for _, prompt := range []string{
		"pass this to ares",
		"pass it to Ares, the label bug",
		"have hermes review the diff",
		"get odysseus to make a fix plan for #53",
		"Ares, fix the null pointer in auth.js",
		"plan it with odysseus",
		"debug this with hades",
		"review the dropdown change with hermes",
	} {
		got := promptOut(t, prompt)
		if !strings.Contains(got, "[KRATOS ROUTE]") {
			t.Errorf("%q: expected a direct-route hint, got:\n%s", prompt, got)
			continue
		}
		if strings.Contains(got, "kratos:auto\")") {
			t.Errorf("%q: direct route must not demand kratos:auto, got:\n%s", prompt, got)
		}
	}
	got := promptOut(t, "get odysseus to make a fix plan")
	if !strings.Contains(got, "kratos:plan") {
		t.Errorf("Odysseus routes inline via kratos:plan, got:\n%s", got)
	}
}

// Everything that is not a single addressed quick-route god keeps the full
// activation block: "Kratos" itself, two gods, a god mentioned in passing, a
// skill-god like Athena.
func TestPromptSubmit_FullBlockOtherwise(t *testing.T) {
	for _, prompt := range []string{
		"Kratos, build a login page",
		"ares and hermes should both look at this",
		"the ares module keeps failing",
		"ask athena to write the PRD",
	} {
		got := promptOut(t, prompt)
		if !strings.Contains(got, "You MUST invoke the Kratos skill") {
			t.Errorf("%q: expected the full activation block, got:\n%s", prompt, got)
		}
	}
}
