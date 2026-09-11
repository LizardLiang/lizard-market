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
