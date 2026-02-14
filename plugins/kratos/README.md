# Kratos - The God of War (v2.0)

> *"I am what the gods have made me."* - Now, the gods serve **you**.

Kratos is the master orchestrator plugin that commands specialist **agents** to deliver features and wisdom. Version 2.0 transforms Kratos from a coding-only plugin into a **general-purpose project assistant** with persistent memory, external research capabilities, and git history expertise.

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

Then add the auto-activation block to your `CLAUDE.md` (see [INSTALL.md - Step 4](INSTALL.md#step-4-enable-auto-activation)).

---

## Architecture v2.0

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
                            ┌────────┴────────┐
                            │ Delivered Value │
                            └─────────────────┘
```

## The Pantheon (Agents)

| Agent | Domain | Specialty | Model (Normal) |
|-------|--------|-----------|----------------|
| **Metis** | Project Knowledge | Codebase analysis, Arena documentation | Sonnet |
| **Clio** | Git History | Blame, commit logs, contributor mapping | Sonnet |
| **Mimir** | External Research | Web, GitHub, Best practices, Documentation | Sonnet |
| **Athena** | Product Management | PRDs, PM reviews, requirements | Opus |
| **Daedalus** | Decomposition | Feature phases, dependencies, platform-native tasks | Sonnet |
| **Hephaestus**| Engineering | Technical specifications, blueprints | Opus |
| **Apollo** | Architecture | System design, SA reviews | Sonnet |
| **Artemis** | Quality Assurance | Test planning, test cases | Sonnet |
| **Ares** | Implementation | Code writing, bug fixes, refactoring | Sonnet |
| **Hermes** | Peer Review | Code review, quality audits | Opus |

---

## Commands

| Command | Purpose |
|---------|---------|
| `/kratos:main` | **Master Command** - Handles any request (auto-classifies) |
| `/kratos:inquiry`| **Knowledge Seek** - Routes questions to Metis, Clio, or Mimir |
| `/kratos:decompose`| **Decompose** - Break features into phases (files, Notion, Linear) |
| `/kratos:quick` | **Simple Tasks** - Direct routing for tests, fixes, refactors |
| `/kratos:recall` | **Session Resume** - Where did we stop? (uses persistent memory) |
| `/kratos:status` | **Battlefield View** - Status of all active features |
| `/kratos:start` | **New Feature** - Initialize a complex journey |
| `/kratos:next` | **Auto-Pilot** - Kratos decides and executes the next step |
| `/kratos:assign` | **Direct Command** - Manually assign a mission to a god |
| `/kratos:approve`| **Blessing** - Approve a document to proceed to next stage |

---

## New in v2.0

### 🧠 Persistent Memory
Kratos now remembers every battle. All sessions, agent spawns, decisions, and file changes are recorded in a SQLite database (`.claude/.kratos/memory.db`).
- Use `/kratos:recall` to see recent activity.
- Context is automatically injected into new sessions to maintain continuity.

### ⚡ Execution Modes
Tailor Kratos's power to your needs:
- **Eco Mode** (`eco:`): Uses Haiku/Sonnet for maximum token efficiency.
- **Normal Mode**: Balanced performance (Default).
- **Power Mode** (`power:`): Uses Opus for ALL agents for maximum quality.

### 🔍 Inquiry Mode
Ask anything about your project, history, or the world:
- *"Kratos, who wrote the auth module?"* (Routes to **Clio**)
- *"Kratos, how is the database organized?"* (Routes to **Metis**)
- *"Kratos, what are the best practices for rate limiting?"* (Routes to **Mimir**)

### 📚 Arena & Insights
- **The Arena** (`.claude/.Arena/`): Project-specific knowledge (Architecture, Tech Stack, etc.).
- **Insights** (`.claude/.Arena/insights/`): Cached external research from Mimir (API docs, best practices).

---

## The Pipeline (Complex Tasks)

For building new features, Kratos follows an 8-stage divine path:

```
[0] Research (Metis) → [1] PRD (Athena) → [2] PRD Review (Athena) →
[2.5] Decompose (Daedalus, optional) → [3] Tech Spec (Hephaestus) →
[4] PM Review (Athena) → [5] SA Review (Apollo) →
[6] Test Plan (Artemis) → [7] Implementation (Ares) → [8] Code Review (Hermes)
```

---

## Usage Examples

### Information Seeking
```bash
# Ask about history
/kratos:inquiry Who worked on the login page in the last month?

# Research best practices (Eco mode)
eco: what's the most efficient way to handle large file uploads in Node.js?

# Project exploration
/kratos:inquiry Explain the current architecture
```

### Task Execution
```bash
# Simple task
/kratos:quick Add unit tests for UserService.js

# Complex feature
/kratos:main Build a multi-tenant subscription system

# Resume work
/kratos:recall
/kratos:next
```

### Power Mode for Critical Work
```bash
power: review the payment processing logic for security vulnerabilities
```

---

*"The cycle ends here. We must be better than this."* - Kratos guides your project to victory through divine orchestration.

