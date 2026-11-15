# Agent Outcome-Quality Audit — 2026-07-04

Full audit of all 18 god-agent definitions + pipeline/hooks, focused on what lets
bad deliverables pass. Severity-ranked. Line refs verified at commit a60eea2 (v2.83.0).

> **Status update (same day, working tree):** P0 items 1-3 FIXED — hooks.json stage
> keys corrected + `TestHooksJSONStageKeysExist` guard; Hermes fan-out restructured
> (children are `general-purpose`, parent marks tiers, fail-open coaching removed,
> `<kratos-bin>`/absolute paths everywhere) in both agents/hermes.md and
> command-mode-suffix/hermes.md; Ares wave/clarification relay landed upstream in
> 8c36f7a. P0-4 path drift fixed upstream in 8c36f7a (Metis spawn, Prometheus reads).
> Live-eval discoveries also fixed: committed binaries had lost exec bits (100644 →
> "Permission denied" at hook time; restored + `chmod` guard in Makefile build-all),
> and `kratos pipeline update` errored on missing stage keys and misfiled verdicts
> (now upserts the stage and routes verdicts to stage-specific schema fields with
> validation — see pipeline_verdict_test.go). P1/P2 prompt-level items remain open.

---

## P0 — Broken enforcement (fix first; invalidates other quality assumptions)

### 1. Dead quality gates: hooks.json stage keys don't exist in check.go
`hooks/hooks.json:130,140,150,160,237,247,257,267` pass `4-spec-review-sa`,
`5-test-plan`, `7-prd-alignment`, `8-review`. `go/internal/cli/check.go:42-90`
(`stageChecks`) uses `5-spec-review-sa`, `6-test-plan`, `8-prd-alignment`, `9-review`.
Unknown stage → fail-open (check.go:155-159, 240-244). Net: deliverable/verdict gates
for **Apollo, Artemis, Hera, Cassandra are silent no-ops**. Only Athena (auto-detect)
and Daedalus gates fire.
**Fix:** correct 8 literals in hooks.json; make `--verify` with an explicit unknown
`--stage` fail closed; add a Go test asserting every `--stage` literal in hooks.json
exists in `stageChecks`.

### 2. Hermes fan-out defeats its own tier gate
- Children spawned as `subagent_type: "kratos:hermes"` (hermes.md:167,199,227) →
  each child reloads full hermes.md (recursion hazard) and each child's SubagentStart
  **resets hermes-checklist.json + block_count** (hook.go:370-395), clobbering marks.
- `block_count` is shared: 3 children push it to fail-open threshold for everyone
  (hook.go:884-887); prompts even teach "gate fails open after 3 attempts"
  (hermes.md:187,213,247).
- Bare `kratos hermes-list check` (hermes.md:146+, hook.go:402) — binary not on PATH
  → tiers never marked → fail-open path again.
**Fix:** children as plain Task with self-contained tier prompts; parent alone marks
tiers; key checklist/block_count by agent_id; `<kratos-bin>` everywhere; delete the
fail-open coaching.

### 3. Ares waits for a user that can't answer; its stop gate verifies prose
ares.md:178-183 "stop and ask the user" per wave — Ares is a Task subagent
(hermes.md:315 states subagents can't ask). Stop gate (hook.go:550-582) checks only
message text patterns — no disk check for implementation-notes.md, no test results.
**Fix:** waves → checkpoint notes in implementation-notes.md, orchestrator commits;
add disk check + require `Test Results: ... Failed: 0` line in gate.

### 4. Stage 0 spawn prompt orders legacy flat Arena files
`pipeline/stages.md:26` demands `project-overview.md, tech-stack.md, architecture.md…`
(flat); metis.md:23-33 produces sharded dirs (`project/overview.md`,
`architecture/system-design.md`). Spawn prompt wins → downstream readers
(apollo.md:42, hephaestus.md:227) miss the Arena. Same drift class: Prometheus reads
3 nonexistent flat paths (prometheus.md:38-42) and the wrong prior-plan path
(`plan.md` vs `plans/<slug>.md` written by strategy.md:75-89); Mimir caches to
`insights/` while arena-protocol.md:42 says `research/`.

---

## P1 — Highest-leverage quality improvements (cross-cutting)

1. **Requirement traceability chain (biggest realistic gain).** Nothing between PRD
   and Hera verifies coverage, and Hera's mechanism is fictional: PRD FR/AC ids →
   Artemis TC-ids → Ares tags tests with TC-ids (`// TC-012`) → Hera greps ids
   (stack-aware: `_test.go`, `test_*.py`), reuses ids verbatim (today she renumbers,
   hera.md:67), and re-runs mapped tests individually instead of one green-suite
   pass (hera.md:103-108 — the `npm test || yarn test || pytest` chain also cascades
   runners on legitimate test failure; replace with single detected command).
   Add `## Requirement Coverage` tables to tech-spec + decomposition templates.

2. **Verdicts must require proof of work.** Every reviewer can legally return an
   empty-findings pass: Nemesis (approved with no coverage evidence, nemesis.md:285-311),
   Apollo (may review from the stage-4 summary without reading tech-spec.md,
   apollo.md:66-70), Hera (green suite ⇒ aligned), Cassandra (no file:line/evidence
   requirement, cassandra.md:91-114). Require per-requirement/per-flow coverage
   tables; findings cite file:line + concrete failure path; "aligned/approved"
   invalid without a populated table.

3. **Give producers the reviewer's rubric.** Nemesis's flag taxonomy
   ([VAGUE_METRIC], [UNTESTABLE_AC]…) is the best quality spec in the plugin but
   only the reviewer sees it. Extract to `references/prd-quality-rubric.md`; Athena
   self-checks before completing (saves a full opus round-trip). Same for Themis:
   adopt Odysseus's zero-open-facets coverage gate (odysseus.md:101) — today Step-1
   gray areas can silently vanish from context.md.

4. **Ground every emitted path/command in the repo.** Daedalus invents target files
   and verify commands without one Glob (daedalus.md:44-138); Odysseus validation
   commands unverified (odysseus.md:168-170); Ares has no lint/build discovery
   (ares.md:393) and no pre-existing-failure baseline. Shared protocol rule: "every
   concrete file path Glob-verified or marked (new); every command traced to
   package.json/Makefile/CI config."

5. **Close the Athena↔Nemesis revision loop.** Verdict `revisions` → "Athena
   rewrites" (stages.md:74) but Athena has no REVISE_PRD mission, Nemesis no
   re-review protocol (prior-findings resolution check), no max-round escalation.
   Loops re-review from scratch → goalpost-moving.

6. **Anti-fabrication beyond timestamps.** Clio must report `Query runtime: [X]s`
   it never measured (clio.md:108); Metis must report "Coverage: N% of files
   examined" with no counting procedure (arena-templates.md:122); Mimir computes
   cache TTLs against a guessed "today"; Ananke writes `added:` dates from nowhere.
   One protocol rule: any date/duration/count must come from a command run this
   session, else omit.

7. **Model-tier mismatches (modes.md).** Eco puts haiku on: Apollo reviewing sonnet
   specs (author outranks gatekeeper), Cassandra security analysis, Artemis
   quick-mode test *writing*, Hermes parent orchestrating opus children. Floor
   reviewers/security at sonnet or add to the mandatory eco-warning list.

---

## P2 — Notable per-agent items (selected; severity high/med)

- **Athena**: never runs `kratos spec validate` on her own delta though a gate
  checks it (athena.md:166); no feasibility grounding against architecture; decisions.md
  reviewer list omits Nemesis (athena.md:105).
- **Nemesis**: reviews in a vacuum — no Arena constraints/specs read, no gap-analysis
  Q&A transcript passed (stages.md:52-67); told to "use context7" without the tool
  (nemesis.md:58 vs :4); severity freely self-assigned → verdict gameable;
  `prd-challenge.md` vs `prd-review.md` naming drift (stages.md:83, agent-protocol.md:130).
- **Hephaestus**: no PRD-coverage self-check; two diverging WRITE_SPEC sections
  (hephaestus.md:140-157 vs 176-216); template's Open Questions section invites the
  "no unresolved assumptions" violation; `CODEBASE_SCAN_RESULT` vs `CODEBASE_CONTEXT`
  variable drift; no strawman guard on the 2-3 approaches.
- **Apollo**: three incompatible severity taxonomies (apollo.md:77-79 vs 168-170 vs
  template); finding format copy-pasted from code review (file:line + rule tags for
  a markdown spec, apollo.md:153); no Stage-5 gate logic in stages.md for
  concerns/unsound; may relitigate user-locked gray-area decisions.
- **Artemis**: output pre-fills "Requirements covered: [X/Y] (100%)" (artemis.md:131)
  — prompts the claim; no negative/edge-case quota; undefined behavior on Apollo
  verdict `concerns`.
- **Hera**: never reads implementation-notes.md deviations table; `misaligned` can
  effectively never fire; coverage formula double-counts (hera.md:127).
- **Hades** (strongest file): quick.md:86 weakens mandatory instrumentation
  ("if inconclusive") vs hades.md:57 "always both"; gate proves location, not cause —
  require Expected/Actual pair; Confidence HIGH/MED/LOW has no rubric.
- **Clio**: plain `git blame` for authorship (formatter/rename noise) — need
  `-w -C -C` + `log -S`; 6-month default window corrupts absence claims ("never
  happened" on bounded search); no ANSWER-first output contract.
- **Mimir** (weakest on research quality): no source hierarchy/recency/version
  requirements; no ≥2-source cross-verification before caching (cached errors become
  "authoritative" via Arena); `gh api /advisories?severity=high` fetches global
  advisories, not the package's (mimir.md:85,131 — use `?affects=<pkg>`/OSV);
  WebSearch in tool list but no procedure uses it.
- **Ananke**: `<kratos-bin>` never injected inline (no SubagentStart on inline
  commands) → binary path dead in the primary invocation; two-path storage with no
  reconciliation → silent todo loss; positional fallback ids act on wrong items.
- **Themis**: contradictory single-invocation vs leftover WRITE_CONTEXT two-phase
  protocol (themis.md:146-150 vs 186-212); hand-edits status.json against CLI-first
  protocol (themis.md:273-277); empty bash block in the codebase scout step.
- **Cassandra**: severity hardcoded by category (Security=CRITICAL regardless of
  context) pre-breaks the verdict formula; standalone-mode trigger text includes
  "pipeline stage 9" → can legally skip the deliverable; no git-diff-based delta
  scouting.
- **Stale stage numbering everywhere**: hermes.md:24,85-88 ("stage 11", "Stage 8
  implementation"), ares.md:381 ("9-implementation"), agent-handoff-spec.md internal
  contradictions, agent-protocol.md:138-149 (Athena stages 2/6 that no longer exist),
  prd-template.md:143 ("Stage 10" Hera). Add canonical stage table to
  agent-protocol.md + a lint test over stage-name literals.

---

## Measuring improvements

`harness/` (this dir) grades deliverable quality per stage against a deterministic
fixture, including seeded-defect grading for Hera/Hermes/Cassandra (AC-3/AC-5
violations they must catch). Run before/after prompt changes; diff failure counts.
The repo-root `test-harness/` covers process compliance; together they cover both axes.
