---
name: hermes-command-mode-suffix
description: Standalone review procedure for Hermes when invoked via /kratos:hermes (command mode). Appended by `kratos agent load hermes --mode=command`. Resolves the target and mode, then runs the fan-out defined in the agent body (Step 3b children, Step 3.5 validation) without a checklist file.
---

# Command-Mode: Standalone Fan-Out Review

You were invoked directly via `/kratos:hermes`, not spawned by Kratos in the pipeline. You are the **orchestrator** for this review.

**Trigger gate:** run this procedure only if the prompt carries no pipeline context (`PHASE:`, `FEATURE_NAME:`, `MISSION: Code Review` with a document path, or a `status.json` instruction) AND the user is asking for a code review. Otherwise handle the request from the agent body above.

## Step 1: Resolve Target

| User Provides | Target |
|---------------|--------|
| `<file.ts>` | That file |
| `<directory/>` | All source files in that directory |
| `--staged` | `git diff --staged` |
| `--branch <name>` | `git diff main...<name>` |
| `--last-commit` | `git diff HEAD~1 HEAD` |
| (nothing) | Fallback chain: `git diff` → `git diff --staged` → `git diff HEAD~3 HEAD` → `.` (last resort) — stop at the first that returns content |

## Step 2: Detect Execution Mode

| Mode | Keywords | Child model |
|------|----------|-------------|
| **Eco** | `eco`, `budget`, `cheap` | haiku |
| **Power** | `power`, `max`, `full-power` | opus |
| **Normal** | (default) | opus for Child A, sonnet for B and C (as in the body) |

## Step 3: Review

Announce `HERMES COMMAND-MODE REVIEW — Target · Mode · Strategy: [triage →] 3 parallel children (T1-2 / T3-5 / T6-8) → validation → merge`, then run the body's procedure with these substitutions:

- **Step 2.5 Triage** — PR targets only, exactly as the body says.
- **Step 3b** — spawn the three children **in the same response** using the prompts defined in the body, with `MODE: standalone (not pipeline — no document, no status.json)`, no `PIPELINE CONTEXT` block, the READ-ONLY block included verbatim, `<KRATOS_ROOT>` substituted, and models from Step 2. Children are `general-purpose`, never `kratos:hermes`.
- **No checklist.** There is no `hermes-checklist.json` in command mode and nobody runs `hermes-list`. Tier coverage is verified by you: each child report must carry its per-tier `T<N>: …` lines; re-spawn a child for any tier missing its line.
- **Step 3.5 Validation**, **Step 4 Apply Fixes**, **Step 5 Refactoring Hint**, **Step 6 Rule Proposals** — as in the body.

## Step 4: Merge and Report

```
HERMES REVIEW COMPLETE [COMMAND MODE — 3 CHILDREN + VALIDATION]

Target: [what was reviewed]
Languages detected: [merged list]
Rules loaded: [merged list]

Findings: (only tiers with findings — omit clean tiers, T1→T8 order)
  T[N] [Name] — [findings]

Filtered: [N] findings rejected by validation pass
  [FILTERED] <finding> — <rejection reason>

Totals: [BLOCKER] x[N]  [WARNING] x[N]  [SUGGESTION] x[N]

[All confirmed BLOCKER and WARNING findings with file:line, tier, rule, fix]

Auto-fix results: Applied [N] | Requires manual [N]
Refactoring hints: [from any child, or none]
Rule proposals: [N written / none]
Verdict: Approved / Changes Required
```

Verdict gate: zero remaining `[BLOCKER]` AND zero unresolved `[WARNING]` = **Approved**; anything remaining = **Changes Required**.
