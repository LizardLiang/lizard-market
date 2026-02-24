# Kratos User Guide

Detailed reference for Kratos v2.0 features. For quick reference, see the plugin CLAUDE.md.

---

## Inquiry Mode

Kratos handles information-seeking requests through **Inquiry Mode**, routing to specialized knowledge agents.

### Auto-Classification

```
"What does this project do?"           → INQUIRY → Metis
"Who wrote the auth module?"           → INQUIRY → Clio
"Best way to implement caching?"       → INQUIRY → Mimir
"Break down the auth system"           → DECOMPOSITION → Daedalus
"Add tests for UserService"            → SIMPLE → Artemis
"Build OAuth2 authentication"          → COMPLEX → Full pipeline
```

### The Three Knowledge Agents

| Agent | Domain | Example Queries |
|-------|--------|----------------|
| **Metis** | Project knowledge, tech stack, code exploration | "What libraries are we using?", "Where are API endpoints?" |
| **Clio** | Git history, blame, contributors, timeline | "Who wrote this?", "Recent commits?", "What changed?" |
| **Mimir** | External research, GitHub, docs, best practices | "Best practice for X?", "Find Stripe docs", "Security advisories" |

### Insights Caching

Mimir caches research to `.claude/.Arena/insights/` with TTL:

| Research Type | TTL |
|---------------|-----|
| Best practices | 30 days |
| API documentation | 14 days |
| Security advisories | 7 days |

Cleanup: `/kratos:clean-insights`

---

## Execution Modes

| Mode | Trigger Keywords | Strategy |
|------|------------------|----------|
| **Normal** | (default) | 2 Opus / 5 Sonnet |
| **Eco** | `eco`, `budget`, `cheap`, `efficient` | 0 Opus / 2 Sonnet / 5 Haiku |
| **Power** | `power`, `max`, `full-power`, `don't care about cost` | 7 Opus |

### Model Routing by Mode

| Agent | Normal | Eco | Power |
|-------|--------|-----|-------|
| Mimir | sonnet | haiku | opus |
| Clio | sonnet | haiku | opus |
| Metis | sonnet | haiku | opus |
| Daedalus | sonnet | haiku | opus |
| Athena | opus | sonnet | opus |
| Hephaestus | opus | sonnet | opus |
| Apollo | sonnet | haiku | opus |
| Artemis | sonnet | haiku | opus |
| Ares | sonnet | haiku | opus |
| Hermes | sonnet | haiku | opus |

---

## Session Continuity

Kratos remembers where you left off across sessions.

| Command | Description |
|---------|-------------|
| `/kratos:recall` | See what you were working on last time |
| `/kratos:recall --global` | See recent sessions across all projects |
| `/kratos:install-hooks` | Install memory hooks globally |

Memory is stored in `~/.kratos/memory.db`. Natural language recall works too:
- "Where did we stop?"
- "What were we working on?"

### Installation

```bash
node plugins/kratos/hooks/install.cjs
```

---

## Implementation Modes

At Stage 7, Kratos asks how you want implementation handled:

| Mode | Description | Output |
|------|-------------|--------|
| **Ares Mode** | AI implements the code directly | `implementation-notes.md` + code |
| **User Mode** | Detailed task files for manual implementation | `tasks/` folder with guides |

### User Mode Commands

| Command | Description |
|---------|-------------|
| `/kratos:task-complete <id>` | Mark a task as complete |
| `/kratos:task-complete 01 02` | Mark multiple tasks complete |
| `/kratos:task-complete all` | Mark all tasks complete, triggers code review |

---

## Complete Agent Roster

| Agent | Domain | Entry Points |
|-------|--------|--------------|
| **Mimir** | External research, GitHub, docs, best practices | Inquiry mode, Athena delegation |
| **Clio** | Git blame, commits, contributors, timeline | Inquiry mode |
| **Daedalus** | Feature decomposition, platform-native tasks | Pipeline Stage 2.5, `/kratos:decompose` |
| **Metis** | Project knowledge, Arena documentation | Inquiry mode, research |
| **Athena** | PRD creation, requirements, PM reviews | Pipeline Stage 1, 2, 4 |
| **Hephaestus** | Technical specifications, architecture | Pipeline Stage 3 |
| **Apollo** | Architecture review | Pipeline Stage 5 |
| **Artemis** | Test planning, QA strategy | Pipeline Stage 6 |
| **Ares** | Implementation, coding | Pipeline Stage 7 |
| **Hermes** | Code review, quality gate | Pipeline Stage 8 |

---

## Arena Structure

```
.claude/
├── .Arena/
│   ├── project-overview.md
│   ├── tech-stack.md
│   ├── architecture.md
│   ├── file-structure.md
│   ├── conventions.md
│   └── insights/          # Cached research (Mimir, TTL-based)
└── feature/<name>/
    ├── status.json
    ├── prd.md
    ├── decomposition.md   # Optional (Daedalus)
    ├── tech-spec.md
    └── ...
```

---

## All Commands

| Command | Description |
|---------|-------------|
| `/kratos:main` | Full pipeline orchestration |
| `/kratos:quick` | Simple tasks, direct agent routing |
| `/kratos:inquiry` | Information requests |
| `/kratos:decompose` | Feature decomposition |
| `/kratos:start` | Initialize new feature |
| `/kratos:next` | Auto-advance pipeline |
| `/kratos:status` | Dashboard of all features |
| `/kratos:recall` | Session continuity |
| `/kratos:approve` | Approve current stage |
| `/kratos:assign` | Delegate to specific agent |
| `/kratos:gate-check` | Verify prerequisites |
| `/kratos:task-complete` | Mark tasks complete (User Mode) |
| `/kratos:clean-insights` | Clean stale research cache |
| `/kratos:install-hooks` | Install memory hooks |
| `/kratos:check-arena-staleness` | Check Arena freshness |
| `/kratos:integrate-arena-deltas` | Sync Arena after merge |
| `/kratos:main-with-memory` | Pipeline with session recording |

---

## Decomposition (Daedalus)

Output targets:
- **Local files** — `decomposition.md` in the feature folder
- **Notion** — Native page with inline database and task rows
- **Linear** — Project with phase issues, sub-issues, and Fibonacci estimates
- **Multiple** — Local + Notion/Linear

Pipeline integration: Auto-offered after PRD review for complex features.