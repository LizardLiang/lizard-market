# Code Review

## Document Info
| Field | Value |
|-------|-------|
| **Feature** | [Name] |
| **Reviewer** | Hermes (Code Review Agent) |
| **Date** | [Date] |
| **Verdict** | Approved / Changes Required |

---

## Review Summary
[Overall assessment]

---

## Files Reviewed

| File | Lines | Status | Issues |
|------|-------|--------|--------|
| [path] | [N] | Pass/Fail | [N] |

---

## Correctness Review

### Spec Compliance
| Spec Item | Implementation | Status |
|-----------|---------------|--------|
| [Item from tech-spec] | [What was done] | Pass/Fail |

### Requirements Coverage
| Requirement | Implemented | Tested | Status |
|-------------|-------------|--------|--------|
| FR-001 | Yes/No | Yes/No | Pass/Fail |

---

## Code Quality

### Strengths
- [Strength 1]
- [Strength 2]

### Issues Found

#### Critical Issues (Must Fix)
| File:Line | Issue | Recommendation |
|-----------|-------|----------------|
| [location] | [Problem] | [Fix] |

#### Major Issues (Should Fix)
| File:Line | Issue | Recommendation |
|-----------|-------|----------------|
| [location] | [Problem] | [Fix] |

#### Minor Issues (Consider)
| File:Line | Issue | Recommendation |
|-----------|-------|----------------|
| [location] | [Problem] | [Fix] |

---

## Testing Review

### Test Coverage
| Type | Expected | Actual | Status |
|------|----------|--------|--------|
| Unit | [N] | [N] | Pass/Fail |
| Integration | [N] | [N] | Pass/Fail |
| E2E | [N] | [N] | Pass/Fail |

### Test Quality
- **Assertions**: [Adequate/Insufficient]
- **Edge Cases**: [Covered/Missing]
- **Mocking**: [Appropriate/Excessive]

### Test Results
```
[Test output]
```

---

## Security Review

| Check | Status | Notes |
|-------|--------|-------|
| Input Validation | Pass/Fail | [Notes] |
| Authentication | Pass/Fail | [Notes] |
| Authorization | Pass/Fail | [Notes] |
| Data Protection | Pass/Fail | [Notes] |
| Injection Prevention | Pass/Fail | [Notes] |

---

## Performance Review

| Check | Status | Notes |
|-------|--------|-------|
| Query Efficiency | Pass/Fail | [Notes] |
| Resource Usage | Pass/Fail | [Notes] |
| Caching | Pass/Fail | [Notes] |
| Async Operations | Pass/Fail | [Notes] |

---

## Summary

### Issues by Severity
| Severity | Count |
|----------|-------|
| Critical | [N] |
| Major | [N] |
| Minor | [N] |

### Overall Metrics
| Metric | Value |
|--------|-------|
| Files Reviewed | [N] |
| Lines of Code | [N] |
| Test Coverage | [%] |
| Issues Found | [N] |

---

## Verdict

**[APPROVED / CHANGES REQUIRED]**

Record it with `<kratos-bin> pipeline update --stage 9 --verdict approved|changes-required`.

### Approved
Code meets quality standards and is ready for merge.

### Changes Required
Code needs the following fixes before approval:
1. [Required change 1]
2. [Required change 2]

---

## Next Steps

- [ ] Address critical issues
- [ ] Address major issues
- [ ] Re-run tests
- [ ] Request re-review
