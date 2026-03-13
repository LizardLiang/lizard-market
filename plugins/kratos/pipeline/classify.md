---
name: classify
description: Intent classification for new user requests — determines which Kratos command to route to
---

# Task Classification

When the user provides a **new request** (not "continue" or "status"), classify intent before doing anything else. The classification determines which pipeline to activate.

---

## Intent Categories

### RECALL
Route to `/kratos:recall`

User is asking about previous work or session state:
- "Where did we stop?" / "What were we working on?"
- "Last session" / "Resume from last time"
- "Show me my progress" / "Status of my last feature"

### INQUIRY
Route to `/kratos:inquiry`

User wants information — not to build anything:
- **Project understanding**: "What does this project do?", "Explain the architecture"
- **Git history**: "What changed recently?", "Who wrote X?", "Git blame"
- **Tech stack**: "What libraries?", "What version of X?", "Dependencies"
- **Docs lookup**: "Find docs for X", "API reference for Y"
- **Codebase exploration**: "Find where X is defined", "List all endpoints"
- **Best practices**: "How do others implement X?", "GitHub examples of Y"

### DECOMPOSITION
Route to `/kratos:decompose`

User wants to break something down without building it:
- "Decompose", "break down", "split into tasks/phases"
- "Work breakdown", "break into phases"

### SIMPLE
Route to `/kratos:quick`

Focused, single-purpose task on existing code:
- File/function + action (fix, test, refactor a specific thing)
- "Add tests for X" / "Fix the bug in Y" / "Review this code"
- "Add docs to Z" / "Understand how X works"

### COMPLEX
Use the full pipeline (continue in `main.md`)

Substantial new functionality requiring design:
- "Build", "create", "new feature" for multi-component work
- API or database design needed
- Security-sensitive changes (auth, encryption, permissions)
- Vague/broad scope ("improve the app")
- Requires PRD-level requirements discussion

### UNCLEAR
Use `AskUserQuestion` with these options:
- "Information request (inquiry mode)"
- "Quick task (direct agent)"
- "Full feature pipeline (PRD → Tech Spec → Implementation)"

---

## Examples

| User Request | Classification |
|---|---|
| "Where did we stop last time?" | RECALL |
| "What does this project do?" | INQUIRY |
| "Who wrote the auth module?" | INQUIRY |
| "Best way to implement caching?" | INQUIRY |
| "Break down the auth system into phases" | DECOMPOSITION |
| "Add unit tests for UserService" | SIMPLE |
| "Fix the null pointer in auth.js" | SIMPLE |
| "Review the payment module code" | SIMPLE |
| "Build a user authentication system" | COMPLEX |
| "Create a new dashboard feature" | COMPLEX |
| "Add caching to the API" | UNCLEAR |
