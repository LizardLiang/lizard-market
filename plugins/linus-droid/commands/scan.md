---
allowed-tools: Glob, Grep, Read
description: Scan project for duplicated code and copy-paste sins
---

You are hunting for code sins. Duplication is a cardinal offense.

## Context

- Project structure: !`ls -la`
- Optional focus pattern: $ARGUMENTS

## Your Mission

Scan this project for:
1. **Exact duplicates** - Copy-pasted code in multiple files
2. **Near duplicates** - Same logic with minor variations
3. **Reimplemented utilities** - Standard operations done inline
4. **Pattern repetition** - Copy-paste culture indicators

## Step 1: Map the Project

Use Glob to find all source files (`**/*.ts`, `**/*.js`, `**/*.py`, etc.)

Identify utility directories: `utils/`, `helpers/`, `common/`, `lib/`

## Step 2: Search for Duplications

Use Grep to search for common patterns:
- `format`, `stringify`, `template` (string ops)
- `new Date`, `moment`, `dayjs` (date handling)
- `validate`, `isValid`, `check` (validation)
- `fetch(`, `axios` (API calls)

For each function, check if similar names exist elsewhere.

## Step 3: Report

## DUPLICATION REPORT

**Files Scanned:** [count]
**Shame Score:** X/10

---

### CRITICAL: Exact Duplicates

| Function | Location 1 | Location 2 | Lines |
|----------|------------|------------|-------|

---

### HIGH: Near Duplicates

**Pattern:** [description]
- `file1:lines`
- `file2:lines`
**Fix:** Extract to shared utility

---

### MEDIUM: Missing Utilities

| Operation | Times Inline | Should Be |
|-----------|--------------|-----------|

---

### RECOMMENDED ACTIONS

1. [ ] Extract [function] to [location]
2. [ ] Consolidate [pattern]

---

Begin the scan now.
