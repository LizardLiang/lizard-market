# Spec Delta Template

A spec delta is the durable, file-based record of how one feature changes one capability's living spec. It lives at `.claude/feature/<name>/spec-delta/<capability>.md` and survives independently of pipeline stage progression — it is never lost by skipping a stage (see `references/arena-protocol.md`).

**Critical constraint**: the file must start directly with an operation section header (`## ADDED Requirements`, `## MODIFIED Requirements`, `## REMOVED Requirements`, or `## RENAMED Requirements`). Do **not** put a title heading or any other content before the first operation header — `kratos spec validate` and `kratos spec archive` both parse from the first `##` line.

---

```markdown
## ADDED Requirements

### Requirement: {New requirement name, under 50 chars, unique}

The system SHALL {precise, testable behavior statement}.

#### Scenario: {short description}

- **WHEN** {trigger}
- **THEN** {outcome}

## MODIFIED Requirements

### Requirement: {Exact existing requirement name from the living spec}

The system SHALL {updated behavior statement — this replaces the requirement's body in the living spec}.

#### Scenario: {short description}

- **WHEN** {trigger}
- **THEN** {outcome}

## REMOVED Requirements

### Requirement: {Exact existing requirement name from the living spec}

{Optional: one line explaining why this requirement no longer applies.}

## RENAMED Requirements

- FROM: `{Exact existing requirement name}`
- TO: `{New requirement name}`
```

---

## Rules

1. **No title before the first operation header.** The file starts with `## ADDED Requirements`, `## MODIFIED Requirements`, `## REMOVED Requirements`, or `## RENAMED Requirements` — whichever sections apply. Omit sections with no entries; do not leave an empty `## ADDED Requirements` header with nothing under it.
2. **At least one operation section with at least one entry** — an empty delta fails validation.
3. **No duplicate `### Requirement:` headers** across ADDED/MODIFIED/REMOVED in the same delta file — a requirement can only be the target of one operation per delta.
4. **MODIFIED / REMOVED targets must already exist** in the current living spec (`.claude/.Arena/specs/<capability>/spec.md`) — trimmed, case-sensitive match on the `### Requirement:` header. Targeting a requirement that doesn't exist is a validation error.
5. **ADDED targets must NOT already exist** in the living spec — if it already exists, use MODIFIED instead. This check is against the living spec, not the code: if the capability has no living spec yet or the requirement was never recorded in it, it is ADDED — even for a bug fix to existing behavior.
6. **RENAMED entries** are FROM/TO bullet pairs. FROM must exist in the living spec; TO must not collide with any other requirement name after the rename is applied.
7. **Every ADDED/MODIFIED requirement needs a SHALL statement and ≥1 scenario** — same rules as the spec shard template. `kratos spec validate --strict` promotes missing-scenario/missing-SHALL findings from warnings to errors.
8. **Merge order on archive is fixed**: RENAMED → REMOVED → MODIFIED → ADDED. Conflicts block the entire archive — no partial merge.
