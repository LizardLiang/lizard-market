# Arena Protocol — Shared Procedures

This document defines how all Kratos agents read from and write to the Arena memory system. Read the sections relevant to your mission.

---

## What is the Arena

The Arena (`.claude/.Arena/`) is the project's persistent knowledge base. It contains stable, project-wide information that agents need across features and sessions.

Arena is a **pull system** — agents read what they need, when they need it. Nothing is injected automatically.

---

## Reading from Arena

### Step 1: Check the index

Before reading any Arena file, read the index to understand what exists:

```
Read: .claude/.Arena/index.md
```

If the index does not exist, Arena has not been bootstrapped. Skip Arena reads and proceed without it — do not create Arena files unless your agent definition explicitly lists write responsibilities.

### Step 2: Read relevant shards only

Read only the files relevant to your current task. Do not read the entire Arena.

```
.claude/.Arena/
  index.md                  ← always read first
  glossary.md               ← domain terms, naming conventions
  constraints.md            ← hard limits, compliance, security rules
  debt.md                   ← known issues, active workarounds
  project/                  ← project purpose, module overviews
  architecture/             ← system design, component decisions
  conventions/              ← code patterns, "always/never" rules
  tech-stack/               ← languages, frameworks, build commands
  features/                 ← digest of past completed features
  research/                 ← Mimir's cached external research
  review-rules/             ← Hermes review standards and proposals
```

### Step 3: Use what you find

Arena information is authoritative. If a `conventions/` shard says "never do X", follow it. If `architecture/` documents a decision, respect it in your design. If `features/` shows a prior feature used a pattern, consider consistency.

---

## Writing to Arena

Only write to Arena if your agent definition lists explicit write responsibilities. Do not write to Arena shards you do not own.

### Pre-write checklist

Before writing any Arena shard:

1. **Read the target shard** — does the information already exist? If yes, skip the write.
2. **Check for superseded entries** — does your new entry replace an existing one on the same topic? If yes, replace it rather than appending.
3. **Check the line count** — if `## Entries` exceeds 80 lines, consolidate before adding (compress related entries, remove entries whose source feature no longer exists).
4. **Never touch `## Permanent`** — unless you are Athena or Hephaestus writing a permanent rule. All other agents append to `## Entries` only.

### Entry format

Every entry in a shard must include evidence:

```
[YYYY-MM-DD | <agent> | <source>] <content>
```

| Field | Value |
|-------|-------|
| Date | Today's date in YYYY-MM-DD |
| Agent | Your agent name (e.g. `hephaestus`, `hermes`, `ares`) |
| Source | Feature name that produced this entry (e.g. `kratos-hooks`), or `project-setup` for global decisions |

**Example entries:**
```markdown
[2026-03-13 | hephaestus | kratos-hooks] hook-commands: binary subcommand, never standalone .cjs
[2026-03-13 | hermes | payment-feature] error-handling: always wrap external API calls in try/catch
[2026-01-01 | athena | project-setup] auth: never store plaintext passwords
```

### Shard file structure

Every sharded file uses this layout:

```markdown
# <shard-name>

## Permanent
[entries that must never be pruned — written by Athena or Hephaestus only]

## Entries
[regular entries — subject to pruning and replacement]
```

Flat files (`glossary.md`, `constraints.md`, `debt.md`) do not use this split. They use a simple dated list. `constraints.md` entries are all implicitly permanent.

### Append-only

Never overwrite a shard file entirely. Always read first, modify in place or append. If two agents write the same shard near-simultaneously, the result may contain a duplicate entry — this is acceptable and will be pruned by the next writing agent.

### Creating a new shard

If your entry does not fit any existing shard:

1. Create a new file at the appropriate path (e.g. `.claude/.Arena/conventions/new-domain.md`)
2. Use the standard shard structure with `## Permanent` and `## Entries`
3. Add your entry under `## Entries`
4. Update `index.md` — see below

### Permanent entries

Permanent entries are decisions intended to outlast any single feature and never be revisited.

**Who can write to `## Permanent`**: Metis (bootstrapping baseline entries during initial Arena creation), Athena (requirements/constraints origin), and Hephaestus (architectural decisions).

**How to mark permanent**: place the entry under `## Permanent` instead of `## Entries`. No special syntax required beyond the standard evidence format.

**Distinction from `constraints.md`**:
- `constraints.md` — hard limits with external origin (compliance, legal, performance SLA)
- `## Permanent` in a shard — internal team decisions intended to never be revisited

---

## Updating index.md

Always update `index.md` as the last step after any Arena write.

If you created a new shard, add it to the appropriate table in the index with today's date.
If you updated an existing shard, update its `Updated` date.

```markdown
## Project Knowledge
| File | Contents | Updated |
|------|----------|---------|
| project/overview.md | Project purpose, goals, users | 2026-03-13 |
| architecture/api-design.md | API structure, endpoint patterns | 2026-03-13 |
| conventions/hooks.md | Hook architecture decisions | 2026-03-13 |
...
```

---

## Pruning rules

Applied by the writing agent during the pre-write read:

| Condition | Action |
|-----------|--------|
| New entry supersedes existing entry on same topic | Replace existing, do not append |
| Existing entry's source feature no longer exists | Remove |
| Two entries say the same thing | Keep the more recent one |
| `## Entries` exceeds 80 lines | Consolidate before adding |
| Entry is in `## Permanent` | Never touch |

---

## What does NOT belong in Arena

| Item | Why | Where instead |
|------|-----|---------------|
| Package manager | Handled by PreToolUse `fix-pm` hook | Hook |
| Feature-specific context | Orchestrator passes in agent prompt | Agent prompt |
| Full PRD / tech spec | Too large, source of truth elsewhere | `.claude/feature/<name>/` |
| Git history | Read git directly | `git log` |
| Session-specific state | Does not persist usefully | `status.json` |
| Anything derivable from the codebase | Agents can read the filesystem | Filesystem |
