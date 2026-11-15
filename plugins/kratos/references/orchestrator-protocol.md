# Orchestrator Protocol — Kratos-only Procedures

Read by Kratos (the orchestrator in `commands/main.md`, `commands/quick.md`, `pipeline/*.md`). Never injected into gods — the agent-facing shared rules are in `references/agent-protocol.md`.

---

## Path Resolution

Plugin-internal paths are written as `<KRATOS_ROOT>/...`. Resolution is deterministic, not LLM text-substitution:

- **Spawned subagents**: the SubagentStart hook (`hooks/path-inject.cjs`) injects the resolved absolute plugin root and the `<kratos-bin>` path into the god's context. Leave `<KRATOS_ROOT>` and `<kratos-bin>` verbatim inside spawn prompts.
- **Inline command-mode gods** (e.g. `/kratos:ares`): the generated launcher loads the body via `kratos agent load <name> --resolve`, which substitutes both tokens before the god sees the text.
- **Kratos itself**: when you read a plugin file with the Read tool, substitute the root printed by the launcher's `KRATOS_ROOT=` line. If nothing resolved it (binary unavailable and the JS fallback in `launch.cjs` failed), fall back to `plugins/kratos/` relative to the project root (in-repo installs).

Project-artifact paths (`.claude/feature/...`, `.claude/.Arena/...`) stay relative to the project root (git repository root).

Templates come from the CLI: `<kratos-bin> template get <template-name>` (omit `.md`).

---

## Spawn Prompt Fields

Alongside `MISSION:` / `FEATURE:` / `FOLDER:`, include two scope-control fields when spawning agents that write files:

- `NON-GOALS:` — what this spawn must NOT touch, lifted from the PRD's Non-Goals or the tech-spec's scope section. Agents treat this as a scope fence: work that would cross it gets logged (e.g. as debt in implementation-notes.md), never done "while you're here".
- `STOP-CONDITIONS:` — the named early-return signals for this agent (missing prerequisite → report the owning upstream agent; genuine ambiguity → `<AGENT> NEEDS CLARIFICATION`; wave boundary → checkpoint). Naming them makes stopping the expected move, not a failure.

Both are advisory for read-only spawns (Explore-style searches) and mandatory for implementation spawns.

Do not add "Read `<KRATOS_ROOT>/agents/<x>.md` before starting" to spawn prompts — `subagent_type: "kratos:<x>"` already loads the definition, and the SubagentStart hook injects the protocol block.

---

## Spawning Athena

Athena runs at Stage 1 only (`prd.md`); a Nemesis verdict of `revisions` re-spawns her at Stage 1. The `check --init --stage 1-prd` hook row injects her deliverable expectations, so a normal spawn needs no extra step. `pending_stage` is an optional override read by `check --init` when no `--stage` is given:

```bash
# Optional: pin --init to stage 1 on a re-spawn after other stages have run
<kratos-bin> pipeline set-pending --feature FEATURE_NAME --stage 1
# After Athena completes (clears the field)
<kratos-bin> pipeline set-pending --feature FEATURE_NAME --stage ""
```
