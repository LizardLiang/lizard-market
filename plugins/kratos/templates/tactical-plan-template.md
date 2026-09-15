# Tactical Plan — File Templates

Fetched by Odysseus with `<kratos-bin> template get tactical-plan-template`. Two shapes of the same file at `.claude/.Arena/tactical-plans/<slug>.md`: the **draft** opened before the first question, and the **ready** plan it is rewritten into in place.

## Draft (written before the first AskUserQuestion)

```markdown
---
status: draft
started: <ISO8601 — from `kratos now`>
---

> **DRAFT — clarification loop in progress. NOT ready for Ares.**
> If you are reading this, a plan session ended before it finished. The decisions
> below are real and already paid for. Resume with `/kratos:plan <task title>`.

# Tactical Plan: <Task Title>

## Request
<the user's original request, verbatim>

## Locked Decisions
<!-- one entry appended per answered question, oldest first -->
_None yet._

## Decision Tree
<the facet tree — every facet `[open]` at this point>

## Discovery Ledger
<the four-quadrant ledger from the Quadrant Sweep>
```

Locked Decisions entry, appended before the next question is asked:

```markdown
- **<facet>** — Q: <the question you asked> → **A: <the user's answer, verbatim>**
  <one line of any consequence the answer implies, if it isn't obvious>
```

## Ready (same path, rewritten in place at PLAN_READY)

```markdown
---
status: ready
started: <ISO8601 — carried over from the draft>
completed: <ISO8601>
---

# Tactical Plan: <Task Title>

## Request
<verbatim — carried over, never paraphrased>

## Summary
<2-4 sentences: goal, current context, intended result>

## Implementation Plan
1. <Concrete ordered step, with target area or file when known.>
2. <Next step.>
3. <Continue until Ares can execute without making major decisions.>

## Validation
- <Test, build, review, or manual verification command/scenario.>

## Assumptions
- <Assumption with risk-if-wrong, or "None.">

## Spec Delta
Capability: <capability> · File: `.claude/feature/<slug>/spec-delta/<capability>.md` · Validated: <yes / skipped — binary unavailable>
Status: **pending** — promote with `/kratos:spec-archive <slug>` after implementation.
Requirements: <one line per `### Requirement:` authored, one per covered facet>

## Discovery Ledger
<the four-quadrant ledger — every unknown-unknown technique shows what it surfaced or "nothing surfaced">

## Locked Decisions
<carried over verbatim from the draft — every question and answer, oldest first; never summarized, dropped, or reordered>

## Decision Tree
<every facet `[leaf]` or `[assumed: X]`; no `[open]` branches. ASCII format:>
Task: <title>
├── <facet>? → <answer> ✓ [leaf]
│   └── <sub-question>? → <answer> ✓ [leaf]
└── <facet>? → [assumed: X]

## Clarity
Target <t> · Approach <a> · Validation <v> → ambiguity <n> (PLAN_READY at ≤ 0.10) · Facets: <N covered / N total, 0 open> · Sweep: <run — M facets surfaced>

## Handoff To Ares
Use this plan as the execution contract. If implementation uncovers a major mismatch, stop and report the mismatch before changing direction.
```
