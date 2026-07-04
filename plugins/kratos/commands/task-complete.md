---
name: task-complete
description: Mark implementation tasks complete in User Mode
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

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

### Step 1: Complete Tasks (CLI)

One call does everything deterministic — feature auto-detection, stage-7 + User-Mode validation, batch validation (atomic: an unknown ID fails the whole batch with the file untouched), marking tasks complete with real timestamps, progress-bar rendering, and auto-advancing stage 7 → complete / stage 8 → ready when the last task completes:

```bash
<kratos-bin> pipeline tasks complete <task-id> [task-id2] ... --json
<kratos-bin> pipeline tasks complete all --json
```

The JSON returns `completed_now[]`, `already_complete[]` (idempotent no-ops), `remaining[]`, `total`/`completed`/`pct`, a pre-rendered `bar`, `all_complete`, and `advanced`. On validation errors the command exits non-zero with the exact reason (wrong stage / not in user mode / unknown IDs + available list) — render the matching error block from Output Formats below.

To show progress without changing anything: `<kratos-bin> pipeline tasks list --json`.

### Step 2: Update Task Docs (optional)

- Task file: change `Status` from `Pending` to `Complete`
- `00-overview.md`: update Task Index status column and Progress Tracking section

### Step 3: Handle All Complete

When the CLI returns `all_complete: true` (status.json stages already advanced when `advanced: true`):

1. **Spawn Hera** (PRD alignment check, stage 8):
   ```
   Task(
     subagent_type: "kratos:hera",
     model: "sonnet",
     prompt: "MISSION: PRD Alignment Check
   FEATURE: [feature-name]
   FOLDER: .claude/feature/[feature-name]/
   MODE: User Mode (implementation done by user)

   CRITICAL: You MUST create the file prd-alignment.md before completing. If `prd.md` is missing when you need it, stop and report Athena as the owning upstream agent to Kratos.

   Verify every acceptance criterion in prd.md is covered by a test and that tests pass. Create prd-alignment.md with verdict. Update status.json.",
     description: "hera - prd alignment check (user mode)"
   )
   ```

    If Hera returns **aligned**, first check for a pending spec delta and offer to archive it (same procedure as Ares Mode — see `<KRATOS_ROOT>/pipeline/stages.md` Stage 8 section: "After Hera Returns: Spec Archive Offer"):
   ```bash
   <kratos-bin> spec list --changes
   ```
   If this feature has a pending delta, ask the user to confirm archiving before continuing. Then immediately spawn Hermes + Cassandra in parallel (stage 9):
   ```
   Task(
     subagent_type: "kratos:hermes",
     model: "opus",
     prompt: "MISSION: Code Review
   FEATURE: [feature-name]
   FOLDER: .claude/feature/[feature-name]/
   MODE: User Mode (implementation done by user)

    CRITICAL: You MUST create the file code-review.md before completing. Kratos validates the deliverable after you finish.

   Review implementation code. Create code-review.md with verdict. Update status.json.",
     description: "hermes - code review (user mode)"
   )

   Task(
     subagent_type: "kratos:cassandra",
     model: "sonnet",
     prompt: "MISSION: Risk Analysis
   MODE: pipeline
   FEATURE: [feature-name]
   FOLDER: .claude/feature/[feature-name]/

    CRITICAL: You MUST create the file risk-analysis.md before completing. Kratos validates the deliverable after you finish.

   Analyze changed files in this feature for security, breaking changes, edge cases, scalability, and dependency risks.
   Create risk-analysis.md with severity-rated findings. Update status.json.",
     description: "cassandra - risk analysis (user mode)"
   )
   ```

   If Hera returns **gaps**, report missing coverage to user — Ares must be re-spawned to fill the gaps.
   If Hera returns **misaligned**, block and escalate to user.

---

## Fallback (binary unavailable)

Only if `<kratos-bin>` is missing, do the CLI's work by hand:

1. Find the active feature in `.claude/feature/*/status.json`; verify stage 7 is active and `mode` is `"user"` (render the error blocks below otherwise).
2. Validate every task ID against `pipeline["7-implementation"].tasks.items[]` before editing anything; unknown ID → error listing available tasks.
3. Set each task's `status` to `"complete"`, recompute `total`/`completed`, update top-level `updated` with a real timestamp:
   ```bash
   TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
   ```
4. When every task is complete, write in the same edit:
   ```json
   {
      "stage": "8-prd-alignment",
      "pipeline": {
        "7-implementation": { "status": "complete", "completed": "$TS" },
        "8-prd-alignment": { "status": "ready" }
      }
   }
   ```
   Then continue with Step 3 (spawn Hera).

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

Advancing to Stage 9: PRD Alignment Check
Summoning: HERA (model: sonnet)

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
