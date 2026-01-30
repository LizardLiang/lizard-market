---
name: memory
description: Memory persistence for Kratos - records every step of the journey
---

# Kratos Memory Skill

This skill enables persistent memory for Kratos, recording every step of the journey to SQLite.

## Database Location

Memory is stored at: `.claude/.kratos/memory.db`

## Initialization

Before first use, initialize the database:

```bash
python plugins/kratos/memory/kratos_memory.py init
```

Or with Rust (faster):
```bash
cd plugins/kratos/memory && cargo build --release
./target/release/kratos-memory init
```

---

## Memory Recording Protocol

### When Starting a Session

At the VERY BEGINNING of any Kratos invocation, record the session:

```bash
python plugins/kratos/memory/kratos_memory.py session start "<project>" --feature="<feature>" --request="<user_request>"
```

**Capture the returned `session_id`** - you'll need it for all subsequent recordings.

### When Spawning an Agent

BEFORE spawning any agent via Task tool, record the step:

```bash
python plugins/kratos/memory/kratos_memory.py step "<session_id>" "agent_spawn" "Spawning <agent> for stage <N>" \
  --agent="<agent_name>" \
  --model="<opus|sonnet|haiku>" \
  --stage=<0-8> \
  --target="<feature_name>"
```

### When Agent Completes

AFTER an agent returns, record the result:

```bash
python plugins/kratos/memory/kratos_memory.py step "<session_id>" "agent_complete" "Agent <name> completed <task>" \
  --agent="<agent_name>" \
  --result="<success|failure|needs_revision>" \
  --files-modified='["path1", "path2"]'
```

### When Files Change

When any file is created, modified, or deleted:

```bash
python plugins/kratos/memory/kratos_memory.py file-change "<session_id>" "<file_path>" "<created|modified|deleted>" "<description>"
```

### When Making Decisions

When choosing between alternatives or making architectural decisions:

```bash
python plugins/kratos/memory/kratos_memory.py decision "<session_id>" "<question>" "<choice>" \
  --type="<architecture|implementation|trade_off|direction>" \
  --feature="<feature_name>" \
  --rationale="<why_this_choice>"
```

### When Stage Completes

Update feature progress:

```bash
python plugins/kratos/memory/kratos_memory.py feature stage "<feature_name>" <stage_number>
```

### When Session Ends

Record the session summary:

```bash
python plugins/kratos/memory/kratos_memory.py session end "<session_id>" "<summary>" "completed"
```

---

## Step Types

| Type | When to Use |
|------|-------------|
| `agent_spawn` | Before spawning any agent |
| `agent_complete` | After agent returns |
| `command` | When executing a Kratos command |
| `file_modify` | When modifying files directly |
| `decision` | When making a choice |
| `error` | When encountering errors |
| `gate_check` | When checking gate prerequisites |
| `stage_transition` | When moving between stages |

## Decision Types

| Type | When to Use |
|------|-------------|
| `architecture` | System design choices |
| `implementation` | Code implementation choices |
| `trade_off` | Choosing between competing concerns |
| `direction` | Project direction decisions |

---

## Querying Memory

### Get Journey Summary

```bash
python plugins/kratos/memory/kratos_memory.py summary [project] [days]
```

### Search Steps

```bash
python plugins/kratos/memory/kratos_memory.py query search "<query>"
```

### Get Feature Decisions

```bash
python plugins/kratos/memory/kratos_memory.py query decisions "<feature_name>"
```

### Get Recent File Changes

```bash
python plugins/kratos/memory/kratos_memory.py query files [days]
```

---

## Integration Points

### In main.md (Kratos Main Command)

Add at the START of any session:
1. Check for active session: `session active`
2. If none, start new: `session start`
3. Store `session_id` for this invocation

### Before Each Agent Spawn

1. Record: `step <session_id> agent_spawn ...`
2. Then spawn the agent via Task tool
3. After return: `step <session_id> agent_complete ...`

### In status.json Updates

When updating `status.json`:
1. Record: `feature stage <name> <stage>`
2. Record file change for status.json

### On Victory or Session End

1. Record: `session end <session_id> "<summary>"`

---

## Memory Schema Overview

| Table | Purpose |
|-------|---------|
| `sessions` | Track each Kratos invocation |
| `steps` | Every action taken |
| `features` | Pipeline progress per feature |
| `file_changes` | All file modifications |
| `decisions` | Important choices made |

---

## Performance Notes

- SQLite with WAL mode: ~1ms writes
- Python script: ~50ms cold start, ~10ms warm
- Rust binary: ~2ms total execution
- FTS5 search: ~5ms for 1000+ entries

For highest performance in long sessions, prefer the Rust binary.

---

## Example Session Recording

```
# Start session
SESSION=$(python kratos_memory.py session start "my-project" --feature="user-auth" --request="Build user authentication")

# Record agent spawn
python kratos_memory.py step "$SESSION" "agent_spawn" "Spawning Athena for PRD creation" \
  --agent="athena" --model="opus" --stage=1 --target="user-auth"

# After agent completes
python kratos_memory.py step "$SESSION" "agent_complete" "Athena completed PRD" \
  --agent="athena" --result="success" --files-modified='[".claude/feature/user-auth/prd.md"]'

# Record file creation
python kratos_memory.py file-change "$SESSION" ".claude/feature/user-auth/prd.md" "created" "PRD for user authentication feature"

# Update feature stage
python kratos_memory.py feature stage "user-auth" 1

# Record decision
python kratos_memory.py decision "$SESSION" "Authentication method?" "JWT with refresh tokens" \
  --type="architecture" --feature="user-auth" --rationale="Stateless, scalable, industry standard"

# End session
python kratos_memory.py session end "$SESSION" "Completed PRD creation for user-auth feature" "completed"
```

---

## Automatic Context Injection

At session start, query recent memory to inject context:

```bash
# Get summary of recent work
python kratos_memory.py summary "<project>" 7

# Get decisions for current feature
python kratos_memory.py query decisions "<feature_name>"
```

Use this context to inform agent prompts with historical decisions and patterns.
