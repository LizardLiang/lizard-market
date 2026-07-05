---
name: spec-view
description: View living specs — list all capabilities with requirement counts, render one capability's spec, and surface pending spec deltas
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

> The `KRATOS_ROOT` value echoed above is the plugin's absolute root — substitute it for every `<KRATOS_ROOT>` reference below (fallback: `plugins/kratos/` from project root). `<kratos-bin>` resolves to `<KRATOS_ROOT>/bin/kratos`, falling back to `~/.kratos/bin/kratos`.

# Kratos: Spec View

Display the project's living behavioral specs (`.claude/.Arena/specs/<capability>/spec.md`) and any pending (un-archived) spec deltas. **This command is read-only** — it never writes anything. To change specs, use `/kratos:spec-archive` (promote a feature's delta) or `/kratos:spec-backfill` (migrate pre-existing shipped features).

*"The Arena remembers what the system SHALL do. I merely read its record aloud."*

---

## Usage

```
/kratos:spec-view                 # overview: all capabilities + pending deltas
/kratos:spec-view <capability>    # render one capability's living spec
```

---

## Workflow

### No argument: Overview

Run both:

```bash
<kratos-bin> spec list             # living capability shards + requirement counts
<kratos-bin> spec list --changes   # pending un-archived deltas across all features
```

Render the Overview format below from the combined output.

### With argument: Single Capability

```bash
<kratos-bin> spec show <capability>
```

Render the Single Capability format below: summarize the frontmatter (capability, author, updated, git_hash), then present the Purpose and Requirements sections faithfully — do not paraphrase or reorder requirements.

If it errors with "no living spec for capability", the argument may be a **feature** with a pending delta instead — try:

```bash
<kratos-bin> spec diff <argument>
```

If that shows a delta, render it as the Pending Delta view. If both come up empty, report not found and list the available capabilities from `spec list`.

### Fallback (binary unavailable)

Only if `<kratos-bin>` is missing:

1. **List**: read each `.claude/.Arena/specs/*/spec.md`; the requirement count is the number of `### Requirement:` headers in the shard.
2. **Pending deltas**: glob `.claude/feature/*/spec-delta/*.md` — exclude anything under `spec-delta/archived/`.
3. **Show**: read `.claude/.Arena/specs/<capability>/spec.md` directly.

---

## Output Format

### Overview

```
⚔️ KRATOS: THE ARENA'S RECORD ⚔️

Living Specs (.claude/.Arena/specs/):
┌──────────────────────────────┬──────────────┐
│ Capability                   │ Requirements │
├──────────────────────────────┼──────────────┤
│ auth-system                  │ 12           │
│ billing                      │ 5            │
└──────────────────────────────┴──────────────┘

Pending Spec Deltas (not yet archived):
│ user-auth            → capability: auth-system
│ oauth-integration    → capability: auth-system

💡 View one spec: /kratos:spec-view <capability>
💡 Promote a delta: /kratos:spec-archive <feature>
```

Omit the Pending Spec Deltas section entirely when there are none.

### Single Capability

```
⚔️ KRATOS: LIVING SPEC — [capability] ⚔️

Author: [author]   Updated: [updated]   Git: [git_hash]

## Purpose
[purpose text verbatim]

## Requirements ([N])

### Requirement: [Name]
[SHALL statement and scenarios, verbatim from the shard]

...
```

### Pending Delta (argument matched a feature, not a capability)

```
⚔️ KRATOS: PENDING SPEC DELTA — [feature] ⚔️

No living spec named "[feature]" — but this feature has an un-archived delta:

capability: [capability]
  + [added requirement]
  ~ [modified requirement]
  - [removed requirement]
  -> [old name] => [new name]

💡 Promote it: /kratos:spec-archive [feature]
```

### Empty View

```
⚔️ KRATOS: THE ARENA'S RECORD ⚔️

No living specs found.

The Arena holds no record yet. Specs appear when:
> A feature ships through the pipeline and archives its spec delta
> You run /kratos:spec-backfill to migrate pre-existing shipped features
```

---

## Kratos's Voice

Report with clarity and authority — the specs are the system's contract, read them faithfully:
- **Verbatim**: requirement names and SHALL statements are durable identities; never paraphrase them
- **Actionable**: point to `/kratos:spec-archive` or `/kratos:spec-backfill` when the user's next step is a mutation

**Note:** Spec dashboards use emoji as visual indicators. This is a functional exception to the "no emoji unless requested" rule.

*"I see all. The Arena's record is open to me."*

---

**Reading the Arena's record now...**
