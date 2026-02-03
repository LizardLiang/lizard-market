# Task Template for User Mode Implementation

This template is used by Ares when creating task files for User Mode implementation.

---

## Template Structure

```markdown
# Task [ID]: [Task Title]

## Document Info

| Field | Value |
|-------|-------|
| **Task ID** | [XX] |
| **Target File** | `[file path]` |
| **Action** | Create / Modify / Delete |
| **Estimated Effort** | [X minutes/hours] |
| **Dependencies** | [Task IDs or "None"] |
| **Status** | Pending / In Progress / Complete |

---

## What to Do

[Clear, concise description of what needs to be accomplished. 2-3 sentences max.]

---

## Why Do It This Way

[Explain the reasoning behind this approach. Reference tech-spec decisions, architectural patterns, or best practices that inform this implementation.]

---

## How to Do It

### Step 1: [Action]
[Detailed instruction]

### Step 2: [Action]
[Detailed instruction]

### Step 3: [Action]
[Detailed instruction]

[Continue as needed...]

---

## Code

> **Copy-paste ready.** This code is complete and production-ready.

\`\`\`[language]
[COMPLETE implementation code - not pseudocode, not partial snippets]
[Include all imports, all functions, all exports]
[This should be directly copy-pasteable into the target file]
\`\`\`

---

## Code Explanation

### [Section/Function Name]

\`\`\`[language]
[Relevant code snippet]
\`\`\`

**What it does:** [Explanation]

**Why:** [Reasoning]

### [Next Section/Function Name]

[Continue for all significant sections...]

---

## Acceptance Criteria

- [ ] [Criterion 1 - specific, testable]
- [ ] [Criterion 2 - specific, testable]
- [ ] [Criterion 3 - specific, testable]
- [ ] Code compiles/runs without errors
- [ ] All existing tests still pass
- [ ] New tests pass (if applicable)

---

## Testing This Task

### Manual Testing

1. [Step-by-step manual test procedure]
2. [Expected result]

### Automated Tests

[Reference to test file or test commands to run]

\`\`\`bash
[test command]
\`\`\`

---

## Common Issues

| Issue | Solution |
|-------|----------|
| [Common problem 1] | [How to fix] |
| [Common problem 2] | [How to fix] |

---

## Related Tasks

| Task | Relationship |
|------|--------------|
| [Task ID] | [blocks this / blocked by this / related] |

---

## Notes

[Any additional context, warnings, or tips for the implementer]
```

---

## Usage Instructions for Ares

When creating task files using this template:

1. **Replace all [placeholders]** with actual values
2. **Code section is MANDATORY** - must be complete, copy-paste ready
3. **Code Explanation is MANDATORY** - break down every significant section
4. **Acceptance Criteria must be testable** - avoid vague criteria
5. **File naming**: `XX-descriptive-name.md` where XX is zero-padded number

### Example File Name

```
01-create-user-model.md
02-add-authentication-middleware.md
03-implement-login-endpoint.md
```

### Quality Checklist

Before finalizing a task file:

- [ ] Code is COMPLETE (not partial/pseudocode)
- [ ] All imports included
- [ ] All exports included
- [ ] No TODO comments in code
- [ ] Code follows project conventions
- [ ] Explanation covers all significant sections
- [ ] Acceptance criteria are specific and testable
- [ ] Dependencies are correctly identified
