# Kratos - The God of War (v2.3.0)

> *"I am what the gods have made me."* - Now, the gods serve **you**.

Kratos is the master orchestrator plugin that commands specialist **agents** to deliver features and wisdom. It handles everything from quick bug fixes to full 8-stage feature pipelines — with persistent memory, external research capabilities, and git history expertise.

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
| **Hephaestus** | Engineering | Technical specifications, blueprints | Opus |
| **Apollo** | Architecture | System design, SA reviews | Opus |
| **Artemis** | Quality Assurance | Test planning, test cases | Sonnet |
| **Ares** | Implementation | Code writing, bug fixes, refactoring | Sonnet |
| **Hermes** | Peer Review | Code review, quality audits | Opus |
| **Hades** | Debugging | Error location, proof of failure, root cause | Sonnet |

---

## Commands

| Command | Purpose |
|---------|---------|
| `/kratos:main` | **Master Command** — Handles any request (auto-classifies) |
| `/kratos:quick` | **Simple Tasks** — Direct routing for tests, fixes, reviews, debug |
| `/kratos:inquiry` | **Knowledge Seek** — Routes questions to Metis, Clio, or Mimir |
| `/kratos:decompose` | **Decompose** — Break features into phases (files, Notion, Linear) |
| `/kratos:recall` | **Session Resume** — Where did we stop? (uses persistent memory) |
| `/kratos:status` | **Battlefield View** — Status of all active features |

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

For building new features, Kratos follows an 8-stage divine path:

```
[0] Research (Metis, optional)
[1] PRD (Athena)
[2] PRD Review (Athena)
[2.5] Decompose (Daedalus, optional)
[3] Tech Spec (Hephaestus)
[4] PM Review (Athena) ─┐ parallel
[5] SA Review (Apollo)  ─┘
[6] Test Plan (Artemis)
[7] Implementation (Ares)
[8] Code Review (Hermes)
```

Pipeline state is tracked in `.claude/feature/<name>/status.json`. When the Kratos binary is installed, agents use `kratos pipeline update` to write real timestamps and maintain history. Without the binary, agents fall back to editing the file directly.

---

## Persistent Memory

All sessions, agent spawns, decisions, and file changes are recorded in a SQLite database. Use `/kratos:recall` to resume where you left off — context is automatically injected into new sessions.

### Arena & Insights

- **The Arena** (`.claude/.Arena/`): Project-specific knowledge — architecture, tech stack, conventions.
- **Insights** (`.claude/.Arena/insights/`): Cached external research from Mimir (TTL-based).

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
