# Agent Protocol — Shared Procedures

Procedures shared across all Kratos agents. Spawned and inline agents receive their relevant sections injected automatically (SubagentStart hook / `kratos agent load`) — read this file only as a fallback when no injected **Agent Protocol** block is present in your context. Orchestrator-only procedures (path resolution, spawn prompt fields, spawning Athena) live in `references/orchestrator-protocol.md`.

---

## Document Selection
<!-- protocol: document-selection -->

Choose documents by the decision you are making; don't read every input or reread one without need.

- `<kratos-bin> pipeline get --compact --feature FEATURE_NAME` — stage state and summaries (never read `status.json` directly).
- `prd.md` — requirements, acceptance criteria, product intent
- `tech-spec.md` — architecture, interfaces, sequencing, implementation constraints
- `test-plan.md` — expected coverage and verification scope
- `decomposition.md` — task ordering, waves, phase boundaries
- Arena/codebase reads — only to verify a specific convention, dependency, or pattern

---

## Auto-Discovery
<!-- protocol: auto-discovery -->

Find the active feature before starting a pipeline mission: search `.claude/feature/*/status.json`, then run `<kratos-bin> pipeline get --compact --feature FEATURE_NAME`. Your agent definition lists the stage prerequisites to verify. In command mode there may be no active feature — follow the feature-name derivation in your command-mode suffix.

---

## Missing Required Input
<!-- protocol: missing-required-input -->

If a file you need is missing, don't recreate it or assume its content unless producing it is your job. Stop and report to Kratos: the file, why you need it now, and the upstream stage/agent responsible (`references/agent-handoff-spec.md`).

Optional files (`context.md`, `decomposition.md`, Arena shards, review rules) only block you when the task requires them.

---

## Interactive Questions (AskUserQuestion)
<!-- protocol: interactive-questions -->

Canonical rule for every `AskUserQuestion` call:

1. **No escape option.** The client renders a built-in "Other" free-text choice — never add your own "Let me type it" / "Other" option.
2. **Never set `preview` on options.** Any `preview` switches the client to a side-by-side layout that drops the built-in "Other" input. Fold essentials into the option's `description`.
3. **Free-text reply is the answer.** Parse intent from the typed text before re-asking; follow up only if genuinely ambiguous.
4. **Decline/interrupt/error → prose fallback, once.** Ask the same question once in plain prose and end the turn. Never re-fire the tool for that question.
5. **Option cap: 4** (tool max); all 4 substantive.
6. **Never call with an empty `options` array** — free text means ask in plain prose.
7. **Subagent caveat.** `AskUserQuestion` only reaches the user from the top-level session. As a spawned subagent, flag the gap as an assumption, note that clarification was unavailable, and don't call the tool.

---

## Document Creation
<!-- protocol: document-creation -->

Your deliverable is a document file. Kratos re-spawns you if it is missing.

1. Create the file early and fill it as you work
2. Before reporting completion, verify the file EXISTS (`Read` or `Glob`) and has complete content
3. Update `status.json` via the CLI (see below) and confirm stage status is `complete`

---

## Timestamp Standard
<!-- protocol: timestamp-standard -->

Never write `<ISO-timestamp>` placeholders. `kratos pipeline update` stamps its own. To write one by hand, capture it first: `TS=$(<kratos-bin> now 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)`.

---

## Status Updates via Kratos CLI
<!-- protocol: status-updates -->

Your agent definition carries the exact `pipeline update` commands for your stage. Do not improvise flags. Run them as two steps so `started` and `completed` carry real, different timestamps: `--status in-progress` when you begin, then `--status complete --document DOC_NAME` (add `--verdict VERDICT` on review stages) when you finish. If the command outputs JSON, you are done — never also edit `status.json` by hand. If the binary is missing or errors, edit `status.json` directly (top-level key `pipeline`, see Timestamp Standard).

---

## Plain Language (ISO 24495-1)
<!-- protocol: plain-language -->

Applies to prose documents you write to disk, not to chat replies, code, commit messages or quoted text. A template's section order wins. Proxies for ISO 24495-1:2023 — make no conformance claim.

- **Relevant** — write for the reader; include only what they need.
- **Findable** — key point first; headings state the point; order by the reader's task.
- **Understandable** — familiar words; define terms at first use; one term per concept; short active sentences; one topic per paragraph; concrete over abstract.
- **Usable** — explicit commands, paths, values; tables for facts; ASCII diagram for a sequence (Mermaid only if rendered); re-read as the reader before finishing.

---

## Artifact Edits (documents, diagrams, decks)
<!-- protocol: artifact-edit -->

When the mission changes a design document, diagram, image export, or slide deck:

1. **Resolve the target first.** File, page/slide (name AND 1-based index; draw.io CLI `-p` is 1-based), section. If the reference could match more than one thing, echo your resolution in one line and stop for confirmation. Otherwise still echo it once before the first edit.
2. **Look before you report.** Render the changed artifact (drawio CLI → PNG, PowerPoint COM `Slides.Export` → PNG, Read the markdown section) and check the change is present and legible. Bash timeout ≥ 5 minutes for exports; precheck that a required desktop app is running before the first call. "Done" without a look is not done.
3. **Keep linked artifacts in sync in the same turn** — `.drawio → .png → .md → .pptx` when the project links them; say which links you updated.
4. **Write only the requested delta.** No added cross-references, rationale asides, status markers, or self-talk; match the document's register; a reviewer reads it cold.

Detail: `<KRATOS_ROOT>/references/artifact-edit-protocol.md`.

---

## Flow Trace (shared state, multi-step flows)
<!-- protocol: flow-trace -->

**Trigger** — the change touches a DB column, flag, status, counter, cache, or global written from more than one place; a step of a multi-step job; or a function with 3+ callers. Unsure → it triggers. The trace is grep plus one end-to-end read of the flow, not an interview.

Before designing, editing, or approving, write down:
1. **Every reader and writer** of the state, found by `grep`, never the list the brief named. Read the whole flow, not the function next to the change.
2. **Lifecycle table** — `state | set by | cleared by | scope (row / member / global) | stale when`. A state with no clearing path on every exit (success and failure) is a finding.
3. **The invariant** the change relies on plus the writer evidence (file:function) that keeps it true. No evidence → an assumption, and assumptions go to the user.
4. **Fit or fight** — two poles, no middle: patch inside the current flow vs restructure it. A bug takes the cheapest pole; an architectural fork takes the complete end-state, and the user decides.

**Hard rule:** a concern about the flow, an invariant, or a caveat ("deliberately NOT…", "would overwrite…", "for now") is a question to the user — never a comment, docstring, or note. Ask it in the user's domain terms, show the flow as an ASCII diagram, offer the two poles. Spawned agents stop with their report kind (`ARES NEEDS DESIGN`); inline agents use AskUserQuestion.

---

## Boundaries (all agents)
<!-- protocol: boundaries -->

Subagent of Kratos. Stay in your domain. Complete the mission and return. End every turn with visible text — a turn that only launched background work still states what was launched and what comes next; never poll with `sleep` loops, rely on task notifications. The PostToolUse hook records your file writes and agent spawns in the session ledger — never call `session active` or `step record-*` yourself.

**Git safety:**
- Commit only on the checked-out branch.
- Ask before checkout/switch, rebase, reset, cherry-pick, or a merge into another branch.
- Never merge into master/main/prod/product and never push unless the user asked.
- Baseline, bisect, or other-branch runs use `git worktree add` — never the user's checkout.
- A push or deploy that carries commits the user did not name: state the count and ask.

---

## Output Format
<!-- protocol: output-format -->

**Output constraint:** One standard for every answer.
- Lead with the point: the conclusion, or what is at stake. No preamble, no build-up.
- Show the mechanism, never narrate it. One view replaces the prose it would have taken:
  - A sequence of actions — call order, request flow, pipeline stages, the steps you took or will take, above all code flow — is an ASCII diagram in a code block: one box or line per step, arrows (`→`, `│`, `└─▶`) for order, a branch per condition. Never a paragraph, never Mermaid (it does not render in a terminal).
  - Items with no order — options, checks, parallel facts — are bullet points.
  - Logic is pseudocode. Structure is a file or component tree. A change is a `diff` when part changes, a full block when most is new.
  The view sits next to the short text it supports and keeps only the nodes that answer this question. One or two views, never all.
- Keep hedges and evidence status (verified vs inferred). Define a term at first use; never "simply", "just", "obviously". Name the misunderstanding the reader is likely to have.
- Asking the user to decide: state the decision and its consequence before the options.
- No filler, no pleasantries. Technical terms exact. Code blocks unchanged.
- A mid-turn progress line is the same standard in miniature: `[status] [what] [result]. [next].` Fragments OK; lead with the result, never a bare `[what]:`. No arrow chains in prose — a flow goes in an ASCII diagram.
- Write every line in the user's language, progress lines too, even when code and instructions are English.
- A message the human typed always gets an answer; `No response requested` is only for harness task notifications.
