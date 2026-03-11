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

### Component Diagram
```
[ASCII diagram of components and their relationships]
```

### Key Design Decisions
| Decision | Rationale | Alternatives Considered |
|----------|-----------|------------------------|
| [Choice] | [Why] | [Other options] |

---

## 3. Data Model

### Database Schema
```sql
-- Table definitions
CREATE TABLE table_name (
    id UUID PRIMARY KEY,
    ...
);
```

### Entity Relationships
```
[Entity relationship diagram]
```

### Data Migration
[Migration strategy if modifying existing data]

---

## 4. API Design

### Endpoints

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

### Authentication
[How auth is handled]

### Authorization
[Permission model]

### Data Protection
[Sensitive data handling]

---

## 6. Performance Considerations

### Expected Load
[Traffic estimates]

### Optimization Strategies
- [Strategy 1]
- [Strategy 2]

### Caching
[Caching approach]

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

## 8. Testing Strategy

### Unit Tests
- [What to test]

### Integration Tests
- [What to test]

### E2E Tests
- [What to test]

---

## 9. Rollout Plan

### Feature Flags
[If applicable]

### Rollback Plan
[How to revert if issues]

---

## 10. Open Questions

| Question | Status | Resolution |
|----------|--------|------------|
| [Question] | Open/Resolved | [Answer] |