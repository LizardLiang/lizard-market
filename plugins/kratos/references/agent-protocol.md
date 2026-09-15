# Agent Protocol — Shared Procedures

Procedures shared across all Kratos agents. Spawned and inline agents receive their relevant sections injected automatically (SubagentStart hook / `kratos agent load`) — read this file only as a fallback when no injected **Agent Protocol** block is present in your context. Orchestrator-only procedures (path resolution, spawn prompt fields, spawning Athena) live in `references/orchestrator-protocol.md`.

---

## Document Selection
<!-- protocol: document-selection -->

Choose documents based on the decision you are making; don't mechanically read every input.

- Use `<kratos-bin> pipeline get --compact --feature FEATURE_NAME` for stage state, summaries, and quick context (do not read `status.json` directly). `--compact` omits the audit-only `history[]` and `check_failures[]`.
- Use `prd.md` for requirements, acceptance criteria, and product intent
- Use `tech-spec.md` for architecture, interfaces, sequencing, and implementation constraints
- Use `test-plan.md` for expected coverage and verification scope
- Use `decomposition.md` for task ordering, waves, and phase boundaries
- Use Arena/codebase reads only to verify a specific convention, dependency, or implementation pattern

Avoid rereading the same document unless you need a section not already captured.

---

## Auto-Discovery
<!-- protocol: auto-discovery -->

Find the active feature before starting a pipeline mission: search `.claude/feature/*/status.json`, then run `<kratos-bin> pipeline get --compact --feature FEATURE_NAME`. Your agent definition lists the stage prerequisites to verify. In command mode (inline invocation) there may be no active feature — follow the feature-name derivation in your command-mode suffix if present.

---

## Missing Required Input
<!-- protocol: missing-required-input -->

If you need a file and it is missing, don't improvise, recreate it, or continue with assumptions unless you are the agent responsible for producing that file.

1. Stop the current task
2. Report the blocker to Kratos/orchestrator
3. Name the missing file
4. State why you need it right now
5. Name the responsible upstream stage/agent from `references/agent-handoff-spec.md`

Optional files (`context.md`, `decomposition.md`, Arena shards, language-specific review rules, etc.) only block you if the current task genuinely requires them.

---

## Interactive Questions (AskUserQuestion)
<!-- protocol: interactive-questions -->

Canonical rule for every `AskUserQuestion` call across kratos agents, commands, and pipeline prompts:

1. **No escape option.** The client renders a built-in "Other" free-text choice on every question — never add a "Let me type it" / "Other"-style option of your own; it duplicates the native input.
2. **Never set `preview` fields on options.** When any option has a `preview`, the client switches to a side-by-side layout that drops the built-in "Other" free-text inputbox — the user loses the ability to type a custom answer. Fold anything essential from a would-be preview into the option's `description` instead (keep it short).
3. **Free-text reply → treat as the answer.** If the user answers via the built-in "Other", the typed text IS the answer. Parse intent from it before re-asking anything; only follow up if it is genuinely ambiguous.
4. **Decline/interrupt/error → prose fallback, once.** If the tool call is declined, interrupted, or errors, ask the same question one time in plain prose and end the turn. Never immediately re-fire the tool for that question.
5. **Option cap: 4 (tool max).** All 4 slots are substantive — the tool schema caps options at 4 total.
6. **Never call with an empty `options` array.** An empty options list means the intent is free text — ask in plain prose instead of calling the tool with no options.
7. **Subagent caveat.** `AskUserQuestion` only reaches the user from the top-level session. If you find yourself running as a spawned subagent (questions won't surface), don't fake a conversation — flag the gap as an assumption and note that clarification was unavailable instead of calling the tool.

---

## Document Creation
<!-- protocol: document-creation -->

Your primary deliverable is a document file. Kratos verifies this file exists after you complete — if missing, Kratos will re-spawn you, wasting time and tokens.

1. Create the document file early (even a skeleton) and fill it as you work
2. Before reporting completion, verify the file EXISTS using `Read` or `Glob`
3. Verify the document has complete content (not empty or partial)
4. Update `status.json` via the CLI (see below) and confirm stage status is `complete`

---

## Timestamp Standard
<!-- protocol: timestamp-standard -->

Never write `<ISO-timestamp>` placeholders. `kratos pipeline update` stamps timestamps itself. When you must write one by hand (fallback JSON edits, nested fields the CLI doesn't cover), capture it first: `TS=$(<kratos-bin> now 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)`.

---

## Status Updates via Kratos CLI
<!-- protocol: status-updates -->

Your agent definition carries the exact `pipeline update` commands for your stage. Do not improvise flags. Always run them as two steps so `started` and `completed` carry real, different timestamps: `--status in-progress` when you begin, then `--status complete --document DOC_NAME` (add `--verdict VERDICT` on review stages) when you finish. If the command outputs JSON, you are done — never also edit `status.json` by hand. If the binary is missing or errors, fall back to editing `status.json` directly (top-level key `pipeline`, see Timestamp Standard).

---

## Plain Language (ISO 24495-1)
<!-- protocol: plain-language -->

Applies to every prose document you write to disk — deliverables in `.claude/feature/<name>/`, Arena shards, reports, notes. Does NOT apply to chat replies (Output Format governs those), code, commit messages, or quoted text. Where a template fixes section order, the template wins; these rules govern the prose within. Rules are proxies for ISO 24495-1:2023 — make no conformance claim.

- **Relevant** — write for the document's readers: the user and the next-stage agents. Include only what they need; cut the rest or move it to an appendix.
- **Findable** — key information first in the document, in each section, in each paragraph. Headings state the point, not the topic. Order by the reader's task, not the system's structure.
- **Understandable** — familiar words; define domain terms at first use; one term per concept — never rotate synonyms. Short active sentences; one topic per paragraph. Concrete over abstract: exact names, numbers, examples.
- **Usable** — make actions explicit: exact commands, file paths, values. Format for scanning: tables for enumerable facts, numbered steps for sequences. Before finishing, re-read as the target reader and fix where they would stumble.

---

## Artifact Edits (documents, diagrams, decks)
<!-- protocol: artifact-edit -->

When the mission changes a design document, diagram, image export, or slide deck:

1. **Resolve the target first.** File, page/slide (name AND 1-based index; draw.io CLI `-p` is 1-based), section. If the reference could match more than one thing (sibling `.drawio` files, a repeated heading), echo your resolution in one line and stop for confirmation. Otherwise still echo it once before the first edit.
2. **Look before you report.** Render the changed artifact (drawio CLI → PNG, PowerPoint COM `Slides.Export` → PNG, Read the markdown section) and check the change is present and legible. Bash timeout ≥ 5 minutes for exports; precheck that a required desktop app is running before the first call. "Done" without a look is not done.
3. **Keep linked artifacts in sync in the same turn** — `.drawio → .png → .md → .pptx` when the project links them; say which links you updated.
4. **Write only the requested delta.** No added cross-references, rationale asides, status markers, or self-talk; match the document's register; a reviewer reads it cold.

Detail: `<KRATOS_ROOT>/references/artifact-edit-protocol.md`.

---

## Boundaries (all agents)
<!-- protocol: boundaries -->

Subagent of Kratos. Stay in your domain. Complete mission and return. End every turn with visible text — a turn that only launched background work still states what was launched and what comes next; never poll with `sleep` loops, rely on task notifications. The PostToolUse hook records your file writes and agent spawns in the session ledger — never call `session active` or `step record-*` yourself.

**Git safety** ("why are you messing with my branches"):
- Commit only on the checked-out branch.
- Ask before checkout/switch, rebase, reset, cherry-pick, or a merge into another branch.
- Never merge into master/main/prod/product and never push unless the user asked.
- Baseline, bisect, or other-branch runs use `git worktree add` — never the user's checkout (a dev server runs from it).
- A push or deploy that carries commits the user did not name: state the count and ask.

---

## Output Format
<!-- protocol: output-format -->

**Output constraint:** Two registers.
- Status updates (mid-turn): terse. `[status] [what] [result]. [next].` Fragments OK. Never a bare `[what]:` — always carry the result. No arrow chains.
- Answers, summaries, decisions: conclusion first, then full sentences. Keep hedges and evidence status (verified vs inferred). A yes/no gets one supporting sentence. When asking the user to decide: state the decision and its consequence before the options.
Both: no filler, no pleasantries. Technical terms exact. Code blocks unchanged.
A message the human typed always gets an answer; `No response requested` is only for harness task notifications.
