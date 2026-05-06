# Kratos Roadmap

> Personal assistant feature ideas — to be discussed and prioritized.

---

## Proposed Agents

### Prometheus — Strategic Planner
- **Purpose**: When stuck on "what to build next". Interviews the user one question at a time, reads existing Kratos features (`.claude/feature/`) and Arena knowledge to inform recommendations, then produces a prioritized plan.
- **Output**: Chat response first for discussion. Once approved, saves to `.claude/.Arena/plan.md` so Metis can reference it in future sessions.
- **Interview style**: One question at a time until sufficient context is gathered.
- **Context-aware**: Reads existing in-flight features and Arena to avoid recommending things already in progress.
- **Handoff**: After user approves the plan, Prometheus suggests running `/kratos:main` on the top priority item — passing it to Athena to start the PRD. No direct handoff to Daedalus.
- **Note**: Already referenced in global CLAUDE.md as a subagent but not yet a proper Kratos agent.
- **Status**: `proposed`

### Heracles — Refactor Specialist
- **Purpose**: Executes large-scale refactoring — dead code removal, decoupling, pattern extraction, naming consistency, migration paths (e.g. class → hooks, REST → tRPC).
- **Distinction from Hermes**: Hermes and Cassandra *flag* refactoring opportunities in their reports. Heracles *acts* on them when invoked.
- **Workflow**:
  1. Analyse target scope
  2. Propose a refactoring plan (what, why, impact)
  3. Wait for user approval
  4. Create a git branch automatically before touching any files
  5. Execute
- **Trigger**: Standalone via `/kratos:refactor`. Hermes and Cassandra include a "refactoring recommended" section in their output when relevant, prompting the user to run it.
- **Status**: `proposed`

### Cassandra — Risk Analyst
- **Purpose**: Pre-ship risk analysis. Surfaces security holes (OWASP), breaking changes, edge cases, scalability cliffs (N+1, unbounded loops), dependency CVEs.
- **Pipeline position**: Stage 7 — spawned in parallel with Hermes by the parent orchestrator. Both results merged before returning to user.
- **Also available standalone**: `/kratos:audit` for on-demand deep scans outside the pipeline.
- **Scope**: changed files in pipeline mode; full codebase in audit mode.
- **Output**: severity-rated findings (Critical / High / Medium / Low)
- **Status**: `done`

### Hypnos — Daily Digest
- **Purpose**: Morning briefing agent. Orchestrates the full morning briefing — todos, Jira tasks, git history, in-flight features.
- **Status**: `proposed`

### Ananke — Task Manager
- **Purpose**: Manages the user's personal to-do list. Handles add/complete/remove/list operations. Understands natural language ("add a task to refactor auth", "what's on my list today").
- **Storage**: SQLite via `kratos` binary (`kratos todo` subcommand). Fallback: `.claude/.Arena/todos.md` for users without the binary.
- **Status**: `done`

---

## Proposed Commands

### `/kratos:morning`
- **Routes to**: Hypnos (orchestrates Ananke + Jira + Clio + status)
- **Output**: Daily briefing — todos, Jira tasks assigned to me, yesterday's commits, in-flight features, blockers
- **Runtime modes**:
  - **With `kratos` binary**: todos from SQLite, Jira from binary (`kratos jira list`)
  - **Without binary (fallback)**: todos from `.claude/.Arena/todos.md`, no Jira, git + status only
- **Status**: `proposed`

### `/kratos:audit`
- **Routes to**: Cassandra (standalone, full codebase scan)
- **Purpose**: On-demand deep risk review outside the pipeline
- **Status**: `done`

### `/kratos:refactor [path]`
- **Routes to**: Heracles
- **Purpose**: Kick off a structured refactoring session. Whole repo by default, optional path to target a subsystem.
- **Safety**: Heracles always creates a git branch before executing. Proposal shown before any files are touched.
- **Status**: `proposed`

### `/kratos:standup`
- **Routes to**: Clio + Ananke + status (all run in parallel)
- **Purpose**: Personal EOD reflection. Pulls a complete picture of what happened today.
- **Sources**:
  - Clio — commits made today
  - Ananke — todos completed today (SQLite or fallback file)
  - Kratos feature status — pipeline stages advanced
  - Jira (if `kratos` binary available) — tickets moved/updated
- **Output**: Chat only, ad-hoc. No storage.
- **Format**:
  ```
  ## What I did
  ## What's still in progress
  ## Blockers
  ```
- **Status**: `proposed`

### `/kratos:explain [path]`
- **Routes to**: Metis (deep codebase read) + Clio (git history for context and "why") — run in parallel
- **Purpose**: Personal context restore after time away from a project. Explains architecture, key entry points, data flows, conventions, and non-obvious decisions.
- **Audience**: The user themselves — informal tone, focus on "what do I need to know to work on this again"
- **Output**: Rendered in chat
- **Scope**: Whole repo by default. Optional path argument to target a subsystem (e.g. `/kratos:explain src/auth`)
- **Status**: `proposed`

---

## Proposed Infrastructure

### `kratos` Binary Extensions (Kratos.exe)

#### `kratos todo` subcommand
- `kratos todo add "<text>"` — add a task
- `kratos todo list` — list open tasks
- `kratos todo done <id>` — mark complete
- `kratos todo rm <id>` — remove task
- **Schema**: new `todos` table in existing SQLite DB (id, text, status, created_at, completed_at, source)

#### `kratos jira` subcommand
- `kratos jira config` — store Jira base URL + API token (encrypted or env-var)
- `kratos jira list` — fetch issues assigned to me, output structured text for Hypnos
- `kratos jira sync` — pull assigned issues into local todos table with `source=jira`
- **Auth**: Jira API token via env var `JIRA_API_TOKEN` + `JIRA_BASE_URL` + `JIRA_USER_EMAIL`

---

### CLI-Gated Agent Verification (`kratos check`)
- **Purpose**: Prevent agents from declaring "done" when work is actually incomplete. Agents interact with stage checklists exclusively through CLI — never reading or writing checklist files directly. Verification runs real checks (build, test, file existence), not just boolean flags.
- **Architecture**: Hooks are thin pipes; all logic lives in the binary.
  - `SubagentStart` hook → `kratos check --init --stage <stage>` (sets up checklist, injects CLI instructions into agent)
  - `SubagentStop` hook → `kratos check --verify --stage <stage>` (runs real verification, blocks if incomplete)
- **Verification tiers** (grow incrementally — don't fix what you haven't hit):
  - **Tier 1 — File existence**: Planning/review agents must produce their deliverables
    - Athena: `prd.md` + `decisions.md` exist and non-empty
    - Hephaestus: `tech-spec.md` exists and non-empty
    - Artemis: `test-plan.md` exists and non-empty
    - Cassandra: audit report written
    - Apollo: review file exists + verdict present
  - **Tier 2 — Build + test**: Implementation agents must ship working code
    - Ares: `make build` exits 0 + `make test` exits 0
    - Requires a `Makefile` with `build` and `test` targets
    - If no Makefile exists, `--init` instructs Ares to create one based on the project stack; user reviews it; subsequent features reuse it as-is
  - **Tier 3 — Review completeness**: Review agents must complete all review items
    - Hermes: all 8 tier booleans true (replaces current direct-file approach)
- **Why**: Agents lie. They say "tests pass" when cases are failing. They say "done" with half the deliverables missing. The CLI doesn't trust the agent's word — it runs its own checks. Exit codes don't lie.
- **Depends on**: `kratos` binary (new `check` subcommand)
- **Status**: `proposed`

---

### Decision Log (ADR-style)
- **Purpose**: Cross-cutting log of architectural and technical decisions with full context — answers "why did we choose X" months later.
- **Storage**: SQLite via `kratos` binary. Schema:
  ```sql
  decisions (id, title, context, decision, consequences, tags, feature, created_at)
  ```
- **Tags**: Agent-generated, minimum 3 per decision. Free-form — agent picks tags that best describe the decision (e.g. `["auth", "security", "jwt"]`). No predefined taxonomy.
- **Entry methods**:
  - Terminal: `kratos decision add` — manual entry via CLI
  - Chat: `/kratos:decide "..."` — natural language, agent structures and writes to SQLite via `kratos decision add`
  - Automatic: Hephaestus extracts decisions from specs silently, surfaces them to user ("I found 3 decisions, adding to log") before writing
- **Query via binary**:
  - `kratos decision list [--tag X] [--feature Y]` — filter by tag or feature
  - `kratos decision show <id>` — full detail
- **Metis integration**: Reads decision log only when explicitly asked (e.g. "why did we choose tRPC?")
- **Status**: `proposed`

---

## Proposed Research

### Oh-My-ClaudeCode CLI Architecture
- **Purpose**: Understand how Oh-My-ClaudeCode actually uses the CLI to control agents and commands. Reverse-engineer the dispatch mechanism, hook system, and how it wires up subagents vs skills vs commands at the CLI layer.
- **Goal**: Identify patterns Kratos should adopt, replace, or avoid.
- **Status**: `done` — See [research-omcc-cli-architecture.md](research-omcc-cli-architecture.md)

### Oh-My-ClaudeCode Plan Mode (Deep Questioning)
- **Purpose**: Study how Oh-My-ClaudeCode's plan mode works — specifically, it asks many clarifying questions to deeply understand requirements before proceeding, whereas Kratos (Athena/Themis) asks only a few. Understand the questioning strategy, how answers feed into the plan, and whether this leads to better PRDs/specs.
- **Goal**: Improve Kratos's requirement-gathering depth (Themis discuss stage and Athena PRD stage) by learning from this approach.
- **Status**: `done` — See [research-omcc-plan-mode.md](research-omcc-plan-mode.md)

### RTK Tool for Token Usage Reduction
- **Purpose**: Research the RTK (Response Token Kit / token reduction toolkit) approach to reduce token consumption across Kratos agents. Investigate techniques like response compression, context pruning, selective tool output, and smarter prompt engineering to minimize token waste without losing quality.
- **Goal**: Identify concrete strategies to reduce Kratos's per-feature token cost, especially for Opus-heavy stages.
- **Status**: `proposed`

---

## Priority (Suggested)

| # | Item | Value | Effort |
|---|------|-------|--------|
| 1 | ~~Oh-My-ClaudeCode CLI architecture study~~ | ~~Critical — understand competitor's agent/command dispatch~~ | ~~Low~~ | ✅ done |
| 2 | ~~Oh-My-ClaudeCode plan mode deep-questioning study~~ | ~~Critical — Kratos under-questions requirements vs OMCC~~ | ~~Low~~ | ✅ done |
| 3 | RTK tool for token usage reduction | Critical — reduce cost per feature run | Low |
| 4 | ~~`kratos todo` subcommand + Ananke agent~~ | ~~High — core daily use~~ | ~~Medium~~ | ✅ done |
| 5 | `kratos jira` subcommand | High — Jira users get assigned tasks pulled in | Medium |
| 6 | Hypnos + `/kratos:morning` (full version) | High — ties everything together | Low (after 4+5) |
| 7 | ~~Cassandra + `/kratos:audit`~~ | ~~High — catches pre-ship mistakes~~ | ~~Medium~~ | ✅ done |
| 8 | ~~`/kratos:explain`~~ | ~~High — context restore on old projects~~ | ~~Low~~ | ✅ done |
| 9 | Heracles + `/kratos:refactor` | Medium | Medium |
| 10 | ~~Prometheus (plugin version)~~ | ~~Medium~~ | ~~Low~~ | ✅ done |
| 11 | `/kratos:standup` | Medium | Low |
| 12 | Decision Log (ADR) | Medium | Low |
| 13 | CLI-gated checklist verification (`kratos check` + `AgentStop` hook) | High — enforces agent accountability | Medium |