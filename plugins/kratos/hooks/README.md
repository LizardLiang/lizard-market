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
| `SessionStart` | Claude Code starts | Creates memory session |
| `PostToolUse` | Task/Write/Edit tools | Records agent spawns & file changes |
| `Stop` | Claude Code exits | Ends session with summary, then runs the transcript memory sweep |

## Files

| File | Purpose |
|------|---------|
| `hooks.json` | Hook registration (loaded by Claude Code) |
| `session-start.cjs` | Starts memory session |
| `tool-use.cjs` | Records tool usage |
| `session-end.cjs` | Ends session with summary |
| `memory-sweep.cjs` | Once-per-session transcript sweep for durable user facts (see below) |

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
net: on the final `Stop` of a qualifying session, it blocks once with an instruction for Claude
to review the whole conversation for durable user facts (preferences, habits, weak spots,
corrections, working style — never project/task facts, never secrets), dedupe against
`kratos memory list`, and save at most 3 via `kratos memory add`.

**Guards** — the hook allows the stop silently (no output, no block) whenever any of these trip:

| Guard | Behavior |
|-------|----------|
| `stop_hook_active === true` | Already re-invoked because of this hook — never block twice |
| `KRATOS_MEMORY_SWEEP=off` | Opt-out (see below) |
| Marker `~/.kratos/sweeps/<session_id>` exists | Already swept this session |
| Transcript has fewer than 6 user messages | Session too short to be worth a sweep |
| Transcript contains `IRIS COMPLETE` | Iris already swept her own mission — don't double-sweep |
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
