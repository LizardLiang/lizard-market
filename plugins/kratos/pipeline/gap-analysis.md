---
name: gap-analysis
description: Interactive gap analysis procedure for Kratos to run inline before spawning Athena for PRD creation
---

# Gap Analysis Protocol

You are **Kratos**. Run this procedure directly — do not delegate it. You own the interactive loop because `AskUserQuestion` is only available to the top-level orchestrator.

When the loop reaches WRITE_READY, spawn Athena with `PHASE: CREATE_PRD` and the full Q&A transcript. Athena writes the PRD; you ask the questions.

---

## Step 1: Parse the Requirement

Before scoring, read the request carefully:

- **Explicit**: What did the user literally state?
- **Implicit**: What assumptions would you need to make if Athena started writing right now?
- **Feature Type**: API, UI, Data, Auth, Integration, Mixed?
- **Ambiguity Level**: How many different valid interpretations exist?

---

## Step 2: Run the Gap Checklist

Read `<KRATOS_ROOT>/references/athena-gap-checklist.md` and work through it — including the **Behavioral Lifecycle** group, which forces per-verb coverage (grant/enforce/revoke/…) for stateful features. Each uncovered item is a gap.

**Seed a gap tree** (same discipline as Odysseus's facet enumeration): every checklist item the request does not cover becomes an `[open]` branch. An `[open]` branch is a promise you still owe an answer to — it can only close by becoming a `[leaf]` (answered by the user), `[assumed: X]` (documented assumption with risk-if-wrong), or `[out of scope]` (explicitly marked, one line of why). The clarity score below measures how well-specified the covered ground is; the gap tree is what stops you from scoring a tunnel-visioned slice at 0.05 while an entire lifecycle verb sits unasked.

---

## Step 2b: Run the Quadrant Sweep

The checklist only finds gaps you already know to look for. Read `<KRATOS_ROOT>/references/discovery-quadrants.md` and run the full sweep — evidence check on silently-resolved branches, assumption surfacing (yours / the user's / the repo's), and all six unknown-unknown techniques (premortem, inversion, boundary probe, actor sweep, analogous failures, checklist escape). Fold every discovery into the gap tree as a new `[open]` or `[assumed: X]` branch, and write the **Discovery Ledger** — Step 5 passes it to Athena, and WRITE_READY requires it.

---

## Step 3: Score Clarity

Use the checklist results + the original requirements to score across three weighted dimensions (0.0–1.0 each):

| Dimension | Weight | Evidence Source |
|-----------|--------|-----------------|
| **Goal Clarity** | 0.40 | Can you state what this feature does and why in one sentence without guessing? |
| **Constraint Clarity** | 0.30 | How many Restrictions & Constraints + Data & Integration items are covered? |
| **Success Criteria** | 0.30 | How many Use Cases & Users & Measurement items have concrete, testable answers? |

```
ambiguity = 1 - (goal_clarity × 0.40 + constraint_clarity × 0.30 + criteria_clarity × 0.30)
```

- **WRITE_READY: true** requires **all three**: (a) ambiguity ≤ 0.10, (b) zero `[open]` branches in the gap tree — every checklist gap is a `[leaf]`, `[assumed: X]`, or `[out of scope]` — **and** (c) the Quadrant Sweep was run and its Discovery Ledger is written, with each unknown-unknown technique showing intermediate output or an explicit "nothing surfaced". If all three hold, you can honestly say "Athena could write this PRD without guessing on any major decision or inventing a behavior nobody asked about."
- **WRITE_READY: false** if the score is too high, any branch is still `[open]`, **or** the sweep hasn't been run — keep asking; prefer an `[open]` branch over polishing an already-clear dimension

---

## Step 4: Ask or Proceed

**If WRITE_READY:** skip to Step 5.

**If not WRITE_READY:** ask exactly one question per turn. Pick the highest-priority unresolved gap in the weakest clarity dimension.

Questioning rules:
- **One question per turn** — never batch
- Prioritize: Security > Data integrity > Core functionality > Edge cases > Nice-to-haves
- Every question must include 2–5 concrete options and your recommended answer with brief reasoning

**Depth-first traversal** (critical — do not skip):
Follow one gap all the way to a leaf before moving to a different topic. A leaf is a decision with no further sub-questions given what you now know. For example: if you ask "which database?" and the user says "Postgres", the next question must be about a Postgres-specific concern (schema, connection pooling, migrations) — not a different top-level gap. Only switch topics once the current branch is fully resolved.

Call format:
```
AskUserQuestion(
  question: "[QUESTION]\n\nI'd recommend: [RECOMMENDATION] — [BRIEF_REASONING]. Do you agree?",
  header: "[SHORT_LABEL]",
  options: [
    { label: "[option]", description: "[description]" },
    ...
  ],
  multiSelect: false
)
```

If a gap remains genuinely unresolvable after the user says "TBD" or "doesn't matter", treat it as a documented assumption and move on. Athena will add it to the PRD appendix with a risk-if-wrong assessment.

---

## Step 4b: Loop — Re-score After Every Answer

After the user answers, **do not proceed to Step 5**. Instead:

1. Fold the answer into your understanding and update the gap tree — mark the branch `[leaf]`, or add sub-questions the answer revealed as new `[open]` branches
2. Re-run the ambiguity formula with the new information
3. If **WRITE_READY: false** (score too high **or** any `[open]` branch remains) → identify the next highest-priority gap and go back to Step 4
4. If **WRITE_READY: true** → proceed to Step 5

**You must keep asking until WRITE_READY is true.** Do not stop early because the user gave short answers or because you think you have "enough" — the bar is ambiguity ≤ 0.10 **and** zero `[open]` branches, not "probably fine". A documented assumption clears the gate; an unwritten gap does not.

---

## Step 5: Spawn Athena for PRD Creation

Once WRITE_READY, compile the full Q&A into `CLARIFIED_REQUIREMENTS` and spawn Athena:

```
Task(
  subagent_type: "kratos:athena",
  model: "opus",
  prompt: "MISSION: PRD Creation
PHASE: CREATE_PRD
FEATURE: [feature-name]
FOLDER: .claude/feature/[feature-name]/
ORIGINAL_USER_REQUEST: [verbatim first message — do not paraphrase]

CLARIFIED_REQUIREMENTS:
[Original request]: [verbatim]
[Gap analysis Q&A]:
  Q: [question asked] → A: [user's answer]
  Q: [question asked] → A: [user's answer]
  ...
[Documented assumptions]:
  - [assumption]: [risk-if-wrong]
[Discovery Ledger]:
  [the full four-quadrant ledger table from Step 2b — verbatim]
[Final clarity score]: [X]%

Read <KRATOS_ROOT>/agents/athena.md for the full instruction set before starting.

Create prd.md and decisions.md before completing. The CLARIFIED_REQUIREMENTS above contain the full gap analysis conversation — reconstruct the decision tree from it for the prd.md appendix. Kratos validates the deliverable after you finish.",
  description: "athena - PRD creation"
)
```
