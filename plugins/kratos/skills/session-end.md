---
name: session-end
description: Properly end a Kratos memory session with summary
---

# Session End Skill

This skill ensures proper session termination with a comprehensive summary recorded to memory.

## When to Trigger

This skill should be activated when:
1. User says "done", "finished", "that's all", or similar
2. A feature pipeline reaches VICTORY
3. User explicitly ends the session
4. Before context is lost in long conversations

## Session End Protocol

### Step 1: Identify Active Session

```bash
python plugins/kratos/memory/kratos_memory.py session active
```

If no active session exists, skip memory recording.

### Step 2: Gather Session Statistics

```bash
# Get session steps
python plugins/kratos/memory/kratos_memory.py query steps "<session_id>"
```

From the steps, calculate:
- Total steps taken
- Agents spawned
- Files created/modified
- Decisions made
- Current pipeline stage (if feature-based)

### Step 3: Generate Summary

Create a summary that includes:
- What was requested
- What was accomplished
- Current state (completed, in-progress, blocked)
- Key decisions made
- Files affected

### Step 4: Record Session End

```bash
python plugins/kratos/memory/kratos_memory.py session end "<session_id>" "<summary>" "<status>"
```

Status options:
- `completed` - All requested work done
- `in_progress` - Work partially complete, can resume
- `blocked` - Stopped due to blocker
- `abandoned` - User chose not to continue

### Step 5: Report to User

```
⚔️ SESSION COMPLETE ⚔️

Session: <session_id>
Duration: <time>
Steps: <count>
Agents: <count>

Summary:
<what was accomplished>

Memory preserved at .claude/.kratos/memory.db

Run this to see journey summary:
  python plugins/kratos/memory/kratos_memory.py summary

Until next time, mortal.
```

---

## Automatic Triggers

The session should be automatically ended when:

### Victory Achieved

```python
if feature_status == "stage_8_complete":
    summary = f"Completed {feature_name} feature through all 8 pipeline stages"
    end_session(session_id, summary, "completed")
```

### User Signs Off

Detect phrases like:
- "thanks", "thank you"
- "done", "finished"
- "that's all", "all set"
- "goodbye", "bye"

```python
if user_intent == "session_end":
    summary = generate_summary_from_steps(session_id)
    end_session(session_id, summary, determine_status())
```

### Context Warning

If conversation is getting long and context may be lost:

```
⚠️ Context Warning

This session is getting long. Would you like me to:
1. Save progress and start fresh
2. Continue (risk of context loss)

Saving now will preserve all memory to the database.
```

---

## Summary Generation Template

```
Session Summary:

REQUEST: <initial_request>

ACCOMPLISHED:
- <bullet point of what was done>
- <bullet point>
- <bullet point>

FILES AFFECTED:
- Created: <count> files
- Modified: <count> files
- Deleted: <count> files

DECISIONS MADE:
- <key decision 1>
- <key decision 2>

CURRENT STATE:
- Feature: <feature_name> at stage <N>
- Status: <completed|in_progress|blocked>

NEXT STEPS (if applicable):
- <what to do next time>
```

---

## Example Usage

```
User: "That's all for today, thanks!"

Kratos:
# Get active session
SESSION=$(python kratos_memory.py session active | jq -r '.session_id')

# Generate summary
⚔️ SESSION ENDING ⚔️

Generating journey summary...

# Query steps
STEPS=$(python kratos_memory.py query steps "$SESSION")

# End session
python kratos_memory.py session end "$SESSION" "Completed PRD and tech spec for user-auth feature. Stage 5 (SA review) ready to begin next session." "in_progress"

⚔️ SESSION COMPLETE ⚔️

Session: abc-123-def
Duration: ~45 minutes
Steps: 12
Agents: 4 (athena x2, hephaestus, apollo)

Summary:
Completed PRD and tech spec for user-auth feature.
Stage 5 (SA review) ready to begin next session.

Files Created:
- .claude/feature/user-auth/prd.md
- .claude/feature/user-auth/prd-review.md
- .claude/feature/user-auth/tech-spec.md

Decisions:
- Architecture: JWT with refresh tokens
- Implementation: Redis for session storage

Memory preserved. Resume anytime with:
  /kratos:main-with-memory

Until next time, mortal.
```

---

## Integration with Claude Code Hooks

If using Claude Code hooks, you can create a session-end hook:

```javascript
// hooks/session-end.js
module.exports = {
  name: 'kratos-session-end',
  trigger: 'session_end',
  handler: async (context) => {
    const { exec } = require('child_process');

    // Get active session
    exec('python plugins/kratos/memory/kratos_memory.py session active', (err, stdout) => {
      if (err || !stdout) return;

      const session = JSON.parse(stdout);
      if (session && session.session_id) {
        // End the session
        exec(`python plugins/kratos/memory/kratos_memory.py session end "${session.session_id}" "Session ended by user" "in_progress"`);
      }
    });
  }
};
```

This ensures memory is always recorded, even if the user exits abruptly.
