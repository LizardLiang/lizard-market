# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build the Go binary (dev tree lives at repo-root kratos-dev/, outside the shipped plugin)
cd kratos-dev/go && make build        # outputs to plugins/kratos/bin/kratos

# Regenerate commands/<god>.md + SKILL.md god regions from agents/*.md frontmatter
cd kratos-dev/go && make gen

# Verify generated launchers/SKILL.md match agents/*.md (no write; make ci + publish.sh gate)
cd kratos-dev/go && make gen-check

# Full local CI gate (sync-assets, gen-check, go test -race, golangci-lint) —
# there is no GitHub Actions CI; this is the only gate. See kratos-dev/CI.md.
cd kratos-dev/go && make ci

# Initialize DB after build (hooks ship with the plugin via hooks/hooks.json — nothing to install;
# `kratos uninstall` removes legacy global hooks left by old installs)
./plugins/kratos/bin/kratos init

# Release: tag, then build/archive/checksum/changelog and create the GitHub
# release (local replacement for the removed release workflow)
kratos-dev/release.sh

# Publish to the dedicated distribution repo (LizardLiang/kratos) after release.sh
kratos-dev/publish.sh
```

## Architecture Notes

- **`commands/`** — `main.md`, `quick.md`, `review.md`, etc. are hand-written. The 19 god launchers (`commands/<god>.md`, e.g. `ares.md`, `athena.md`) are **generated** from `agents/*.md` frontmatter by `kratos-dev/go/cmd/gencommands` — see `kratos-dev/codegen/README.md`. Never hand-edit a launcher carrying `generated: true`; run `make gen` instead.
- **`references/`** — `agent-protocol.md` holds the sections injected into every god; nothing orchestrator-only lives there. Orchestrator-only rules go in `orchestrator-protocol.md` (read by Kratos only).
- **Go binary** — source lives at repo-root `kratos-dev/go/`, outside `plugins/kratos/`, so plugin installs copy only runtime files. `make build` outputs to `plugins/kratos/bin/`, which is **gitignored** (`plugins/kratos/.gitignore`) — binaries are never committed. `kratos-dev/release.sh` builds them per-platform off the tag and attaches them as release assets, so a local rebuild produces no git diff.

## Important Conventions

- Plugin-internal paths are written as `<KRATOS_ROOT>/...`. Resolution is deterministic, not LLM substitution: the SubagentStart hook (`hooks/path-inject.cjs`) injects the resolved root into every spawned subagent's context, and `kratos agent load <name> --resolve` (used by every generated launcher) substitutes the token before an inline command-mode god ever sees its own body. Orchestrators pass `<KRATOS_ROOT>` through spawn prompts unmodified; `plugins/kratos/` from project root is the last-resort fallback. See `references/agent-protocol.md`.
- Launchers load a god in two `!` lines, `agent load <god> --resolve --part body` and `--part extras` (lessons, protocol block, command-mode suffix), because Claude Code inlines at most 30,000 characters per `!` line and a whole god is 31–34 KB — one line arrives as a 2 KB `<persisted-output>` preview. `TestAgentLoadPartsFitInlineBudget` fails `make test` when any part would exceed 29,000 bytes; trim the agent file, never raise the budget.
- `schema.sql` lives in `kratos-dev/go/internal/db/` (a legacy copy sits in `kratos-dev/memory/`).
- Dev-only assets (Go source, eval harness, legacy memory prototype, CI docs) live in repo-root `kratos-dev/`. There is no CI service — `make ci` (in `kratos-dev/go/Makefile`) is the gate, run locally; see `kratos-dev/CI.md`. Nothing outside runtime needs lives in `plugins/kratos/` — installs copy the whole plugin dir.
- **`LizardLiang/kratos` is the main publish repo** — a runtime-only mirror users install from (`claude plugin marketplace add LizardLiang/kratos`). After each release commit + tag here, run `kratos-dev/publish.sh` to force-push the plugin subtree (and tag) there. Development never happens in that repo.
- Hook commands use fallback chains: try `${CLAUDE_PLUGIN_ROOT}/bin/kratos` first, then `~/.kratos/bin/kratos`.
- The Go binary is optional — all agents gracefully fall back to direct file edits when it's unavailable.
- Version is tracked in `.claude-plugin/plugin.json`.
