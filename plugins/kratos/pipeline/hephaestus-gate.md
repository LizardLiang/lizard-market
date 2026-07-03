---
name: hephaestus-gate
description: Tech spec orchestration procedure for Kratos — codebase scan, approach selection, gray areas, and spec writing across three sub-phases
---

# Hephaestus Gate — Stage 4

You are **Kratos**. Run this procedure for Stage 4. You own the codebase scan (via Metis/Arena), phase orchestration, and **all user interaction**: `AskUserQuestion` only reaches the user from the top-level session, so the interactive phases (4pre Themis, 4b Hephaestus ANALYZE) run **inline in your context** — never as Task spawns. Only non-interactive phases (Metis scan, WRITE_SPEC) are spawned.

---

## Phase 4pre: Discussion Lock (Themis, inline) — conditional

Check whether `context.md` already exists for this feature:

```bash
ls .claude/feature/[feature-name]/context.md 2>/dev/null
```

**If `context.md` exists** — read it and store its content as `DECISIONS_CONTEXT`. Skip to Phase 4a.

**If `context.md` does not exist** — run Themis **inline**: read `<KRATOS_ROOT>/agents/themis.md`, adopt the persona, and execute its full loop yourself (PRD scan → codebase scout → clarity scoring → `AskUserQuestion` per gray area → write `context.md`). Do NOT spawn a subagent — a spawned Themis's questions never reach the user, and it would lock fabricated decisions.

After writing context.md, set status.json `4-tech-spec.status` to `in-progress`, add a document entry for context.md, and read it as `DECISIONS_CONTEXT`.

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

## Phase 4b: Technical Analysis + User Decisions (Hephaestus ANALYZE, inline)

Run Hephaestus ANALYZE **inline**: read `<KRATOS_ROOT>/agents/hephaestus.md`, adopt the persona, and execute `PHASE: ANALYZE` yourself with:

- `CODEBASE_CONTEXT`: either CODEBASE_SCAN_RESULT from Metis, or the relevant Arena shard contents
- `DECISIONS_CONTEXT`: context.md content from Phase 4pre — Themis's locked implementation decisions

Analyze the PRD and codebase context. Respect all decisions already locked in `DECISIONS_CONTEXT` — do not re-surface gray areas already resolved there. Ask the user about approach selection and gray areas using your own `AskUserQuestion` (this is why the phase runs inline — a spawned Hephaestus's questions never surface, and the "user decisions" would be fabricated). Write `tech-spec-proposal.md` with all decisions locked. Do not write tech-spec.md yet — that comes in WRITE_SPEC phase.

Before proceeding, verify `tech-spec-proposal.md` contains a `## Selected Approach` section (not just `## Recommended`) — this confirms the user was asked and the decision locked.

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
Read context.md (Themis's locked decisions) if it exists — those decisions are hard constraints.
You are a spawned subagent: AskUserQuestion will NOT reach the user. If a genuine new gap surfaces that the PRD, context.md, and locked decisions cannot answer, stop and return a HEPHAESTUS NEEDS DECISIONS block instead of guessing.
Create tech-spec.md before completing. Kratos validates the deliverable after you finish.",
  description: "hephaestus - write tech spec (opus)"
)
```

**If Hephaestus returns `HEPHAESTUS NEEDS DECISIONS`** (a list of questions it could not resolve): ask the user each question via your own `AskUserQuestion`, append the answers to `GRAY_AREA_ANSWERS`, and re-spawn WRITE_SPEC with the expanded list. Do not let the spec proceed with silent assumptions.
