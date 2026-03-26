# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build the Go binary (from go/ directory)
cd plugins/kratos/go && make build        # outputs to ../bin/kratos

# Run all Go tests
cd plugins/kratos/go && make test

# Run a single test file or package
cd plugins/kratos/go && go test ./internal/cli/ -run TestRecall -v

# Test with race detector
cd plugins/kratos/go && make test-race

# Lint (requires golangci-lint)
cd plugins/kratos/go && make lint

# Initialize DB + install hooks after build
./bin/kratos init && ./bin/kratos install
```

## Architecture Overview

Kratos is a **Claude Code plugin** (`.claude-plugin/plugin.json`) that orchestrates specialist subagents through an 11-stage feature pipeline. It has two runtime layers:

### 1. Markdown Layer (the plugin brain)
- **`agents/`** — Agent definitions (one `.md` per god-agent). Each file is a subagent prompt loaded by Claude Code's Agent tool via `subagent_type: "kratos:<name>"`.
- **`skills/`** — Skill definitions invoked via `/kratos:<command>`. The `auto/SKILL.md` is the main router that classifies user intent and dispatches to the correct command.
- **`commands/`** — Slash command implementations (`main.md`, `quick.md`, `review.md`, etc.). These are the entry points users invoke.
- **`pipeline/`** — Stage orchestration logic. `stages.md` has exact Agent tool invocations for each stage (0-11). `next.md` handles stage progression. `classify.md` routes requests to quick-path vs full pipeline.
- **`templates/`** — Document templates agents fill in (PRD, tech spec, test plan, code review, etc.).
- **`rules/`** — Code review standards (tiered). `default.md` is the baseline; language-specific files (e.g., `react.md`) auto-load based on file types.
- **`references/`** — Protocol docs agents read: `agent-protocol.md` (shared procedures), `arena-protocol.md` (knowledge base read/write rules), `status-json-schema.md`.
- **`modes/`** — Execution mode overrides (`eco-mode.md`, `power-mode.md`) that change which model each agent uses.
- **`hooks/`** — Claude Code hooks (`hooks.json` + JS/Go implementations). Key hooks: SubagentStart/Stop gates for Ares/Hephaestus/Hermes, PreToolUse for npm→project-PM rewriting.

### 2. Go Binary Layer (optional, enhances pipeline tracking)
- **`go/cmd/kratos/main.go`** — CLI entry point using Cobra.
- **`go/internal/cli/`** — Command implementations: `hook.go` (all hook subcommands), `pipeline.go` (stage updates), `session.go`/`recall.go` (session tracking), `todo.go`, `status.go`.
- **`go/internal/db/`** — SQLite layer with WAL mode. `schema.sql` is embedded. Session/step/feature CRUD. DB lives at `~/.kratos/memory.db`.
- **`go/internal/models/`** — Data models for sessions and steps.
- **`go/internal/formatter/`** — Output formatting for CLI display.
- Uses `modernc.org/sqlite` (pure Go, no CGO required).

### Key Data Flow
1. User invokes `/kratos:main "Build X"` → `skills/auto/SKILL.md` routes to `commands/main.md`
2. `commands/main.md` reads `pipeline/classify.md` to determine quick-path vs pipeline
3. For pipeline: reads `pipeline/stages.md` to spawn the correct agent at the current stage
4. Each agent reads its definition from `agents/<name>.md`, reads relevant `references/`, fills `templates/`
5. Agents write deliverables to `.claude/feature/<name>/` and update `status.json` via the Go binary (or direct file edit as fallback)
6. Hooks in `hooks.json` enforce quality gates (TODO-first for Ares, deliverable verification on stop)

### Arena (`.claude/.Arena/`)
Pull-model knowledge base in the target project. Agents read what they need; Metis bootstraps it. Sharded by domain (architecture, conventions, tech-stack, etc.). Each agent has specific read/write permissions defined in `references/arena-protocol.md`.

## Important Conventions

- All paths in agent prompts are relative to the **project root** (git repo root), not the plugin directory.
- `schema.sql` exists in both `go/internal/db/` and `memory/src/` — keep them in sync on schema changes.
- Hook commands use fallback chains: try `${CLAUDE_PLUGIN_ROOT}/bin/kratos` first, then `~/.kratos/bin/kratos`.
- The Go binary is optional — all agents gracefully fall back to direct file edits when it's unavailable.
- Version is tracked in `.claude-plugin/plugin.json` (currently v2.27.0).