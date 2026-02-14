# Decomposition Linear Output Guide

This guide instructs Daedalus on how to create native Linear output for feature decompositions.

---

## Step 1: Load MCP Tools

Use **ToolSearch** to load the required Linear MCP tools:

```
ToolSearch(query: "+linear create")
ToolSearch(query: "+linear list")
ToolSearch(query: "+linear update")
```

Required tools:
- `mcp__plugin_linear_linear__list_teams` — Discover available teams
- `mcp__plugin_linear_linear__create_project` — Create decomposition project
- `mcp__plugin_linear_linear__create_issue` — Create phase and task issues
- `mcp__plugin_linear_linear__create_issue_label` — Create phase labels
- `mcp__plugin_linear_linear__list_issue_labels` — Check existing labels
- `mcp__plugin_linear_linear__update_issue` — Set relations and blocking

---

## Step 2: Discover Team

List available teams:

```
mcp__plugin_linear_linear__list_teams()
```

- **If one team**: Use it automatically
- **If multiple teams**: Use AskUserQuestion to let user select
- **Record the team ID** for all subsequent operations

---

## Step 3: Create Phase Labels

Check for existing labels first, then create any missing ones:

```
mcp__plugin_linear_linear__list_issue_labels(teamId: "[team-id]")
```

Create phase labels with distinct colors:

| Label Name | Color |
|-----------|-------|
| `phase-1` | `#4285f4` (Blue) |
| `phase-2` | `#34a853` (Green) |
| `phase-3` | `#fbbc04` (Yellow) |
| `phase-4` | `#ea4335` (Orange) |
| `phase-5` | `#9c27b0` (Purple) |
| `phase-6` | `#00bcd4` (Cyan) |

Also create domain labels as needed (e.g., `data-layer`, `api`, `ui`, `auth`, `testing`).

---

## Step 4: Create Project

Create a Linear project to contain the decomposition:

```
mcp__plugin_linear_linear__create_project(
  name: "[Feature Name] Decomposition",
  description: "[Executive summary]\n\nPhases: [N] | Tasks: [N]\nCritical Path: Phase [X] → Phase [Y] → Phase [Z]",
  teamIds: ["[team-id]"]
)
```

Record the project ID.

---

## Step 5: Create Parent Issues (Phases)

For each phase, create a parent issue:

```
mcp__plugin_linear_linear__create_issue(
  title: "Phase [N]: [Phase Name]",
  description: "## Scope\n[What IS in this phase]\n\n## Boundaries\n[What is NOT]\n\n## Dependencies\n[What this depends on / blocks]\n\n## Acceptance Criteria\n- [ ] [Criterion 1]\n- [ ] [Criterion 2]",
  teamId: "[team-id]",
  projectId: "[project-id]",
  labelIds: ["[phase-N-label-id]"]
)
```

Record each phase issue ID for parent linking.

---

## Step 6: Create Sub-Issues (Tasks)

For each task within a phase, create a sub-issue:

```
mcp__plugin_linear_linear__create_issue(
  title: "[Phase#].[Task#] [Task Name]",
  description: "[See description format below]",
  teamId: "[team-id]",
  projectId: "[project-id]",
  parentId: "[phase-issue-id]",
  labelIds: ["[phase-N-label-id]", "[domain-label-id]"],
  estimate: [fibonacci-points]
)
```

### Point Estimates (Fibonacci)

Map effort to Fibonacci points:

| Effort | Points | Description |
|--------|--------|-------------|
| XS | 1 | Trivial change, < 30 min |
| S | 2 | Small task, < 1 hour |
| M | 3 | Medium task, 1-4 hours |
| L | 5 | Large task, 4-8 hours |
| XL | 8 | Very large task, 1-2 days |
| XXL | 13 | Epic-sized, 2+ days |

### Sub-Issue Description Format

```markdown
## Scope
- [What this task covers]
- [Specific deliverable]

## Boundaries
- [What is NOT in this task]
- [Deferred to: Phase X, Task Y]

## Target Files
- `path/to/file1.ts`
- `path/to/file2.ts`

## Technical Notes
- [Important consideration]
- [Pattern to follow]

## Acceptance Criteria
- [ ] [Criterion 1 - specific, testable]
- [ ] [Criterion 2 - specific, testable]
- [ ] [Criterion 3 - specific, testable]
```

---

## Step 7: Set Blocking Relations

After all issues are created, set blocking relationships between phases:

```
mcp__plugin_linear_linear__update_issue(
  id: "[phase-2-issue-id]",
  // Set that Phase 2 is blocked by Phase 1
  // Use the relation fields available in Linear's API
)
```

For each dependency in the decomposition:
1. Find the blocking issue ID (upstream phase)
2. Find the blocked issue ID (downstream phase)
3. Update the blocked issue to reference the blocker

---

## Step 8: Cross-Reference Sibling Issues

In the description of each phase parent issue, add cross-references to related phases by their Linear issue ID:

```markdown
## Related Phases
- Depends on: [LINEAR-ID] Phase 1: [Name]
- Blocks: [LINEAR-ID] Phase 3: [Name]
```

Linear will auto-link these when issue IDs are mentioned.

---

## Output Verification

Before reporting completion, verify:
1. Project exists: "[Feature Name] Decomposition"
2. Phase labels are created
3. All phase parent issues exist with correct labels
4. All sub-issues exist under correct parents
5. Point estimates are set on all sub-issues
6. Blocking relations are established
7. Cross-references are present in phase descriptions

---

## Completion Message

```
DAEDALUS COMPLETE (Linear)

Output: Linear Project "[Feature Name] Decomposition"
Team: [Team Name]
Phases: [N] parent issues
Tasks: [N] sub-issues
Total Points: [sum of estimates]

Linear structure:
📁 Project: [Feature Name] Decomposition
  ├── 🏷️ Phase 1: [Name] ([N] tasks, [N] points)
  │     ├── 1.1 [Task] (2 pts)
  │     ├── 1.2 [Task] (3 pts)
  │     └── 1.3 [Task] (5 pts)
  ├── 🏷️ Phase 2: [Name] ([N] tasks, [N] points)
  │     └── ...
  └── 🏷️ Phase N: [Name] ([N] tasks, [N] points)

Blocking: Phase 1 → Phase 2 → Phase 4
                  → Phase 3 → Phase 4
```
