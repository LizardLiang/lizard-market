# Decomposition Notion Output Guide

This guide instructs Daedalus on how to create native Notion output for feature decompositions.

---

## Step 1: Load MCP Tools

Use **ToolSearch** to load the required Notion MCP tools:

```
ToolSearch(query: "+Notion create")
ToolSearch(query: "+Notion search")
ToolSearch(query: "+Notion update")
```

Required tools:
- `mcp__plugin_Notion_notion__notion-create-pages` — Create pages and databases
- `mcp__plugin_Notion_notion__notion-create-database` — Create inline databases
- `mcp__plugin_Notion_notion__notion-search` — Find existing pages
- `mcp__plugin_Notion_notion__notion-update-page` — Update page properties

---

## Step 2: Find or Create Parent Location

Search Notion for an existing project page to nest under:

```
mcp__plugin_Notion_notion__notion-search(query: "[project name]")
```

If no suitable parent exists, create the decomposition as a top-level page.

---

## Step 3: Create Parent Page

Create the decomposition parent page: **"[Feature Name] - Decomposition"**

Page content structure:

### Header Section
- **Info Callout** (type: info):
  ```
  📐 Feature Decomposition

  Phases: [N] | Tasks: [N] | Critical Path: Phase [X] → Phase [Y] → Phase [Z]

  [Executive summary - 2-3 sentences on decomposition strategy]
  ```

### Dependency Map
- **Code block** with the ASCII dependency diagram (same as local template)

### Implementation Order
- **Numbered list** with phase order and reasoning:
  ```
  1. Phase 1: [Name] — [Reason this goes first]
  2. Phase 2: [Name] — [Reason]
  3. Phase 3: [Name] — [Reason]
  ```

---

## Step 4: Create Inline Database

Create an inline database on the parent page with these properties:

| Property | Type | Values |
|----------|------|--------|
| **Name** | title | Task name |
| **Phase** | select | Phase 1 (🔵), Phase 2 (🟢), Phase 3 (🟡), Phase 4 (🟠), Phase 5 (🔴), Phase 6 (🟣) |
| **Priority** | select | P0 - Critical, P1 - High, P2 - Medium, P3 - Low |
| **Size** | select | XS (< 30min), S (< 1hr), M (1-4hr), L (4-8hr), XL (8hr+) |
| **Status** | select | Not Started, In Progress, Done, Blocked |
| **Depends On** | relation | (self-relation to same database) |
| **Target Files** | rich_text | File paths affected |

### Color Coding for Phase Select

Assign distinct colors to each phase select option for visual clarity:
- Phase 1: Blue
- Phase 2: Green
- Phase 3: Yellow
- Phase 4: Orange
- Phase 5: Red
- Phase 6: Purple

---

## Step 5: Create Task Rows

For each task in the decomposition, create a database row with:

### Row Properties
```
Name: "[Phase#].[Task#] [Task Name]"
Phase: "Phase [N]"
Priority: "[P0-P3]"
Size: "[XS-XL]"
Status: "Not Started"
Target Files: "[comma-separated file paths]"
```

### Row Page Content

Each task row should have page content with:

1. **Scope Callout** (type: tip / green):
   ```
   ✅ Scope
   - [What this task covers]
   - [Specific deliverable]
   ```

2. **Boundaries Callout** (type: warning / yellow):
   ```
   ⚠️ Boundaries
   - [What is NOT in this task]
   - [Deferred to Phase X]
   ```

3. **Technical Notes Toggle**:
   ```
   ▶ Technical Notes
     - [Important consideration]
     - [Pattern to follow]
   ```

4. **Acceptance Criteria** (to-do list):
   ```
   ☐ [Criterion 1]
   ☐ [Criterion 2]
   ☐ [Criterion 3]
   ```

---

## Step 6: Add Cross-Cutting Concerns

After all task rows, add to the parent page:

### Cross-Cutting Concerns Section
- **Table** with columns: Concern, Affected Phases, Strategy

### Risk Assessment Section
- **Table** with columns: Risk, Probability, Impact, Affected Phases, Mitigation

---

## Step 7: Set Relations

After all rows are created, update the **Depends On** relation for tasks that have dependencies on other tasks.

---

## Output Verification

Before reporting completion, verify:
1. Parent page exists with title "[Feature Name] - Decomposition"
2. Info callout contains accurate phase/task counts
3. Dependency map is present as code block
4. Inline database exists with all properties
5. All task rows are created with correct phase assignments
6. Page content exists on each task row (scope, boundaries, criteria)
7. Relations are set for dependent tasks

---

## Completion Message

```
DAEDALUS COMPLETE (Notion)

Output: Notion page "[Feature Name] - Decomposition"
Phases: [N]
Tasks: [N]
Database: Inline with [N] rows

Notion structure:
📄 [Feature Name] - Decomposition
  ├── 📋 Info callout (summary)
  ├── 💻 Dependency map (code block)
  ├── 📝 Implementation order (list)
  ├── 🗄️ Task database ([N] rows)
  ├── 📊 Cross-cutting concerns (table)
  └── ⚠️ Risk assessment (table)
```
