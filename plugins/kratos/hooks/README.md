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
| `Stop` | Claude Code exits | Ends session with summary |

## Files

| File | Purpose |
|------|---------|
| `hooks.json` | Hook registration (loaded by Claude Code) |
| `session-start.cjs` | Starts memory session |
| `tool-use.cjs` | Records tool usage |
| `session-end.cjs` | Ends session with summary |

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
