---
name: hephaestus-gate
description: Tech spec orchestration procedure for Kratos — codebase scan, approach selection, gray areas, and spec writing across three sub-phases
---

# Hephaestus Gate — Stage 4

You are **Kratos**. Run this procedure for Stage 4. Neither Task nor AskUserQuestion is available to Hephaestus at runtime, so you own both the codebase scan and the user interaction. Hephaestus does only the technical analysis and document writing.

---

## Phase 4a: Codebase Context (conditional)

Check whether the Arena already covers the feature's technical domains before spawning Metis.

```bash
ls .claude/.Arena/architecture/ .claude/.Arena/tech-stack/ .claude/.Arena/conventions/ 2>/dev/null
```

**If Arena shards exist for the relevant domains** — read them directly and pass their content to Hephaestus as `CODEBASE_CONTEXT`. Skip Metis. Record in `decisions.md`:
```markdown
## Codebase Scan Decision (Kratos — Stage 4)
Decision: Skip — Arena has sufficient context
Sources: [list of Arena shards read]
```

**If Arena is empty or missing the relevant domains** — spawn Metis. Read `.claude/feature/<name>/prd.md` first to identify the technical domains and architectural unknowns, then:

```
Task(
  subagent_type: "kratos:metis",
  model: "haiku",
  prompt: "MISSION: Codebase Scan
PHASE: CODEBASE_SCAN
FEATURE: [feature-name]

Read plugins/kratos/agents/metis.md for the full instruction set before starting.

METIS_SEARCH_DIRECTIVE:
  FEATURE_DOMAIN: [what is being built — one phrase]
  SEARCH_TARGETS:
    - area: [domain name] — [specific question to answer]
  FILE_HINTS:
    - [path pattern or directory likely to contain relevant code]
  QUESTIONS_TO_ANSWER:
    - [concrete question about existing patterns, dependencies, or constraints]

Return CODEBASE_SCAN_RESULT inline. Do not create any files.",
  description: "metis - codebase scan for tech spec (haiku)"
)
```

Record in `decisions.md`:
```markdown
## Codebase Scan Decision (Kratos — Stage 4)
Decision: Spawned Metis — Arena missing [domain(s)]
Key findings: [2-3 bullet points from CODEBASE_SCAN_RESULT]
```

---

## Phase 4b: Technical Analysis (Hephaestus ANALYZE)

Spawn Hephaestus with `PHASE: ANALYZE` and pass the codebase context (either from Metis or Arena):

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "opus",
  prompt: "MISSION: Technical Analysis
PHASE: ANALYZE
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/hephaestus.md for the full instruction set before starting.

CODEBASE_CONTEXT:
[paste either CODEBASE_SCAN_RESULT from Metis, or the relevant Arena shard contents]

Analyze the PRD and codebase context. Write tech-spec-proposal.md at .claude/feature/[feature-name]/tech-spec-proposal.md.
Do not write tech-spec.md yet — that comes after user input on approaches and gray areas.",
  description: "hephaestus - approach analysis"
)
```

Wait for `tech-spec-proposal.md` to appear before proceeding.

---

## Phase 4c: User Decisions (Kratos)

Read `tech-spec-proposal.md`. It contains the approaches and gray areas Hephaestus identified.

**Step 1 — Approach selection:**

```
AskUserQuestion(
  question: "Hephaestus has identified [N] implementation approaches:\n\n[paste approach names + one-line descriptions from tech-spec-proposal.md]\n\nRecommended: [RECOMMENDED] — [rationale]",
  header: "Approach selection",
  options: [
    { label: "[Approach A name]", description: "[effort] | [key pro] | [codebase fit]" },
    { label: "[Approach B name]", description: "[effort] | [key pro] | [codebase fit]" },
    { label: "Use recommended", description: "[recommended approach name]" }
  ]
)
```

Record the answer as `APPROACH_SELECTED`.

**Step 2 — Gray areas:**

For each `GA-*` entry in `tech-spec-proposal.md`, call `AskUserQuestion` once per turn:

```
AskUserQuestion(
  question: "[context from gray area] [the concrete question]",
  header: "[gray area title, max 30 chars]",
  options: [
    { label: "[Option A]", description: "[tradeoff]" },
    { label: "[Option B]", description: "[tradeoff]" },
    { label: "Defer to Hephaestus", description: "Let the spec author decide" }
  ]
)
```

Collect all answers. Ask one at a time.

---

## Phase 4d: Spec Writing (Hephaestus WRITE_SPEC)

Once all decisions are locked, spawn Hephaestus to write the final spec:

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "opus",
  prompt: "MISSION: Write Tech Spec
PHASE: WRITE_SPEC
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read plugins/kratos/agents/hephaestus.md for the full instruction set before starting.

APPROACH_SELECTED: [approach name]
GRAY_AREA_ANSWERS:
  - [GA-1 title]: [user's answer]
  - [GA-2 title]: [user's answer]
  ...

The codebase scan results are in decisions.md and tech-spec-proposal.md — read them, do not re-scan.
Create tech-spec.md before completing. Kratos validates the deliverable after you finish.",
  description: "hephaestus - write tech spec (opus)"
)
```
