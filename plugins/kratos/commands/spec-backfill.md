---
name: spec-backfill
description: Migrate pre-existing shipped features into living specs (one-time or occasional sweep)
---

# Kratos: Spec Backfill

Generate living specs from features that shipped **before** the spec-lifecycle feature existed — they have no `spec-delta/` and never went through `kratos spec archive`. This is a one-time (or occasional) migration, not part of the normal per-feature flow.

*"Run this once after adopting the spec-lifecycle feature on an established project, so the living spec layer isn't empty just because it started late."*

---

## Usage

```
/kratos:spec-backfill
```

No arguments — it scans the whole project.

---

## Workflow

### Step 1: Run Backfill

```bash
<kratos-bin> spec backfill
```

This scans `.claude/feature/*/status.json` for features whose `8-prd-alignment` stage has verdict `aligned` (shipped), reads each one's `prd.md`, extracts requirements from its P0/P1/P2 tables, and merges them into `.claude/.Arena/specs/<capability>/spec.md` — creating the shard if it doesn't exist yet.

**Idempotent**: re-running adds zero new requirements the second time (deduplicated by requirement name within each capability). Safe to run repeatedly.

**User-initiated, no auto-commit**: this never runs automatically — only when the user explicitly invokes it. Report the changed files so the user can review and commit when ready.

### Step 2: Known Limitation — Capability Grouping

Backfill has no semantic signal to cluster pre-existing features into shared capabilities the way Athena does when authoring a delta for a new feature. It falls back to **using each feature's name as its own capability slug**. If several backfilled features actually describe one capability, tell the user they may want to manually consolidate the resulting `specs/<capability>/` directories afterward — this is not automated.

### Step 3: Report

```
SPEC BACKFILL COMPLETE

Scanned: [N] aligned feature(s)

Capabilities created/updated:
  [capability-1]: +[N] requirement(s)
  [capability-2]: +[N] requirement(s)
  ...

Note: capability grouping defaults to one-capability-per-feature (no semantic
clustering for pre-existing features). Consolidate manually if desired.

Review .claude/.Arena/specs/ and commit when ready.
```

If nothing was scanned (no aligned features with a readable `prd.md`), report that plainly — there is nothing to backfill yet.

---

## When to Use

- Once, right after adopting the spec-lifecycle feature on a project with existing shipped features.
- Occasionally, if features somehow shipped (`aligned` verdict) without ever going through `kratos spec archive` — this sweeps them up rather than leaving the living spec layer permanently missing that history.

Not needed for features that go through the normal pipeline: those get a `spec-delta/` from Athena and are promoted via `kratos spec archive` (auto-offered on Hera `aligned`, or `/kratos:spec-archive`).
