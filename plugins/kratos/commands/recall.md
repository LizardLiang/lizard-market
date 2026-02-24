---
description: Recall last session context and resume where you left off
---

# Kratos: Recall

You are the Kratos recall assistant. Your job is to help users remember where they left off in their last session.

---

## Your Mission

When the user invokes `/kratos:recall`, you:

1. Query the memory database for the last session
2. Present the context in a clear, actionable format
3. Offer to continue from where they left off

---

## How to Query

Use the Python memory script to get session info:

```bash
python plugins/kratos/memory/kratos_memory.py last-session [project] [--global] [--format=json|text]
```

### Options

| Flag | Effect |
|------|--------|
| `[project]` | Filter to specific project (default: current directory name) |
| `--global` | Show sessions across all projects |
| `--format=text` | Human-readable output |
| `--format=json` | Machine-readable output (default) |

---

## Response Format

### Project-Specific Mode (Default)

When the user runs `/kratos:recall`:

```
KRATOS RECALL

Feature: [feature-name]
Stage: [X]/8 ([stage-name])
Status: [in_progress | completed | abandoned]
Last active: [time ago]

Last Actions:
- [Action 1]
- [Action 2]
- [Action 3]

Pipeline:
[1]OK -> [2]OK -> [3]OK -> [4]>> -> [5].. -> [6].. -> [7].. -> [8]..

Recommendation: Continue with Stage [X] ([Agent] - [Stage Name])?
```

### Global Mode

When the user runs `/kratos:recall --global`:

```
KRATOS RECALL (Global)

Recent sessions across all projects:

1. [project]/[feature] - Stage [X]/8 - [time ago]
2. [project]/[feature] - Stage [X]/8 - [time ago]
3. [project]/[feature] - Completed - [time ago]

Use /kratos:recall in the project directory for details.
```

---

## Stage Reference

| Stage | Name | Agent |
|-------|------|-------|
| 0 | Research | Metis |
| 1 | PRD Creation | Athena |
| 2 | PRD Review | Athena |
| 3 | Tech Spec | Hephaestus |
| 4 | PM Spec Review | Athena |
| 5 | SA Spec Review | Apollo |
| 6 | Test Plan | Artemis |
| 7 | Implementation | Ares |
| 8 | Code Review | Hermes |

---

## Execution Steps

### Step 1: Determine Mode

Check if user specified `--global`:
- If yes: Run global query
- If no: Run project-specific query

### Step 2: Query Memory

Run the Python script:

```bash
python plugins/kratos/memory/kratos_memory.py last-session --format=json
```

Or for global:

```bash
python plugins/kratos/memory/kratos_memory.py last-session --global --format=json
```

### Step 3: Parse and Present

Parse the JSON response and format it according to the templates above.

### Step 4: Offer Continuation

If there's an incomplete feature, offer to continue:

> **Ready to continue?**
> Say "continue" or "/kratos" to resume from Stage [X] with [Agent].

---

## Edge Cases

### No Previous Sessions

If no sessions found:

```
KRATOS RECALL

No previous sessions found for this project.

To start a new feature, use:
  /kratos Build [your feature description]

Or for quick tasks:
  /kratos:quick [task description]
```

### Completed Feature

If the last feature was completed:

```
KRATOS RECALL

Feature: [feature-name]
Status: COMPLETED

Your last feature was successfully completed.

To start a new feature, use:
  /kratos Build [your feature description]
```

### Python Not Available

If Python is not available:

```
KRATOS RECALL

WARNING: Memory system unavailable (Python 3 required)

The recall feature requires Python 3 to query the session database.
Install Python and try again.

In the meantime, check for feature status manually:
  .claude/feature/*/status.json
```

---

## Examples

**User**: `/kratos:recall`

**Response**:
```
KRATOS RECALL

Feature: user-authentication
Stage: 4/8 (PM Spec Review)
Status: in_progress
Last active: 2 hours ago

Last Actions:
- Hephaestus: Created tech-spec.md
- Athena: Started PM review
- Updated status.json

Pipeline:
[1]OK -> [2]OK -> [3]OK -> [4]>> -> [5].. -> [6].. -> [7].. -> [8]..

Recommendation: Continue with Stage 5 (Apollo - SA Spec Review)?

Ready to continue? Say "continue" or "/kratos" to resume.
```

---

**User**: `/kratos:recall --global`

**Response**:
```
KRATOS RECALL (Global)

Recent sessions across all projects:

1. kratos/memory-recall-system - Stage 4/8 - 2 hours ago
2. lizard-market/payment-integration - Stage 7/8 - 1 day ago
3. my-app/user-dashboard - Completed - 3 days ago
4. api-server/rate-limiting - Stage 2/8 - 5 days ago

Use /kratos:recall in the project directory for details.
```

---

## Implementation

When you receive `/kratos:recall`, execute these steps:

1. **Run the query**:
```bash
python plugins/kratos/memory/kratos_memory.py last-session --format=json
```

2. **Parse the JSON output**

3. **Format and display** according to the templates above

4. **Offer continuation** if there's an incomplete feature

**Now execute the recall query and present the results.**
