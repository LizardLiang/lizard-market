# Kratos - The God of War (v2.24.0)

> *"I am what the gods have made me."* - Now, the gods serve **you**.

Kratos is the master orchestrator plugin that commands specialist **agents** to deliver features and wisdom. It handles everything from quick bug fixes to full 11-stage feature pipelines — with persistent memory, external research capabilities, and git history expertise.

## Installation

For the full step-by-step guide (building the binary, installing hooks, configuring auto-activation), see **[INSTALL.md](INSTALL.md)**.

### Quick Start

```bash
# 1. Build binary
cd plugins/kratos/go && go build -ldflags="-s -w" -o ../bin/kratos ./cmd/kratos && cd ..

# 2. Initialize database & install hooks
./bin/kratos init && ./bin/kratos install

# 3. Verify
./bin/kratos status
```

Then install the plugin and add the auto-activation block to your `CLAUDE.md` (see [INSTALL.md - Step 2](INSTALL.md#step-2-install-the-plugin-into-claude-code) and [Step 5](INSTALL.md#step-5-enable-auto-activation)).

> **Note**: The binary is optional. Kratos works without it — agents fall back to direct file edits. With the binary, `status.json` gets real timestamps and full pipeline history.

---

## Architecture

```
                         ⚔️ KRATOS ⚔️
                    Master Orchestrator
            (Memory Enabled • Pipeline Orchestration)
                             │
   ┌─────────────────────────┼─────────────────────────────────────────┐
   │                         │                                         │
   ▼                         ▼                                         ▼
┌─────────┐            ┌───────────┐                             ┌───────────┐
│  METIS  │            │   CLIO    │                             │   MIMIR   │
│ Research│            │ Git Hist  │                             │ Ext Res   │
└────┬────┘            └─────┬─────┘                             └─────┬─────┘
     │                       │                                         │
     └───────────────────────┼───────────────────┐                     │
                             │                   │                     │
                             ▼                   ▼                     ▼
┌─────────┐            ┌───────────┐       ┌───────────┐         ┌───────────┐
│ ATHENA  │            │HEPHAESTUS │       │  APOLLO   │         │  HERMES   │
│   PM    │            │ Tech Spec │       │ Architect │         │ Code Rev  │
└────┬────┘            └─────┬─────┘       └─────┬─────┘         └─────┬─────┘
     │                       │                   │                     │
     └───────────────────────┴─────────┬─────────┴─────────────────────┘
                                       │
                              ┌────────┴────────┐
                              │  ARES & ARTEMIS │
                              │  Impl & Quality │
                              └────────┬────────┘
                                       │
                            ┌──────────┴──────────┐
                            │        HADES        │
                            │  Debug (on-demand)  │
                            └─────────────────────┘
```

## The Pantheon (Agents)

| Agent | Domain | Specialty | Model (Normal) |
|-------|--------|-----------|----------------|
| **Metis** | Project Knowledge | Codebase analysis, Arena documentation | Sonnet |
| **Clio** | Git History | Blame, commit logs, contributor mapping | Sonnet |
| **Mimir** | External Research | Web, GitHub, best practices, documentation | Sonnet |
| **Athena** | Product Management | PRDs, PM reviews, requirements | Opus |
| **Daedalus** | Decomposition | Feature phases, dependencies, platform-native tasks | Sonnet |
| **Themis** | Discuss / Decision Lock | Debates implementation choices, writes context.md | Sonnet |
| **Hephaestus** | Engineering | Technical specifications, blueprints | Opus |
| **Apollo** | Architecture | System design, SA reviews | Opus |
| **Artemis** | Quality Assurance | Test planning, test cases | Sonnet |
| **Ares** | Implementation | Code writing, bug fixes, refactoring | Sonnet |
| **Hera** | PRD Alignment | Verifies implementation covers all acceptance criteria | Sonnet |
| **Hermes** | Peer Review | Code review, quality audits | Opus |
| **Cassandra** | Risk Analysis | Security, breaking changes, CVEs | Sonnet |
| **Hades** | Debugging | Error location, proof of failure, root cause | Sonnet |
| **Ananke** | Task Management | Personal todo list (binary + file fallback) | Sonnet |

---

## Hooks & Quality Gates

Kratos ships Claude Code hooks that enforce workflow discipline automatically — no configuration needed after `./bin/kratos install`.

### SubagentStart — TODO-First Gate

Fires before **Ares** and **Hephaestus** begin work. Injects a mandatory reminder that agents must write a numbered TODO list before making any tool calls.

### SubagentStop — Deliverable Verification

Fires when **Ares** or **Hephaestus** attempt to finish. Blocks completion and forces continuation if:

| Agent | Check |
|-------|-------|
| **Ares** | Must have written a TODO list, mentioned specific files modified, and declared completion |
| **Hephaestus** | Spec must cover at least 2 of: architecture, data model, API, implementation, schema, interface |

When `stop_hook_active` is true (hook-triggered re-run), the gate passes automatically to prevent infinite loops.

### PreToolUse — Package Manager Auto-Correction

Intercepts every `Bash` tool call containing `npm` and rewrites it to the project's actual package manager, detected from lockfiles in the project root:

| Lockfile | Detected PM |
|----------|-------------|
| `bun.lockb` | `bun` |
| `yarn.lock` | `yarn` |
| `pnpm-lock.yaml` | `pnpm` |

If no alternative lockfile is found, `npm` commands pass through unchanged.

---

## Commands

| Command | Purpose |
|---------|---------|
| `/kratos:main` | **Master Command** — Handles any request (auto-classifies) |
| `/kratos:quick` | **Simple Tasks** — Direct routing for tests, fixes, reviews, debug |
| `/kratos:review` | **Code Review** — Standards-enforced review with severity tiers and auto-fix |
| `/kratos:inquiry` | **Knowledge Seek** — Routes questions to Metis, Clio, or Mimir |
| `/kratos:decompose` | **Decompose** — Break features into phases (files, Notion, Linear) |
| `/kratos:recall` | **Session Resume** — Where did we stop? (uses persistent memory) |
| `/kratos:status` | **Battlefield View** — Status of all active features |

---

## Code Review Standards

Kratos ships with a tiered review standard that Hermes enforces on every review:

| Tier | Name | What it checks |
|------|------|---------------|
| 1 | **Correct** | Logic, edge cases, silent failures |
| 2 | **Safe** | Security, injection, secrets, auth |
| 3 | **Clear** | Readability, naming, complexity |
| 4 | **Minimal** | Dead code, over-engineering |
| 5 | **Consistent** | Project conventions |
| 6 | **Resilient** | Error handling, cleanup |
| 7 | **Performant** | N+1, blocking ops, waste |

Rules live in `rules/` (global baseline) and `.claude/.Arena/review-rules/` (project-specific, higher priority). Language-specific rules (React, TypeScript, Python, etc.) are loaded automatically based on detected file types.

```bash
/kratos:review src/auth.ts           # review a file
/kratos:review --staged              # review staged changes
/kratos:review --branch feat/login   # review a branch diff
/kratos:review src/components/ power # full directory, power mode
```

Hermes reports `[BLOCKER]`, `[WARNING]`, and `[SUGGESTION]` findings — BLOCKERs must be resolved before approval. Auto-fixable issues are proposed with diffs and applied with confirmation.

---

## Execution Modes

Tailor Kratos's power to your needs by prefixing any request:

| Mode | Trigger | Models Used |
|------|---------|-------------|
| **Eco** | `eco:`, `budget:`, `cheap:` | Haiku/Sonnet — token efficient |
| **Normal** | (default) | Balanced Opus/Sonnet mix |
| **Power** | `power:`, `max:`, `full-power:` | Opus for all agents |

---

## The Pipeline (Complex Features)

For building new features, Kratos follows an 11-stage divine path:

```
[0] Research (Metis, optional)
[1] PRD (Athena)
[2] PRD Review (Athena)
[3] Decompose (Daedalus, optional)
[4] Discuss (Themis, optional) ← locks decisions before Hephaestus specs
[5] Tech Spec (Hephaestus)
[6] PM Review (Athena) ─┐ parallel
[7] SA Review (Apollo)  ─┘
[8] Test Plan (Artemis)
[9] Implementation (Ares)
[10] PRD Alignment (Hera)
[11] Review (Hermes + Cassandra)
```

Pipeline state is tracked in `.claude/feature/<name>/status.json`. When the Kratos binary is installed, agents use `kratos pipeline update` to write real timestamps and maintain history. Without the binary, agents fall back to editing the file directly.

---

## Persistent Memory

All sessions, agent spawns, decisions, and file changes are recorded in a SQLite database. Use `/kratos:recall` to resume where you left off — context is automatically injected into new sessions.

### Arena — Shared Project Knowledge

The Arena (`.claude/.Arena/`) is Kratos's pull-model knowledge base. Agents read what they need from it, nothing is injected automatically. Metis bootstraps it on first run; all other agents append to it as part of their missions.

**Structure:**
```
.claude/.Arena/
├── index.md                  ← always read first — registry of all shards
├── glossary.md               ← domain terms (flat dated list)
├── constraints.md            ← hard limits with external origin
├── debt.md                   ← known issues, active workarounds
├── project/overview.md       ← project purpose, goals, users
├── architecture/             ← system design, component decisions
├── tech-stack/               ← one shard per stack layer
├── conventions/              ← one shard per coding domain
├── features/                 ← digest of past completed features
├── research/                 ← Mimir's cached external research (TTL)
└── review-rules/             ← Hermes review standards and proposals
```

Sharded files use an evidence format for every entry:
```
[YYYY-MM-DD | agent | feature-name] entry content
```

Each shard has a `## Permanent` section (written only by Metis at bootstrap, Athena, and Hephaestus) and an `## Entries` section (written by any authorized agent, subject to pruning). See `references/arena-protocol.md` for the full read/write protocol.

**Who reads what:**

| Agent | Reads | Writes |
|-------|-------|--------|
| Metis | — (bootstrapper) | All shards (initial) |
| Athena | project/, glossary.md, constraints.md | glossary.md, constraints.md |
| Hephaestus | architecture/, tech-stack/, conventions/, glossary.md | architecture/, tech-stack/, conventions/ (+ Permanent) |
| Apollo | architecture/, constraints.md, tech-stack/, conventions/ | — |
| Artemis | tech-stack/, conventions/ | — |
| Ares | conventions/, tech-stack/, debt.md | conventions/, tech-stack/, debt.md |
| Hermes | conventions/, constraints.md, review-rules/ | debt.md, conventions/, review-rules/ |
| Hades | architecture/, debt.md | debt.md |

---

## Usage Examples

```bash
# Ask about git history
/kratos:inquiry Who worked on the login page last month?

# Research best practices (Eco mode)
eco: what's the most efficient way to handle large file uploads in Node.js?

# Debug an error
/kratos:quick debug: TypeError: Cannot read properties of undefined

# Simple task
/kratos:quick Add unit tests for UserService.js

# Complex feature
/kratos:main Build a multi-tenant subscription system

# Power mode for critical review
power: review the payment processing logic for security vulnerabilities

# Resume previous work
/kratos:recall
```

---

*"The cycle ends here. We must be better than this."* — Kratos guides your project to victory through divine orchestration.
