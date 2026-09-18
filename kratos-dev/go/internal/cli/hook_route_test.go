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

// TestIsHarnessPseudoPrompt pins the shared predicate: a <task-notification>,
// a subagent hand-back ("Another Claude session sent a message" plus an
// <agent-message> block), or a bare <agent-message> all count; a real user
// turn that merely mentions "another Claude session" mid-sentence does not.
func TestIsHarnessPseudoPrompt(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   bool
	}{
		{
			name:   "task notification",
			prompt: "<task-notification>\n<task-id>abc</task-id>\n</task-notification>",
			want:   true,
		},
		{
			name:   "subagent hand-back wrapper",
			prompt: "Another Claude session sent a message\n<agent-message from=\"agent-1\">\nDone. ares, hermes reviewed it.\n</agent-message>",
			want:   true,
		},
		{
			name:   "a bare agent-message block",
			prompt: "<agent-message from=\"agent-1\">\nyou do it\n</agent-message>",
			want:   true,
		},
		{
			name:   "leading whitespace does not hide the marker",
			prompt: "\n\n  Another Claude session sent a message\n<agent-message from=\"a\">hi</agent-message>",
			want:   true,
		},
		{
			name:   "a real user turn merely mentioning another Claude session",
			prompt: "could another Claude session help review this diff?",
			want:   false,
		},
		{
			name:   "a real user turn asking about hand-backs",
			prompt: "why does the hand-back message start with Another Claude session sent a message?",
			want:   false,
		},
		{
			name:   "empty prompt",
			prompt: "",
			want:   false,
		},
		// The following two shapes are the SAME preamble text, once bare and once
		// wrapped in <system-reminder> — the shape copied from a real transcript
		// (this machine's own project jsonl, 2026-09-18); no customer text, just
		// the structure Claude Code 2.1.276 adds in front of a task-notification.
		{
			name: "task notification with the SYSTEM NOTIFICATION preamble",
			prompt: "[SYSTEM NOTIFICATION - NOT USER INPUT]\n" +
				"This is an automated background-task event, NOT a message from the user.\n" +
				"Do NOT interpret this as user acknowledgement, confirmation, or response to any pending question.\n" +
				"\n" +
				"<task-notification>\n<task-id>abc</task-id>\n</task-notification>",
			want: true,
		},
		{
			name: "the same preamble wrapped in a system-reminder tag",
			prompt: "<system-reminder>\n" +
				"[SYSTEM NOTIFICATION - NOT USER INPUT]\n" +
				"This is an automated background-task event, NOT a message from the user.\n" +
				"\n" +
				"<task-notification>\n<task-id>abc</task-id>\n</task-notification>\n" +
				"</system-reminder>",
			want: true,
		},
		{
			name:   "a bare hand-back wrapped in a system-reminder tag",
			prompt: "<system-reminder>\nAnother Claude session sent a message\n<agent-message from=\"a\">hi</agent-message>\n</system-reminder>",
			want:   true,
		},
		{
			name:   "a real user turn mentioning system-reminder mid-sentence",
			prompt: "does the <system-reminder> block ever hide a real task-notification?",
			want:   false,
		},
	}
	for _, tc := range cases {
		if got := isHarnessPseudoPrompt(tc.prompt); got != tc.want {
			t.Errorf("%s: isHarnessPseudoPrompt(%q) = %v, want %v", tc.name, tc.prompt, got, tc.want)
		}
	}
}

// TestPromptSubmit_SkipsHandBackPseudoPrompt pins the keyword-injection gap: a
// subagent hand-back is model output, and a report naming "ares, hermes" fired
// the Kratos keyword injection in the 2026-09-18 review — steering a
// system-level instruction from text the user never wrote.
func TestPromptSubmit_SkipsHandBackPseudoPrompt(t *testing.T) {
	handback := "Another Claude session sent a message\n" +
		"<agent-message from=\"agent-1\">\n" +
		"Review complete. ares, hermes both signed off on the change.\n" +
		"</agent-message>"
	if got := promptOut(t, handback); got != "" {
		t.Errorf("a subagent hand-back must pass through untouched, got:\n%s", got)
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
