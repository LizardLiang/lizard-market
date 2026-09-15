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

The trailer belongs only to the turn that was pointed here. It is not a standing rule: a later
turn prints no `Kratos:` line unless the hook or a command points here again, and a compaction
summary must not carry it forward as a per-turn requirement (one session printed
`Kratos: nothing to sweep` after eight consecutive turns, including "push it to master").

## Sweep 1 — user facts

Review the conversation for durable **user** facts: preferences, habits, weak spots,
corrections, working style.

- NOT project/task/repo facts — those belong in the project's Arena. When in doubt, save nothing.
- A fact that is only true inside this project (a tool quirk, a file layout, a naming rule for this
  repo) — and every fact that names a host, path, or repo file — MUST use `--project "<project-root>"`.
  If `--project` is rejected, skip the fact; never save it global.
- One choice is not a preference: save only a repeated or explicitly stated pattern. One-off
  decisions go to the handoff (Sweep 3).
- Never cite list positions (`#7`) — quote the text or use `--replace <id>`.
- One lesson → one store: memory OR feedback, never both.
- Do not write literal dotenv filenames in fact text — a policy hook blocks them; say "the env file".
- Never secrets.

1. Save at most 3 (no prior `memory list` — `add` checks the whole store itself):

   ```bash
   <kratos-bin> memory add "<fact>" --category <preference|habit|weak-spot|context> [--project "<project-root>"]
   ```

   - Only those four categories.
   - Each fact ≤200 characters — write it short the first time; the CLI rejects longer text and
     never truncates.
   - The CLI rejects a near-duplicate — same words (Jaccard) or the same content words in
     fewer/more words (overlap) — and names the existing memory id, its text, and which check
     fired. If the new fact supersedes it, re-run with `--replace <id>`; if both are genuinely
     distinct, `--force`. Never save a third rewording of the same lesson.

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

## Sweep 3 — handoff (only when this session changed project files)

Rewrite `.claude/.Arena/handoff.md` (create `.claude/.Arena/` if absent) in at most 12 lines: the
date, the files/pages/slides touched this session, in-flight items with their state, and the next
step. Overwrite — the latest handoff always wins. SessionStart prints its head after a compaction
and offers it on "continue", so a `/clear` never loses the working targets and nobody has to type
`@file#L12-40` anchors again. Skip this sweep when nothing changed on disk.

If nothing durable surfaces in any sweep, save nothing and print `Kratos: nothing to sweep`.
(A handoff rewrite alone counts as swept: `Kratos: swept 0, handoff updated`.)
