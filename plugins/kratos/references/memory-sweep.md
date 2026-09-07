# Memory Sweep Protocol

Two sweeps over the conversation since the last sweep, executed **quietly**: no narration
between tool calls. Finish with exactly one short line — `Kratos: swept <n>` (n = memories plus
lessons saved) or `Kratos: nothing to sweep` — so the turn has visible output and the harness does
not ask you to "continue". Nothing else. Do NOT call `EndConversation`, `Stop`, or any other
session-ending tool: "no narration" means emit no text, not end the session.

`<kratos-bin>` below is the kratos binary path given by whatever pointed you here (the Stop-hook
instruction, or a command like `/kratos:wrap`). `<project-root>` is the repository root of the
current working directory.

The sweep can run several times in a long session. Only look at what happened since the previous
`Kratos: swept` / `Kratos: nothing to sweep` line (or since the session start if there is none).

## Sweep 1 — user facts

Review the conversation for durable **user** facts: preferences, habits, weak spots,
corrections, working style.

- NOT project/task/repo facts — those belong in the project's Arena. When in doubt, save nothing.
- A fact that is only true inside this project (a tool quirk, a file layout, a naming rule for this
  repo) may be saved with `--project "<project-root>"` so it is injected only there.
- Never secrets.

1. Skim the recent list for overlap: `<kratos-bin> memory list --limit 40`
2. Save at most 3:

   ```bash
   <kratos-bin> memory add "<fact>" --category <preference|habit|weak-spot|context> [--project "<project-root>"]
   ```

   - Only those four categories.
   - Each fact ≤200 characters — write it short the first time; the CLI rejects longer text and
     never truncates.
   - The CLI rejects a near-duplicate and names the existing memory id. If the new fact
     supersedes it, re-run with `--replace <id>`; if both are genuinely distinct, `--force`.
     Never save a third rewording of the same lesson.

## Sweep 2 — agent lessons

Only if the user corrected or redirected work a specific Kratos god-agent had just delivered.

1. Dedupe first: `<kratos-bin> feedback list --agent <god>`
2. Save at most 2:

   ```bash
   <kratos-bin> feedback add --agent <god> "<lesson>"
   ```

   - A lesson is what that agent should do differently next time, ≤200 characters.
   - Only corrections clearly attributable to one agent's finished output — general
     preferences belong in memory, not feedback.

If nothing durable surfaces in either sweep, save nothing and print `Kratos: nothing to sweep`.
