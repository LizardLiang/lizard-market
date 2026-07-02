---
name: hephaestus-gate
description: Tech spec orchestration procedure for Kratos — codebase scan, approach selection, gray areas, and spec writing across three sub-phases
---

# Hephaestus Gate — Stage 4

You are **Kratos**. Run this procedure for Stage 4. Hephaestus has AskUserQuestion and handles user interaction directly during ANALYZE. You own the codebase scan (via Metis/Arena) and phase orchestration.

---

## Phase 4pre: Discussion Lock (Themis) — conditional

Check whether `context.md` already exists for this feature:

```bash
ls .claude/feature/[feature-name]/context.md 2>/dev/null
```

**If `context.md` exists** — read it and store its content as `DECISIONS_CONTEXT`. Skip to Phase 4a.

**If `context.md` does not exist** — spawn Themis to surface PRD-level gray areas and lock implementation direction:

```
Task(
  subagent_type: "kratos:themis",
  model: "sonnet",
  prompt: "MISSION: Discussion Lock (Pipeline Phase 4pre)
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/themis.md for the full instruction set before starting.

Surface implementation gray areas from prd.md, debate options with the user, and lock decisions into context.md.
After writing context.md, set status.json `4-tech-spec.status` to `in-progress` and add a document entry for context.md.",
  description: "themis - discussion lock (phase 4pre)"
)
```

Wait for `context.md` to appear before proceeding. Then read it as `DECISIONS_CONTEXT`.

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

Read <KRATOS_ROOT>/agents/metis.md for the full instruction set before starting.

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

## Phase 4b: Technical Analysis + User Decisions (Hephaestus ANALYZE)

Spawn Hephaestus with `PHASE: ANALYZE` and pass the codebase context (either from Metis or Arena). Hephaestus will ask the user directly about approach selection and gray areas via AskUserQuestion, then write `tech-spec-proposal.md` with locked decisions.

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "opus",
  prompt: "MISSION: Technical Analysis
PHASE: ANALYZE
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/hephaestus.md for the full instruction set before starting.

CODEBASE_CONTEXT:
[paste either CODEBASE_SCAN_RESULT from Metis, or the relevant Arena shard contents]

DECISIONS_CONTEXT:
[paste context.md content from Phase 4pre — Themis's locked implementation decisions]

Analyze the PRD and codebase context. Respect all decisions already locked in DECISIONS_CONTEXT — do not re-surface gray areas already resolved there.
Ask the user about approach selection and gray areas using AskUserQuestion. Write tech-spec-proposal.md with all decisions locked.
Do not write tech-spec.md yet — that comes in WRITE_SPEC phase.",
  description: "hephaestus - analysis + user decisions"
)
```

Wait for `tech-spec-proposal.md` to appear before proceeding. Verify it contains a `## Selected Approach` section (not just `## Recommended`) — this confirms Hephaestus asked the user and locked the decision.

---

## Phase 4c: Spec Writing (Hephaestus WRITE_SPEC)

Once all decisions are locked, spawn Hephaestus to write the final spec:

```
Task(
  subagent_type: "kratos:hephaestus",
  model: "opus",
  prompt: "MISSION: Write Tech Spec
PHASE: WRITE_SPEC
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/

Read <KRATOS_ROOT>/agents/hephaestus.md for the full instruction set before starting.

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
