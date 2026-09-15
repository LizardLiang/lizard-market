# Arena Protocol — Shared Procedures

How Kratos agents read from and write to the Arena, the project's persistent knowledge base at `.claude/.Arena/`. Arena is a **pull system**: agents read what they need, when they need it; nothing is injected automatically.

**Read-only gods** (Apollo, Artemis, Cassandra, Hera, Clio, Hades, Mimir on the codebase side): only § Reading applies to you. Skip the rest.

---

## Reading

1. Read `.claude/.Arena/index.md` first — the registry of every shard. If it does not exist, Arena has not been bootstrapped: proceed without it and do not create Arena files unless your agent definition lists write responsibilities.
2. Read only the shards your task needs. Layout: flat files `glossary.md`, `constraints.md`, `debt.md`; sharded directories `project/`, `architecture/`, `conventions/`, `tech-stack/`, `features/` (digests of completed features), `insights/` (Mimir's cached research, TTL-based), `review-rules/` (Hermes standards; `proposals/` holds unconfirmed drafts), `specs/` (living behavioral specs, see below).
3. Arena information is authoritative. A `conventions/` "never do X" is a rule; an `architecture/` decision is respected in your design; a `features/` precedent argues for consistency.

---

## Writing

Only write to Arena if your agent definition lists explicit write responsibilities, and only to the shards it names.

### Pre-write checklist (also the pruning rules)

1. **Read the target shard.** If the information already exists, skip the write.
2. **Supersede, don't append.** If your entry replaces an existing one on the same topic, replace it. Two entries saying the same thing → keep the newer.
3. **Remove entries whose source feature no longer exists.**
4. **Consolidate above 80 lines.** If `## Entries` exceeds 80 lines, compress related entries before adding.
5. **Never touch `## Permanent`** unless you are Metis (bootstrapping baseline), Athena (requirements/constraints), or Hephaestus (architecture decisions).

### Entry format

Every entry carries evidence: `[YYYY-MM-DD | <agent> | <source>] <content>`, where source is the feature name that produced it or `project-setup` for global decisions.

```markdown
[2026-03-13 | hephaestus | kratos-hooks] hook-commands: binary subcommand, never standalone .cjs
[2026-01-01 | athena | project-setup] auth: never store plaintext passwords
```

### Shard structure

```markdown
# <shard-name>

## Permanent
[decisions intended to outlast any single feature — Metis, Athena, Hephaestus only]

## Entries
[regular entries — subject to pruning and replacement]
```

Flat files (`glossary.md`, `constraints.md`, `debt.md`) are simple dated lists; `constraints.md` entries are implicitly permanent (hard limits with external origin — compliance, legal, SLA — as opposed to `## Permanent`, which holds internal decisions).

Never overwrite a shard wholesale: read, then modify in place or append. A duplicate from two near-simultaneous writers is acceptable; the next writer prunes it.

### New shards and the index

If your entry fits no existing shard, create one at the appropriate path (e.g. `conventions/new-domain.md`) with the standard structure. After any Arena write, update `index.md` last: add the new shard to its table with today's date, or refresh the `Updated` date of the shard you touched.

---

## Behavioral Specs (`specs/`)

`specs/<capability>/spec.md` is the living, capability-organized behavioral contract — the distilled `### Requirement: <Name>` + SHALL statement + scenarios that survive after a feature folder is forgotten. It is not a PRD; the PRD stays in `.claude/feature/<name>/prd.md`.

Features never edit `specs/` directly. Athena (pipeline) or Odysseus (quick path) author a **delta** at `.claude/feature/<name>/spec-delta/<capability>.md`; `kratos spec archive <feature>` merges it mechanically after implementation. Format, requirement identity, ADDED/MODIFIED semantics and merge order are defined in `templates/spec-delta-template.md` and `templates/spec-shard-template.md` — fetch them with `<kratos-bin> template get`. Any agent may read `specs/` for context.

---

## What does NOT belong in Arena

| Item | Where instead |
|------|---------------|
| Feature-specific context | The spawn prompt |
| Full PRD / tech spec | `.claude/feature/<name>/` (only the distilled contract lives in `specs/`) |
| Git history | `git log` |
| Session-specific state | `status.json` |
| Anything derivable from the codebase | The filesystem |
