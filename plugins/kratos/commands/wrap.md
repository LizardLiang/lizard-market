---
name: wrap
description: Write a session handoff for the next session, run the memory sweep inline, and prepare to /clear
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

> The `KRATOS_ROOT` value echoed above is the plugin's absolute root — substitute it for every `<KRATOS_ROOT>` reference below (fallback: `plugins/kratos/` from project root). `<kratos-bin>` resolves to `<KRATOS_ROOT>/bin/kratos`, falling back to `~/.kratos/bin/kratos`.

# Kratos: Wrap

Write down what this session knows before it's gone. A bare `/clear` loses working context — decisions, gotchas, next steps — that lives only in this conversation. Wrap captures it to `.claude/.Arena/handoff.md` so the next session boots with full context, then runs the same memory sweep the Stop hook would, so nothing gets swept twice.

*"What the mind holds, the page must keep — or it is lost when the mind empties."*

---

## Critical: run inline, never as a subagent

This command MUST execute in the main conversation. Do **not** spawn a subagent (`Task`/`Agent`) for any part of it — the entire point is capturing *this* session's own working context. A subagent starts with none of it and would write an empty or fabricated handoff.

---

## Step 1: Fetch the handoff structure

```bash
<kratos-bin> template get handoff-template
```

If the binary is unavailable or the command fails, use this inline fallback structure instead:

```markdown
# Session Handoff

**Date:** [YYYY-MM-DD HH:MM]

## Shipped
- [outcome]

## Decisions
- [decision] — [rationale]

## In-flight
- [item] — [state / what's left]

## Gotchas
- [gotcha]

## Next steps
1. [step]
```

## Step 2: Write the handoff

Fill the structure from the **conversation's own context** — not from the DB, not from `recall`, not guessed. Write from what actually happened this session:

- **Shipped** — concrete outcomes that landed (files created/modified, bugs fixed, decisions executed). One bullet each.
- **Decisions** — each with a one-line rationale (the why, not just the what). Include a rejected alternative only if it would otherwise get re-litigated next session.
- **In-flight** — work started but unfinished: what's done, what's left, where to pick up.
- **Gotchas** — non-obvious traps the next session would otherwise rediscover the hard way.
- **Next steps** — ordered, actionable, what to do first.

Keep it concise — **~80 lines max**. This is a handoff, not a transcript.

Create `.claude/.Arena/` if it doesn't exist, then write (overwrite) `.claude/.Arena/handoff.md`. Overwriting is intentional — the latest handoff always wins; there is no history to preserve.

## Step 3: Run the memory sweep inline

Mirror the same sweep the Stop-hook (`memory-sweep.cjs`) would otherwise run, so wrapping now means the hook has nothing left to do (see Step 4).

1. **User facts** — review the conversation for durable user facts (preferences, habits, weak spots, corrections, working style — not project/task facts, never secrets). Project/task/repo facts belong in the project's Arena, not memory — when in doubt, save nothing.
   ```bash
   <kratos-bin> memory list
   ```
   Dedupe against that list, then save at most 3:
   ```bash
   <kratos-bin> memory add "<fact>" --category <preference|habit|weak-spot|context>
   ```
   Use only those four categories. Each fact ≤200 characters.

2. **Agent lessons** — if the user corrected or redirected work a specific Kratos god-agent had just delivered this session:
   ```bash
   <kratos-bin> feedback list --agent <god>
   ```
   Dedupe against that list, then save at most 2:
   ```bash
   <kratos-bin> feedback add --agent <god> "<lesson>"
   ```
   A lesson is what that agent should do differently next time, ≤200 characters. Only corrections clearly attributable to one agent's finished output — general preferences belong in memory, not feedback.

If nothing durable surfaces in either sweep, save nothing — do not force it.

## Step 4: Finish

Print the literal marker line `KRATOS WRAP COMPLETE` (this tells the Stop-hook sweep to skip re-running — see `hooks/memory-sweep.cjs`), then suggest `/clear`.

---

## Output Format

```
KRATOS WRAP

Handoff written: .claude/.Arena/handoff.md
Memory: [N] fact(s) saved / none
Feedback: [N] lesson(s) saved for [agent] / none

KRATOS WRAP COMPLETE

Ready to /clear — next session prints a one-line notice; say "continue" (or run /kratos:recall) to load this handoff.
```

---

**Now write the handoff from this session's own context.**
