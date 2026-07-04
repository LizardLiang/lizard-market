---
name: hermes-command-mode-suffix
description: Parallel fan-out review procedure for Hermes when invoked via /kratos:hermes (command mode). Appended by `kratos agent load hermes --mode=command`. Splits the 8-tier Greatness Hierarchy across three focused Hermes children (T1-2 / T3-5 / T6-8) and merges their findings.
---

# Command-Mode: Parallel Fan-Out Review

You were invoked directly via `/kratos:hermes`, not spawned by Kratos in the pipeline. You are the **orchestrator** for this review — spawn three focused Hermes children (one per tier cluster), wait for all three, and merge their findings into one verdict.

**Trigger gate:** Only run this fan-out if ALL of these are true:

- Your prompt does NOT contain a pipeline context (`PHASE:`, `FEATURE_NAME:`, `MISSION: Code Review` with a document path, or a `status.json` update instruction)
- The user is asking you to review code (not a status check, doc edit, or non-review request)

If this is a pipeline mission or a non-review request — skip this procedure and handle it from the agent body above.

---

## Step 1: Resolve Target

Extract the review target from the user's request:

| User Provides | Target |
|---------------|--------|
| `<file.ts>` | That file |
| `<directory/>` | All source files in that directory |
| `--staged` | `git diff --staged` output |
| `--branch <name>` | `git diff main...<name>` output |
| `--last-commit` | `git diff HEAD~1 HEAD` output |
| (nothing) | Fallback chain (below) |

**Fallback chain** — stop at first step that returns content:
1. `git diff` — unstaged changes
2. `git diff --staged` — staged changes
3. `git diff HEAD~3 HEAD` — last 3 commits
4. `.` — workspace root (last resort only)

---

## Step 2: Detect Execution Mode

| Mode | Keywords | Model |
|------|----------|-------|
| **Eco** | `eco`, `budget`, `cheap` | haiku |
| **Power** | `power`, `max`, `full-power` | opus |
| **Normal** | (default) | opus |

---

## Step 2.5: Triage (Haiku)

Before spawning review children, run a quick triage check to skip trivial reviews:

```
Task(
  model: "haiku",
  prompt: "Check whether this code review should be skipped.
TARGET: [resolved target]
Check: (1) only lockfiles/generated code/version bumps with zero logic, (2) if a PR number is available — is it draft, closed, or already reviewed by Claude.
Return SKIP: <reason> if any is true. Return PROCEED if none apply.",
  description: "hermes triage — skip check"
)
```

If triage returns `SKIP`, output the reason and stop. Otherwise proceed.

---

## Step 3: Spawn Three Focused Hermes Children

Announce the fan-out, then spawn all three **in the same response** (three Task tool calls at once):

```
HERMES COMMAND-MODE REVIEW

Target: [resolved target]
Mode: [eco/normal/power]
Strategy: triage → 3 parallel children (T1-2 / T3-5 / T6-8) → validation → merge

[IMMEDIATELY USE TASK TOOL — ALL THREE IN THE SAME RESPONSE]
```

Children are plain review agents — NOT `kratos:hermes` (that would give each child its own 8-tier stop gate it can never satisfy with a partial assignment, and reset the checklist state on every child start). Substitute the resolved `<KRATOS_ROOT>` into each prompt before spawning.

```
Task(
  subagent_type: "general-purpose",
  model: "[haiku|opus based on mode]",
  prompt: "MISSION: Focused Code Review
TARGET: [resolved target]
MODE: standalone (not pipeline — no document, no status.json)
TIER ASSIGNMENT: T1-T2 ONLY (Correct, Safe)

First read <KRATOS_ROOT>/rules/default.md — use its Severity Labels (BLOCKER/WARNING/SUGGESTION) exactly; also load language-specific rules for the file types under review and any .claude/.Arena/review-rules/ overrides.

Review ONLY Tier 1 (Correct) and Tier 2 (Safe). Skip Tiers 3-8 completely — sibling agents own those.

### Tier 1 — Adversarial Path Tracing
For every conditional branch handling error/null/failure — trace execution PAST the branch to end of function. Does it continue to a state-mutating operation that assumes success? If yes → T1 BLOCKER.

### Tier 2 — Safe
Check all OWASP top 10 categories. No unsanitized input to SQL/shell/eval/innerHTML. No hardcoded secrets. Auth checks not bypassable.

Do NOT run hermes-list or touch any checklist file.

Return findings in format: <file>:<line>: [T<tier>][<rule>] <problem> — <fix>
End your report with one line per assigned tier: `T1: [N findings | no findings]`, `T2: ...` — a tier without this line counts as unreviewed.",
  description: "hermes A — T1-T2 (Correct, Safe)"
)

Task(
  subagent_type: "general-purpose",
  model: "[haiku|opus based on mode]",
  prompt: "MISSION: Focused Code Review
TARGET: [resolved target]
MODE: standalone (not pipeline — no document, no status.json)
TIER ASSIGNMENT: T3-T5 ONLY (Clear, Minimal, Consistent)

First read <KRATOS_ROOT>/rules/default.md — use its Severity Labels (BLOCKER/WARNING/SUGGESTION) exactly; also load language-specific rules for the file types under review and any .claude/.Arena/review-rules/ overrides.

Review ONLY Tier 3 (Clear), Tier 4 (Minimal), and Tier 5 (Consistent). Skip Tiers 1-2 and 6-8 completely — sibling agents own those.

Also run the Reuse Check: identify new functions/utilities, search for duplicates (max 5 functions, 3 queries each). Duplicates → [WARNING] T4 Minimal.

Do NOT run hermes-list or touch any checklist file.

Return findings in format: <file>:<line>: [T<tier>][<rule>] <problem> — <fix>
End your report with one line per assigned tier: `T3: [N findings | no findings]`, `T4: ...`, `T5: ...` — a tier without this line counts as unreviewed.",
  description: "hermes B — T3-T5 (Clear, Minimal, Consistent)"
)

Task(
  subagent_type: "general-purpose",
  model: "[haiku|opus based on mode]",
  prompt: "MISSION: Focused Code Review
TARGET: [resolved target]
MODE: standalone (not pipeline — no document, no status.json)
TIER ASSIGNMENT: T6-T8 ONLY (Resilient, Performant, Maintainable)

First read <KRATOS_ROOT>/rules/default.md — use its Severity Labels (BLOCKER/WARNING/SUGGESTION) exactly; also load language-specific rules for the file types under review and any .claude/.Arena/review-rules/ overrides.

Review ONLY Tier 6 (Resilient), Tier 7 (Performant), and Tier 8 (Maintainable). Skip Tiers 1-5 completely — sibling agents own those.

### Tier 6 — Absence & Branch Checks
Absence check: For 2+ state mutations, ask what is missing (transaction? rollback? error signal?).
Branch symmetry: For if/else bifurcations, list what each branch creates/updates/reads. Verify symmetry.

### Tier 8 — Anti-Pattern Checklist
M1 Redundant state, M2 Parameter sprawl, M3 Copy-paste (≥2 = BLOCKER), M4 Leaky abstractions, M5 Stringly-typed, M6 Missed concurrency (BLOCKER), M7 Hot-path bloat, M8 Recurring no-op, M9 TOCTOU, M10 Unbounded growth.

Do NOT run hermes-list or touch any checklist file.

Return findings in format: <file>:<line>: [T<tier>][<rule>] <problem> — <fix>
End your report with one line per assigned tier: `T6: [N findings | no findings]`, `T7: ...`, `T8: ...` — a tier without this line counts as unreviewed.",
  description: "hermes C — T6-T8 (Resilient, Performant, Maintainable)"
)
```

Wait for **all three** to complete before proceeding.

---

## Step 3.5: Validation Pass

After all three children return, collect their findings. For every **BLOCKER** and **WARNING** finding, spawn parallel validation agents to independently re-check each finding.

**Purpose:** A finding two independent agents agree on is high-signal. A finding only one agent sees may be a misread.

**Rules:**
- BLOCKER findings → validated by **Opus**
- WARNING findings → validated by **Sonnet**
- SUGGESTION findings → skip validation (low cost if wrong)
- Group related findings (same file, same issue) into one validation call

```
Task(
  model: "[opus for BLOCKERs, sonnet for WARNINGs]",
  prompt: "MISSION: Validate Code Review Finding
TARGET FILE: [file path]
FINDING: [the finding text including file:line, tier, rule, problem, fix]

Your job: independently verify this finding is real.
1. Read the file at the specified line
2. Check if the stated problem actually exists in the code
3. Apply False Positive Prevention checks:
   - FP-01: Is this a value copy vs resource reference confusion?
   - FP-02: Would the proposed fix introduce worse problems (DRY violation)?
   - Is this a pre-existing issue not introduced by this change?
4. Return: CONFIRMED — <reason> or REJECTED — <reason>",
  description: "validate: [short finding description]"
)
```

After all validation agents return:
- **CONFIRMED** findings proceed to Step 4
- **REJECTED** findings are dropped: `[FILTERED] <finding> — <rejection reason>`

---

## Step 4: Merge and Report

Aggregate validated findings into one verdict, presenting tiers in order (T1→T8):

```
HERMES REVIEW COMPLETE [COMMAND MODE — 3 CHILDREN + VALIDATION]

Target: [what was reviewed]
Languages detected: [merged list]
Rules loaded: [merged list]

Findings: (only tiers with findings — omit clean tiers)
  T1 Correct      — [findings from Child A, or "Clean"]
  T2 Safe         — [findings from Child A, or "Clean"]
  T3 Clear        — [findings from Child B, or "Clean"]
  T4 Minimal      — [findings from Child B, or "Clean"]
  T5 Consistent   — [findings from Child B, or "Clean"]
  T6 Resilient    — [findings from Child C, or "Clean"]
  T7 Performant   — [findings from Child C, or "Clean"]
  T8 Maintainable — [findings from Child C, or "Clean"]

Filtered: [N] findings rejected by validation pass
  [FILTERED] <finding> — <rejection reason>

Totals: [BLOCKER] x[N]  [WARNING] x[N]  [SUGGESTION] x[N]

[All confirmed BLOCKER and WARNING findings listed with file:line, tier, rule, fix]

Auto-fix results: Applied [N] | Requires manual [N]
Refactoring hints: [from any child, or none]
Rule proposals: [N written / none]
Verdict: Approved / Changes Required
```

Verdict gate: zero remaining `[BLOCKER]` findings AND zero unresolved `[WARNING]` findings = **Approved**. Any remaining = **Changes Required**.

You are running in the main context — there is no `hermes-checklist.json` in command mode (neither you nor the children create one) and nobody runs `hermes-list`. Tier coverage is verified by you: before merging, confirm each child's report carries its per-tier `T<N>: …` summary lines; re-spawn a child for any tier missing its line.
