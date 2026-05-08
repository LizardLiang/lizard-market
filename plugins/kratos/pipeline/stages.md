---
name: stages
description: Exact Task invocations for each pipeline stage (0–11)
---

# Pipeline Stage Invocations

This file contains the exact Task tool calls for each pipeline stage. Read the relevant section when you need to spawn an agent for a specific stage.

---

## Stage 0: Research Project (Metis) — Optional Pre-flight

```
Task(
  subagent_type: "kratos:metis",
  model: "sonnet",
  prompt: "MISSION: Research Project
TARGET: [project root or specific area]
OUTPUT: .claude/.Arena/

Create ALL Arena documents before completing: project-overview.md, tech-stack.md, architecture.md, file-structure.md, conventions.md. Verify they exist before reporting completion.

Analyze the codebase and document findings in the Arena. This knowledge will guide all other gods.",
  description: "metis - research project"
)
```

---

## Stage 1: Create PRD (Athena) — Single Phase

Athena handles gap analysis and user clarification internally via AskUserQuestion. Spawn once:

```
Task(
  subagent_type: "kratos:athena",
  model: "opus",
  prompt: "MISSION: Gap Analysis + PRD Creation
PHASE: GAP_ANALYSIS
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
REQUIREMENTS: [user's requirements]

Read plugins/kratos/agents/athena.md for the full instruction set before starting.

Analyze requirements for gaps. If clear (ambiguity ≤ 0.20), write the PRD immediately. If unclear, use AskUserQuestion to gather clarification (up to 4 questions per round, max 3 rounds), then write the PRD.

Create prd.md before completing. Kratos validates the deliverable after you finish.",
  description: "athena - gap analysis + PRD"
)
```

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

Read plugins/kratos/agents/nemesis.md for the full instruction set before starting.

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

Read plugins/kratos/agents/daedalus.md for the full instruction set before starting.

Create decomposition.md at .claude/feature/[feature-name]/decomposition.md (for local target).

Read the decomposition template at plugins/kratos/templates/decomposition-template.md for the local file format.
[If Notion target]: Read plugins/kratos/templates/decomposition-notion-template.md
[If Linear target]: Read plugins/kratos/templates/decomposition-linear-template.md

This decomposition enriches the feature — downstream agents (Hephaestus, Artemis, Ares, Hermes) will reference your work.",
  description: "daedalus - decompose feature (pipeline)"
)
```

If user says No: set `stages["3-decomposition"].status` to `"skipped"` in status.json. See `plugins/kratos/references/status-json-schema.md`.

---

## Stage 4: Tech Spec (Hephaestus) — Four Sub-Phases

Stage 4 runs as four sequential sub-phases. Kratos orchestrates all spawns — agents cannot spawn each other directly.

---

### Sub-Phase 0: Produce Directive

Hephaestus reads the PRD and outputs a targeted scan directive for Metis, plus a resume context payload for his own Phase 1.

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "sonnet",
  prompt: "MISSION: Produce Codebase Scan Directive
PHASE: PRODUCE_DIRECTIVE
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/hephaestus.md for the full instruction set before starting.

Read prd.md. Identify what the codebase must answer before you can propose implementation approaches. Return HEPHAESTUS_DIRECTIVE_RESULT. Do not scan the codebase yourself.",
  description: "hephaestus - produce directive (sonnet)"
)
```

---

### Sub-Phase 0.5: Dispatch Handler (Kratos)

After Phase 0 completes, parse `HEPHAESTUS_DIRECTIVE_RESULT`:

1. Extract `METIS_SEARCH_DIRECTIVE` block
2. Store `RESUME_CONTEXT` block (Kratos holds this for Phase 1 re-injection)
3. Read `DISPATCH_TO` / `DISPATCH_PHASE` / `DISPATCH_RETURN_TO` / `DISPATCH_RETURN_PHASE`
4. Spawn Metis:

```
Task(
  subagent_type: "kratos:metis",
  model: "haiku",
  prompt: "MISSION: Codebase Scan
PHASE: CODEBASE_SCAN
FEATURE: [feature-name]

Read plugins/kratos/agents/metis.md for the full instruction set before starting.

METIS_SEARCH_DIRECTIVE:
[paste METIS_SEARCH_DIRECTIVE block from Hephaestus Phase 0 verbatim]

Return CODEBASE_SCAN_RESULT inline. Do not create any files.",
  description: "metis - codebase scan (haiku)"
)
```

---

### Sub-Phase 1: Approach Selection + Gray Areas + Tech Spec

When Metis returns, merge RESUME_CONTEXT + CODEBASE_SCAN_RESULT and spawn Hephaestus:

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "opus",
  prompt: "MISSION: Propose Approaches + Clarify + Write Tech Spec
PHASE: IDENTIFY_GRAY_AREAS
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/hephaestus.md for the full instruction set before starting.

[paste RESUME_CONTEXT from Phase 0 verbatim]

[paste CODEBASE_SCAN_RESULT from Metis verbatim]

Use AskUserQuestion to present 2-3 named approaches and get the user's choice. Then ask gray-area questions one at a time. After collecting all answers, write tech-spec.md in the same invocation.

Create tech-spec.md before completing. Kratos validates the deliverable after you finish.",
  description: "hephaestus - approach + gray areas + tech spec (opus)"
)
```

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

Read plugins/kratos/agents/apollo.md for the full instruction set before starting.

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

Read plugins/kratos/agents/artemis.md for the full instruction set before starting.

Create test-plan.md before completing. Kratos validates the deliverable after you finish.

Use Artemis's document-selection policy. If a needed prerequisite file is missing, stop and report the owning upstream agent to Kratos. Create comprehensive test-plan.md. Update status.json.",
  description: "artemis - create test plan"
)
```

After Stage 6 completes: read `plugins/kratos/pipeline/pre-implementation.md` and execute its procedure.

---

## Stage 7a: Implement Feature — Ares Mode

```
Task(
  subagent_type: "kratos:ares",
  model: "sonnet",
  prompt: "MISSION: Implement Feature
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/ares.md for the full instruction set before starting.

Create implementation-notes.md before completing. Kratos validates the deliverable after you finish.

Use Ares's document-selection policy. If a needed prerequisite file is missing, stop and report the owning upstream agent to Kratos. Create implementation-notes.md. Update status.json.",
  description: "ares - implement feature"
)
```

---

## Stage 7b: Create Implementation Tasks — User Mode

```
Task(
  subagent_type: "kratos:ares",
  model: "sonnet",
  prompt: "MISSION: Create Implementation Tasks (User Mode)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/ares.md for the full instruction set before starting.

You are in USER MODE — create task files, do not implement code yourself.

1. Read templates: plugins/kratos/templates/task-file-template.md and task-overview-template.md
2. Create the tasks/ folder in the feature directory
3. Create 00-overview.md following the overview template
4. Create numbered task files (01-xxx.md, 02-xxx.md, etc.) following the task template
5. Each task file must contain complete, copy-paste ready code
6. Update status.json with mode: 'user' and the tasks array",
  description: "ares - create implementation tasks (user mode)"
)
```

After User Mode completes: do NOT spawn Hermes automatically. Tell the user to work through tasks with `/kratos:task-complete <id>`.

---

## Stage 8: PRD Alignment Check (Hera)

```
Task(
  subagent_type: "kratos:hera",
  model: "sonnet",
  prompt: "MISSION: PRD Alignment Check
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/hera.md for the full instruction set before starting.

Create prd-alignment.md before completing. If `prd.md` is missing when you need it, stop and report Athena as the owning upstream agent to Kratos.

Verify every acceptance criterion in prd.md is covered by a test and that tests pass. Create prd-alignment.md with verdict. Update status.json.",
  description: "hera - prd alignment check"
)
```

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

Read plugins/kratos/agents/hermes.md for the full instruction set before starting.

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

Read plugins/kratos/agents/cassandra.md for the full instruction set before starting.

Create risk-analysis.md before completing. Kratos validates the deliverable after you finish.

Use Cassandra's document-selection policy. If a needed prerequisite file is missing, stop and report the owning upstream agent to Kratos. Create risk-analysis.md with severity-rated findings. Update status.json.",
  description: "cassandra - risk analysis"
)
```

Wait for both to complete, then present merged results to the user.
