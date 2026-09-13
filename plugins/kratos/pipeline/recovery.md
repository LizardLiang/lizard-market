---
name: recovery
description: Stage-by-stage troubleshooting and pipeline recovery procedures for Kratos orchestrator
---

# Pipeline Recovery

Read this when a stage produces an unexpected verdict, an agent fails to deliver, or the pipeline is stuck.

---

## Stage Troubleshooting

### Stage 1 (Athena) — PRD Creation

**Athena wrote a PRD that drifted from the original request**
- This is caught by the self-alignment check in Athena's `decisions.md`
- Look for `## Intent Alignment` section — it records `Alignment: rewritten N times`
- If drift is detected after the fact: re-spawn Athena with the original request verbatim and `PHASE: CREATE_PRD` with explicit note to address the original ask

**prd.md is incomplete or skeleton-only**
- Athena sometimes creates a skeleton early and fails to fill it
- Re-spawn Athena with `PHASE: CREATE_PRD` and all clarified requirements

---

### Stage 2 (Nemesis) — PRD Review

**Nemesis verdict is `rejected` but user disagrees**
- Escalate: present Nemesis's specific objections to the user
- Ask user to either: address the objections (re-write PRD), or override with explicit acknowledgment
- Never proceed past a `rejected` verdict without user confirmation

**Nemesis verdict is `revisions` but loop count is high (3+ iterations)**
- Present the outstanding revision items to the user
- Ask if they want to: (a) continue revising, (b) accept the PRD with known gaps documented, or (c) restart from scratch
- A PRD that cannot converge after 3 Athena/Nemesis cycles likely has a fundamental scope problem

---

### Stage 4 (Hephaestus) — Tech Spec

**Hephaestus is presenting too many approaches (more than 3)**
- This is a violation of its own protocol — it should present exactly 2-3 options
- If it happens: ask Hephaestus to consolidate to the top 2-3 options

**Codebase scan (Phase 4a) was skipped or failed**
- Hephaestus has no Task tool and cannot spawn Metis. Kratos owns the scan: `<KRATOS_ROOT>/pipeline/hephaestus-gate.md` Phase 4a (read the Arena shards, or spawn Metis `PHASE: CODEBASE_SCAN` on haiku).
- If `decisions.md` has no `## Codebase Scan Decision (Kratos — Stage 4)` entry, or WRITE_SPEC ran without `CODEBASE_CONTEXT`: run Phase 4a yourself, record the decision entry, then re-spawn Hephaestus WRITE_SPEC with the scan results in `decisions.md` and `tech-spec-proposal.md`.

**tech-spec.md is missing the implementation sequence**
- The spec must contain a step-by-step ordered list of changes
- Apollo will catch this — verdict `unsound` means Hephaestus must revise

---

### Stage 5 (Apollo) — Architecture Review

**Apollo verdict is `unsound` but Hephaestus keeps producing the same spec**
- Likely: Hephaestus is not incorporating Apollo's specific objections
- Solution: read `spec-review-sa.md`, extract the exact objections, and re-spawn Hephaestus with those objections quoted verbatim in the mission

**Apollo is reviewing without enough context**
- Apollo should read: `tech-spec.md` for design detail, `prd.md` for requirements, Arena/architecture for prior decisions
- If Apollo's review seems surface-level, re-spawn with explicit instruction to read both documents before evaluating

---

### Stage 6 (Artemis) — Test Plan

**test-plan.md does not map to PRD requirements**
- Every test case should reference a requirement ID (REQ-001, REQ-002, etc.)
- Re-spawn Artemis with explicit instruction to read `prd.md` first and ensure every P0/P1 requirement has at least one test case

---

### Stage 7 (Ares) — Implementation

**Ares is implementing outside the spec scope (scope creep)**
- Ares has a rule: "Only modify lines traceable to the spec or request"
- If creep detected: point to the specific additions, ask Ares to remove them and log as debt in `implementation-notes.md`

**Ares has a failing test it cannot fix**
- After one re-spawn attempt: stop, present the failing test to the user, ask for guidance
- Never re-spawn Ares more than once for the same failing test

**User Mode tasks are marked complete but code was not actually written**
- `/kratos:task-complete` is self-reported — it does not verify code was written
- Hera (Stage 8) will catch gaps — tests not passing means alignment will fail

---

### Stage 8 (Hera) — PRD Alignment

**Hera verdict is `gaps` in a loop (adding coverage → still gaps)**
- After 2 Ares re-spawns for coverage: escalate to user
- Present the specific uncovered acceptance criteria
- Ask if they want to: (a) adjust the acceptance criteria, (b) accept with documented gaps, or (c) continue

**Hera verdict is `misaligned`**
- This is the most serious outcome — it means the implementation answers a different question than the PRD
- Do NOT attempt to patch this with Ares re-spawning
- Escalate to user: present the specific misalignment (what PRD says vs what was implemented)
- Options: re-write the feature (return to Stage 1), or amend the PRD to match implementation (return to Stage 1, Athena revises)

---

### Stage 9 (Hermes + Cassandra) — Review

**Hermes BLOCKER survives after one Ares fix**
- Do NOT re-spawn Ares again — surface the unresolved BLOCKER to user
- Present: the original BLOCKER, what Ares did, why it persists
- Ask user how to proceed

**Cassandra verdict is `blocked`** (any CRITICAL finding, or 4+ HIGH findings)
- VICTORY is blocked — do not proceed
- Present the CRITICAL / HIGH findings (security, breaking changes) to user
- Ares addresses those findings; then re-run Stage 9

**Hermes approved but Cassandra blocked (or vice versa)**
- VICTORY requires BOTH: Hermes `approved` AND Cassandra `clear|caution`
- If only one passes: surface the remaining blocker, fix it, re-run only the failing review

---

## Stage Transition Logic

> **Fallback / reference — `<kratos-bin> pipeline next` encodes this table.** Kratos uses the CLI (`commands/main.md` Step 2); consult this table only when the binary is unavailable or you need to sanity-check its output.

| Stage Complete | Verdict | Next |
|----------------|---------|------|
| *(new feature)* | — | **1-prd** — read `<KRATOS_ROOT>/pipeline/gap-analysis.md` and run the inline gap analysis loop. Do NOT spawn Athena with PHASE: GAP_ANALYSIS. |
| 1-prd | — | 2-prd-review (nemesis) |
| 2-prd-review | Approved | Complexity check → optional decomposition → optional discuss → 4-tech-spec |
| 2-prd-review | Revisions | 1-prd (athena) — revise PRD and re-review |
| 2-prd-review | Rejected | Blocked — escalate to user, fundamental PRD issue |
| 3-decomposition | Complete/Skipped | **4-tech-spec** — read `<KRATOS_ROOT>/pipeline/hephaestus-gate.md` and run the 3-phase gate (Metis scan → Hephaestus ANALYZE → user questions → Hephaestus WRITE_SPEC). Do NOT spawn Hephaestus directly. |
| 4-tech-spec | — | 5-spec-review-sa (apollo) |
| 5-spec-review-sa | Sound | 6-test-plan (artemis) |
| 5-spec-review-sa | Concerns/Unsound | 4-tech-spec (hephaestus) |
| 6-test-plan | — | Pre-implementation gate → 7-implementation (ares) |
| 7-implementation | Ares Mode | 8-prd-alignment (hera) |
| 7-implementation | User Mode | Wait — user completes tasks, then `/kratos:task-complete all` |
| 8-prd-alignment | Aligned | Spec archive offer (`<KRATOS_ROOT>/commands/spec-archive.md` § "Offer after implementation") → 9-review (hermes + cassandra parallel) |
| 8-prd-alignment | Gaps | 7-implementation (ares) — add missing test coverage AND/OR remove scope-creep code Hera flagged |
| 8-prd-alignment | Misaligned | Blocked — escalate to user, fundamental scope issue |
| 9-review | Approved + risk CLEAR/CAUTION | **Ship gate** — run `<kratos-bin> verify --final --feature FEATURE_NAME`. VICTORY **only** on exit 0; any non-zero output → BLOCKED with the listed failures. |
| 9-review | Approved + risk BLOCKED | Blocked — fix risks, re-run stage 9 |
| 9-review | Changes Required | 7-implementation (ares) |

---

## Recovery Procedures

### Re-running a single stage

```bash
# 1. Check current state
<kratos-bin> pipeline get --feature <name>

# 2. Read the required document to understand what failed
cat .claude/feature/<name>/<document>.md

# 3. Reset the stage
# Edit .claude/feature/<name>/status.json
# Change the target stage "status" to "pending"
# Clear "started" and "completed" timestamps

# 4. Say "continue" — Kratos re-spawns the stage agent
```

### Recovering from a corrupted status.json

If `status.json` is malformed or missing:

```bash
# Reconstruct manually — check which documents exist to infer completed stages
ls .claude/feature/<name>/
# prd.md exists → stage 1 complete
# prd-challenge.md exists → stage 2 complete
# tech-spec.md exists → stage 4 complete
# etc.
```

### Abandoning a feature

1. Move or delete `.claude/feature/<name>/`
2. The feature will no longer appear in `/kratos:status`
3. Arena shards with entries from this feature are not automatically cleaned — prune manually if desired
