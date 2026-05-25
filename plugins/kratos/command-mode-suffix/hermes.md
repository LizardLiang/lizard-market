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

## Step 3: Spawn Three Focused Hermes Children

Announce the fan-out, then spawn all three **in the same response** (three Task tool calls at once):

```
HERMES COMMAND-MODE REVIEW

Target: [resolved target]
Mode: [eco/normal/power]
Strategy: 3 focused review children (T1-2 / T3-5 / T6-8)

[IMMEDIATELY USE TASK TOOL — ALL THREE IN THE SAME RESPONSE]
```

```
Task(
  subagent_type: "kratos:hermes",
  model: "[haiku|opus based on mode]",
  prompt: "MISSION: Focused Code Review
TARGET: [resolved target]
MODE: standalone (not pipeline — no document, no status.json)
TIER ASSIGNMENT: T1-T2 ONLY (Correct, Safe)

Review ONLY Tier 1 (Correct) and Tier 2 (Safe). Skip Tiers 3-8 completely — sibling agents own those.

After completing each assigned tier, mark it via Bash:
  kratos hermes-list check T1
  kratos hermes-list check T2

If the SubagentStop gate blocks you citing tiers T3-T8, do NOT review or mark them — they belong to sibling agents. Simply attempt to stop again; the gate fails open after 3 attempts.

Follow Steps 1, 3b (T1-T2 only), 4 (auto-fix for your findings), 5, 6, and Reuse Check from your agent instructions. Use standalone output format — findings list, no document.",
  description: "hermes A — T1-T2 (Correct, Safe)"
)

Task(
  subagent_type: "kratos:hermes",
  model: "[haiku|opus based on mode]",
  prompt: "MISSION: Focused Code Review
TARGET: [resolved target]
MODE: standalone (not pipeline — no document, no status.json)
TIER ASSIGNMENT: T3-T5 ONLY (Clear, Minimal, Consistent)

Review ONLY Tier 3 (Clear), Tier 4 (Minimal), and Tier 5 (Consistent). Skip Tiers 1-2 and 6-8 completely — sibling agents own those.

After completing each assigned tier, mark it via Bash:
  kratos hermes-list check T3
  kratos hermes-list check T4
  kratos hermes-list check T5

If the SubagentStop gate blocks you citing tiers outside T3-T5, do NOT review or mark them — they belong to sibling agents. Simply attempt to stop again; the gate fails open after 3 attempts.

Follow Steps 1, 3b (T3-T5 only), 4 (auto-fix for your findings), 5, 6, and Reuse Check from your agent instructions. Use standalone output format — findings list, no document.",
  description: "hermes B — T3-T5 (Clear, Minimal, Consistent)"
)

Task(
  subagent_type: "kratos:hermes",
  model: "[haiku|opus based on mode]",
  prompt: "MISSION: Focused Code Review
TARGET: [resolved target]
MODE: standalone (not pipeline — no document, no status.json)
TIER ASSIGNMENT: T6-T8 ONLY (Resilient, Performant, Maintainable)

Review ONLY Tier 6 (Resilient), Tier 7 (Performant), and Tier 8 (Maintainable). Skip Tiers 1-5 completely — sibling agents own those.

After completing each assigned tier, mark it via Bash:
  kratos hermes-list check T6
  kratos hermes-list check T7
  kratos hermes-list check T8

If the SubagentStop gate blocks you citing tiers T1-T5, do NOT review or mark them — they belong to sibling agents. Simply attempt to stop again; the gate fails open after 3 attempts.

Follow Steps 1, 3b (T6-T8 only), 4 (auto-fix for your findings), 5, 6, and Reuse Check from your agent instructions. Use standalone output format — findings list, no document.",
  description: "hermes C — T6-T8 (Resilient, Performant, Maintainable)"
)
```

Wait for **all three** to complete before proceeding to Step 4.

---

## Step 4: Merge and Report

Aggregate the three children's findings into one verdict, presenting tiers in order (T1→T8):

```
HERMES REVIEW COMPLETE [COMMAND MODE — 3 CHILDREN]

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

Totals: [BLOCKER] x[N]  [WARNING] x[N]  [SUGGESTION] x[N]

[All BLOCKER and WARNING findings listed with file:line, tier, rule, fix]

Auto-fix results: Applied [N] | Requires manual [N]
Refactoring hints: [from any child, or none]
Rule proposals: [N written / none]
Verdict: Approved / Changes Required
```

Verdict gate (same as single-Hermes): zero remaining `[BLOCKER]` findings AND zero unresolved `[WARNING]` findings = **Approved**. Any remaining = **Changes Required**.

You are running in the main context — you do NOT have a `hermes-checklist.json` and must NOT run `kratos hermes-list`. Tier coverage is guaranteed by the three children collectively owning all 8 tiers.
