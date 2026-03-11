# Test Plan

## Document Info
| Field | Value |
|-------|-------|
| **Feature** | [Name] |
| **Author** | Artemis (QA Agent) |
| **Date** | [Date] |
| **PRD Version** | [Version] |
| **Tech Spec Version** | [Version] |

---

## 1. Test Overview

### Scope
[What this test plan covers]

### Out of Scope
[What is not tested and why]

### Test Approach
[Overall testing strategy]

---

## 2. Requirements Coverage Matrix

| Req ID | Requirement | Test Cases | Priority |
|--------|-------------|------------|----------|
| FR-001 | [Requirement] | TC-001, TC-002 | P0 |
| FR-002 | [Requirement] | TC-003 | P1 |

---

## 3. Test Cases

### Unit Tests

#### TC-001: [Test Name]
| Field | Value |
|-------|-------|
| **Requirement** | FR-001 |
| **Type** | Unit |
| **Priority** | P0 |

**Preconditions**:
- [Condition 1]

**Test Steps**:
1. [Step 1]
2. [Step 2]

**Expected Result**:
- [Expected outcome]

**Edge Cases**:
- [Edge case 1]: [Expected behavior]

---

### Integration Tests

#### TC-010: [Integration Test Name]
| Field | Value |
|-------|-------|
| **Requirement** | FR-001, FR-002 |
| **Type** | Integration |
| **Priority** | P0 |

**Components Tested**:
- [Component 1]
- [Component 2]

**Preconditions**:
- [Condition]

**Test Steps**:
1. [Step]

**Expected Result**:
- [Result]

---

### API Tests

#### TC-020: [API Test Name]
| Field | Value |
|-------|-------|
| **Endpoint** | POST /api/resource |
| **Type** | API |
| **Priority** | P0 |

**Request**:
```json
{
  "field": "value"
}
```

**Expected Response**:
```json
{
  "status": "success"
}
```

**Error Cases**:
| Input | Expected Code | Expected Message |
|-------|--------------|------------------|
| [Invalid input] | 400 | [Error message] |

---

### E2E Tests

#### TC-030: [E2E Test Name]
| Field | Value |
|-------|-------|
| **User Flow** | [Flow name] |
| **Type** | E2E |
| **Priority** | P0 |

**Scenario**:
```gherkin
Given [precondition]
When [action]
Then [result]
```

---

## 4. Edge Cases & Boundaries

| Category | Test Case | Input | Expected |
|----------|-----------|-------|----------|
| Boundary | Min value | [value] | [result] |
| Boundary | Max value | [value] | [result] |
| Invalid | Empty input | "" | [error] |
| Invalid | Null | null | [error] |
| Edge | Special chars | [chars] | [result] |

---

## 5. Security Tests

| Test | Description | Expected |
|------|-------------|----------|
| Auth bypass | Attempt unauthorized access | 401/403 |
| Injection | SQL/XSS attempts | Sanitized/blocked |
| Rate limit | Excessive requests | 429 |

---

## 6. Performance Tests

| Test | Scenario | Threshold |
|------|----------|-----------|
| Response time | Normal load | < 200ms |
| Throughput | Peak load | > 100 req/s |
| Memory | Extended use | < 512MB |

---

## 7. Test Data Requirements

| Data Set | Purpose | Source |
|----------|---------|--------|
| [Dataset] | [What for] | [Where from] |

---

## 8. Test Environment

| Environment | Purpose | Config |
|-------------|---------|--------|
| Unit | Local testing | [Config] |
| Integration | CI/CD | [Config] |
| Staging | Pre-prod | [Config] |

---

## 9. Acceptance Criteria Verification

| AC ID | Acceptance Criteria | Test Cases | Pass Criteria |
|-------|--------------------|-----------:|---------------|
| AC-001 | [Criteria from PRD] | TC-001 | [How to verify] |

---

## 10. Test Summary

| Type | Count | P0 | P1 | P2 |
|------|-------|----|----|----|
| Unit | [N] | [N] | [N] | [N] |
| Integration | [N] | [N] | [N] | [N] |
| API | [N] | [N] | [N] | [N] |
| E2E | [N] | [N] | [N] | [N] |
| **Total** | [N] | [N] | [N] | [N] |
