---
name: pre-implementation
description: Ensure decomposition exists and select implementation mode before spawning Ares
---

# Pre-Implementation Gate

This procedure runs between Stage 8 (Test Plan) and Stage 9 (Implementation). It ensures Ares receives a structured task queue rather than a monolithic spec, which prevents context rot and produces a bisectable git history.

---

## Step 1: Check for Decomposition

Check if `decomposition.md` exists at `.claude/feature/<name>/decomposition.md`.

**If it already exists** (created at Stage 3) — skip to Step 2. Daedalus's earlier work is still valid.

**If it does NOT exist** — spawn Daedalus now. The tech-spec is a better input than the PRD at this stage because it contains the implementation plan Ares will follow.

```
Task(
  subagent_type: "kratos:daedalus",
  model: "[sonnet|haiku|opus based on mode]",
  prompt: "MISSION: Decompose Feature (Pre-Implementation)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
INPUT: Read tech-spec.md (primary) and prd.md (context)
OUTPUT_TARGETS: local

Create decomposition.md at .claude/feature/[feature-name]/decomposition.md

This decomposition feeds directly into Ares. Every task must include:
- wave: which execution wave it belongs to (determines parallelism)
- verify: a runnable command Ares uses to confirm the task is done before committing

Read the template at plugins/kratos/templates/decomposition-template.md.",
  description: "daedalus - pre-implementation decomposition"
)
```

After Daedalus completes, verify `decomposition.md` exists before proceeding.

---

## Step 2: Select Implementation Mode

Ask the user how they want implementation handled:

```
AskUserQuestion(
  question: "How should implementation be handled?",
  options: [
    "Ares Mode (AI implements the code directly)",
    "User Mode (create detailed task files for manual implementation)"
  ]
)
```

Update status.json: set `stages["9-implementation"].status` to `"in-progress"` and `stages["9-implementation"].mode` to `"ares"` or `"user"`. See `plugins/kratos/references/status-json-schema.md` for schema.

---

## Returns

- `"ares-mode"` — proceed to Stage 9a
- `"user-mode"` — proceed to Stage 9b
