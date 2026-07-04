# Kratos Outcome-Quality Harness

Grades what god-agents **produce**, complementing `../../test-harness/` which grades
process compliance (timestamps, CLI usage, spawn patterns).

## Pieces

- `mk-fixture.sh <dest> <stage-key>` — deterministic fixture project (tiny Node token
  API + `rate-limit` feature). Seeds golden upstream docs so any stage runs in
  isolation without live prior stages. For stages 8/9 it seeds an implementation that
  deliberately violates **AC-3** (no Retry-After) and **AC-5** (quota consumed before
  auth) — reviewer agents are graded on catching them.
- `validate.sh <stage-key> <feature-dir> [project-root]` — deterministic checks:
  deliverable exists/non-trivial, no unresolved placeholders, required template
  sections, valid + non-fabricated status.json timestamps, verdict in allowed set,
  PRD AC-id coverage cross-refs (spec/test-plan), file:line evidence in reviews,
  rubber-stamp detection. `HARNESS_SEEDED_GAPS=1` enables seeded-defect grading.
  Exit code = failure count.
- `run-eval.mjs` — SDK runner: builds a fixture per stage, spawns ONLY the target
  agent via Task (so SubagentStart/Stop hooks fire), then validates.

## Usage

```bash
# deps (or reuse ../../test-harness/node_modules — the runner falls back to it)
npm i @anthropic-ai/claude-agent-sdk

node run-eval.mjs --stage 6-test-plan
node run-eval.mjs --stage 8-prd-alignment,9-review   # seeded-defect grading
node run-eval.mjs --stage all --model claude-sonnet-4-6
```

Results land in `results/<run-id>/` (`messages.jsonl` per stage + `report.json`).

Validated without LLM spend: mock rubber-stamp review → 5 failures; mock quality
review catching both seeded defects → 0 failures.

## Extending

- New seeded defects: edit the stage-8/9 branch of `mk-fixture.sh`, add matching
  grep checks in `validate.sh`.
- Regression baseline: commit a `report.json` and diff failure counts per stage
  across prompt changes to agents/*.md.
