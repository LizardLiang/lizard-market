# Ares — User Mode Procedure

Fetched by Ares with `<kratos-bin> template get ares-user-mode-template` when the mission specifies **User Mode**: Ares writes detailed task files for the user to implement instead of writing the code.

## Step 1: Read the task templates

```bash
<kratos-bin> template get task-file-template
<kratos-bin> template get task-overview-template
```

## Step 2: Read the relevant documents

Same document-selection rules as implementation mode: `pipeline get --compact` for state and summaries; `test-plan.md` for verification goals; `tech-spec.md` only when summaries are not enough; `prd.md` only for requirement context the summaries lack; `decisions.md` / `decomposition.md` only when they affect task structure. A missing required file → stop and name the owning upstream agent.

## Step 3: Create `.claude/feature/<name>/tasks/`

## Step 4: Plan the breakdown

Atomic tasks (completable in one sitting), ordered by dependency, grouped logically. Typical order: data models/types → migrations → service layer → API endpoints/controllers → UI components → tests → configuration.

## Step 5: Write `00-overview.md`

Follow `task-overview-template`: list every task in the Task Index, draw the dependency graph, estimate effort per task, initialize progress tracking.

## Step 6: Write one `XX-descriptive-name.md` per task

Follow `task-file-template`. Every task file must have:

1. A complete, production-ready, copy-paste **Code** section — all imports, all exports, no TODOs, no pseudocode
2. A **Code Explanation** covering every significant section
3. Testable, specific **Acceptance Criteria**

## Step 7: Update pipeline state

```bash
<kratos-bin> pipeline update --feature FEATURE_NAME --stage 7 --status in-progress --mode user
```

Then patch the task list into `status.json` under `pipeline["7-implementation"].tasks` (the CLI has no structured task write; capture `TS=$(<kratos-bin> now)` for any timestamp you write):

```json
"tasks": {
  "total": <N>,
  "completed": 0,
  "items": [
    { "id": "01", "name": "<Task title>", "file": "01-<name>.md", "status": "pending" },
    { "id": "02", "name": "<Task title>", "file": "02-<name>.md", "status": "pending" }
  ]
}
```

## Step 8: Output

```
ARES COMPLETE (User Mode)

Mission: Create Implementation Tasks

Task list:
1. [x] <task — final status>
[... every registered task, with its end state]

Documents:
- tasks/00-overview.md
- tasks/01-<name>.md
- [... every task file]

Task Summary:
- Total tasks: [N]
- Estimated effort: [X hours]
- Dependencies: [summary of task order]

User Instructions:
1. Read .claude/feature/<name>/tasks/00-overview.md
2. Complete tasks in dependency order
3. Mark each complete with /kratos:task-complete <id>; when all done: /kratos:task-complete all

Note: Each task file contains complete, copy-paste ready code.
```
