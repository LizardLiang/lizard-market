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
(Include if feature involves external integrations - gathered via context7)

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

This section documents research conducted by Mimir to inform this PRD.

### Research Conducted
| Topic | Source | Key Finding |
|-------|--------|-------------|
| [Topic researched] | Mimir (GitHub, docs, web) | [Summary of findings] |
| [Topic 2] | context7 API docs | [API details] |

### Recommended Approach
[Based on Mimir's research, the recommended implementation approach]

**Why this approach:**
- [Reason 1 from research]
- [Reason 2 from research]

**Alternatives considered:**
- [Alternative 1] - [Why not chosen]
- [Alternative 2] - [Why not chosen]

### Cached Insights
[If Mimir cached any research]
- `.claude/.Arena/insights/[topic]-[date].md` - [What it contains]

---

## 10. Requirements Analysis (Appendix)

This section documents the analytical process used to gather requirements.

### Gaps Identified During Analysis
| Area | Gap Identified | Resolution |
|------|----------------|------------|
| [Category] | [What was missing from initial requirement] | User clarified / Assumption made / Open question |

### Assumptions Made
| Assumption | Basis | Risk if Wrong |
|------------|-------|---------------|
| [What we assumed] | [Why this seemed reasonable] | [What could go wrong] |

### Open Questions
| Question | Impact if Unresolved | Owner |
|----------|---------------------|-------|
| [Unanswered question] | [What could fail or need rework] | [Who should answer] |

### Requirements Completeness
- **Initial requirement detail level**: Sparse / Moderate / Detailed
- **Questions asked**: [N] questions across [M] rounds
- **Gaps filled**: [X] of [Y] identified gaps resolved
- **Confidence level**: Low / Medium / High