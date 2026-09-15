# Kratos Hooks

Claude Code plugin hooks: quality gates for god-agents, session ledger recording, and the transcript memory sweep.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kratos Plugin                             │
├─────────────────────────────────────────────────────────────┤
│  hooks.json (uses ${CLAUDE_PLUGIN_ROOT})                    │
│       ↓                                                      │
│  Hook Scripts (.cjs files) — some call launch.cjs → kratos   │
│       ↓                                                      │
│  Go binary (kratos)                                          │
│       ↓                                                      │
│  SQLite (global: ~/.kratos/memory.db)                       │
└─────────────────────────────────────────────────────────────┘
```

## How It Works

The plugin registers hooks via `hooks.json`. Claude Code loads them automatically when the Kratos plugin is enabled. Go-implemented hooks run through `launch.cjs`, which resolves the binary and forwards the subcommand.

| Event | Matcher | Command | Action |
|-------|---------|---------|--------|
| `UserPromptSubmit` | all | `launch.cjs hook prompt-submit` | Detects Kratos god keywords (skill activation) and resume phrases (on-demand session-handoff injection, once per session — see below) |
| `SessionStart` | all | `session-start.cjs` | Registers the session in the ledger, injects the output constraint, `KRATOS_BIN`, stored user preferences, and one-line pointers (fresh handoff, pending spec deltas, draft plans, legacy-hook warning). Preserves the edit gate's ledger keys across compaction/resume |
| `SessionEnd` | all | `session-end.cjs` | Closes the session's ledger row with a one-line summary and removes its state file |
| `PermissionRequest` | `Read` | `permission-read.cjs` | Auto-allows Read requests scoped under `CLAUDE_PLUGIN_ROOT` or `~/.kratos/`; every other path falls through to the normal prompt |
| `PreToolUse` | `Write\|Edit\|MultiEdit\|NotebookEdit\|Bash\|PowerShell\|Agent\|Task` | `launch.cjs hook edit-gate` | Inline edit gate: denies source edits the inline god should dispatch — Odysseus to plans and spec deltas, Iris to two source files per turn. The gate only ever denies: a permitted call gets no decision and keeps Claude Code's normal permission prompt, so a classifier miss costs a prompt rather than an unattended `rm -rf` |
| `PostToolUse` | `Agent\|Task\|Write\|Edit\|MultiEdit` | `tool-use.cjs` (async) | Records agent spawns and project file changes (`.claude/feature/` and `.claude/.Arena/` count; `.claude/tmp/`, `.kratos/`, `.git/` and the scratchpad do not) |
| `PostToolUse` | `Write\|Edit` | `launch.cjs hook spec-delta-check` | Validates a just-written spec delta and blocks on a malformed one |
| `SubagentStart` | `kratos:.*` | `path-inject.cjs` | Injects the resolved `<KRATOS_ROOT>` / `<kratos-bin>`, the agent's composed protocol block, and its stored feedback lessons |
| `SubagentStart` | `kratos:ares`, `kratos:hephaestus`, `kratos:hermes` | `launch.cjs hook subagent-start` | Ares: markdown task-list gate; Hephaestus: TODO gate + Arena reminder; Hermes: creates `hermes-checklist.json` and injects tier instructions |
| `SubagentStart` | `kratos:athena`, `apollo`, `artemis`, `hera`, `cassandra`, `daedalus` | `launch.cjs check --init --stage <key>` | Announces the stage's expected deliverables |
| `SubagentStop` | `kratos:ares`, `hephaestus`, `hermes`, `nemesis`, `athena` | `launch.cjs hook subagent-stop` | Content gates: Ares task list/files/landed commit, Hephaestus spec sections + file on disk, Hermes 8-tier checklist, Nemesis `prd-challenge.md`, Athena spec-delta validation |
| `SubagentStop` | `kratos:athena`, `apollo`, `artemis`, `hera`, `cassandra`, `daedalus` | `launch.cjs check --verify --stage <key>` | Tier 1 deliverable check (file exists, verdict present) with a retry counter |
| `Stop` | all | `memory-sweep.cjs` | Periodic transcript memory sweep (see below) |

### SubagentStop response shape

Claude Code honors exactly one shape for a SubagentStop/Stop block:

```json
{"decision": "block", "reason": "Ares quality gate failed: ..."}
```

An allow is an empty object (`{}`) with exit 0. Every Go gate (`hook subagent-stop`, `check --verify`, `hook spec-delta-check`) emits this shape. The former `{"ok": true|false, "reason": ...}` output was ignored by the harness, which made every gate advisory.

## Files

| File | Purpose |
|------|---------|
| `hooks.json` | Hook registration (loaded by Claude Code) |
| `launch.cjs` | Shim that finds the kratos binary (plugin `bin/`, then `~/.kratos/bin/`) and forwards any subcommand — hooks call it for `hook prompt-submit` and `hook edit-gate` (the inline edit gate: reads `inline_god` from the session ledger and denies the edits that belong to a dispatched god, Go source `kratos-dev/go/internal/cli/hook_editgate.go`, replaced `plan-mode-guard.cjs` in v2.109), launchers for `agent load <god> --resolve --part body\|extras`. With no binary it serves `agent load` from `agents/<god>.md` on disk. A stale binary is handled twice: a pre-2.108 binary that rejects `--part` retries the body line without the flag, and one that answers an unknown subcommand with cobra's help text prints nothing instead of dumping that help into the hook's stdout |
| `kratos-bin.cjs` | Shared binary resolver (`resolveBinary`, `platformBinaryName`) used by every other script |
| `ensure-binary.cjs` | Downloads the platform binary from GitHub Release assets into `~/.kratos/bin/` when no plugin-local binary exists; spawned detached by `session-start.cjs` |
| `session-start.cjs` | SessionStart: ledger registration, output constraint, memories, pointers. Every spawn carries a timeout; the serial sum stays under ~4 s of the 5 s budget and the plugin-bin copy runs last. Writes `~/.kratos/sessions/<id>.json` while keeping keys other hooks store there |
| `session-end.cjs` | SessionEnd: closes the ledger row (two 600 ms calls; the SessionEnd budget stays 1.5 s because plugin hook timeouts don't raise it) |
| `tool-use.cjs` | PostToolUse: records agent spawns and project file changes |
| `permission-read.cjs` | PermissionRequest: scoped Read auto-allow via `hookSpecificOutput.decision.behavior` |
| `path-inject.cjs` | SubagentStart: `<KRATOS_ROOT>` / `<kratos-bin>` resolution, protocol block, feedback lessons |
| `memory-sweep.cjs` | Stop: periodic transcript sweep for durable user facts and per-agent lessons (see below) |

All scripts that call the binary use `spawnSync` with an argv array — hook payload text (descriptions, paths, summaries) never passes through a shell.

## On-Demand Session Handoff (`hook prompt-submit`)

`/kratos:wrap` writes `.claude/.Arena/handoff.md`. Rather than auto-injecting its content into every fresh session, the content is injected on demand by the Go `UserPromptSubmit` hook (`kratos-dev/go/internal/cli/hook.go`, `handoffInjectionContext`):

- **Trigger**: the prompt matches a resume phrase (`continue`, `resume`, `keep going`, `where were we`, `where did we stop`, `pick up` — word-boundary, case-insensitive, checked against the same sanitized text used for god-keyword matching).
- **Freshness**: same 7-day mtime gate as the `session-start.cjs` notice and `/kratos:wrap`.
- **Once per session**: a marker file at `~/.kratos/handoff-injections/<session_id>` suppresses repeat injections; markers older than 7 days are pruned on write (mirrors `memory-sweep.cjs`'s `pruneOldMarkers`). No `session_id` in the payload → the guard is skipped (always injects) rather than silently dropping the handoff.
- **Byte cap**: content is capped at 8KB without splitting a multi-byte UTF-8 rune (`capUTF8Bytes`).
- **Merged with keyword injection**: a god-keyword match and a resume-phrase match are independent — either, both, or neither may fire; the hook merges both contexts into one `additionalContext` and only passes the prompt through untouched when both are empty. (A bare "continue" with no god keyword still injects the handoff.)
- **Fails open** on every error — a missing/stale/unreadable handoff or unresolvable `cwd` degrades to "no injection." A marker I/O failure does *not* suppress this run's injection; it only means the once-per-session guard may not take effect next time. No error path ever blocks the prompt.

Keyword matching strips fenced and inline code, URLs, absolute and relative paths (`plugins/kratos`, `kratos-dev/go`) and hyphenated compounds (`kratos-dev`, `kratos-bin`) before looking for a god name, so file references never trigger the skill; "Kratos, build X" and "ask Athena to …" still do.

`/kratos:recall` is the explicit manual path to the same handoff file — it reads `handoff.md` directly (with or without the binary) and works regardless of resume-phrase detection. Note recall presents the file uncapped, whereas the on-demand hook caps injected content at 8KB (`capUTF8Bytes`), so the two paths can differ for an unusually large handoff.

## Global Storage

Memory is stored globally at `~/.kratos/`:

```
~/.kratos/
├── memory.db            # SQLite database
├── sessions/<id>.json   # One state file per Claude Code session (keyed by its session_id)
└── sweeps/<id>.json     # Memory-sweep cadence marker per session
```

This allows memory to persist across all projects. Session state is per Claude Code session
(`session_id` from the hook payload), so concurrent windows never share or end each other's
session — the previous single `active-session.json` did exactly that.

## No Manual Setup Required

Since the plugin uses `hooks.json` with `${CLAUDE_PLUGIN_ROOT}`:
- Hooks are registered automatically when plugin is enabled
- No need to edit `~/.claude/settings.json`
- Works in any directory

## Manual Commands

You can still use the CLI directly:

```bash
# Set database path (or it uses global default)
export KRATOS_MEMORY_DB=~/.kratos/memory.db

# Get summary
kratos status

# Recall recent sessions
kratos recall

# View active session
ls ~/.kratos/sessions/
```

## Transcript Memory Sweep (`memory-sweep.cjs`)

Registered on `Stop` (`session-end.cjs` moved to `SessionEnd`, which fires once when the session
actually ends). Where Iris's inline memory capture only catches facts flagged during an Iris
mission, this hook is a session-wide safety net that fires periodically: `Stop` runs after every
assistant turn, and the hook keeps a per-session marker (`~/.kratos/sweeps/<session_id>.json`)
with the transcript byte offset already scanned plus human/assistant message counters. Once at
least 10 new human messages AND 25 new assistant turns have accumulated since the previous sweep,
it emits a sweep, resets the counters, and re-arms — up to 8 sweeps per session. It quietly injects
a one-sentence instruction for Claude via `hookSpecificOutput.additionalContext` (no `decision`
field, so no Stop-hook-error styling — see below) pointing at `references/memory-sweep.md`, the
full two-target protocol: (1) review the whole conversation for durable user facts (preferences,
habits, weak spots, corrections, working style — never project/task facts, never secrets), dedupe
against `kratos memory list`, and save at most 3 via `kratos memory add`; (2) identify corrections
the user made to a specific god-agent's finished work and save at most 2 as per-agent lessons via
`kratos feedback add --agent <god>`. Lessons are re-injected at that agent's next spawn by
`path-inject.cjs` (≤5, current-project first via `feedback list --prefer-project`; fail-open —
any error just drops the lessons block).

This hook used to emit `{"decision":"block","reason":<instruction>}`. Every Stop-hook block —
regardless of wording — renders as a red "Stop hook error: <reason>" in the transcript, so the
sweep now uses the quiet-continuation channel instead. That makes it advisory: Claude is expected
to follow the injected instruction, but nothing forces another turn the way `block` did.

`additionalContext` still renders as a visible "Stop hook feedback" line — there is no fully
invisible Stop channel. That's why the injected instruction is a single sentence (the wall of
protocol text lives in `references/memory-sweep.md`) and why the protocol tells Claude to run
the sweep with zero user-visible output: no narration, no closing 📝 note.

**Guards** — the hook allows the stop silently (no output, no injection) whenever any of these trip:

| Guard | Behavior |
|-------|----------|
| `stop_hook_active === true` | Already re-invoked because of this hook — never re-emit on a hook-blocked continuation |
| `KRATOS_MEMORY_SWEEP=off` | Opt-out (see below) |
| Fewer than 10 new human messages or 25 new assistant turns since the last sweep | Not armed yet — the marker is updated and nothing is emitted |
| 8 sweeps already emitted this session | Per-session cap reached |
| New transcript tail contains `IRIS COMPLETE` or `KRATOS WRAP COMPLETE` | An inline sweep already ran (Iris mission, `/kratos:wrap`) — counters reset, no double-sweep |
| Transcript file missing or unreadable | Fail open — never block blind |
| `kratos` binary unresolvable | No CLI, no sweep |
| `references/memory-sweep.md` missing | Partial install — no protocol, no sweep |

On a qualifying turn the hook writes the marker first (so a hung or interrupted sweep never
causes a repeat emission), prunes markers older than 7 days, then emits
`{"hookSpecificOutput":{"hookEventName":"Stop","additionalContext":"<sweep instruction>"}}`.

**Opt-out**: set `KRATOS_MEMORY_SWEEP=off` in your environment to disable the sweep entirely.
`session-end.cjs` and the other hooks are unaffected.

## Troubleshooting

**Hooks not running?**
- Ensure Kratos plugin is enabled: `kratos@kratos` in settings.json
- Check `~/.kratos/` directory was created
- Restart Claude Code after enabling plugin

**No data recorded?**
- Run: `kratos init` (binary at `${CLAUDE_PLUGIN_ROOT}/bin/kratos` or `~/.kratos/bin/kratos`)
- Rebuild if missing: `cd kratos-dev/go && make build`

**A gate never blocks?**
- Check the hook's stdout: a block must be `{"decision":"block","reason":"..."}` at the top level; anything else is treated as allow

**View hook errors:**
- Check Claude Code logs for hook execution errors
