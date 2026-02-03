# Task Overview Template for User Mode Implementation

This template is used by Ares when creating the 00-overview.md file for User Mode implementation.

---

## Template Structure

```markdown
# Implementation Tasks: [Feature Name]

## Feature Summary

| Field | Value |
|-------|-------|
| **Feature** | [Feature Name] |
| **Created** | [Date] |
| **Total Tasks** | [N] |
| **Estimated Total Effort** | [X hours] |
| **Tech Spec** | [Link to tech-spec.md] |
| **Test Plan** | [Link to test-plan.md] |

---

## Task Index

| # | Task | Target File | Effort | Dependencies | Status |
|---|------|-------------|--------|--------------|--------|
| 01 | [Task title] | `[file]` | [X min] | - | Pending |
| 02 | [Task title] | `[file]` | [X min] | 01 | Pending |
| 03 | [Task title] | `[file]` | [X min] | 01, 02 | Pending |
| ... | ... | ... | ... | ... | ... |

---

## Dependency Graph

\`\`\`
[ASCII diagram showing task dependencies]

Example:
    ┌─────┐
    │ 01  │  (Create data models)
    └──┬──┘
       │
   ┌───┴───┐
   │       │
┌──▼──┐ ┌──▼──┐
│ 02  │ │ 03  │  (Service layer)
└──┬──┘ └──┬──┘
   │       │
   └───┬───┘
       │
    ┌──▼──┐
    │ 04  │  (API endpoints)
    └──┬──┘
       │
    ┌──▼──┐
    │ 05  │  (Tests)
    └─────┘
\`\`\`

---

## Estimated Total Effort

| Category | Tasks | Time |
|----------|-------|------|
| Data Layer | 01-02 | [X hours] |
| Business Logic | 03-04 | [X hours] |
| API Layer | 05-06 | [X hours] |
| Tests | 07-08 | [X hours] |
| **Total** | **[N]** | **[X hours]** |

---

## How to Use These Tasks

### Recommended Workflow

1. **Read the overview** (this file) to understand the full scope
2. **Check dependencies** before starting any task
3. **Complete tasks in order** when dependencies exist
4. **Mark tasks complete** using `/kratos:task-complete <id>`
5. **Run tests** after completing related tasks

### Commands

| Command | Description |
|---------|-------------|
| \`/kratos:task-complete 01\` | Mark task 01 as complete |
| \`/kratos:task-complete 01 02 03\` | Mark multiple tasks complete |
| \`/kratos:task-complete all\` | Mark all complete → triggers code review |
| \`/kratos:status\` | Check overall feature progress |

### Task File Structure

Each task file (\`XX-name.md\`) contains:
- **What to Do** - Clear description
- **Why Do It This Way** - Reasoning
- **How to Do It** - Step-by-step guide
- **Code** - Complete, copy-paste ready implementation
- **Code Explanation** - Line-by-line breakdown
- **Acceptance Criteria** - Checklist for verification

---

## Related Documents

| Document | Purpose |
|----------|---------|
| [prd.md](../prd.md) | Product requirements |
| [tech-spec.md](../tech-spec.md) | Technical specification |
| [test-plan.md](../test-plan.md) | Test coverage plan |

---

## Progress Tracking

### Current Status

| Status | Count | Tasks |
|--------|-------|-------|
| Pending | [N] | 01, 02, 03, ... |
| In Progress | [N] | - |
| Complete | [N] | - |

### Completion Progress

\`\`\`
[░░░░░░░░░░░░░░░░░░░░] 0% (0/N tasks)
\`\`\`

---

## Notes

[Any important context about the implementation order, gotchas, or tips]
```

---

## Usage Instructions for Ares

When creating the overview file:

1. **File name is always** `00-overview.md`
2. **Task Index must be complete** - list ALL tasks
3. **Dependency Graph is required** - helps visualize order
4. **Effort estimates should be realistic** - not optimistic
5. **Progress Tracking** - initialize with all tasks as Pending

### Quality Checklist

Before finalizing the overview:

- [ ] All tasks from tech-spec are represented
- [ ] Dependencies correctly identified
- [ ] Effort estimates are reasonable
- [ ] Dependency graph accurately reflects task order
- [ ] Commands section is accurate
- [ ] Links to related documents work
