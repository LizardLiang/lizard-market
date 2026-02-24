---
description: Mark implementation tasks complete in User Mode
---

# Kratos: Task Complete

Mark one or more implementation tasks as complete when using User Mode.

---

## Usage

```
/kratos:task-complete <task-id> [task-id2] [task-id3] ...
/kratos:task-complete all
```

### Examples

```
/kratos:task-complete 01
/kratos:task-complete 01 02 03
/kratos:task-complete all
```

---

## Workflow

### Step 1: Parse Arguments

Extract task IDs from the command arguments:
- Single ID: `01`
- Multiple IDs: `01 02 03`
- Special keyword: `all`

### Step 2: Find Active Feature

Search for the active feature:
```
.claude/feature/*/status.json
```

Verify:
1. Feature exists
2. Stage 7 (implementation) is active
3. Mode is "user" (User Mode)

### Step 3: Validate Tasks

For each task ID provided:
1. Check if task file exists: `tasks/XX-*.md`
2. Check if task is in status.json tasks array
3. Verify task is not already complete

### Step 4: Update Status

For each valid task:

1. **Update status.json**:
   ```json
   {
     "pipeline": {
       "7-implementation": {
         "tasks": {
           "items": [
             { "id": "01", "name": "...", "status": "complete" }
           ]
         }
       }
     }
   }
   ```

2. **Update task file** (optional):
   - Change `Status` field from `Pending` to `Complete`

3. **Update 00-overview.md**:
   - Update Task Index status column
   - Update Progress Tracking section

### Step 5: Check Completion

After updating, check if ALL tasks are complete:

```javascript
const allComplete = tasks.items.every(t => t.status === 'complete');
```

### Step 6: Handle All Complete

When ALL tasks are complete:

1. **Update status.json**:
   ```json
   {
     "stage": "8-code-review",
     "pipeline": {
       "7-implementation": {
         "status": "complete",
         "completed": "<ISO-timestamp>"
       },
       "8-code-review": {
         "status": "ready"
       }
     }
   }
   ```

2. **Spawn Hermes for code review**:
   ```
   Task(
     subagent_type: "kratos:hermes",
     model: "opus",
     prompt: "MISSION: Code Review
   FEATURE: [feature-name]
   FOLDER: .claude/feature/[feature-name]/
   MODE: User Mode (implementation done by user)

   CRITICAL: You MUST create the file code-review.md before completing. Document creation is MANDATORY - verify it exists before reporting completion.

   Review implementation code. Create code-review.md with verdict. Update status.json.",
     description: "hermes - code review (user mode)"
   )
   ```

---

## Output Formats

### Single Task Marked Complete

```
✅ Task Complete

Task 01 marked as complete.

Progress: [██████░░░░░░░░░░░░░░] 30% (3/10 tasks)

Remaining tasks:
- 04: Add authentication middleware
- 05: Implement login endpoint
- ...

Next: Continue with remaining tasks or `/kratos:task-complete all` when done.
```

### Multiple Tasks Marked Complete

```
✅ Tasks Complete

Marked 3 tasks as complete:
- 01: Create user model
- 02: Add database migrations
- 03: Implement user service

Progress: [████████░░░░░░░░░░░░] 40% (4/10 tasks)

Remaining: 6 tasks
```

### All Tasks Complete (Trigger Code Review)

```
🎉 All Tasks Complete!

All 10 implementation tasks have been marked complete.

Progress: [████████████████████] 100% (10/10 tasks)

Advancing to Stage 8: Code Review
Summoning: HERMES (model: opus)

[TASK TOOL INVOCATION FOR HERMES]
```

### Error: Task Not Found

```
❌ Task Not Found

Task ID '15' does not exist in this feature.

Available tasks:
- 01: Create user model
- 02: Add database migrations
- ...
```

### Error: Not in User Mode

```
❌ Not in User Mode

This feature is using Ares Mode (AI implementation).
The /kratos:task-complete command is only available in User Mode.

Current stage: 7-implementation
Mode: ares
```

### Error: Wrong Stage

```
❌ Wrong Stage

Cannot mark tasks complete - not in implementation stage.

Current stage: 6-test-plan
Required stage: 7-implementation
```

---

## Status JSON Updates

### Task Structure in status.json

```json
{
  "pipeline": {
    "7-implementation": {
      "status": "in-progress",
      "mode": "user",
      "tasks": {
        "total": 10,
        "completed": 3,
        "items": [
          { "id": "01", "name": "Create user model", "file": "01-create-user-model.md", "status": "complete" },
          { "id": "02", "name": "Add migrations", "file": "02-add-migrations.md", "status": "complete" },
          { "id": "03", "name": "User service", "file": "03-user-service.md", "status": "complete" },
          { "id": "04", "name": "Auth middleware", "file": "04-auth-middleware.md", "status": "pending" }
        ]
      }
    }
  }
}
```

---

## Implementation Notes

1. **Idempotent**: Marking an already-complete task as complete should succeed silently
2. **Atomic**: All tasks in a batch should be updated together
3. **Validation**: Always verify task exists before updating
4. **Progress bar**: Use block characters for visual progress (`█` and `░`)
