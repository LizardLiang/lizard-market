# Kratos Memory Hooks

Automatic journey recording via Claude Code plugin hooks.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kratos Plugin                             │
├─────────────────────────────────────────────────────────────┤
│  hooks.json (uses ${CLAUDE_PLUGIN_ROOT})                    │
│       ↓                                                      │
│  Hook Scripts (.cjs files)                                  │
│       ↓                                                      │
│  Go binary (kratos)                                          │
│       ↓                                                      │
│  SQLite (global: ~/.kratos/memory.db)                       │
└─────────────────────────────────────────────────────────────┘
```

## How It Works

The plugin registers hooks via `hooks.json`. Claude Code automatically loads these when the Kratos plugin is enabled.

| Hook | Trigger | Action |
|------|---------|--------|
| `UserPromptSubmit` | Every prompt | Detects Kratos god keywords (skill activation) and resume phrases (on-demand session-handoff injection, once per session — see below) |
| `SessionStart` | Claude Code starts | Creates memory session; prints a one-line notice if a fresh handoff exists (content stays on-demand, not injected here) |
| `PostToolUse` | Task/Write/Edit tools | Records agent spawns & file changes |
| `Stop` | Claude Code exits | Ends session with summary, then runs the transcript memory sweep |

## Files

| File | Purpose |
|------|---------|
| `hooks.json` | Hook registration (loaded by Claude Code) |
| `launch.cjs` | Dispatches `UserPromptSubmit`/`PostToolUse`/etc. to the Go binary's `hook` subcommands (e.g. `hook prompt-submit`) |
| `session-start.cjs` | Starts memory session; prints a one-line handoff notice (no content) |
| `tool-use.cjs` | Records tool usage |
| `session-end.cjs` | Ends session with summary |
| `memory-sweep.cjs` | Once-per-session transcript sweep for durable user facts (see below) |

## On-Demand Session Handoff (`hook prompt-submit`)

`/kratos:wrap` writes `.claude/.Arena/handoff.md`. Rather than auto-injecting its content into every fresh session, the content is injected on demand by the Go `UserPromptSubmit` hook (`kratos-dev/go/internal/cli/hook.go`, `handoffInjectionContext`):

- **Trigger**: the prompt matches a resume phrase (`continue`, `resume`, `keep going`, `where were we`, `where did we stop`, `pick up` — word-boundary, case-insensitive, checked against the same sanitized text used for god-keyword matching).
- **Freshness**: same 7-day mtime gate as the `session-start.cjs` notice and `/kratos:wrap`.
- **Once per session**: a marker file at `~/.kratos/handoff-injections/<session_id>` suppresses repeat injections; markers older than 7 days are pruned on write (mirrors `memory-sweep.cjs`'s `pruneOldMarkers`). No `session_id` in the payload → the guard is skipped (always injects) rather than silently dropping the handoff.
- **Byte cap**: content is capped at 8KB without splitting a multi-byte UTF-8 rune (`capUTF8Bytes`).
- **Merged with keyword injection**: a god-keyword match and a resume-phrase match are independent — either, both, or neither may fire; the hook merges both contexts into one `additionalContext` and only passes the prompt through untouched when both are empty. (A bare "continue" with no god keyword still injects the handoff.)
- **Fails open** on every error — a missing/stale/unreadable handoff or unresolvable `cwd` degrades to "no injection." A marker I/O failure does *not* suppress this run's injection; it only means the once-per-session guard may not take effect next time. No error path ever blocks the prompt.

`/kratos:recall` is the explicit manual path to the same handoff file — it reads `handoff.md` directly (with or without the binary) and works regardless of resume-phrase detection. Note recall presents the file uncapped, whereas the on-demand hook caps injected content at 8KB (`capUTF8Bytes`), so the two paths can differ for an unusually large handoff.

## Global Storage

Memory is stored globally at `~/.kratos/`:

```
~/.kratos/
├── memory.db           # SQLite database
└── active-session.json # Current session info
```

This allows memory to persist across all projects.

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
cat ~/.kratos/active-session.json
```

## Transcript Memory Sweep (`memory-sweep.cjs`)

Registered as a second `Stop` hook (alongside `session-end.cjs`). Where Iris's inline memory
capture only catches facts flagged during an Iris mission, this hook is a session-wide safety
net: on the final `Stop` of a qualifying session, it blocks once with a two-target instruction
for Claude: (1) review the whole conversation for durable user facts (preferences, habits, weak
spots, corrections, working style — never project/task facts, never secrets), dedupe against
`kratos memory list`, and save at most 3 via `kratos memory add`; (2) identify corrections the
user made to a specific god-agent's finished work and save at most 2 as per-agent lessons via
`kratos feedback add --agent <god>`. Lessons are re-injected at that agent's next spawn by
`path-inject.cjs` (≤5, current-project first via `feedback list --prefer-project`; fail-open —
any error just drops the lessons block).

**Guards** — the hook allows the stop silently (no output, no block) whenever any of these trip:

| Guard | Behavior |
|-------|----------|
| `stop_hook_active === true` | Already re-invoked because of this hook — never block twice |
| `KRATOS_MEMORY_SWEEP=off` | Opt-out (see below) |
| Marker `~/.kratos/sweeps/<session_id>` exists | Already swept this session |
| Transcript has fewer than 6 user messages | Session too short to be worth a sweep |
| Transcript contains `IRIS COMPLETE` | Iris already swept her own mission — don't double-sweep |
| Transcript contains `KRATOS WRAP COMPLETE` | `/kratos:wrap` already swept inline before printing its marker — don't double-sweep |
| Transcript file missing or unreadable | Fail open — never block blind |
| `kratos` binary unresolvable | No CLI, no sweep |

On a qualifying session the hook writes the marker file first (so a hung or interrupted sweep
never causes a repeat block), prunes markers older than 7 days, then emits
`{"decision":"block","reason":"<sweep instruction>"}`.

**Opt-out**: set `KRATOS_MEMORY_SWEEP=off` in your environment to disable the sweep entirely.
`session-end.cjs` and the rest of the `Stop` hooks are unaffected.

## Troubleshooting

**Hooks not running?**
- Ensure Kratos plugin is enabled: `kratos@lizard-plugins` in settings.json
- Check `~/.kratos/` directory was created
- Restart Claude Code after enabling plugin

**No data recorded?**
- Run: `kratos init` (binary at `${CLAUDE_PLUGIN_ROOT}/bin/kratos` or `~/.kratos/bin/kratos`)
- Rebuild if missing: `cd go && make build`

**View hook errors:**
- Check Claude Code logs for hook execution errors
