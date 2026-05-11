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

## Stage 1: Create PRD (Athena) — Two-Phase Process

Stage 1 is multi-step because Athena cannot ask the user questions directly (AskUserQuestion is unavailable to subagents). Kratos handles the clarification loop.

### Phase 1: Gap Analysis

```
Task(
  subagent_type: "kratos:athena",
  model: "opus",
  prompt: "MISSION: Gap Analysis
PHASE: GAP_ANALYSIS
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
REQUIREMENTS: [user's requirements]

Read plugins/kratos/agents/athena.md for the full instruction set before starting.

Analyze these requirements for gaps and ambiguities. Score clarity (Step 2b) and include CLARITY_SCORES in output. Return structured questions in the GAP_ANALYSIS_RESULT format targeting the weakest dimension. Do NOT write the PRD yet.",
  description: "athena - gap analysis"
)
```

### Phase 1.5: Clarification Loop (Kratos handles this)

When Athena returns her gap analysis:

1. Parse the `GAP_ANALYSIS_RESULT`
2. Display **clarity progress** to the user:

```
📊 Requirements Clarity

| Dimension   | Score | Weight | Contribution | Gap  |
|-------------|-------|--------|-------------|------|
| Goal        | [X]   | 0.40   | [X×0.40]    | [remaining] |
| Constraints | [X]   | 0.30   | [X×0.30]    | [remaining] |
| Criteria    | [X]   | 0.30   | [X×0.30]    | [remaining] |
| **Total**   |       |        | [sum]       |      |
| **Ambiguity** |     |        | [1 - sum]   |      |

Target: ≤ 0.20 | Current: [ambiguity] | Weakest: [dimension]
```

3. If `WRITE_READY: true` (ambiguity ≤ 0.20) → skip to Phase 2
4. If questions exist → call `AskUserQuestion` for each question **one at a time**:

```
AskUserQuestion(
  question: [Q1_QUESTION from Athena's output],
  options: [mapped from Q1_OPTIONS — each "label | description" becomes an option]
)
```

Call AskUserQuestion with each question sequentially (up to 4). Wait for the answer before asking the next. Never batch questions into a single text output — the tool gives the user clickable options.

After answers are collected: if ambiguity is still > 0.20, re-spawn Athena for another gap analysis round with answers included (max 3 rounds total).

### Phase 2: Write PRD

```
Task(
  subagent_type: "kratos:athena",
  model: "opus",
  prompt: "MISSION: Create PRD
PHASE: CREATE_PRD
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
REQUIREMENTS: [user's original requirements]

Read plugins/kratos/agents/athena.md for the full instruction set before starting.

CLARIFIED_REQUIREMENTS:
[All user answers from the clarification loop:]
- [Q1 header]: [user's answer]
- [Q2 header]: [user's answer]
- ... (all answers from all rounds)

Create prd.md before completing. Kratos validates the deliverable after you finish.",
  description: "athena - create PRD"
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

Run `~/.kratos/bin/kratos template get decomposition-template` for the local file format.
[If Notion target]: Run `~/.kratos/bin/kratos template get decomposition-notion-template`
[If Linear target]: Run `~/.kratos/bin/kratos template get decomposition-linear-template`

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

### Sub-Phase 1: Approach Proposal + Gray Areas

When Metis returns, merge RESUME_CONTEXT + CODEBASE_SCAN_RESULT and spawn Hephaestus Phase 1:

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "opus",
  prompt: "MISSION: Propose Approaches + Identify Gray Areas
PHASE: IDENTIFY_GRAY_AREAS
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/hephaestus.md for the full instruction set before starting.

[paste RESUME_CONTEXT from Phase 0 verbatim]

[paste CODEBASE_SCAN_RESULT from Metis verbatim]

First return HEPHAESTUS_APPROACH_RESULT (2-3 named approaches). Then return HEPHAESTUS_QUESTIONS_RESULT (gray areas). Return both blocks.",
  description: "hephaestus - approach proposal + gray areas (opus)"
)
```

---

### Sub-Phase 1.5: Approach Selection + Clarification Loop (Kratos)

When Hephaestus Phase 1 returns, parse both result blocks:

**Step A — Present approaches:**

```
AskUserQuestion(
  question: "Hephaestus proposes [N] implementation approaches:\n\n[APPROACH_A.NAME]: [DESCRIPTION]\n[APPROACH_B.NAME]: [DESCRIPTION]\n[...]\n\nRecommended: [RECOMMENDED] — [RATIONALE]",
  options: [
    { label: "[APPROACH_A.NAME]", description: "Effort: [EFFORT] | [key pro]" },
    { label: "[APPROACH_B.NAME]", description: "Effort: [EFFORT] | [key pro]" },
    [APPROACH_C if present],
    { label: "Defer to Hephaestus", description: "Use recommended approach" }
  ]
)
```

Record the chosen approach as `CHOSEN_APPROACH`.

**Step B — Gray area Q&A loop:**

If `HEPHAESTUS_QUESTIONS_RESULT` contains questions, run the same clarification loop as before (AskUserQuestion per gray area, max 3 rounds, stop when ambiguity ≤ 0.20).

---

### Sub-Phase 2: Write Tech Spec

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "opus",
  prompt: "MISSION: Create Technical Specification
PHASE: CREATE_TECH_SPEC
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/hephaestus.md for the full instruction set before starting.

CHOSEN_APPROACH: [approach name and one-line description]

DECISIONS:
[Q1_TITLE]
Answer: [user's answer]

[...all answers from all rounds...]

Create tech-spec.md. Kratos validates the deliverable after you finish.",
  description: "hephaestus - create tech spec (opus)"
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

1. Read templates: run `~/.kratos/bin/kratos template get task-file-template` and `~/.kratos/bin/kratos template get task-overview-template`
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
