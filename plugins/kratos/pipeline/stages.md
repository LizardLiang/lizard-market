---
name: stages
description: Exact Task invocations for each pipeline stage (0–11)
---

# Pipeline Stage Invocations

This file contains the exact Task tool calls for each pipeline stage. Read the relevant section when you need to spawn an agent for a specific stage.

**Resolving `<KRATOS_ROOT>`:** Leave `<KRATOS_ROOT>` tokens verbatim in spawn prompts — do not substitute them yourself. The SubagentStart hook (`hooks/path-inject.cjs`) injects the resolved absolute plugin root into every spawned subagent's context, and the subagent uses that injected path wherever it sees `<KRATOS_ROOT>`. `plugins/kratos/` from project root remains the last-resort fallback if no root was injected.

**IMPORTANT — filling `ORIGINAL_USER_REQUEST`:** Copy the user's actual first message (the request that triggered this pipeline) verbatim from your conversation context. Do NOT use the one-sentence description from start.md. Preserve original wording exactly — do not summarize, rephrase, or truncate.

---

## Stage 0: Research Project (Metis) — Optional Pre-flight

```
Task(
  subagent_type: "kratos:metis",
  model: "sonnet",
  prompt: "MISSION: Research Project
TARGET: [project root or specific area]
OUTPUT: .claude/.Arena/

Create ALL Arena documents before completing (sharded layout): index.md, project/overview.md, architecture/system-design.md, architecture/file-structure.md, tech-stack/ shards, conventions/ shards. Verify they exist before reporting completion.

Analyze the codebase and document findings in the Arena. This knowledge will guide all other gods.",
  description: "metis - research project"
)
```

---

## Stage 1: Create PRD — Two Sub-phases

### Stage 1a: Gap Analysis (Kratos, inline)

Read `<KRATOS_ROOT>/pipeline/gap-analysis.md` and run the gap analysis yourself. Use your own `AskUserQuestion` to collect requirements — do not delegate this to Athena.

### Stage 1b: PRD Creation (Athena)

Once WRITE_READY, follow the spawn template at the bottom of `pipeline/gap-analysis.md` to spawn Athena with `PHASE: CREATE_PRD` and the full Q&A transcript.

---

## Stage 2: Review PRD

Spawn Nemesis to review the PRD.

```
Task(
  subagent_type: "kratos:nemesis",
  model: "opus",
  prompt: "MISSION: Review PRD
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
ORIGINAL_USER_REQUEST: [paste the user's request verbatim — same text passed to Athena in Stage 1]

Read <KRATOS_ROOT>/agents/nemesis.md for the full instruction set before starting.

Create prd-challenge.md before completing. Kratos validates the deliverable after you finish.

Review prd.md and create prd-challenge.md. Update status.json with verdict.",
  description: "nemesis - full PRD review (adversarial + user advocate)"
)

```

Wait for completion before proceeding.

**Gate logic:**
- Verdict `approved` → proceed to Stage 3 gate
- Verdict `revisions` → return to Stage 1 (Athena rewrites PRD, Nemesis re-reviews)
- Verdict `rejected` → escalate to user — fundamental PRD issue

---

## Stage 2 → 3 Transition: Optional Decomposition (Daedalus)

After Stage 2 APPROVED verdict, check PRD complexity before spawning Hephaestus.

**Complexity signals** in `prd-review.md`:
- Many requirements / user stories
- Multiple modules/areas flagged
- Cross-cutting concerns (auth, caching, logging)
- External integrations
- Complex data relationships

If signals suggest a complex feature, offer decomposition:

```
AskUserQuestion(
  question: "This feature touches [N] areas with [description]. Decompose into phases before tech spec?",
  options: [
    { label: "Yes, local files", description: "Create decomposition.md in the feature folder" },
    { label: "Yes, Notion", description: "Create native Notion page with task database" },
    { label: "Yes, Linear", description: "Create Linear project with phase issues" },
    { label: "Yes, multiple targets", description: "Output to local files + Notion/Linear" },
    { label: "No, proceed", description: "Skip decomposition, go straight to discuss/tech spec" }
  ]
)
```

If user chooses decomposition:

```
Task(
  subagent_type: "kratos:daedalus",
  model: "[sonnet|haiku|opus based on mode]",
  prompt: "MISSION: Decompose Feature (Pipeline Stage 3)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
INPUT: Read prd.md in the feature folder
OUTPUT_TARGETS: [user selection]

Read <KRATOS_ROOT>/agents/daedalus.md for the full instruction set before starting.

Create decomposition.md at .claude/feature/[feature-name]/decomposition.md (for local target).

Run `<kratos-bin> template get decomposition-template` for the local file format.
[If Notion target]: Run `<kratos-bin> template get decomposition-notion-template`
[If Linear target]: Run `<kratos-bin> template get decomposition-linear-template`

This decomposition enriches the feature — downstream agents (Hephaestus, Artemis, Ares, Hermes) will reference your work.",
  description: "daedalus - decompose feature (pipeline)"
)
```

If user says No: set `stages["3-decomposition"].status` to `"skipped"` in status.json. See `<KRATOS_ROOT>/references/status-json-schema.md`.

---

## Stage 4: Tech Spec

Read `<KRATOS_ROOT>/pipeline/hephaestus-gate.md` and run the full procedure:
- Phase 4pre: Kratos runs Themis **inline** (if no context.md) — gray-area debate via AskUserQuestion → `context.md`
- Phase 4a: Kratos spawns Metis for codebase scan (or reads Arena directly)
- Phase 4b: Kratos runs Hephaestus ANALYZE **inline** — asks user about approaches + gray areas via AskUserQuestion → `tech-spec-proposal.md` with locked decisions
- Phase 4c: Kratos spawns Hephaestus WRITE_SPEC → `tech-spec.md` (if it returns HEPHAESTUS NEEDS DECISIONS, Kratos asks the user and re-spawns)

Interactive phases run inline because AskUserQuestion never reaches the user from a spawned subagent.

---
## Stage 5: Spec Review (Architecture)

Spawn Apollo to review the tech spec:

```
Task(
  subagent_type: "kratos:apollo",
  model: "opus",
  prompt: "MISSION: Review Tech Spec (Architecture)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/apollo.md for the full instruction set before starting.

Create spec-review-sa.md before completing. Kratos validates the deliverable after you finish.

Use Apollo's document-selection policy. If a needed prerequisite file is missing, stop and report the owning upstream agent to Kratos. Create spec-review-sa.md. Update status.json.",
  description: "apollo - SA spec review"
)
```

Wait for completion before proceeding.

---

## Stage 6: Create Test Plan (Artemis)

```
Task(
  subagent_type: "kratos:artemis",
  model: "sonnet",
  prompt: "MISSION: Create Test Plan
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/artemis.md for the full instruction set before starting.

Create test-plan.md before completing. Kratos validates the deliverable after you finish.

Use Artemis's document-selection policy. If a needed prerequisite file is missing, stop and report the owning upstream agent to Kratos. Create comprehensive test-plan.md. Update status.json.",
  description: "artemis - create test plan"
)
```

After Stage 6 completes: read `<KRATOS_ROOT>/pipeline/pre-implementation.md` and execute its procedure.

---

## Stage 7a: Implement Feature — Ares Mode

```
Task(
  subagent_type: "kratos:ares",
  model: "sonnet",
  mode: "acceptEdits",
  prompt: "MISSION: Implement Feature
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
NON-GOALS: [out-of-scope items from prd.md Non-Goals / tech-spec scope section]
STOP-CONDITIONS: missing prerequisite → report the owning upstream agent; genuine ambiguity → ARES NEEDS CLARIFICATION; wave boundary → ARES WAVE CHECKPOINT

Read <KRATOS_ROOT>/agents/ares.md for the full instruction set before starting.

Create implementation-notes.md before completing. Kratos validates the deliverable after you finish.

Use Ares's document-selection policy. If a needed prerequisite file is missing, stop and report the owning upstream agent to Kratos. Create implementation-notes.md. Update status.json.",
  description: "ares - implement feature"
)
```

**Why `mode: "acceptEdits"`**: Ares edits are auto-approved inside the subagent; Hermes review is the quality gate. Without it, a foreground spawn can silently hang on a per-edit permission prompt (observed: 71 minutes of a "running" Ares waiting for one Edit approval). Harnesses without the `mode` param ignore it — harmless.

**Wave checkpoints** (when `decomposition.md` exists): Ares returns `ARES WAVE CHECKPOINT` after each completed wave instead of finishing the mission — a spawned subagent cannot ask the user directly. When you receive it: ask the user via your own `AskUserQuestion` whether to commit a checkpoint, run the commit if accepted, then re-spawn Ares with the same prompt plus `CONTINUE_FROM_WAVE: [N+1]`. Repeat until Ares returns `ARES COMPLETE`.

**Clarification requests**: if Ares returns `ARES NEEDS CLARIFICATION` with a specific question, ask the user via `AskUserQuestion` and re-spawn with the answer appended to the prompt as `CLARIFICATION: [Q] → [A]`.

---

## Stage 7b: Create Implementation Tasks — User Mode

```
Task(
  subagent_type: "kratos:ares",
  model: "sonnet",
  mode: "acceptEdits",
  prompt: "MISSION: Create Implementation Tasks (User Mode)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/ares.md for the full instruction set before starting.

You are in USER MODE — create task files, do not implement code yourself.

1. Read templates: run `<kratos-bin> template get task-file-template` and `<kratos-bin> template get task-overview-template`
2. Create the tasks/ folder in the feature directory
3. Create 00-overview.md following the overview template
4. Create numbered task files (01-xxx.md, 02-xxx.md, etc.) following the task template
5. Each task file must contain complete, copy-paste ready code
6. Update status.json with mode: 'user' and the tasks array",
  description: "ares - create implementation tasks (user mode)"
)
```

After User Mode completes: do NOT spawn Hermes automatically. Tell the user to work through tasks with `/kratos:task-complete <id>` (backed by `<kratos-bin> pipeline tasks complete`, which validates User Mode, updates status.json atomically, and auto-advances 7→complete / 8→ready when the last task lands).

---

## Stage 8: PRD Alignment Check (Hera)

```
Task(
  subagent_type: "kratos:hera",
  model: "sonnet",
  prompt: "MISSION: PRD Alignment Check
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/hera.md for the full instruction set before starting.

Create prd-alignment.md before completing. If `prd.md` is missing when you need it, stop and report Athena as the owning upstream agent to Kratos.

Verify every acceptance criterion in prd.md is covered by a test and that tests pass. Create prd-alignment.md with verdict. Update status.json.",
  description: "hera - prd alignment check"
)
```

### After Hera Returns: Spec Archive Offer

If Hera's verdict is **aligned**, before spawning Stage 9, check whether the feature has a pending spec delta:

```bash
<kratos-bin> spec list --changes
```

If `.claude/feature/[feature-name]/spec-delta/*.md` has any pending (un-archived) file, offer the user a single confirmation prompt:

```
Feature [name] is aligned. Archive its spec delta into the living spec now?
  - Capability: [capability]
  - Changes: [N] added, [N] modified, [N] removed, [N] renamed

Archive? (y/n)
```

If confirmed, run:
```bash
<kratos-bin> spec archive [feature-name]
```

This is **decoupled** from Hera itself — the binary mechanically applies Athena's authored delta, no extra agent spawn. Declining does not lose the delta: it stays on disk until archived via this prompt, `/kratos:spec-archive`, or a later `kratos spec backfill` sweep. Do not auto-commit the resulting spec.md change.

---

## Stage 9: Code Review + Risk Analysis — Parallel

Spawn both agents in the same response:

```
Task(
  subagent_type: "kratos:hermes",
  model: "opus",
  prompt: "MISSION: Code Review
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/hermes.md for the full instruction set before starting.

Create code-review.md before completing. Kratos validates the deliverable after you finish.

Use Hermes's document-selection policy. If a needed prerequisite file is missing, stop and report the owning upstream agent to Kratos. Create code-review.md with verdict. Update status.json.",
  description: "hermes - code review"
)

Task(
  subagent_type: "kratos:cassandra",
  model: "sonnet",
  prompt: "MISSION: Risk Analysis
MODE: pipeline
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/cassandra.md for the full instruction set before starting.

Create risk-analysis.md before completing. Kratos validates the deliverable after you finish.

Use Cassandra's document-selection policy. If a needed prerequisite file is missing, stop and report the owning upstream agent to Kratos. Create risk-analysis.md with severity-rated findings. Update status.json.",
  description: "cassandra - risk analysis"
)
```

Wait for both to complete, then present merged results to the user.
