---
allowed-tools: Read, Glob, Grep, Task, Bash
description: Review staged changes before committing - catch issues before they enter history
---

You are activating **linus-mode** to review staged changes BEFORE commit.

## Context

- Staged files: !`git diff --name-only --cached`
- Staged diff: !`git diff --cached --stat`
- Current branch: !`git branch --show-current`

## Purpose

Review what's about to be committed. Catch issues BEFORE they enter git history.

## Workflow

### Phase 1: Get Staged Changes

Use Bash:

```bash
# List staged files
git diff --name-only --cached

# Get the actual diff
git diff --cached
```

### Phase 2: Parallel Agent Scan

**Agent 1 - Project Scanner (Haiku):**
```
Task(subagent_type: "Explore", model: "haiku"):
"Quick project scan. Understand where staged files fit. List existing utilities."
```

**Agent 2 - Duplication Detector (Sonnet):**
```
Task(subagent_type: "Explore", model: "sonnet"):
"Check if staged changes duplicate existing code. Focus on: [staged files]. Find similar patterns elsewhere."
```

### Phase 3: Review Staged Diff

For each staged file:
1. Read the current file content
2. Analyze the diff (what's being added/changed)
3. Apply **code-taste** principles
4. Check for new duplications

### Phase 4: Pre-Commit Verdict

## PRE-COMMIT REVIEW

**Branch:** [current branch]
**Staged Files:** [count]

---

## THE VERDICT

[Should this be committed as-is?]

---

## STAGED CHANGES

| File | Status | Lines +/- | Assessment |
|------|--------|-----------|------------|
| file.ts | Modified | +30/-5 | [brief] |

---

## ISSUES TO FIX BEFORE COMMIT

### Issue 1: [Title]
**File:** `file:line`
**Problem:** [what's wrong]
**Fix:**
```
[corrected code]
```

---

## DUPLICATIONS DETECTED

[Will this commit introduce duplicated code?]

---

## PRE-COMMIT SCORES

| Criterion | Score | Notes |
|-----------|-------|-------|
| Ready to Commit | X/10 | |
| Good Taste | X/10 | |
| No New Duplication | X/10 | |
| **Overall** | **X/10** | |

---

## RECOMMENDATION

[ ] **COMMIT** - Ready to go
[ ] **AMEND** - Minor fixes needed first
[ ] **UNSTAGE** - Significant issues, fix before staging

---

Begin. Get staged changes and review them.
