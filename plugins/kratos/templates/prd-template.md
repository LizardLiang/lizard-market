# Product Requirements Document (PRD)

## Document Info
| Field | Value |
|-------|-------|
| **Feature** | [Name] |
| **Author** | Athena (PM Agent) |
| **Status** | Draft |
| **Date** | [Date] |
| **Version** | 1.0 |

---

## 1. Executive Summary
[2-3 paragraphs: what, why, and impact]

---

## 2. Problem Statement

### Current Situation
[What exists today and why it's insufficient]

### Target Users
| Persona | Description | Primary Need |
|---------|-------------|--------------|
| [User type] | [Who they are] | [What they need] |

### Pain Points
1. [Pain point 1]
2. [Pain point 2]

---

## 3. Goals & Success Metrics

### Business Goals
- [Goal 1]
- [Goal 2]

### Success Metrics
| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| [Metric] | [Baseline] | [Goal] | [How] |

### Out of Scope
- [Not included 1]
- [Not included 2]

---

## 4. Requirements

### P0 - Must Have
| ID | Requirement | User Story | Acceptance Criteria |
|----|-------------|------------|---------------------|
| FR-001 | [Requirement] | As a [user], I want [action] so that [benefit] | Given [context], When [action], Then [result] |

### P1 - Should Have
| ID | Requirement | User Story | Acceptance Criteria |
|----|-------------|------------|---------------------|
| FR-010 | [Requirement] | [Story] | [Criteria] |

### P2 - Nice to Have
| ID | Requirement | User Story | Acceptance Criteria |
|----|-------------|------------|---------------------|
| FR-020 | [Requirement] | [Story] | [Criteria] |

### Non-Functional Requirements
| Category | Requirement |
|----------|-------------|
| Performance | [Requirement] |
| Security | [Requirement] |
| Scalability | [Requirement] |

### Spec Delta
Durable, cross-feature behavioral contract for this feature: `.claude/feature/[name]/spec-delta/[capability].md` (capability: **[capability-slug]**). See `templates/spec-delta-template.md` — do not duplicate the full requirements above here, only the distilled `### Requirement:` + SHALL statement + scenarios that should survive across features.

---

## 5. User Flows

### Primary Flow: [Name]
```
1. User [action]
2. System [response]
3. User [sees/does]
```

### Error Flows
- **[Error]**: [Handling]

---

## 6. Dependencies & Risks

### Dependencies
| Dependency | Type | Impact |
|------------|------|--------|
| [Dependency] | Internal/External | [Impact] |

### Risks
| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| [Risk] | Low/Med/High | Low/Med/High | [Action] |

---

## 7. Open Questions

| Question | Status |
|----------|--------|
| [Question] | Open/Resolved |

---

## 8. External API Dependencies
(Include only if feature involves external integrations)

### [API Name]
| Aspect | Details |
|--------|---------|
| **Library** | [library name] |
| **Version** | [version] |
| **Key Capabilities** | [what we'll use] |
| **Authentication** | [auth method] |
| **Constraints** | [rate limits, quotas] |

---

## 9. External Research Summary
(Include only if Mimir was summoned)

| Topic | Key Finding | Recommended Approach |
|-------|-------------|---------------------|
| [Topic] | [Finding] | [What to use and why] |

**Alternatives considered:** [Alt 1] — [why not chosen]; [Alt 2] — [why not chosen]

---

## 10. Alignment
> *This section is filled in by Hera at Stage 10. Athena leaves it empty.*

**Status:** pending  
**Coverage:** —  
**Verdict:** pending

| Criterion | Description | Test Case(s) | Status |
|-----------|-------------|--------------|--------|
| — | — | — | — |