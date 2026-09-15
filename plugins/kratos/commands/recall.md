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

## Response Format

### Project-Specific Mode (Default)

When the user runs `/kratos:recall`:

```
KRATOS RECALL

Feature: [feature-name]
Stage: [X]/9 ([stage-name])
Status: [in_progress | completed | abandoned]
Last active: [time ago]

Last Actions:
- [Action 1]
- [Action 2]
- [Action 3]

Pipeline:
[1]OK -> [2]OK -> [3]OK -> [4]>> -> [5].. -> [6].. -> [7].. -> [8].. -> [9]..

Pipeline symbols: `✅` = complete, `>>` = current/in-progress, `..` = pending/not started, `⏭️` = skipped, `❌` = blocked

Recommendation: Continue with Stage [X] ([Agent] - [Stage Name])?
```

### Global Mode

When the user runs `/kratos:recall --global`:

```
KRATOS RECALL (Global)

Recent sessions across all projects:

1. [project]/[feature] - Stage [X]/9 - [time ago]
2. [project]/[feature] - Stage [X]/9 - [time ago]
3. [project]/[feature] - Completed - [time ago]

Use /kratos:recall in the project directory for details.
```

Stage reference: 0 Research (Metis, optional) · 1 PRD (Kratos gap analysis inline, then Athena) · 2 PRD Review (Nemesis) · 3 Decomposition (Daedalus, optional) · 4 Tech Spec (Themis discuss inline, then Hephaestus) · 5 SA Spec Review (Apollo) · 6 Test Plan (Artemis) · 7 Implementation (Ares) · 8 PRD Alignment (Hera) · 9 Review (Hermes + Cassandra).

---

## Execution Steps

### Step 1: Determine Mode

Check if user specified `--global`:
- If yes: Run global query
- If no: Run project-specific query

### Step 2: Query Memory

Run the Go binary (preferred) — pass the project root path as argument:

```bash
<kratos-bin> recall $(git rev-parse --show-toplevel 2>/dev/null || pwd)
```

Add `--incomplete` to show only incomplete features. For global:

```bash
<kratos-bin> recall --global --limit 5
```

(`--limit N` sets the number of recent sessions for `--global`; default 5.)

If the Go binary is unavailable, fall back to scanning `.claude/feature/*/status.json` files directly using Glob and Read tools.

### Step 3: Check for a session handoff

Check whether `.claude/.Arena/handoff.md` exists and is less than 7 days old (same freshness gate as `/kratos:wrap` and the on-demand injection hook). This step works with or without the Go binary — it's a plain file check:

- Use Glob/Read (or `test -f` via Bash) to check `.claude/.Arena/handoff.md` exists.
- If it exists, check its modification time is within 7 days.
- If fresh, Read the file and present its content alongside the recall summary (see Response Format above) — this is the explicit manual path to the same content the resume-phrase hook injects on demand.
- If missing or stale, skip silently — no mention of it in the output.

### Step 3b: Check for unfinished plan drafts

A `/kratos:plan` session that ended mid-clarification leaves a tactical plan marked `status: draft`. It holds real answers the user already gave, and it is invisible to every other recall surface — Odysseus creates no session, no feature dir, and no `status.json`, so a plan-only session is exactly the case Steps 1–3 cannot see. This is a plain file check, binary or not:

- Glob `.claude/.Arena/tactical-plans/*.md` and Read the frontmatter of each.
- Collect any whose frontmatter says `status: draft`, along with the entries under their `## Locked Decisions` heading.
- Apply the same 7-day freshness gate as the handoff.
- If none, skip silently.

### Step 4: Parse and Present

Parse the JSON response and format it according to the templates above. If Step 3 found a fresh handoff, include it under a `## Session Handoff` heading after the pipeline summary. If Step 3b found drafts, list them under a `## Unfinished Plans` heading with their locked-decision count and the resume command:

```
## Unfinished Plans

- `.claude/.Arena/tactical-plans/<slug>.md` — <N> locked decisions, last touched <date>
  Resume: /kratos:plan <task title>   (picks up the existing answers instead of re-asking)
```

### Step 5: Offer Continuation

If there's an incomplete feature, offer to continue:

> **Ready to continue?**
> Say "continue" or "/kratos:main" to resume from Stage [X] with [Agent].

---

## Edge Cases

### No Previous Sessions

If no sessions found:

```
KRATOS RECALL

No previous sessions found for this project.

To start a new feature, use:
  /kratos:main Build [your feature description]

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
  /kratos:main Build [your feature description]
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

Global recall requires the kratos binary (<kratos-bin>).
Showing current project only.
```

---

**Now execute the recall query and present the results.**
