---
name: victory
description: Ship-gate rules and the post-victory feature digest — read only when `pipeline next` reports `ship-gate`
---

# Victory — Ship Gate Rules and Feature Digest

Read this file when `<kratos-bin> pipeline next --json` reports `ship-gate`. The gate command and the VICTORY / BLOCKED formats are in `commands/main.md` § Victory.

## Never edit a reviewer's deliverable to satisfy the gate

The gate reads the structured verdict in status.json first (`code_review_verdict`, `risk_verdict`, `alignment_verdict`, …) and only then the file's Verdict section. If it still blocks, re-spawn that reviewer (Hermes / Cassandra / Hera) to restate its verdict, or report BLOCKED. Appending "APPROVED" to `code-review.md` yourself is a forgery, not a fix — it happened once, and the feature still never shipped.

## After the gate passes: record the feature digest

The per-feature `decisions.md` and `context.md` are stranded in the feature folder. Distill their essence into `.claude/.Arena/features/FEATURE_NAME.md` so the *reasoning* survives alongside the behavioral contract that `spec archive` already promotes. Create `.claude/.Arena/features/` if absent. Write a dated one-paragraph digest:

```markdown
# FEATURE_NAME — [date]

**What & why:** [1–2 sentences: what shipped and the core product decision behind it]
**Key decisions:** [2–4 bullets distilled from decisions.md — decision → rationale, including any rejected alternative that still matters]
**Implementation choices:** [1–2 bullets from context.md <decisions> that a future related feature should know]
**Sign-offs:** Apollo [verdict], Hera aligned, Hermes approved, Cassandra [clear/caution]
```

Keep it to a paragraph — this is a digest, not a copy. A future Themis/Prometheus run reads these to avoid re-deciding settled questions.
