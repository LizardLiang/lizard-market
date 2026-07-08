# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build the Go binary (dev tree lives at repo-root kratos-dev/, outside the shipped plugin)
cd kratos-dev/go && make build        # outputs to plugins/kratos/bin/kratos

# Run all Go tests
cd kratos-dev/go && make test

# Run a single test file or package
cd kratos-dev/go && go test ./internal/cli/ -run TestRecall -v

# Test with race detector
cd kratos-dev/go && make test-race

# Lint (requires golangci-lint)
cd kratos-dev/go && make lint

# Regenerate commands/<god>.md + SKILL.md god regions from agents/*.md frontmatter
cd kratos-dev/go && make gen

# Verify generated launchers/SKILL.md match agents/*.md (no write; CI + publish.sh gate)
cd kratos-dev/go && make gen-check

# Initialize DB + install hooks after build
./plugins/kratos/bin/kratos init && ./plugins/kratos/bin/kratos install

# Publish to the dedicated distribution repo (LizardLiang/kratos) after tagging
kratos-dev/publish.sh
```

## Architecture Overview

Kratos is a **Claude Code plugin** (`.claude-plugin/plugin.json`) that orchestrates specialist subagents through an 11-stage feature pipeline. It has two runtime layers:

### 1. Markdown Layer (the plugin brain)
- **`agents/`** — Agent definitions (one `.md` per god-agent). Each file is a subagent prompt loaded by Claude Code's Agent tool via `subagent_type: "kratos:<name>"`.
- **`skills/`** — Skill definitions invoked via `/kratos:<command>`. The `auto/SKILL.md` is the main router that classifies user intent and dispatches to the correct command.
- **`commands/`** — Slash command implementations (`main.md`, `quick.md`, `review.md`, etc. are hand-written). The 19 god launchers (`commands/<god>.md`, e.g. `ares.md`, `athena.md`) are **generated** from `agents/*.md` frontmatter by `kratos-dev/go/cmd/gencommands` — see `kratos-dev/codegen/README.md`. Never hand-edit a launcher carrying `generated: true`; run `make gen` instead.
- **`pipeline/`** — Stage orchestration logic. `stages.md` has exact Agent tool invocations for each stage (0-11). `next.md` handles stage progression. `classify.md` routes requests to quick-path vs full pipeline.
- **`templates/`** — Document templates agents fill in (PRD, tech spec, test plan, code review, etc.).
- **`rules/`** — Code review standards (tiered). `default.md` is the baseline; language-specific files (e.g., `react.md`) auto-load based on file types.
- **`references/`** — Protocol docs agents read: `agent-protocol.md` (shared procedures), `arena-protocol.md` (knowledge base read/write rules), `status-json-schema.md`.
- **`modes/`** — Execution mode reference (`modes.md`) with the eco/normal/power model matrix for every agent.
- **`hooks/`** — Claude Code hooks (`hooks.json` + JS/Go implementations). Key hooks: SubagentStart/Stop gates for Ares/Hephaestus/Hermes, PreToolUse for npm→project-PM rewriting.

### 2. Go Binary Layer (optional, enhances pipeline tracking)

Source lives at repo-root `kratos-dev/go/` — outside `plugins/kratos/` so plugin installs copy only runtime files. `make build` outputs committed binaries into `plugins/kratos/bin/`.

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

- Plugin-internal paths are written as `<KRATOS_ROOT>/...`. Orchestrators resolve the actual root (from `${CLAUDE_PLUGIN_ROOT}` or the skill base directory) and substitute it into agent prompts before spawning; `plugins/kratos/` from project root is the fallback for in-repo installs. See `references/agent-protocol.md`.
- `schema.sql` lives in `kratos-dev/go/internal/db/` (a legacy copy sits in `kratos-dev/memory/`).
- Dev-only assets (Go source, eval harness, legacy memory prototype, CI docs) live in repo-root `kratos-dev/`; GitHub workflows in repo-root `.github/workflows/kratos-*.yml`. Nothing outside runtime needs lives in `plugins/kratos/` — installs copy the whole plugin dir.
- **`LizardLiang/kratos` is the main publish repo** — a runtime-only mirror users install from (`claude plugin marketplace add LizardLiang/kratos`). After each release commit + tag here, run `kratos-dev/publish.sh` to force-push the plugin subtree (and tag) there. Development never happens in that repo.
- Hook commands use fallback chains: try `${CLAUDE_PLUGIN_ROOT}/bin/kratos` first, then `~/.kratos/bin/kratos`.
- The Go binary is optional — all agents gracefully fall back to direct file edits when it's unavailable.
- Version is tracked in `.claude-plugin/plugin.json`.