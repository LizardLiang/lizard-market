# Kratos Memory System

A lightweight, fast SQLite-based memory system for tracking Kratos agent orchestration journeys.

## Overview

The Kratos Memory System records every step of your development journey:
- **Sessions**: Each time Kratos is invoked
- **Steps**: Every agent spawn, completion, decision, and action
- **Features**: Pipeline progress for each feature
- **File Changes**: All file modifications
- **Decisions**: Important architectural and implementation choices

## Quick Start

### 1. Initialize the Database

```bash
# Using Python
python plugins/kratos/memory/kratos_memory.py init

# Or using Rust (faster)
cd plugins/kratos/memory
cargo build --release
./target/release/kratos-memory init
```

The database is created at `.claude/.kratos/memory.db`

### 2. Start a Session

```bash
python kratos_memory.py session start "my-project" --feature="user-auth" --request="Build authentication"
# Returns: {"session_id": "abc-123-def"}
```

### 3. Record Steps

```bash
# Record agent spawn
python kratos_memory.py step "abc-123-def" "agent_spawn" "Spawning Athena" \
  --agent="athena" --model="opus" --stage=1

# Record completion
python kratos_memory.py step "abc-123-def" "agent_complete" "Athena done" \
  --agent="athena" --result="success"
```

### 4. End Session

```bash
python kratos_memory.py session end "abc-123-def" "Completed PRD creation" "completed"
```

## CLI Reference

### Session Commands

```bash
# Start session
kratos_memory.py session start <project> [--feature=X] [--request=X]

# End session
kratos_memory.py session end <session_id> [summary] [status]

# Get active session
kratos_memory.py session active [--project=X]
```

### Step Commands

```bash
kratos_memory.py step <session_id> <type> <action> [options]

# Options:
#   --agent=<name>          Agent name (athena, ares, etc.)
#   --model=<opus|sonnet>   Model used
#   --stage=<0-8>           Pipeline stage
#   --target=<path>         Target file/feature
#   --result=<outcome>      Result of action
#   --files-modified='["path1"]'  JSON array of files
```

### Feature Commands

```bash
# Create/update feature
kratos_memory.py feature create <name> <project> [--stage=X] [--status=X]

# Mark stage complete
kratos_memory.py feature stage <name> <stage_number>

# Get feature
kratos_memory.py feature get <name>
```

### File Change Commands

```bash
kratos_memory.py file-change <session_id> <path> <type> [description]

# Types: created, modified, deleted, renamed
```

### Decision Commands

```bash
kratos_memory.py decision <session_id> <question> <choice> [options]

# Options:
#   --type=<architecture|implementation|trade_off|direction>
#   --feature=<name>
#   --rationale=<why>
```

### Query Commands

```bash
# Get recent sessions
kratos_memory.py query sessions [project] [days]

# Get session steps
kratos_memory.py query steps <session_id>

# Get recent file changes
kratos_memory.py query files [days]

# Get feature decisions
kratos_memory.py query decisions <feature_name>

# Full-text search
kratos_memory.py query search <query> [type=steps|decisions]
```

### Summary Command

```bash
kratos_memory.py summary [project] [days]
```

## Database Schema

### Tables

| Table | Purpose |
|-------|---------|
| `sessions` | Track each Kratos invocation |
| `steps` | Every action taken during sessions |
| `features` | Pipeline progress per feature |
| `file_changes` | All file modifications |
| `decisions` | Important choices made |

### Step Types

| Type | When to Use |
|------|-------------|
| `agent_spawn` | Before spawning any agent |
| `agent_complete` | After agent returns |
| `command` | When executing a Kratos command |
| `file_modify` | When modifying files directly |
| `decision` | When making a choice |
| `error` | When encountering errors |
| `gate_check` | When checking prerequisites |
| `stage_transition` | When moving between stages |

### Decision Types

| Type | When to Use |
|------|-------------|
| `architecture` | System design choices |
| `implementation` | Code implementation choices |
| `trade_off` | Choosing between competing concerns |
| `direction` | Project direction decisions |

## Performance

- **SQLite with WAL mode**: ~1ms writes
- **Python script**: ~50ms cold start, ~10ms warm
- **Rust binary**: ~2ms total execution
- **FTS5 search**: ~5ms for 1000+ entries

For highest performance, use the Rust binary:

```bash
cd plugins/kratos/memory
cargo build --release
./target/release/kratos-memory --help
```

## Integration with Kratos

Use the memory-enhanced Kratos command:

```
/kratos:main-with-memory
```

Or add memory recording manually to your workflow by following the protocol in `skills/memory.md`.

## Example Output

### Journey Summary

```json
{
  "period_days": 30,
  "project": "my-project",
  "sessions": {
    "total": 15,
    "completed": 12,
    "total_steps": 156,
    "total_agents": 48
  },
  "features": {
    "total": 3,
    "completed": 2
  },
  "agent_usage": {
    "athena": 18,
    "ares": 12,
    "hermes": 8,
    "hephaestus": 6,
    "apollo": 4
  },
  "file_changes": {
    "created": 45,
    "modified": 89,
    "deleted": 3
  },
  "recent_decisions": [
    {
      "question": "Authentication method?",
      "choice": "JWT with refresh tokens",
      "decision_type": "architecture"
    }
  ]
}
```

## File Structure

```
plugins/kratos/memory/
├── schema.sql           # Database schema
├── kratos_memory.py     # Python CLI tool
├── Cargo.toml           # Rust project config
├── src/
│   └── main.rs          # Rust CLI tool
└── README.md            # This file
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KRATOS_MEMORY_DB` | `.claude/.kratos/memory.db` | Database path |

## License

Part of the Kratos plugin for Claude Code.
