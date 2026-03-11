---
name: recall
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

Use the Go binary (preferred) or status.json fallback to get session info:

```bash
# Go binary (primary method) — pass the project root path as argument
~/.kratos/bin/kratos recall $(git rev-parse --show-toplevel 2>/dev/null || pwd)

# Global recall (all projects)
~/.kratos/bin/kratos recall --global --limit 5

# Incomplete features only
~/.kratos/bin/kratos recall $(git rev-parse --show-toplevel 2>/dev/null || pwd) --incomplete

# Fallback: scan status.json files directly (if Go binary unavailable)
# Use Glob to find .claude/feature/*/status.json and Read to parse them
```

### Options

| Flag | Effect |
|------|--------|
| `[project]` | Project root path (required unless `--global`) |
| `--global` | Show sessions across all projects |
| `--incomplete` | Show only incomplete features |
| `--limit N` | Number of recent sessions for `--global` (default: 5) |

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

Pipeline symbols: `✅` = complete, `>>` = current/in-progress, `..` = pending/not started, `⏭️` = skipped, `❌` = blocked

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

Run the Go binary (preferred):

```bash
~/.kratos/bin/kratos recall $(git rev-parse --show-toplevel 2>/dev/null || pwd)
```

Or for global:

```bash
~/.kratos/bin/kratos recall --global --limit 5
```

If the Go binary is unavailable, fall back to scanning `.claude/feature/*/status.json` files directly using Glob and Read tools.

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

### Go Binary Not Available

If the Go binary is not available:

```
KRATOS RECALL

Note: Memory binary unavailable. Falling back to status file scan.
```

Fall back to scanning `.claude/feature/*/status.json` files directly using Glob and Read tools. Parse the JSON to reconstruct session context — read `current_stage`, `pipeline_status`, `updated`, and `history` to build the recall summary.

**Limitation:** Global recall (`--global`) requires the Go binary because it searches across all projects. Without the binary, only the current project's features are searchable. If `--global` is requested without the binary, inform the user:

```
KRATOS RECALL

Global recall requires the kratos binary (~/.kratos/bin/kratos).
Showing current project only.
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

1. **Run the query** (Go binary preferred, status.json fallback):
```bash
~/.kratos/bin/kratos recall $(git rev-parse --show-toplevel 2>/dev/null || pwd) 2>/dev/null
```
If the binary is unavailable, use Glob to find `.claude/feature/*/status.json` and Read to parse them.

2. **Parse the JSON output**

3. **Format and display** according to the templates above

4. **Offer continuation** if there's an incomplete feature

**Now execute the recall query and present the results.**
