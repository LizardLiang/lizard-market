# Agent Protocol — Shared Procedures

This document contains procedures shared across all Kratos agents. Read the sections relevant to your mission.

---

## Path Resolution

All paths in agent instructions (e.g., `plugins/kratos/templates/...`, `plugins/kratos/references/...`, `.claude/feature/...`) are relative to the **project root** (the git repository root). When reading templates, references, or feature files, resolve paths from the project root, not from the plugin directory.

---

## Document Creation

Your primary deliverable is a document file. Kratos verifies this file exists after you complete — if it's missing, Kratos will re-spawn you to try again, wasting time and tokens. To avoid this:

1. Create the document file early in your mission (even a skeleton) and fill it as you work
2. Before reporting completion, verify the file EXISTS using `Read` or `Glob`
3. Verify the document has complete content (not empty or partial)
4. Update `status.json` via the CLI (see below) and confirm the stage status is `complete`

---

---

## Status Updates via Kratos CLI

Update pipeline status using the exact command format below. Do NOT improvise flags or invent new ones.

**CRITICAL**: For authentic timestamps, always use the two-step process:

### Step 1: Mark Work as Started
```bash
# When you BEGIN work, immediately mark as in-progress
~/.kratos/bin/kratos pipeline update --feature FEATURE_NAME --stage STAGE_NAME --status in-progress
```

### Step 2: Mark Work as Complete  
```bash
# When you FINISH work, mark as complete with deliverables
~/.kratos/bin/kratos pipeline update --feature FEATURE_NAME --stage STAGE_NAME --status complete --document DOC_NAME

# For review stages, include verdict:
~/.kratos/bin/kratos pipeline update --feature FEATURE_NAME --stage STAGE_NAME --status complete --verdict VERDICT --document DOC_NAME
```

### Examples
```bash
# PRD Creation (two steps):
~/.kratos/bin/kratos pipeline update --feature auth-system --stage 1-prd --status in-progress
# ... do the actual PRD work ...
~/.kratos/bin/kratos pipeline update --feature auth-system --stage 1-prd --status complete --document prd.md

# Review (two steps):
~/.kratos/bin/kratos pipeline update --feature auth-system --stage 2-prd-review --status in-progress
# ... do the actual review work ...
~/.kratos/bin/kratos pipeline update --feature auth-system --stage 2-prd-review --status complete --verdict approved --document prd-review.md
```

**Why Two Steps**: This ensures `started` and `completed` have different timestamps, preventing zero-duration work periods that appear fabricated.

- If the command outputs JSON → done.
- **Never edit status.json directly.** The CLI is the only sanctioned write path — it enforces schema consistency and real timestamps.

### Spawning Athena (stages 1, 2, 6)

Athena runs at three different stages. Before spawning Athena, set `pending_stage` so the
`kratos check --init` hook injects the correct deliverable expectations at SubagentStart:

```bash
# Before spawning Athena for stage 2 or 6 (stage 1 is already set at feature init):
~/.kratos/bin/kratos pipeline set-pending --feature FEATURE_NAME --stage STAGE_NAME

# After Athena completes (clears the field):
~/.kratos/bin/kratos pipeline set-pending --feature FEATURE_NAME --stage ""
```

Omitting this step causes `--init` to read the previous completed stage and tell Athena
to produce the wrong deliverable.

---

## Session Tracking

Record your work in the active Kratos session so Kratos can reconstruct what happened.

```bash
PROJECT=$(basename $(git rev-parse --show-toplevel 2>/dev/null || pwd))
SESSION_ID=$(~/.kratos/bin/kratos session active "$PROJECT" 2>/dev/null | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

# Record your spawn at start (replace AGENT_NAME, MODEL, DESCRIPTION)
~/.kratos/bin/kratos step record-agent "$SESSION_ID" AGENT_NAME MODEL "DESCRIPTION"

# Record each document you create or modify
~/.kratos/bin/kratos step record-file "$SESSION_ID" "path/to/file" "created"
```

If the binary is unavailable, skip session tracking silently — it's useful but not critical.