# Decomposition Template for Local File Output

This template is used by Daedalus when creating `decomposition.md` for a feature.

---

## Template Structure

```markdown
# Feature Decomposition

## Document Info

| Field | Value |
|-------|-------|
| **Feature** | [Name] |
| **Author** | Daedalus (Decomposition Agent) |
| **Date** | [Date] |
| **PRD Version** | [Version or "N/A - raw input"] |
| **Total Phases** | [N] |
| **Total Tasks** | [N] |

---

## Executive Summary

[2-3 paragraphs explaining the decomposition strategy. Why was the feature split this way? What is the critical path? What are the key dependencies between phases? What is the recommended implementation order and why?]

---

## Dependency Map

\`\`\`
[ASCII diagram showing phase relationships]

Example:
  Phase 1 (Data Layer)
      │
      ├──► Phase 2 (Service Layer)
      │        │
      │        ├──► Phase 4 (API Layer)
      │        │        │
      │        │        └──► Phase 5 (UI Layer)
      │        │
      │        └──► Phase 3 (Auth Layer)
      │                 │
      │                 └──► Phase 4 (API Layer)
      │
      └──► Phase 6 (Cross-Cutting: Logging & Monitoring)
\`\`\`

---

## Suggested Implementation Order

| Order | Phase | Reason |
|-------|-------|--------|
| 1 | [Phase Name] | [Why this goes first - e.g., "Foundation layer, no dependencies"] |
| 2 | [Phase Name] | [Why this goes second - e.g., "Depends on data models from Phase 1"] |
| 3 | [Phase Name] | [Reason] |

---

## Phase 1: [Phase Name]

### Scope
What IS included in this phase:
- [Capability 1]
- [Capability 2]
- [Capability 3]

### Boundaries
What is NOT included (deferred to other phases):
- [Excluded item 1] → Phase [N]
- [Excluded item 2] → Phase [N]

### Dependencies

| Relationship | Phase | Description |
|-------------|-------|-------------|
| **Depends on** | [Phase N or "None"] | [What is needed from that phase] |
| **Blocks** | [Phase N] | [What this phase provides to downstream] |

### Tasks

| # | Task | Target Files | Effort |
|---|------|-------------|--------|
| 1.1 | [Task description] | `path/to/file.ts` | S |
| 1.2 | [Task description] | `path/to/file.ts`, `path/to/other.ts` | M |
| 1.3 | [Task description] | `path/to/file.ts` | L |

> Effort: **S** = Small (< 1 hour), **M** = Medium (1-4 hours), **L** = Large (4+ hours)

### Technical Notes
- [Important technical consideration for this phase]
- [Pattern to follow, library to use, constraint to respect]

### Acceptance Criteria
- [ ] [Criterion 1 - specific, testable]
- [ ] [Criterion 2 - specific, testable]
- [ ] [Criterion 3 - specific, testable]

---

## Phase 2: [Phase Name]

[Repeat the same structure as Phase 1 for each subsequent phase]

---

## Cross-Cutting Concerns

| Concern | Affected Phases | Handling Strategy |
|---------|----------------|-------------------|
| [Error Handling] | [1, 2, 4] | [How errors are handled across phases] |
| [Authentication] | [3, 4, 5] | [Auth strategy spanning phases] |
| [Logging] | [All] | [Logging approach] |
| [Testing] | [All] | [Test strategy per phase] |

---

## Risk Assessment

| Risk | Probability | Impact | Affected Phases | Mitigation |
|------|-------------|--------|-----------------|------------|
| [Risk description] | Low/Med/High | Low/Med/High | [Phase numbers] | [How to mitigate] |
| [Risk description] | Low/Med/High | Low/Med/High | [Phase numbers] | [How to mitigate] |
```

---

## Usage Instructions for Daedalus

When creating `decomposition.md` using this template:

1. **Replace all [placeholders]** with actual values
2. **Phase count should be 2-6** — fewer than 2 means decomposition isn't needed, more than 6 means phases are too granular
3. **Tasks should be atomic** — each task should be completable independently within its phase
4. **Effort estimates are relative** — S/M/L, not absolute hours
5. **Acceptance criteria must be testable** — avoid vague criteria like "works correctly"
6. **Dependencies must be explicit** — every phase must declare what it depends on and what it blocks
7. **Boundaries are as important as scope** — clearly state what is NOT in each phase
8. **The dependency map is mandatory** — it gives the big picture at a glance
