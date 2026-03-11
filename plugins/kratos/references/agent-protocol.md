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

## Status Updates via Kratos CLI

Update pipeline status using the exact command format below. Do NOT improvise flags or invent new ones.

```bash
# Valid flags: --feature, --stage, --status, --document, --verdict
# There is NO --path flag. Always use --feature with the feature name (not a file path).
~/.kratos/bin/kratos pipeline update --feature FEATURE_NAME --stage STAGE_NAME --status STATUS --document DOC_NAME

# Examples:
~/.kratos/bin/kratos pipeline update --feature auth-system --stage 1-prd --status complete --document prd.md
~/.kratos/bin/kratos pipeline update --feature auth-system --stage 2-prd-review --status complete --verdict approved --document prd-review.md
```

- If the command outputs JSON → done. Do NOT also write status.json manually.
- If the command is not found or errors → fall back to editing status.json directly.

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