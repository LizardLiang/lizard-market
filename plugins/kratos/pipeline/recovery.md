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

**Metis sub-call was skipped or failed**
- Hephaestus must call Metis before writing the spec — this is non-negotiable
- If Hephaestus attempts to write without the scan: re-spawn with explicit instruction "You MUST spawn Metis (haiku) for codebase scan before writing tech-spec.md"

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

**Cassandra verdict is `critical`**
- VICTORY is blocked — do not proceed
- Present the critical findings (security, breaking changes) to user
- Ares addresses critical findings; then re-run Stage 9

**Hermes approved but Cassandra critical (or vice versa)**
- VICTORY requires BOTH: Hermes `approved` AND Cassandra `clear|caution`
- If only one passes: surface the remaining blocker, fix it, re-run only the failing review

---

## Recovery Procedures

### Re-running a single stage

```bash
# 1. Check current state
./bin/kratos pipeline get --feature <name>

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
