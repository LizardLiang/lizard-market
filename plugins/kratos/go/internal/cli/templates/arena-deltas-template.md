# Arena Deltas for Feature: {feature-name}

**Base Arena Hash**: {git-hash}  
**Feature Branch**: {branch-name}  
**Created**: {timestamp}  
**Last Updated**: {timestamp}

---

## Purpose

This file captures feature-specific discoveries and changes that are NOT yet in the Master Arena. After this feature merges to main, these deltas will be integrated into the Master Arena documents.

---

## External Research (Athena + Mimir)

**Research Conducted**:
- {topic} researched via {source}
- Key findings: {summary}

**Cached Insights**:
- `.claude/.Arena/insights/{topic}-{date}.md` - {description}

---

## Codebase Discoveries (Hephaestus)

**New Directories**:
- `{path}/` - {purpose}

**New Files**:
- `{path}/file.ts` - {purpose}

**Dependencies Added**:
- `{package}@{version}` - {purpose}

**Architecture Changes**:
- {description of new patterns or services}

**Source References**:
- See tech-spec.md section X for details

---

## Architecture Validation (Apollo)

**Patterns Verified**:
- {pattern} follows existing {convention}

**Notes**:
- {any architectural concerns or recommendations}

---

## Implementation Details (Ares)

**Files Created**:
| File | Purpose | Status |
|------|---------|--------|
| {path} | {description} | Done |

**Files Modified**:
| File | Changes | Status |
|------|---------|--------|
| {path} | {description} | Done |

**Key Implementation Notes**:
- {important implementation details}
- {any deviations from tech spec}

---

## Code Review Notes (Hermes)

**Quality Assessment**:
- Code quality: {assessment}
- Test coverage: {percentage}%
- Security review: {passed/concerns}

**Ready for Integration**:
- [x] All code complete
- [x] Tests passing
- [x] No security issues
- [x] Follows conventions

---

## Integration Checklist

When integrating these deltas into Master Arena:

### tech-stack.md
- [ ] Add new dependencies: {list}
- [ ] Update versions: {list}

### architecture.md
- [ ] Document new services: {list}
- [ ] Update component diagram
- [ ] Add new patterns: {list}

### file-structure.md
- [ ] Add new directories: {list}
- [ ] Update key files list

### conventions.md
- [ ] Document any new conventions discovered
- [ ] Note any exceptions to existing patterns

### project-overview.md
- [ ] Update if feature significantly changes project scope

---

## Conflicts Resolved

If this delta contradicted Master Arena, document here:

| Arena Claimed | Delta Found | Resolution |
|---------------|-------------|------------|
| {old value} | {new value} | {how resolved} |

---

## Notes

- This file is temporary and will be deleted after integration
- Master Arena remains read-only during feature development
- Agents read: Master Arena + This Delta = Combined View
