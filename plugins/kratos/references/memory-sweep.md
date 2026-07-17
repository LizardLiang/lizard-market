# Memory Sweep Protocol

Two sweeps over the finished conversation, executed **silently**: no narration between tool
calls and **no final message** — after the last tool call, stop without producing any
user-visible text. This runs at session end; anything printed is noise the user already asked
not to see.

`<kratos-bin>` below is the kratos binary path given by whatever pointed you here (the Stop-hook
instruction, or a command like `/kratos:wrap`).

## Sweep 1 — user facts

Review the conversation for durable **user** facts: preferences, habits, weak spots,
corrections, working style.

- NOT project/task/repo facts — those belong in the project's Arena. When in doubt, save nothing.
- Never secrets.

1. Dedupe first: `<kratos-bin> memory list`
2. Save at most 3:

   ```bash
   <kratos-bin> memory add "<fact>" --category <preference|habit|weak-spot|context>
   ```

   - Only those four categories.
   - Each fact ≤200 characters — write it short the first time.

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

If nothing durable surfaces in either sweep, save nothing. Either way, end the turn with no
visible output.
