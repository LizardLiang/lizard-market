# Kratos - The God of War

> *"I am what the gods have made me."* - Now, the gods serve **you**.

Kratos is the master orchestrator plugin that commands specialist **agents** to deliver features. Like the God Slayer himself, Kratos delegates to Athena, Hephaestus, Apollo, Artemis, Ares, and Hermes - each a true subagent spawned via the Task tool.

## Architecture

```
                         ⚔️ KRATOS ⚔️
                    Master Orchestrator
                    (Delegates via Task tool)
                             │
   ┌─────────────────────────┼─────────────────────────────────────────┐
   │                         │                                         │
   ▼                         ▼                                         │
┌─────────┐    ┌────────────┬───────┴───────┬────────────┬─────────────┐
│  METIS  │    ▼            ▼               ▼            ▼             ▼
│  (opus) │ ┌─────────┐ ┌───────────┐   ┌─────────┐ ┌───────────┐ ┌─────────┐
│Research │ │ ATHENA  │ │HEPHAESTUS │   │  APOLLO │ │  ARTEMIS  │ │  ARES   │
└────┬────┘ │  (opus) │ │   (opus)  │   │  (opus) │ │ (sonnet)  │ │(sonnet) │
     │      │   PM    │ │ Tech Spec │   │SA Review│ │    QA     │ │  Impl   │
     │      └─────────┘ └───────────┘   └─────────┘ └───────────┘ └─────────┘
     │           │            │               │            │             │
     ▼           │            │               │            │      ┌──────┴──────┐
┌─────────┐      │            │               │            │      │   HERMES    │
│ .Arena  │◄─────┴────────────┴───────────────┴────────────┴──────│   (opus)    │
│(shared) │      All gods can read Arena for context              │ Code Review │
└─────────┘                                                       └─────────────┘
                                     │
                            ┌────────┴────────┐
                            │ Delivered Value │
                            └─────────────────┘
```

## Agents (Subagents)

| Agent | File | Model | Domain |
|-------|------|-------|--------|
| **Metis** | `agents/metis.md` | opus | Project research, codebase analysis |
| **Athena** | `agents/athena.md` | opus | PRD creation, PM reviews |
| **Hephaestus** | `agents/hephaestus.md` | opus | Technical specifications |
| **Apollo** | `agents/apollo.md` | opus | Architecture review |
| **Artemis** | `agents/artemis.md` | sonnet | Test planning |
| **Ares** | `agents/ares.md` | sonnet | Implementation |
| **Hermes** | `agents/hermes.md` | opus | Code review |

## Commands

| Command | Purpose |
|---------|---------|
| `/kratos:main` | The main orchestrator - handles any request |
| `/kratos:start` | Begin a new feature journey |
| `/kratos:status` | View the battlefield - all features and their state |
| `/kratos:next` | Kratos decides and executes the next move |
| `/kratos:approve` | Grant blessing to proceed |
| `/kratos:gate-check` | Verify readiness before battle |

## Skills

| Skill | Purpose |
|-------|---------|
| `/kratos:auto` | Auto-determine and execute next action |

## The Pipeline

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           THE PATH OF DESTRUCTION                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  [0]         [1]         [2]           [3]          [4]           [5]      │
│ Research →  PRD    →  PRD Review  →  Tech Spec → PM Review  → SA Review    │
│   🔍         📋         🔍            📐          👁️           👁️         │
│  Metis     Athena     Athena      Hephaestus    Athena       Apollo        │
│  (opus)    (opus)     (opus)        (opus)      (opus)       (opus)        │
│ optional                                                                    │
│                              ↓                                              │
│                                                                             │
│          [6]              [7]              [8]                              │
│       Test Plan  →   Implementation  → Code Review   →    VICTORY          │
│          🧪               ⚒️               🔬              🏆              │
│        Artemis          Ares            Hermes                              │
│       (sonnet)        (sonnet)          (opus)                              │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## How It Works

1. **Kratos receives request** from user
2. **Kratos reads status.json** to understand current state
3. **Kratos spawns appropriate agent** via Task tool
4. **Agent executes mission** (creates document, updates status)
5. **Kratos reports results** and offers next action
6. **Repeat until VICTORY**

### Key Principle: Delegation

Kratos **NEVER** does the work himself. He is an orchestrator who:
- Understands what needs to be done
- Spawns the right agent via Task tool
- Reports results to the user

Each agent is spawned as a **true subagent** (subprocess) with:
- Its own context window
- Focused domain knowledge
- Specific tools for its mission

## The Arena

The **Arena** (`.claude/.Arena/`) is where Metis documents project knowledge. All gods can reference it for battlefield awareness.

```
.claude/.Arena/
├── project-overview.md      # High-level summary
├── tech-stack.md            # Languages, frameworks, dependencies
├── architecture.md          # System design, patterns
├── file-structure.md        # Directory organization
└── conventions.md           # Coding standards found
```

**Benefits:**
- **Battlefield awareness** - Kratos knows the terrain before battle
- **Better agent context** - All gods can reference Arena
- **Onboarding acceleration** - Quick project understanding
- **Reusable knowledge** - Arena persists across sessions

---

## Gates (Enforced by Kratos)

| Gate | Requirement | Unlocks |
|------|-------------|---------|
| **Gate 1** | PRD Review: ✅ Approved | Tech Spec |
| **Gate 2** | Tech Spec: ✅ Complete | Spec Reviews |
| **Gate 3** | PM + SA Reviews: Both ✅ | Test Plan |
| **Gate 4** | Test Plan: Created | Implementation |
| **Gate 5** | Code Review: ✅ Approved | VICTORY |

## Feature Folder Structure

```
.claude/feature/<feature-name>/
├── status.json              # Kratos's ledger - tracks everything
├── prd.md                   # Athena's creation
├── prd-review.md            # Athena's review
├── tech-spec.md             # Hephaestus's blueprint
├── spec-review-pm.md        # Athena's spec review
├── spec-review-sa.md        # Apollo's analysis
├── test-plan.md             # Artemis's battle plan
├── implementation-notes.md  # Ares's log
├── code-review.md           # Hermes's verdict
└── [source files]           # Implemented code
```

## Usage

### Start a New Feature
```
/kratos:main Build a user authentication feature

⚔️ KRATOS ⚔️

No active feature. Initializing...

Feature: user-authentication
Stage: 0 → 1 (PRD Creation)
Summoning: ATHENA (model: opus)

[Task tool spawns athena agent]
```

### Continue Through Pipeline
```
User: "continue"

⚔️ KRATOS ⚔️

Feature: user-authentication
Stage: 1 → 2 (PRD Review)
Summoning: ATHENA (model: opus)

[Task tool spawns athena agent for review]
```

### Check Status
```
/kratos:status

⚔️ KRATOS: BATTLEFIELD STATUS ⚔️

Feature: user-authentication
Progress: ████████░░░░░░░░ 50% (Stage 4/8)

Pipeline:
[1]✅ → [2]✅ → [3]✅ → [4]🔄 → [5]⏳ → [6]🔒 → [7]🔒 → [8]🔒

Current: PM Spec Review (in-progress)
Next: SA Spec Review (can run in parallel)
```

---

*"The cycle ends here. We must be better than this."* - Kratos guides your features to victory through his divine agents.
