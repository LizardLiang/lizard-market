---
name: classify
description: Intent classification for new user requests — determines which Kratos command to route to
---

# Task Classification

When the user provides a **new request** (not "continue" or "status"), classify intent before doing anything else. The classification determines which pipeline to activate.

---

## Clarity Pre-Check (run first, before category routing)

A build verb ("build", "create", "add", "fix", "refactor") does **not** mean the request is clear. Before assigning a category, check the request for three signals:

- **Goal** — is it discernible *what* should change or exist afterward?
- **Target** — is the *where* identifiable (a file, module, feature, or a concrete surface)?
- **Success** — is there any sense of *done* (behavior, criteria, or an obvious implicit bar)?

If **all three are present**, proceed to category routing normally.

If **one or more is missing** (e.g. "improve the app", "fix the thing", "make it better", "add caching" with no target/policy), do **not** silently route into SIMPLE/COMPLEX on the strength of the verb alone:
- If the request is substantial/new functionality → route to **COMPLEX**; the gap-analysis loop (`gap-analysis.md`) will elicit the missing signals.
- If the request is a focused change on existing code but underspecified → ask **one** `AskUserQuestion` to pin the missing signal(s) before routing to SIMPLE. SIMPLE/`quick` has no elicitation phase, so an unclear SIMPLE request must be clarified here or it will be built on assumptions.

This closes the gap where a vaguely-phrased request rides a build verb straight past discovery.

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

## Daedalus Inclusion Signals (Stage 3 — post Stage-2 approval)

After Stage 2 is approved, Kratos checks these signals to decide whether to offer Stage 3 (Daedalus decomposition). If **any two** signals are present, offer decomposition. If none, skip directly to Stage 4.

| Signal | Present when... |
|--------|-----------------|
| **File span** | Feature touches or creates >3 files across different modules |
| **Parallel subproblems** | Two or more independent sub-tasks can be built/tested without the other |
| **Cross-cutting concern** | Change affects multiple layers (API + DB + UI, or auth + middleware + models) |
| **Effort estimate** | PRD or discussion suggests >1 day of implementation work |
| **User-requested** | User explicitly says "decompose" or "break this into tasks" |

If fewer than two signals are present and the user did not request decomposition, skip Stage 3 and proceed to Stage 4 (Hephaestus).

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
