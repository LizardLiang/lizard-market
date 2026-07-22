# Agent Protocol — Shared Procedures

Procedures shared across all Kratos agents. Spawned and inline agents receive their relevant sections injected automatically (SubagentStart hook / `kratos agent load`) — read this file only as a fallback when no injected **Agent Protocol** block is present in your context. Orchestrators read § Spawn Prompt Fields and § Spawning Athena here directly.

---

## Path Resolution
<!-- protocol: path-resolution -->

Plugin-internal paths are written as `<KRATOS_ROOT>/...` (e.g., `<KRATOS_ROOT>/references/...`). Resolution is deterministic, not LLM text-substitution:

- **Spawned subagents**: the SubagentStart hook (`hooks/path-inject.cjs`) injects the resolved absolute plugin root into your context alongside the `<kratos-bin>` path. Use the injected root wherever you see `<KRATOS_ROOT>`. Orchestrators do not (and should not) rewrite `<KRATOS_ROOT>` in spawn prompts themselves.
- **Inline command-mode gods** (e.g. `/kratos:ares`): the generated launcher loads your body via `kratos agent load <name> --resolve`, which substitutes `<KRATOS_ROOT>` and `<kratos-bin>` before you ever see the text — by the time you're reading your persona, the tokens are already gone.
- **Fallback**: if you receive an unsubstituted `<KRATOS_ROOT>` reference (no root was injected and no `--resolve` ran — e.g. the binary is unavailable and the JS fallback in `launch.cjs` also couldn't resolve it), fall back to `plugins/kratos/` relative to the project root (in-repo installs).

Project-artifact paths (e.g., `.claude/feature/...`, `.claude/.Arena/...`) remain relative to the **project root** (git repository root).

Templates are retrieved via the CLI: `'<kratos-path>' template get <template-name>` (omit the `.md` extension). The CLI handles file location regardless of where the plugin is installed.

**Kratos binary**: The `SubagentStart` hook injects the resolved absolute path. Wherever instructions show `<kratos-bin>`, substitute the literal path from the hook directly — e.g. `'/usr/local/bin/kratos' <subcommand>`. If no path was injected, skip all kratos calls and report to Kratos that the binary is unavailable.

---

## Document Selection
<!-- protocol: document-selection -->

Choose documents based on the decision you are making; don't mechanically read every input.

- Use `<kratos-bin> pipeline get --compact --feature FEATURE_NAME` for stage state, summaries, and quick context (do not read `status.json` directly). The `--compact` flag omits the audit-only `history[]` and `check_failures[]`, which grow every stage and waste tokens — always prefer it unless you specifically need the audit trail.
- Other deterministic pipeline work also belongs to the CLI, never hand computation: `pipeline next --json` (next stage, agents, gate check per the transition table), `pipeline status [feature] --json` (dashboard: progress %, health, conflicts), `pipeline tasks list|complete --json` (User Mode task bookkeeping with atomic writes and auto-advance), `slug <text>` (kebab-case slugs; add `--dated` to prepend today's local date `YYYY-MM-DD-` when minting artifact names — feature folders, tactical/strategic plans — so they sort chronologically).
- Use `prd.md` for requirements, acceptance criteria, and product intent
- Use `tech-spec.md` for architecture, interfaces, sequencing, and implementation constraints
- Use `test-plan.md` for expected coverage and verification scope
- Use `decomposition.md` for task ordering, waves, and phase boundaries
- Use Arena/codebase reads only to verify a specific convention, dependency, or implementation pattern

Avoid rereading the same document unless you need a section not already captured.

---

## Auto-Discovery
<!-- protocol: auto-discovery -->

Find the active feature and read pipeline state before starting any mission:

```
Search: .claude/feature/*/status.json
```

Then read the pipeline state:
```bash
<kratos-bin> pipeline get --compact --feature FEATURE_NAME
```

Your agent definition lists the stage-specific prerequisites to verify. In command mode (inline invocation), Auto-Discovery may find no active feature — follow the feature name derivation instructions in your command-mode suffix if present.

---

## Missing Required Input
<!-- protocol: missing-required-input -->

If you need a file and it is missing, don't improvise, recreate it, or continue with assumptions unless you are the agent responsible for producing that file.

1. Stop the current task
2. Report the blocker to Kratos/orchestrator
3. Name the missing file
4. State why you need it right now
5. Name the responsible upstream stage/agent from `references/agent-handoff-spec.md`

Optional files (`context.md`, `decomposition.md`, Arena shards, language-specific review rules, etc.) only block you if the current task genuinely requires them.

---

## Interactive Questions (AskUserQuestion)
<!-- protocol: interactive-questions -->

Canonical rule for every `AskUserQuestion` call across kratos agents, commands, and pipeline prompts:

1. **No escape option.** The client renders a built-in "Other" free-text choice on every question — never add a "Let me type it" / "Other"-style option of your own; it duplicates the native input.
2. **Never set `preview` fields on options.** When any option has a `preview`, the client switches to a side-by-side layout that drops the built-in "Other" free-text inputbox — the user loses the ability to type a custom answer. Fold anything essential from a would-be preview into the option's `description` instead (keep it short).
3. **Free-text reply → treat as the answer.** If the user answers via the built-in "Other", the typed text IS the answer. Parse intent from it before re-asking anything; only follow up if it is genuinely ambiguous.
4. **Decline/interrupt/error → prose fallback, once.** If the tool call is declined, interrupted, or errors, ask the same question one time in plain prose and end the turn. Never immediately re-fire the tool for that question.
5. **Option cap: 4 (tool max).** All 4 slots are substantive — the tool schema caps options at 4 total.
6. **Never call with an empty `options` array.** An empty options list means the intent is free text — ask in plain prose instead of calling the tool with no options.
7. **Subagent caveat.** `AskUserQuestion` only reaches the user from the top-level session. If you find yourself running as a spawned subagent (questions won't surface), don't fake a conversation — flag the gap as an assumption and note that clarification was unavailable instead of calling the tool.

---

## Spawn Prompt Fields (recommended)
<!-- protocol: spawn-prompt-fields -->

Alongside the usual `MISSION:` / `FEATURE:` / `FOLDER:` fields, orchestrators SHOULD include two scope-control fields when spawning agents that write files:

- `NON-GOALS:` — what this spawn must NOT touch, lifted from the PRD's Non-Goals or the tech-spec's scope section. Agents treat this as a scope fence: work that would cross it gets logged (e.g., as debt in implementation-notes.md), never done "while you're here".
- `STOP-CONDITIONS:` — the named early-return signals for this agent (e.g., missing prerequisite → report the owning upstream agent; genuine ambiguity → `<AGENT> NEEDS CLARIFICATION`; wave boundary → checkpoint). Naming them in the packet makes stopping the expected move, not a failure.

Both fields are advisory for read-only spawns (Explore-style searches) but mandatory-in-spirit for implementation spawns — a spawn prompt without a scope fence invites scope creep.

---

## Document Creation
<!-- protocol: document-creation -->

Your primary deliverable is a document file. Kratos verifies this file exists after you complete — if missing, Kratos will re-spawn you, wasting time and tokens.

1. Create the document file early (even a skeleton) and fill it as you work
2. Before reporting completion, verify the file EXISTS using `Read` or `Glob`
3. Verify the document has complete content (not empty or partial)
4. Update `status.json` via the CLI (see below) and confirm stage status is `complete`

---

## Timestamp Standard
<!-- protocol: timestamp-standard -->

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
<!-- protocol: status-updates -->

Update pipeline status using the exact command format below. Do NOT improvise flags or invent new ones.

**CRITICAL**: For authentic timestamps, always use the two-step process:

### Step 1: Mark Work as Started
```bash
# When you BEGIN work, immediately mark as in-progress
<kratos-bin> pipeline update --feature FEATURE_NAME --stage STAGE_NUMBER --status in-progress
```

### Step 2: Mark Work as Complete  
```bash
# When you FINISH work, mark as complete with deliverables
<kratos-bin> pipeline update --feature FEATURE_NAME --stage STAGE_NUMBER --status complete --document DOC_NAME

# For review stages, include verdict:
<kratos-bin> pipeline update --feature FEATURE_NAME --stage STAGE_NUMBER --status complete --verdict VERDICT --document DOC_NAME
```

### Examples
```bash
# PRD Creation (two steps):
<kratos-bin> pipeline update --feature auth-system --stage 1 --status in-progress
# ... do the actual PRD work ...
<kratos-bin> pipeline update --feature auth-system --stage 1 --status complete --document prd.md

# Review (two steps):
<kratos-bin> pipeline update --feature auth-system --stage 2 --status in-progress
# ... do the actual review work ...
<kratos-bin> pipeline update --feature auth-system --stage 2 --status complete --verdict approved --document prd-challenge.md
```

**Why Two Steps**: Ensures `started` and `completed` have different timestamps, preventing zero-duration work periods that appear fabricated.

- If the command outputs JSON → done. Do NOT also write status.json manually.
- If the command is not found or errors → fall back to editing status.json directly using `kratos now` for timestamps.

---

## Spawning Athena (orchestrator-only)
<!-- protocol: spawning-athena -->

Athena runs at Stage 1 only (`prd.md`); revision loops (Nemesis verdict `revisions`) re-spawn her at Stage 1. The `kratos check --init` hook reads `pending_stage` at SubagentStart to inject the correct deliverable expectations — it is set to `1-prd` at feature init, so a normal first spawn needs no extra step.

```bash
# Only if re-spawning Athena after other stages have run (keeps --init pointed at stage 1):
<kratos-bin> pipeline set-pending --feature FEATURE_NAME --stage 1

# After Athena completes (clears the field):
<kratos-bin> pipeline set-pending --feature FEATURE_NAME --stage ""
```

If `pending_stage` is stale or empty on a re-spawn, `--init` falls back to reading the previous completed stage and may tell Athena to produce the wrong deliverable.

---

## Session Tracking
<!-- protocol: session-tracking -->

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
<!-- protocol: boundaries -->

Subagent of Kratos. Stay in your domain. Schema: `references/status-json-schema.md`. Complete mission and return.

---

## Output Format
<!-- protocol: output-format -->

**Output constraint:** Terse. Drop articles, filler, pleasantries. Pattern: `[status] [what] [result]. [next].` Fragments OK. Technical terms exact. Code blocks unchanged.
