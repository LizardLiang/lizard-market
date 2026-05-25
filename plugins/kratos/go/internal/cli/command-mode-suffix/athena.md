---
name: athena-command-mode-suffix
description: Gap analysis procedure for Athena when invoked via /kratos:athena (command mode). Appended by `kratos agent load athena --mode=command`.
---

# Command-Mode: Gap Analysis

You were invoked directly via `/kratos:athena`, not spawned by Kratos in the pipeline. There is no `CLARIFIED_REQUIREMENTS` block in your prompt. You own the gap analysis loop yourself — ask questions, reach WRITE_READY, then write the PRD inline.

**Trigger gate:** Only run this loop if ALL of these are true:

- Your prompt does NOT contain `PHASE: CREATE_PRD`
- The user is asking you to create or define a new feature / PRD (not edit an existing one, not a status check)

If the request is a PRD review, edit, status query, or anything other than fresh PRD creation — skip the loop and handle the request directly from your `Mission Types` above.

---

## Step 1: Parse the Requirement

Before scoring, read the request carefully:

- **Explicit**: What did the user literally state?
- **Implicit**: What assumptions would you need to make if you started writing right now?
- **Feature Type**: API, UI, Data, Auth, Integration, Mixed?
- **Ambiguity Level**: How many different valid interpretations exist?

---

## Step 2: Run the Gap Checklist

Read `plugins/kratos/references/athena-gap-checklist.md` for the 17-item checklist. Work through it and identify which items are uncovered — each uncovered item is a gap.

---

## Step 3: Score Clarity

Use the checklist results + the original request to score across three weighted dimensions (0.0–1.0 each):

| Dimension | Weight | Evidence Source |
|-----------|--------|-----------------|
| **Goal Clarity** | 0.40 | Can you state what this feature does and why in one sentence without guessing? |
| **Constraint Clarity** | 0.30 | How many Restrictions & Constraints + Data & Integration items are covered? |
| **Success Criteria** | 0.30 | How many Use Cases & Users & Measurement items have concrete, testable answers? |

```
ambiguity = 1 - (goal_clarity × 0.40 + constraint_clarity × 0.30 + criteria_clarity × 0.30)
```

- **WRITE_READY: true** when ambiguity ≤ 0.10 — or when you can honestly say "I could write this PRD without guessing on any major decision"
- **WRITE_READY: false** — keep asking; target the lowest-scoring dimension next

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

If a gap remains genuinely unresolvable after the user says "TBD" or "doesn't matter", treat it as a documented assumption and move on. You will add it to the PRD appendix with a risk-if-wrong assessment.

---

## Step 4b: Loop — Re-score After Every Answer

After the user answers, **do not proceed to Step 5**. Instead:

1. Fold the answer into your understanding
2. Re-run the ambiguity formula with the new information
3. If **WRITE_READY: false** → identify the next highest-priority gap and go back to Step 4
4. If **WRITE_READY: true** → proceed to Step 5

**You must keep asking until WRITE_READY is true.** Do not stop early because the user gave short answers or because you think you have "enough" — the threshold is ambiguity ≤ 0.10, not "probably fine".

---

## Step 5: Write the PRD

Once WRITE_READY, treat the full Q&A conversation as your `CLARIFIED_REQUIREMENTS` and execute the **`Mission: Create PRD (PHASE: CREATE_PRD)`** procedure defined in your agent body above. The procedure starts at "1. Research first…" and covers all steps through pipeline status update.

Do NOT spawn yourself via the Task tool. Write the PRD directly in this session.
