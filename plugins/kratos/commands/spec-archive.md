---
name: spec-archive
description: Manually promote a feature's spec delta into its living spec — works regardless of pipeline stage
---

# Kratos: Spec Archive

Merge a feature's spec delta(s) (`.claude/feature/<name>/spec-delta/*.md`) into their living spec(s) (`.claude/.Arena/specs/<capability>/spec.md`). This is the manual, anytime path — it does not require Hera to have run. Use it for User Mode features, direct agent runs, or a feature that was abandoned and later resumed.

*"Skipping Hera never strands the delta — it persists as a file until promoted, by this command, the automatic offer after an Aligned verdict, or `kratos spec backfill`."*

---

## Usage

```
/kratos:spec-archive <feature-name>
```

---

## Workflow

### Step 1: Locate the Feature and Its Delta

```bash
<kratos-bin> spec diff <feature-name>
```

This prints each pending delta's capability and its `+`/`~`/`-`/`->` changes. If it reports no delta found, stop and tell the user — nothing to archive.

### Step 2: Validate First

```bash
<kratos-bin> spec validate <feature-name>
```

If validation fails, show the errors and stop. Do not archive an invalid delta.

### Step 3: Confirm With the User

Show the diff from Step 1 and ask for confirmation before merging:

```
Archive spec delta for [feature-name]?
  Capability: [capability]
  Changes: [N] added, [N] modified, [N] removed, [N] renamed

If this feature's stage 8-prd-alignment status is not "complete" with verdict "aligned",
warn explicitly: "No alignment check ran for this feature — archiving promotes the delta
based on Athena's authorship alone."

Proceed? (y/n)
```

### Step 4: Archive

```bash
<kratos-bin> spec archive <feature-name>
```

This merges in the fixed order RENAMED → REMOVED → MODIFIED → ADDED, blocking entirely (no partial merge) on any conflict — a MODIFIED/REMOVED target missing from the living spec, an ADDED requirement that already exists, or a RENAMED pair that collides. On success it moves the delta file to `spec-delta/archived/` (the durable historical record) and updates the living spec's frontmatter (`updated`, `git_hash`).

**Do NOT auto-commit.** Report the changed files so the user can review and commit when ready.

### Step 5: Report

```
SPEC ARCHIVED

Feature: [feature-name]
Capability: [capability]
Changes merged: [N] added, [N] modified, [N] removed, [N] renamed

Living spec updated: .claude/.Arena/specs/[capability]/spec.md
Delta archived: .claude/feature/[feature-name]/spec-delta/archived/[capability].md

Review and commit these changes when ready.
```

---

## Error Handling

**No delta found**: report `.claude/feature/<name>/spec-delta/` is empty or missing, and stop.

**Validation failed**: show the `kratos spec validate` output verbatim, and stop — do not archive.

**Conflict on archive**: `kratos spec archive` reports which capability and which requirement caused the conflict (target missing, or already exists). Nothing is written for that feature — report the conflict and let the user decide whether to fix the delta or the living spec.
