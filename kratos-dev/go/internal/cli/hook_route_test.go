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

// The pass/hand/send/give/forward form needs no object: "pass to Odysseus"
// addresses the god as clearly as "pass it to Odysseus".
func TestPromptSubmit_DirectRouteWithoutObject(t *testing.T) {
	for _, prompt := range []string{
		"pass to Odysseus",
		"send to ares",
		"ok, hand to hermes",
		"forward to hades please",
	} {
		got := promptOut(t, prompt)
		if !strings.Contains(got, "[KRATOS ROUTE]") {
			t.Errorf("%q: expected a direct-route hint, got:\n%s", prompt, got)
		}
	}
}

// A prompt that questions earlier work names a god but asks for an answer,
// not a route: it gets the soft note, never the "Do NOT respond" block or a
// direct route.
func TestPromptSubmit_ComplaintGetsSoftNote(t *testing.T) {
	for _, c := range []struct{ prompt, god string }{
		{"why did you do it without having ares review the plan", "Ares"},
		{"Why are you asking odysseus again?", "Odysseus"},
		{"why didn't hermes catch this", "Hermes"},
		{"why pass it to ares", "Ares"},
		{"you shipped this without having hermes look at it", "Hermes"},
		{"you edited three files without asking odysseus first", "Odysseus"},
	} {
		got := promptOut(t, c.prompt)
		want := "[KRATOS NOTE] The user mentions " + c.god + " while questioning earlier work — answer them first; route to " + c.god + " only if they ask."
		if got != want {
			t.Errorf("%q: expected the soft note\n want: %s\n  got: %s", c.prompt, want, got)
		}
	}
	// "without" alone, not followed by a mentioned god, is not a complaint.
	got := promptOut(t, "have ares fix it without using mocks")
	if !strings.Contains(got, "[KRATOS ROUTE]") {
		t.Errorf("expected a direct route, got:\n%s", got)
	}
}

// A model named with an addressed god spawns that god through the Agent tool
// with the model, instead of the default route. Odysseus leaves the inline
// kratos:plan skill and must return his questions in his final message.
func TestPromptSubmit_ModelOverrideSpawnsAgent(t *testing.T) {
	for _, c := range []struct{ prompt, god, model string }{
		{"have ares fix it using opus", "ares", "opus"},
		{"review the diff with hermes on sonnet", "hermes", "sonnet"},
		{"pass to odysseus using Fable", "odysseus", "fable"},
		{"get odysseus to plan it with haiku", "odysseus", "haiku"},
	} {
		got := promptOut(t, c.prompt)
		want := `Agent(subagent_type: "kratos:` + c.god + `", model: "` + c.model + `")`
		if !strings.Contains(got, "[KRATOS ROUTE]") || !strings.Contains(got, want) {
			t.Errorf("%q: expected route with %s, got:\n%s", c.prompt, want, got)
		}
		if strings.Contains(got, `Skill(skill: "kratos:plan")`) {
			t.Errorf("%q: a model override must not use the inline Skill route, got:\n%s", c.prompt, got)
		}
		if c.god == "odysseus" && !strings.Contains(got, "blocking questions in his final message") {
			t.Errorf("%q: Odysseus spawn must ask for his questions in the final message, got:\n%s", c.prompt, got)
		}
	}
	// Without a model the Odysseus route stays inline.
	if got := promptOut(t, "pass to odysseus"); !strings.Contains(got, `Skill(skill: "kratos:plan")`) {
		t.Errorf("expected the inline kratos:plan route, got:\n%s", got)
	}
}
