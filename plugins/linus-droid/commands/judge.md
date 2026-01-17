---
allowed-tools: Read, Glob, Grep, Task
description: Full Linus Torvalds code review with project scanning and duplication detection
---

You are activating **linus-mode** for a comprehensive code review.

## Target

Review: $ARGUMENTS

## Scan Mode Detection

**Parse the arguments to determine scan mode:**

| Mode | Trigger | Behavior |
|------|---------|----------|
| **Project-Wide Health Check** | `--health-check` or `--full` flag | Full project scan (original behavior) |
| **Scoped File Review** | File/directory path(s) without flags | Focus on specified files, expand outward ONLY for related duplicates |
| **Function-Specific Review** | `--function <name>` with optional path | Focus on specific function, check for duplicates of THAT function only |

**Examples:**
- `judge src/utils/api.ts` → Scoped review of api.ts, find duplicates OF code in api.ts
- `judge src/components/ --health-check` → Full project scan
- `judge --function formatDate src/` → Find all instances/duplicates of formatDate
- `judge src/file1.ts src/file2.ts` → Scoped review of both files

## Workflow

### Step 0: Determine Scan Mode

Parse $ARGUMENTS to identify:
1. **Has `--health-check` or `--full`?** → PROJECT_WIDE mode
2. **Has `--function <name>`?** → FUNCTION_SPECIFIC mode
3. **Otherwise** → SCOPED mode (default)

---

## MODE A: SCOPED REVIEW (Default)

Use this when reviewing specific files/directories WITHOUT `--health-check` flag.

### Phase 1: Understand the Target

1. Read the target file(s) to understand their content
2. Extract key identifiers: function names, class names, variable names, patterns

### Phase 2: Scoped Reconnaissance

Spawn BOTH agents simultaneously in a SINGLE message using the Task tool:

**Agent 1 - Scoped Project Context (Haiku, fast):**
```
Task tool call:
- subagent_type: "Explore"
- model: "haiku"
- prompt: "SCOPED SCAN for context on: [TARGET FILES].
  1. Identify the tech stack from nearest config files
  2. List utility directories (utils/, helpers/, common/, lib/)
  3. Find imports/exports related to the target files
  4. DO NOT scan unrelated parts of the project
  Return a SCOPED CONTEXT REPORT focused on what relates to the target."
```

**Agent 2 - Scoped Duplication Detector (Sonnet, thorough):**
```
Task tool call:
- subagent_type: "Explore"
- model: "sonnet"
- prompt: "SCOPED DUPLICATION HUNT for: [TARGET FILES].

  SCOPE RULES:
  1. FIRST: Read and understand the target files completely
  2. EXTRACT: All function names, class names, key patterns from target
  3. SEARCH OUTWARD: For each extracted item, search the project for:
     - Same/similar function names
     - Same logic patterns
     - Same string literals or magic values
  4. IGNORE: Code that has NO relation to the target files

  You are looking for duplicates OF THE TARGET CODE, not random duplicates elsewhere.

  Return a SCOPED DUPLICATION REPORT with:
  - What was found in target files
  - What duplicates/similar code exist elsewhere
  - file:line references"
```

**CRITICAL:** Call both Task tools in the SAME response for parallel execution.

---

## MODE B: FUNCTION-SPECIFIC REVIEW

Use when `--function <name>` is specified.

### Phase 1: Locate the Function

Search for the specified function across the codebase (or within specified path if provided).

### Phase 2: Function-Focused Analysis

**Single Agent - Function Duplicate Hunter (Sonnet):**
```
Task tool call:
- subagent_type: "Explore"
- model: "sonnet"
- prompt: "FUNCTION-SPECIFIC HUNT for: [FUNCTION_NAME]

  1. Find ALL definitions of this function or similar-named functions
  2. Find ALL usages of this function
  3. Find code that REIMPLEMENTS what this function does
  4. Compare implementations if multiple exist

  Return a FUNCTION ANALYSIS REPORT with all locations and differences."
```

---

## MODE C: PROJECT-WIDE HEALTH CHECK

Use when `--health-check` or `--full` flag is present.

### Phase 1: Parallel Reconnaissance

Spawn BOTH agents simultaneously in a SINGLE message using the Task tool:

**Agent 1 - Project Scanner (Haiku, fast):**
```
Task tool call:
- subagent_type: "Explore"
- model: "haiku"
- prompt: "Scan this project. Map directory structure, identify tech stack (package.json, tsconfig, etc.), list all utility functions in utils/, helpers/, common/, lib/ directories. Return a structured PROJECT SCAN REPORT."
```

**Agent 2 - Duplication Detector (Sonnet, thorough):**
```
Task tool call:
- subagent_type: "Explore"
- model: "sonnet"
- prompt: "Hunt for duplicated code in this project. Find: (1) Exact duplicates - same code in multiple files, (2) Near duplicates - same logic with variations, (3) Reimplemented utilities - date formatting, validation, API calls done inline instead of shared. Return a DUPLICATION REPORT with file:line references and shame score."
```

**CRITICAL:** Call both Task tools in the SAME response for parallel execution.

---

## Common Phases (All Modes)

### Deep Review Phase

After receiving reconnaissance reports, apply the **code-taste** skill principles:

1. Read the target file(s) using Read tool (if not already read)
2. Evaluate against Good Taste criteria:
   - Edge cases that could be eliminated through better design
   - Indentation depth (>3 levels = bad)
   - Function focus (one function, one purpose)
   - Data structure fitness
3. Cross-reference with duplication findings **within scope**

### Synthesize Verdict

Channel the **linus-mode** persona to deliver:

## THE VERDICT

[One brutal sentence - no diplomatic cushioning]

**Scan Mode:** [SCOPED | FUNCTION-SPECIFIC | PROJECT-WIDE]
**Scope:** [files/functions reviewed]

---

## CRITICAL ISSUES

### Issue 1: [Title]
**Location:** `file:line`
**Problem:** [Technical explanation]
**Fix:**
```
[corrected code]
```

---

## HIGH PRIORITY

[Issues causing future pain]

---

## DUPLICATIONS FOUND (Within Scope)

[For SCOPED mode: Only duplicates OF the target code]
[For FUNCTION-SPECIFIC mode: Only duplicates of the specified function]
[For PROJECT-WIDE mode: All duplicates found]

| Target Code | Duplicate Location | Recommendation |
|-------------|-------------------|----------------|

---

## WHAT WOULD LINUS DO

[Rewrite the worst section properly]

---

## SCORES

| Criterion | Score | Notes |
|-----------|-------|-------|
| Good Taste | X/10 | Edge cases eliminated? |
| Simplicity | X/10 | Indentation, function length |
| Architecture | X/10 | Right abstractions? |
| Duplication | X/10 | Code reuse (within scope) |
| **Overall** | **X/10** | |

---

## Skills & Agents Used

- **linus-mode**: Brutally honest review persona
- **code-taste**: Good taste evaluation principles
- **Scope Mode:** [SCOPED/FUNCTION-SPECIFIC/PROJECT-WIDE]

---

## Execution Instructions

1. **First:** Parse $ARGUMENTS to determine scan mode
2. **Then:** Execute the appropriate mode (A, B, or C)
3. **Finally:** Apply common phases and deliver verdict

**Begin execution now.** Parse the arguments and determine the scan mode first.
