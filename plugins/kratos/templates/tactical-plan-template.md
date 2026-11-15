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

## Current Design
_Filled by the Flow Trace when it triggers; else "n/a — no shared state or multi-step flow touched"._

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
run: all-phases | per-phase   # all-phases: the orchestrator chains every phase without stopping; per-phase: report and wait after each
---

# Tactical Plan: <Task Title>

## Request
<verbatim — carried over, never paraphrased>

## Summary
<2-4 sentences: goal, current context, intended result>

## Current Design
<"n/a — no shared state or multi-step flow touched", or:>
Flow: 1. <step — file:function> → 2. … → n. <finalize>
| State | Set by | Cleared by | Scope | Stale when |
|-------|--------|------------|-------|------------|
Invariants: <one per line + writer evidence>
Fit or fight: A) patch in place — <consequence> · B) restructure — <consequence> → **chosen: <pole>** — <why / the user's answer>

## First-time user journey
_Required when the plan adds or changes a screen, flow or setting; else "n/a — no user-facing change"._
- Start: <the empty state a first-time user begins from>
- Prerequisites: <each one, and who performs it — user / system / setup wizard>
- Edit here: <the one page where each job is configured> · Read here: <the one page where its result is seen>
- Cold open: <one paragraph — what a first-time user sees on opening the page and what they do next, or a mock link>
- Existing surfaces for the same thing: <pages/cards that already configure or show this, or "none">

## Phases (optional — split when the plan touches ≥2 subsystems or has more than 6 steps)
| Phase | Scope | Steps | Verify |
|---|---|---|---|
| 1 | <subsystem or layer> | <step numbers> | <command or check that proves this phase landed> |

Each phase must be committable on its own: one fresh Ares spawn per phase (`PHASE: n of m`), implementing only that phase's steps, verifying, committing, and handing back an `ARES PHASE CHECKPOINT` before the next phase starts.

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
<carried over verbatim from the draft — every question and answer, oldest first; never summarized, dropped, or reordered. Any appendix or mechanics section the draft held is carried over too — the ready rewrite appends, never replaces>

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
