# Agent Protocol — Shared Procedures

Procedures shared across all Kratos agents. Read sections relevant to your mission.

---

## Path Resolution

All paths in agent instructions (e.g., `plugins/kratos/references/...`, `.claude/feature/...`) are relative to the **project root** (git repository root). Resolve from project root, not plugin directory.

Templates are retrieved via the CLI: `'<kratos-path>' template get <template-name>` (omit the `.md` extension). The CLI handles file location regardless of where the plugin is installed.

**Kratos binary**: The `SubagentStart` hook injects the resolved absolute path. Wherever instructions show `<kratos-bin>`, substitute the literal path from the hook directly — e.g. `'/usr/local/bin/kratos' <subcommand>`. If no path was injected, skip all kratos calls and report to Kratos that the binary is unavailable.

---

## Document Selection

Choose documents based on the decision you are making; don't mechanically read every input.

- Use `status.json` for stage state, summaries, and quick context
- Use `prd.md` for requirements, acceptance criteria, and product intent
- Use `tech-spec.md` for architecture, interfaces, sequencing, and implementation constraints
- Use `test-plan.md` for expected coverage and verification scope
- Use `decomposition.md` for task ordering, waves, and phase boundaries
- Use Arena/codebase reads only to verify a specific convention, dependency, or implementation pattern

Avoid rereading the same document unless you need a section not already captured.

---

## Missing Required Input

If you need a file and it is missing, don't improvise, recreate it, or continue with assumptions unless you are the agent responsible for producing that file.

1. Stop the current task
2. Report the blocker to Kratos/orchestrator
3. Name the missing file
4. State why you need it right now
5. Name the responsible upstream stage/agent from `references/agent-handoff-spec.md`

Optional files (`context.md`, `decomposition.md`, Arena shards, language-specific review rules, etc.) only block you if the current task genuinely requires them.

---

## Document Creation

Your primary deliverable is a document file. Kratos verifies this file exists after you complete — if missing, Kratos will re-spawn you, wasting time and tokens.

1. Create the document file early (even a skeleton) and fill it as you work
2. Before reporting completion, verify the file EXISTS using `Read` or `Glob`
3. Verify the document has complete content (not empty or partial)
4. Update `status.json` via the CLI (see below) and confirm stage status is `complete`

---

## Timestamp Standard

**Never write `<ISO-timestamp>` placeholders.** Always use a real timestamp.

**Preferred**: Let `kratos pipeline update` stamp timestamps automatically (always uses real time).

**When you must write a timestamp manually** (e.g., fallback JSON edits or nested fields the CLI doesn't cover):

```bash
# Capture a precise ISO8601 timestamp
TS=$(<kratos-bin> now 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
```

Then use `$TS` wherever the schema expects `<ISO8601>`:

```json
{ "started": "2026-03-30T14:05:00Z", "completed": "2026-03-30T14:07:30Z" }
```

`kratos now` outputs RFC3339 with local timezone offset (e.g., `2026-03-30T22:05:00+08:00`). `date` fallback outputs UTC. Both are valid ISO8601.

---

## Status Updates via Kratos CLI

Update pipeline status using the exact command format below. Do NOT improvise flags or invent new ones.

**CRITICAL**: For authentic timestamps, always use the two-step process:

### Step 1: Mark Work as Started
```bash
# When you BEGIN work, immediately mark as in-progress
<kratos-bin> pipeline update --feature FEATURE_NAME --stage STAGE_NAME --status in-progress
```

### Step 2: Mark Work as Complete  
```bash
# When you FINISH work, mark as complete with deliverables
<kratos-bin> pipeline update --feature FEATURE_NAME --stage STAGE_NAME --status complete --document DOC_NAME

# For review stages, include verdict:
<kratos-bin> pipeline update --feature FEATURE_NAME --stage STAGE_NAME --status complete --verdict VERDICT --document DOC_NAME
```

### Examples
```bash
# PRD Creation (two steps):
<kratos-bin> pipeline update --feature auth-system --stage 1-prd --status in-progress
# ... do the actual PRD work ...
<kratos-bin> pipeline update --feature auth-system --stage 1-prd --status complete --document prd.md

# Review (two steps):
<kratos-bin> pipeline update --feature auth-system --stage 2-prd-review --status in-progress
# ... do the actual review work ...
<kratos-bin> pipeline update --feature auth-system --stage 2-prd-review --status complete --verdict approved --document prd-review.md
```

**Why Two Steps**: Ensures `started` and `completed` have different timestamps, preventing zero-duration work periods that appear fabricated.

- If the command outputs JSON → done. Do NOT also write status.json manually.
- If the command is not found or errors → fall back to editing status.json directly using `kratos now` for timestamps.

### Spawning Athena (stages 1, 2, 6)

Athena runs at three different stages. Before spawning Athena, set `pending_stage` so the
`kratos check --init` hook injects the correct deliverable expectations at SubagentStart:

```bash
# Before spawning Athena for stage 2 or 6 (stage 1 is already set at feature init):
<kratos-bin> pipeline set-pending --feature FEATURE_NAME --stage STAGE_NAME

# After Athena completes (clears the field):
<kratos-bin> pipeline set-pending --feature FEATURE_NAME --stage ""
```

Omitting this step causes `--init` to read the previous completed stage and tell Athena
to produce the wrong deliverable.

---

## Session Tracking

Record your work in the active Kratos session so Kratos can reconstruct what happened.

```bash
PROJECT=$(basename $(git rev-parse --show-toplevel 2>/dev/null || pwd))
SESSION_ID=$(<kratos-bin> session active "$PROJECT" 2>/dev/null | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

# Record your spawn at start (replace AGENT_NAME, MODEL, DESCRIPTION)
<kratos-bin> step record-agent "$SESSION_ID" AGENT_NAME MODEL "DESCRIPTION"

# Record each document you create or modify
<kratos-bin> step record-file "$SESSION_ID" "path/to/file" "created"
```

If the binary is unavailable, skip session tracking silently — useful but not critical.

---

## Boundaries (all agents)

Subagent of Kratos. Stay in your domain. Schema: `references/status-json-schema.md`. Complete mission and return.
