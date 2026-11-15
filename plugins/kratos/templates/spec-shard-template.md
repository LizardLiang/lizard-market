# Spec Shard Template

This is the living, capability-organized behavioral spec format (concepts lifted from OpenSpec). One shard = one capability, stored at `.claude/.Arena/specs/<capability>/spec.md` in the target project.

The `### Requirement: <Name>` header is the durable, cross-feature identity for a requirement — it is matched by trimmed, case-sensitive string equality when a delta targets it. Keep names stable; renaming a requirement means authoring a `RENAMED` entry in a delta, not silently changing the header text.

---

```markdown
---
created: {TIMESTAMP}
updated: {TIMESTAMP}
author: {athena|metis}
git_hash: {HASH}
capability: {capability-slug}
---

# {Capability Name}

## Purpose

{1-3 sentences: what this capability does and why it exists. Written for a reader who has never seen the codebase.}

## Requirements

### Requirement: {Name under 50 chars, unique within this capability}

The system SHALL {precise, testable behavior statement}.

#### Scenario: {short description of the scenario}

- **WHEN** {trigger / action / condition}
- **THEN** {expected observable outcome}
- **AND** {additional expected outcome, if any}

#### Scenario: {another scenario for the same requirement, if needed}

- **WHEN** {trigger}
- **THEN** {outcome}

### Requirement: {Next requirement name}

The system SHALL {behavior statement}.

#### Scenario: {description}

- **WHEN** {trigger}
- **THEN** {outcome}
```

---

## Rules

1. **Header identity**: `### Requirement: <Name>` — the name is the ID. Trimmed, case-sensitive match. Never reuse a name for a different behavior; author a `RENAMED` delta entry instead of editing the header text directly.
2. **Name length**: keep the requirement name under 50 characters. It is a label, not a sentence.
3. **SHALL statement required**: every requirement's body must contain a normative statement using "SHALL" before its first `#### Scenario:` block. This is what `kratos spec validate` checks for.
4. **At least one scenario**: every requirement needs 1+ `#### Scenario:` blocks. A requirement with zero scenarios is unverifiable and fails validation.
5. **Scenario bullets**: use `- **WHEN**`, `- **THEN**`, and optionally `- **AND**` bullets. Keep them short, specific, and testable.
6. **One capability per shard**: do not mix requirements from unrelated capabilities in one file. If a delta targets a capability that doesn't fit any existing shard, a new shard is created (Athena assigns this emergently — see `references/arena-protocol.md`).
7. **Frontmatter is required**: `created`, `updated`, `author`, `git_hash`, `capability` must all be present. `archive` updates `updated` and `git_hash` on every merge.
