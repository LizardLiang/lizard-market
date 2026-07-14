---
name: retro
description: Review a god-agent's accumulated feedback lessons, fold stable ones into the agent's instructions, and clear them
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

> The `KRATOS_ROOT` value echoed above is the plugin's absolute root — substitute it for every `<KRATOS_ROOT>` reference below (fallback: `plugins/kratos/` from project root). `<kratos-bin>` resolves to `<KRATOS_ROOT>/bin/kratos`, falling back to `~/.kratos/bin/kratos`.

# Kratos: Retro

Consolidate the per-agent feedback loop. Lessons captured from user corrections (`kratos feedback`) are a **buffer**: they inject at each god's spawn until they either prove stable — then they belong in the agent's definition permanently — or prove stale, then they should go. Retro is where that call gets made, with the user.

*"A god who repeats a mortal's correction twice has not yet learned it. Carve it into their nature."*

---

## Usage

```
/kratos:retro            # overview: lesson counts per agent
/kratos:retro <god>      # review + consolidate one agent's lessons
```

---

## Workflow

### No argument: Overview

```bash
<kratos-bin> feedback list
```

Group the lessons by `agent` and render the Overview format below (agent, lesson count, newest lesson age). Suggest `/kratos:retro <god>` for the agent with the most lessons.

### With `<god>`: Consolidate

1. **List** that agent's lessons, oldest context first:

   ```bash
   <kratos-bin> feedback list --agent <god>
   ```

   Show each lesson with its `id`, text, project, and age.

2. **Classify with the user** — for each lesson, ask (batch into one question set, not one per lesson):
   - **Fold** — stable, applies every time: goes into the agent's definition
   - **Keep** — still testing whether it holds: stays in the buffer, keeps injecting at spawn
   - **Discard** — stale, wrong, or superseded: remove without folding
   - **Promote** — the lesson is review-standards-shaped (a checkable rule about code, not agent behavior): becomes an active rule in the current project's `.claude/.Arena/review-rules/`

3. **Fold** the stable ones into `<KRATOS_ROOT>/agents/<god>.md`:
   - Edit the **body only** — append or extend a `## Learned Lessons` section near the end of the file
   - **NEVER touch the frontmatter** (name, description, model, tools) — it is codegen-owned; frontmatter changes require `make gen` in the dev tree and break the `make gen-check` CI gate
   - Rewrite each lesson as a direct instruction in the agent's voice, merging duplicates

3.5. **Promote** the review-standards-shaped ones directly to active rules:
   - Before writing, check whether the lesson's recorded project matches the current working directory — if it differs, warn the user (a lesson captured in one project may not apply to the one you're standing in) and confirm before proceeding
   - Write the rule into `.claude/.Arena/review-rules/<topic>.md` in the current project, using the format from `rules/default.md` (Project Rule File Format) — create the file/dir if absent, append a new `##` section if the file exists. Derive `<topic>` from the rule's code domain or language (e.g. `go.md`, `security.md`, `conventions.md`), matching the topic-file convention in `rules/default.md` — never name the file after the source agent (e.g. not `hermes.md`)
   - Promotion writes directly to active (no proposals/ draft step) — retro's own user confirmation in step 2 is the confirmation a draft would otherwise wait for

4. **Clear** every folded, discarded, or promoted lesson from the buffer:

   ```bash
   <kratos-bin> feedback rm <id>
   ```

   Kept lessons stay untouched.

5. **Report** using the Result format below: folded N, kept M, discarded K, promoted P.

### Marketplace-install caveat

If this plugin was installed from the marketplace (not a dev checkout), edits to `<KRATOS_ROOT>/agents/<god>.md` live in the plugin cache and are **overwritten on the next plugin update**. Warn the user before folding. Durable alternatives for installed users:
- **Keep** lessons in the buffer — they keep injecting at every spawn regardless
- Promote a truly global, non-agent-specific lesson to `<kratos-bin> memory add` instead

In the Kratos dev tree (`plugins/kratos/` inside the lizard-market repo), folding is durable — commit the agent file change.

### Fallback (binary unavailable)

The feedback store is SQLite-only. If `<kratos-bin>` is missing, report that retro requires the kratos binary (`kratos init` / rebuild) and stop.

---

## Output Format

### Overview

```
⚔️ KRATOS: RETRO — THE GODS' LEDGER ⚔️

Lessons awaiting consolidation:
┌────────────┬─────────┬───────────────┐
│ Agent      │ Lessons │ Newest        │
├────────────┼─────────┼───────────────┤
│ ares       │ 4       │ 2 days ago    │
│ hermes     │ 1       │ 5 days ago    │
└────────────┴─────────┴───────────────┘

💡 Consolidate: /kratos:retro ares
```

When the buffer is empty: report that no god currently carries lessons, and that lessons appear when the session-end sweep captures a user correction of an agent's work.

### Result

```
⚔️ KRATOS: RETRO — <god> ⚔️

Folded into agents/<god>.md (## Learned Lessons):
  ✓ [lesson, as folded]

Promoted to active rules (.claude/.Arena/review-rules/<topic>.md):
  ★ [lesson, as promoted]

Kept in buffer (still injecting at spawn):
  ~ [lesson]

Discarded:
  ✗ [lesson]

Folded 2 · Promoted 1 · Kept 1 · Discarded 1
```

---

## Kratos's Voice

- **Decisive**: recommend fold/keep/discard/promote for each lesson yourself; the user confirms or overrides
- **Faithful**: folded lessons keep the user's intent, but write them as the agent's own standing orders
- **Honest**: always surface the marketplace-install caveat before folding

**Note:** Retro dashboards use emoji as visual indicators. This is a functional exception to the "no emoji unless requested" rule.

*"What the mortal corrects once, the god remembers forever."*

---

**Opening the gods' ledger now...**
