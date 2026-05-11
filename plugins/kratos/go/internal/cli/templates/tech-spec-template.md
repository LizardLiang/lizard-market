# Technical Specification

## Document Info
| Field | Value |
|-------|-------|
| **Feature** | [Name] |
| **Author** | Hephaestus (Tech Spec Agent) |
| **Status** | Draft |
| **Date** | [Date] |
| **PRD Version** | [Version this is based on] |

---

## 1. Overview

### Summary
[Technical summary of the feature]

### Goals
- [Technical goal 1]
- [Technical goal 2]

### Non-Goals
- [What we're explicitly not building]

---

## 2. Architecture

### System Context
[How this feature fits into the overall system]

### Components
*(Skip this subsection if the feature touches fewer than 3 components)*

| Component | Role |
|-----------|------|
| [Name] | [What it does] |

### Key Design Decisions
| Decision | Rationale | Alternatives Considered |
|----------|-----------|------------------------|
| [Choice] | [Why] | [Other options] |

---

## 3. Data Model
*(Skip this entire section if the feature introduces no new or modified database schema)*

### Schema
```sql
-- Table definitions
CREATE TABLE table_name (
    id UUID PRIMARY KEY,
    ...
);
```

### Migration Strategy
*(Skip if no existing data is affected)*
[How existing data will be migrated]

---

## 4. API Design
*(Skip this entire section if the feature introduces no new or modified endpoints)*

### Endpoint Summary

| Method | Path | Purpose | Auth |
|--------|------|---------|------|
| POST | /api/resource | [What it does] | [Required/None] |
| GET | /api/resource/:id | [What it does] | [Required/None] |

### Detailed Example
*(Document only the most complex endpoint. All others use the summary table above.)*

#### POST /api/resource
**Purpose**: [What it does]

**Request**:
```json
{
  "field": "type"
}
```

**Response**:
```json
{
  "field": "type"
}
```

**Errors**:
| Code | Condition |
|------|-----------|
| 400 | [When] |
| 401 | [When] |

---

## 5. Security Considerations
*(Skip this entire section if the feature reuses existing auth middleware and handles no new sensitive data)*

| Concern | Approach |
|---------|----------|
| Authentication | [How auth is handled] |
| Authorization | [Permission model] |
| Data protection | [Sensitive data handling] |

---

## 6. Performance Considerations
*(Skip this entire section if the PRD has no explicit performance requirements)*

| Concern | Approach |
|---------|----------|
| Expected load | [Traffic estimate] |
| Caching | [Strategy] |
| Key optimizations | [What and why] |

---

## 7. Implementation Plan

### Files to Create
| File | Purpose |
|------|---------|
| [path] | [What it does] |

### Files to Modify
| File | Changes |
|------|---------|
| [path] | [What changes] |

### Sequence of Changes
1. [First change]
2. [Second change]
3. [etc.]

---

## 8. Open Questions

| Question | Status | Resolution |
|----------|--------|------------|
| [Question] | Open/Resolved | [Answer] |
