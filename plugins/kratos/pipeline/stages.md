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

CLARIFIED_REQUIREMENTS:
[All user answers from the clarification loop:]
- [Q1 header]: [user's answer]
- [Q2 header]: [user's answer]
- ... (all answers from all rounds)

Create prd.md before completing. Verify it exists before reporting completion.",
  description: "athena - create PRD"
)
```

---

## Stage 2: Review PRD (Athena)

```
Task(
  subagent_type: "kratos:athena",
  model: "opus",
  prompt: "MISSION: Review PRD
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Create prd-review.md before completing. Verify it exists before reporting completion.

Review prd.md and create prd-review.md. Update status.json with verdict.",
  description: "athena - review PRD"
)
```

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

## Stage 3 → 4 Transition: Optional Discuss (Themis)

After Stage 3 (complete or skipped), offer the Discuss phase before spawning Hephaestus:

```
AskUserQuestion(
  question: "Before Hephaestus specs, would you like to lock down implementation decisions? Themis will surface every choice Hephaestus would otherwise guess.",
  options: [
    { label: "Yes, lock decisions first", description: "Themis will surface implementation choices in batches until everything is locked" },
    { label: "No, proceed to tech spec", description: "Skip to Hephaestus — faster but he'll make assumptions" }
  ]
)
```

If user says No: set `stages["4-discuss"].status` to `"skipped"` in status.json.

If user chooses Discuss, Stage 4 runs in two phases. **Themis cannot call AskUserQuestion — Kratos handles all user interaction.**

### Phase 1: Identify Gray Areas

```
Task(
  subagent_type: "kratos:themis",
  model: "[sonnet|haiku|opus based on mode]",
  prompt: "MISSION: Identify Gray Areas (Pipeline Stage 4)
PHASE: IDENTIFY_GRAY_AREAS
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read prd.md, scout the codebase for existing patterns, load any prior context.md files from other features. Score clarity (Step 3b), then identify implementation choices Hephaestus would otherwise guess — target the weakest clarity dimension first (up to 4 per batch). Set MORE_QUESTIONS based on ambiguity score (true if > 0.20).

Return THEMIS_QUESTIONS_RESULT with CLARITY_SCORES. Do NOT write context.md yet.",
  description: "themis - identify gray areas"
)
```

### Phase 1.5: Question Loop (Kratos handles this)

This loop continues until Themis returns `MORE_QUESTIONS: false` (ambiguity ≤ 0.20). Max 5 rounds total to prevent runaway loops.

**After each Themis response**, display the clarity progress to the user:

```
📊 Clarity Progress

| Dimension   | Score | Weight | Contribution | Gap  |
|-------------|-------|--------|-------------|------|
| Goal        | [X]   | 0.40   | [X×0.40]    | [remaining] |
| Constraints | [X]   | 0.30   | [X×0.30]    | [remaining] |
| Criteria    | [X]   | 0.30   | [X×0.30]    | [remaining] |
| **Total**   |       |        | [sum]       |      |
| **Ambiguity** |     |        | [1 - sum]   |      |

Target: ≤ 0.20 | Current: [ambiguity] | Weakest: [dimension]
```

**Each round:**

**Step A — ask this batch:**

For each Q[N] in the current `THEMIS_QUESTIONS_RESULT`, ask and resolve one at a time:

```
AskUserQuestion(
  question: "[Q1_TITLE]\n\n[Q1_CONTEXT]\n\n[If debate mode + recommendation: 'I recommend [Label] — ']",
  options: [mapped from Q1_OPTIONS]
)
```

**If answer is a concrete choice** → record it, move to next question in batch.

**If answer is "Tell me more" / "Explore further"** → spawn Themis FOLLOW_UP for that area:

```
Task(
  subagent_type: "kratos:themis",
  model: "[sonnet|haiku|opus based on mode]",
  prompt: "PHASE: FOLLOW_UP
FEATURE: [feature-name]
GRAY_AREA: [Q1_TITLE]
ORIGINAL_CONTEXT: [Q1_CONTEXT]
USER_WANTS: [user's answer]

Return THEMIS_FOLLOWUP_RESULT. Concrete options only — no further explore.",
  description: "themis - follow-up [Q1_TITLE]"
)
```

Ask the follow-up, record the final answer. Maximum one follow-up per question.

**Step B — check ambiguity and decide:**

After the batch is fully answered, check `MORE_QUESTIONS` (driven by `CLARITY_SCORES.AMBIGUITY`):

- `MORE_QUESTIONS: false` (ambiguity ≤ 0.20) → proceed to Phase 2
- `MORE_QUESTIONS: true` (ambiguity > 0.20) → re-spawn Themis with all answers so far:

```
Task(
  subagent_type: "kratos:themis",
  model: "[sonnet|haiku|opus based on mode]",
  prompt: "PHASE: IDENTIFY_GRAY_AREAS
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

ANSWERED_SO_FAR:
[Q1_TITLE]
Answer: [answer]

[Q2_TITLE]
Answer: [answer]

[...all answers from all previous rounds...]

Surface the next batch of gray areas not yet covered. Return THEMIS_QUESTIONS_RESULT.",
  description: "themis - identify gray areas (round N)"
)
```

Then repeat from Step A with the new batch.

### Phase 2: Write context.md

After all answers are collected, re-spawn Themis with the decisions:

```
Task(
  subagent_type: "kratos:themis",
  model: "[sonnet|haiku|opus based on mode]",
  prompt: "MISSION: Write context.md (Pipeline Stage 4)
PHASE: WRITE_CONTEXT
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

DECISIONS:
[Q1_TITLE]
Answer: [user's answer]

[Q2_TITLE]
Answer: [user's answer]

[...all answers...]

CONTEXT_DATA from Phase 1:
SCOPE_BOUNDARY: [from THEMIS_QUESTIONS_RESULT]
CANONICAL_REFS: [from THEMIS_QUESTIONS_RESULT]
EXISTING_PATTERNS: [from THEMIS_QUESTIONS_RESULT]
REUSABLE_ASSETS: [from THEMIS_QUESTIONS_RESULT]
INTEGRATION_POINTS: [from THEMIS_QUESTIONS_RESULT]
PRIOR_DECISIONS_IMPORTED: [from THEMIS_QUESTIONS_RESULT]

Write context.md with these locked decisions. Update status.json.",
  description: "themis - write context.md"
)
```

---

## Stage 5: Create Tech Spec (Hephaestus)

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "opus",
  prompt: "MISSION: Create Technical Specification
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
PRD: Approved and ready at prd.md

Create tech-spec.md before completing. Verify it exists before reporting completion.

Create tech-spec.md based on the approved PRD. If context.md exists, read it first — it contains locked implementation decisions. Update status.json.",
  description: "hephaestus - create tech spec"
)
```

---

## Stages 6 + 7: Spec Reviews — Run in Parallel

Spawn both agents in the same response:

```
Task(
  subagent_type: "kratos:athena",
  model: "opus",
  prompt: "MISSION: Review Tech Spec (PM Perspective)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Create spec-review-pm.md before completing. Verify it exists before reporting completion.

Verify tech-spec.md aligns with prd.md requirements. Create spec-review-pm.md. Update status.json.",
  description: "athena - PM spec review"
)

Task(
  subagent_type: "kratos:apollo",
  model: "opus",
  prompt: "MISSION: Review Tech Spec (Architecture)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Create spec-review-sa.md before completing. Verify it exists before reporting completion.

Review tech-spec.md for technical soundness. Create spec-review-sa.md. Update status.json.",
  description: "apollo - SA spec review"
)
```

Wait for both to complete before proceeding.

---

## Stage 8: Create Test Plan (Artemis)

```
Task(
  subagent_type: "kratos:artemis",
  model: "sonnet",
  prompt: "MISSION: Create Test Plan
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Create test-plan.md before completing. Verify it exists before reporting completion.

Create comprehensive test-plan.md based on prd.md and tech-spec.md. Update status.json.",
  description: "artemis - create test plan"
)
```

After Stage 8 completes: read `plugins/kratos/pipeline/pre-implementation.md` and execute its procedure.

---

## Stage 9a: Implement Feature — Ares Mode

```
Task(
  subagent_type: "kratos:ares",
  model: "sonnet",
  prompt: "MISSION: Implement Feature
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Create implementation-notes.md before completing. Verify it exists before reporting completion.

Implement according to tech-spec.md. Write tests per test-plan.md. Create implementation-notes.md. Update status.json.",
  description: "ares - implement feature"
)
```

---

## Stage 9b: Create Implementation Tasks — User Mode

```
Task(
  subagent_type: "kratos:ares",
  model: "sonnet",
  prompt: "MISSION: Create Implementation Tasks (User Mode)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

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

## Stage 10: PRD Alignment Check (Hera)

```
Task(
  subagent_type: "kratos:hera",
  model: "sonnet",
  prompt: "MISSION: PRD Alignment Check
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Create prd-alignment.md before completing. Verify it exists before reporting completion.

Verify every acceptance criterion in prd.md is covered by a test and that tests pass. Create prd-alignment.md with verdict. Update status.json.",
  description: "hera - prd alignment check"
)
```

---

## Stage 11: Code Review + Risk Analysis — Parallel

Spawn both agents in the same response:

```
Task(
  subagent_type: "kratos:hermes",
  model: "opus",
  prompt: "MISSION: Code Review
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Create code-review.md before completing. Verify it exists before reporting completion.

Review implementation code. Create code-review.md with verdict. Update status.json.",
  description: "hermes - code review"
)

Task(
  subagent_type: "kratos:cassandra",
  model: "sonnet",
  prompt: "MISSION: Risk Analysis
MODE: pipeline
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Create risk-analysis.md before completing. Verify it exists before reporting completion.

Analyze changed files for security, breaking changes, edge cases, scalability, and dependency risks. Create risk-analysis.md with severity-rated findings. Update status.json.",
  description: "cassandra - risk analysis"
)
```

Wait for both to complete, then present merged results to the user.
