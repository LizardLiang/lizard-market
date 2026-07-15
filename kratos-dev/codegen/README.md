# God Launcher Codegen

`plugins/kratos/commands/<god>.md` (the 19 slash-command launchers) and the
god-derived regions of `plugins/kratos/skills/auto/SKILL.md` are **generated**
from `plugins/kratos/agents/*.md` frontmatter by
`kratos-dev/go/cmd/gencommands`. Generated files are committed — installs copy
the plugin directory verbatim, so there is no build step at install time.

Do not hand-edit `commands/<god>.md` for a launcher whose frontmatter carries
`generated: true`. Edit the agent's frontmatter (or its partial) and run
`make gen` instead — a hand-edit will just get overwritten on the next
regeneration, and `make gen-check` (wired into CI via `go test ./...` and into
`kratos-dev/publish.sh`) will catch drift between the two.

## Commands

Run from `kratos-dev/go`:

```bash
make gen         # regenerate commands/<god>.md + SKILL.md god regions
make gen-check   # verify they already match agents/*.md; exits non-zero + lists drifted files otherwise
```

Equivalent direct invocations: `go run ./cmd/gencommands` and
`go run ./cmd/gencommands --check`.

## How it works

For each `plugins/kratos/agents/<god>.md`, the generator renders
`plugins/kratos/commands/<god>.md` from:

- `name` / `description` (required) — the description is truncated at the
  first `;`, dash-normalized (` - ` → ` — `), and lowercased on the first
  letter unless that first word is an all-caps acronym (`PM`, `QA`, `PRD`).
- `stage` (optional) — appends ` — pipeline Stage <label>` to the command
  description. Quote the value (`stage: "7a"`, `stage: "2→3"`).
- `quick_route: true` (optional) — marks the god as a quick-mode god for
  `SKILL.md`'s Activation bullets (see below).
- `command_refs` (optional, default `standard`) — selects the
  "additional references" hint sentence appended near the end of the
  launcher: `none` (omit the sentence — only Odysseus), `standard`
  (generic, no example), `templates` (+ `templates/`), `arena-protocol`
  (+ `references/arena-protocol.md`), `rules` (+ `` `rules/` `` — only
  Hermes today). None of the sentences mention `agent-protocol.md` — its
  relevant sections arrive pre-composed via `protocol_sections` (below).
- `protocol_sections` (optional) — flat comma-separated list of
  `references/agent-protocol.md` section slugs (the `<!-- protocol: <slug> -->`
  anchors under each `##` heading). `kratos agent load` appends the composed
  block to every launcher body, and the SubagentStart hook
  (`hooks/path-inject.cjs` → `kratos agent protocol <god>`) injects the same
  block into spawned subagents — so agents never Read `agent-protocol.md` at
  runtime. `LoadAgents` validates every slug against the anchors; an unknown
  slug fails `make gen` / `make gen-check` loudly.
- `command_note` (optional) — replaces the standard
  `— do NOT spawn a subagent via the Task tool.` clause after
  `Operate **in the main context**`. Store the *full* replacement text,
  including whatever leading punctuation/spacing is needed (see
  `agents/iris.md` and `agents/hermes.md` for the two current examples).

The loader line (`!cat ".../agents/<god>.md"` vs.
`!node ".../hooks/launch.cjs" agent load <god> --mode=command"`) is derived
automatically: if `plugins/kratos/command-mode-suffix/<god>.md` exists, the
launcher uses the `launch.cjs` loader (today: athena, hermes). No field
needed — just add or remove the suffix file.

## Bespoke tails (partials)

A handful of launchers append extra content beyond the boilerplate body
(Ares, Hera, Prometheus today). That content lives in
`kratos-dev/codegen/partials/<god>.md`, keyed by filename — no `god:` field
needed. Frontmatter:

- `placement` (optional, default `before-request`) — `before-request` inserts
  the partial (behind its own `---` separator) between the refs-hint sentence
  and `Request: $ARGUMENTS`; `after-request` appends it after
  `Request: $ARGUMENTS`.
- `allowed-tools` (optional) — merged into the launcher's frontmatter
  verbatim. Only Ares uses this today.

The body (everything after the closing `---`) is copied into the launcher
as-is.

## SKILL.md regions

`plugins/kratos/skills/auto/SKILL.md`'s frontmatter `description:` god-name
list and the two Activation bullets ("Quick-mode gods" / "All other gods")
are regenerated from the agent roster. Everything else in the file — the
Intent Classification table, Hard Rules, disambiguation prose — is untouched.

The **relative order** of names in these three lists is hand-curated (see
`kratos-dev/go/internal/gencmd/roster.go`'s `CanonicalOrder`,
`QuickGodOrder`, `OwnCommandGodOrder`) rather than derived (e.g.
alphabetically), to avoid reordering diff noise. The generator validates
*membership* against live `quick_route` frontmatter and fails loud on
mismatch — it just doesn't reorder for you.

The Activation bullets are wrapped in inline HTML comment markers
(`<!-- gen:quick-gods -->...<!-- /gen:quick-gods -->` and
`<!-- gen:skill-gods -->...<!-- /gen:skill-gods -->`) so the generator can
find and replace just the name list on subsequent runs.

## Adding a god

1. Create `plugins/kratos/agents/<god>.md` with `name`/`description` (and any
   of the optional fields above). Give it a `protocol_sections` list — every
   agent needs at least `auto-discovery, missing-required-input,
   session-tracking, boundaries, output-format`; deliverable-writing agents
   add `document-selection, document-creation, timestamp-standard,
   status-updates`; user-facing inline gods add `interactive-questions`.
2. If the god needs a bespoke tail, add
   `kratos-dev/codegen/partials/<god>.md`.
3. If the god needs the `launch.cjs` loader, add
   `plugins/kratos/command-mode-suffix/<god>.md`.
4. Append the god's display name to `CanonicalOrder` in
   `kratos-dev/go/internal/gencmd/roster.go`, and to `QuickGodOrder` or
   `OwnCommandGodOrder` depending on its `quick_route` value — the generator
   fails loud at this step if you forget.
5. Run `make gen` from `kratos-dev/go`, review the diff, commit.

## Removing a god

Delete `plugins/kratos/agents/<god>.md` and its entries in `roster.go` /
partials / command-mode-suffix. Its `commands/<god>.md` becomes an orphan
(carries `generated: true` with no matching agent): `make gen` deletes it
automatically; `make gen-check` reports it as drift until you do.

## Safety rails

- **First run / adoption**: `gencommands` refuses to overwrite a
  `commands/*.md` that lacks `generated: true` unless `--adopt` is passed
  (`make gen` doesn't pass it — use `go run ./cmd/gencommands --adopt`
  directly for a genuinely new hand-off).
- **Fail-loud parsing**: a missing `name`/`description`, or any unparseable
  frontmatter line, aborts with the offending file and field named — nothing
  is written.
- **Drift gate**: `kratos-dev/go/internal/gencmd/drift_test.go` runs the same
  check as `make gen-check` inside `go test ./...`, which CI already runs;
  `kratos-dev/publish.sh` also runs the check before publishing.
