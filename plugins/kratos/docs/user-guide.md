# Kratos User Guide

Reference for Kratos v2.110. For installation, see `INSTALL.md`. For the overview and pipeline walkthrough, see `README.md`.

---

## How Kratos Routes a Request

Kratos classifies every request before it spawns an agent.

```
"What does this project do?"           → INQUIRY → Metis
"Who wrote the auth module?"           → INQUIRY → Clio
"Best way to implement caching?"       → INQUIRY → Mimir
"Break down the auth system"           → DECOMPOSITION → Daedalus
"Add tests for UserService"            → SIMPLE → Artemis
"Build OAuth2 authentication"          → COMPLEX → Full pipeline
```

Commands skip classification and route directly. Use commands for day-to-day work. Use `/kratos:main` for substantial new features only.

---

## The Pipeline (Stages 0–9)

| Stage | Agent | Deliverable | Notes |
|-------|-------|-------------|-------|
| 0 | Metis | Arena shards | Optional pre-flight codebase scan |
| 1 | Athena | `prd.md`, `decisions.md` | Gap analysis, one question at a time, then the PRD |
| 2 | Nemesis | `prd-challenge.md` | Adversarial + user-advocate review; verdict approved / revisions / rejected |
| 3 | Daedalus | `decomposition.md` | Optional; phases and dependencies; targets local file, Notion or Linear |
| 3b | Themis | `context.md` | Optional; locks implementation decisions before the spec |
| 4 | Hephaestus | `tech-spec.md` | Approach selection runs inline; spec written by the subagent |
| 5 | Apollo | `spec-review-sa.md` | Verdict sound / concerns / unsound |
| 6 | Artemis | `test-plan.md` | Test cases mapped to requirements |
| 7 | Ares | `implementation-notes.md` or `tasks/` | Ares Mode (AI implements) or User Mode (task files) |
| 8 | Hera | `prd-alignment.md` | Verdict aligned / gaps / misaligned |
| 9 | Hermes + Cassandra | `code-review.md`, `risk-analysis.md` | Run in parallel; verdicts approved / changes-required and clear / caution / blocked |

Pipeline state lives in `.claude/feature/<name>/status.json`. With the binary installed, agents write it through `kratos pipeline update`. See `references/status-json-schema.md` for the fields.

### Stage 7 User Mode Commands

| Command | Description |
|---------|-------------|
| `/kratos:task-complete <id>` | Mark one task complete |
| `/kratos:task-complete 01 02` | Mark several tasks complete |
| `/kratos:task-complete all` | Mark all tasks complete; stage 7 completes and stage 8 becomes `ready` |

---

## The God Roster (19 agents)

| Agent | Domain | Entry points |
|-------|--------|--------------|
| **Athena** | PRD creation, requirements | Stage 1, `/kratos:athena` |
| **Nemesis** | Adversarial PRD review | Stage 2, `/kratos:nemesis` |
| **Daedalus** | Feature decomposition | Stage 3, `/kratos:decompose` |
| **Themis** | Decision lock (`context.md`) | Stage 3b, `/kratos:themis` |
| **Hephaestus** | Technical specification | Stage 4, `/kratos:hephaestus` |
| **Apollo** | Architecture review | Stage 5, `/kratos:apollo` |
| **Artemis** | Test planning | Stage 6, `/kratos:quick` (tests), `/kratos:artemis` |
| **Ares** | Implementation | Stage 7, `/kratos:quick` (fixes), `/kratos:ares` |
| **Hera** | PRD alignment verification | Stage 8, `/kratos:hera` |
| **Hermes** | Code review | Stage 9, `/kratos:review`, `/kratos:hermes` |
| **Cassandra** | Risk analysis | Stage 9, `/kratos:audit`, `/kratos:cassandra` |
| **Metis** | Project knowledge, Arena bootstrap | Stage 0, `/kratos:inquiry`, `/kratos:explain`, `/kratos:metis` |
| **Mimir** | External research | `/kratos:inquiry`, Athena delegation, `/kratos:mimir` |
| **Clio** | Git history | `/kratos:inquiry`, `/kratos:explain`, `/kratos:clio` |
| **Hades** | Debugging with proof | `/kratos:quick debug:`, `/kratos:hades` |
| **Odysseus** | Tactical plan mode | `/kratos:plan`, `/kratos:odysseus` |
| **Prometheus** | Strategic planning | `/kratos:strategy`, `/kratos:prometheus` |
| **Iris** | Daily briefing and secretary | `/kratos:iris` |
| **Ananke** | Personal todo list | `/kratos:ananke`, via Iris |

---

## Execution Modes

Prefix a request with a keyword to change the model routing. Normal is the default.

| Mode | Trigger keywords |
|------|------------------|
| **Eco** | `eco`, `ecomode`, `eco-mode`, `efficient`, `save-tokens`, `budget`, `cheap`, `low-cost` |
| **Power** | `power`, `powermode`, `power-mode`, `max`, `maximum`, `full-power`, `best quality`, `cost no concern` |

### Model by Mode (from `modes/modes.md`)

| Agent | Normal | Eco | Power |
|-------|--------|-----|-------|
| Metis | sonnet | haiku | opus |
| Athena | opus | sonnet | opus |
| Nemesis | opus | sonnet | opus |
| Daedalus | sonnet | haiku | opus |
| Hephaestus | opus | sonnet | opus |
| Apollo | opus | haiku | opus |
| Artemis | sonnet | haiku | opus |
| Ares | sonnet | haiku | opus |
| Hera | sonnet | haiku | opus |
| Cassandra | sonnet | haiku | opus |
| Hermes | opus | haiku | opus |
| Ananke | haiku | haiku | sonnet |
| Clio, Hades, Iris, Mimir, Odysseus, Themis | sonnet | haiku | opus |
| Prometheus | opus | sonnet | opus |

Quick-mode tasks (tests, fixes, refactor, review, research, docs, debug, plan) use sonnet in normal, haiku in eco, opus in power.

---

## Commands

| Command | Description |
|---------|-------------|
| `/kratos:main` | Full pipeline orchestration |
| `/kratos:quick` | Simple tasks; direct agent routing |
| `/kratos:review` | Code review with severity tiers and auto-fix |
| `/kratos:inquiry` | Questions; routes to Metis, Clio or Mimir |
| `/kratos:explain` | Explain a codebase or subsystem |
| `/kratos:audit` | Pre-ship risk audit (Cassandra) |
| `/kratos:plan` | Tactical implementation plan (Odysseus) |
| `/kratos:strategy` | Strategic build plan (Prometheus) |
| `/kratos:decompose` | Break a feature into phases (files, Notion, Linear) |
| `/kratos:iris` | Daily briefing and assistant |
| `/kratos:status` | Dashboard of all features |
| `/kratos:recall` | Resume the last session |
| `/kratos:wrap` | Write a session handoff and run the memory sweep |
| `/kratos:retro` | Fold a god's accumulated lessons into its definition |
| `/kratos:task-complete` | Mark User Mode tasks complete |
| `/kratos:spec-view` | View living specs and pending deltas |
| `/kratos:spec-archive` | Promote a feature's spec delta into its living spec |
| `/kratos:spec-backfill` | Generate living specs from shipped features |
| `/kratos:spec-export` | Export living specs to HTML or Markdown |
| `/kratos:greet` | Print a motivational message |
| `/kratos:<god>` | Run one god from the roster above inline in the main session |

### Pipeline-Internal Files (`pipeline/`, read by the orchestrator)

`classify.md` (quick path vs pipeline), `start.md`, `stages.md` (spawn instructions for stages 0–9), `next.md`, `gate-check.md`, `hephaestus-gate.md`, `pre-implementation.md`, `gap-analysis.md`, `recovery.md`, `memory.md`, `check-arena-staleness.md`, `integrate-arena-deltas.md`.

---

## Session Continuity

Memory lives in `~/.kratos/memory.db`; initialize it once with `~/.kratos/bin/kratos init`. `/kratos:recall` shows the last session (`--global` for all projects). `/kratos:wrap` writes a handoff before `/clear`. Natural language works too: "Where did we stop?".

---

## Arena Structure

The Arena (`.claude/.Arena/`) is the pull-model project knowledge base. Metis bootstraps it; agents read only the shards they need.

```
.claude/.Arena/
├── index.md              ← registry of all shards; always read first
├── glossary.md           ← domain terms
├── constraints.md        ← hard limits, compliance, security rules
├── debt.md               ← known issues, active workarounds
├── project/overview.md   ← project purpose, goals, users
├── architecture/         ← system-design.md, file-structure.md
├── tech-stack/           ← one shard per layer
├── conventions/          ← one shard per coding domain
├── features/             ← digests of completed features
├── insights/             ← Mimir's cached external research (TTL)
├── review-rules/         ← Hermes review standards and proposals
└── specs/                ← living, capability-organized specs
```

Every entry carries evidence: `[YYYY-MM-DD | agent | feature-name] content`. See `references/arena-protocol.md` for read and write permissions.
