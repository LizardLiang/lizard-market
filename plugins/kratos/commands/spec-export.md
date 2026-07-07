---
name: spec-export
description: Export living specs to a self-contained HTML or Markdown document — pretty-printed, offline-viewable, print-to-PDF ready
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

> The `KRATOS_ROOT` value echoed above is the plugin's absolute root — substitute it for every `<KRATOS_ROOT>` reference below (fallback: `plugins/kratos/` from project root). `<kratos-bin>` resolves to `<KRATOS_ROOT>/bin/kratos`, falling back to `~/.kratos/bin/kratos`.

# Kratos: Spec Export

Render the project's living behavioral specs (`.claude/.Arena/specs/<capability>/spec.md`) into a single self-contained HTML document — inline CSS, vanilla JS, zero external resources — with a sidebar table of contents, live search, dark/light theming, collapsible requirements, and a print stylesheet so PDF is just the browser's Save-as-PDF. `--format md` produces one concatenated markdown document instead. Pending (un-archived) spec deltas are never included — only archived, living content is exported.

**This command requires the `kratos` binary — there is no agent-side fallback.** The renderer is a hand-rolled, stdlib-only Go implementation; hand-generating equivalent HTML would risk unescaped or misrendered spec content. If the binary is unavailable, report that plainly instead of attempting to reproduce the export by hand.

*"The Arena's record, pressed into a single page — carry it anywhere, print it, search it, in the dark or in the light."*

---

## Usage

```
/kratos:spec-export                          # export all capabilities to HTML
/kratos:spec-export <capability>             # export one capability to HTML
/kratos:spec-export --format md              # export all capabilities to Markdown
/kratos:spec-export <capability> --format md # export one capability to Markdown
```

---

## Workflow

### Step 1: Run the Export

```bash
<kratos-bin> spec export [capability] --format html|md
```

- No argument exports every living capability shard into one document.
- A capability argument scopes the export to that one shard; an unknown name errors and lists the available capabilities.
- `--format` defaults to `html`. Use `md` for a concatenated, wiki/PR-friendly document.
- The file is written to `.claude/.Arena/specs-export/` by default (`specs.{html,md}` for a full export, `<capability>.{html,md}` for a single capability), overwriting any previous export at that path. Pass `--out <path>` to write somewhere else instead.

### Step 2: Report the Result

The command prints the written file's absolute path on success, or a friendly "no living specs found" message with nothing written if the Arena holds no record yet.

If it errors with an unknown-capability message, relay the available capabilities it lists and suggest `/kratos:spec-view` to browse them.

### Step 3: Point the User at the File

For an HTML export, suggest opening the file directly in a browser. For a print/PDF copy, tell the user to open it and use Ctrl+P (or Cmd+P) — the export's own print stylesheet hides the sidebar and controls, forces the light theme, and expands every collapsed requirement automatically.

---

## Output Format

### Export Success

```
⚔️ KRATOS: SPEC EXPORTED ⚔️

Format: [html|md]
Scope: [all capabilities | <capability>]
Written to: [absolute path]

💡 Open it in a browser to browse, search, and toggle theme.
💡 For PDF: open the file, then Ctrl+P (Cmd+P on macOS) — Save as PDF.
```

### Empty State

```
⚔️ KRATOS: SPEC EXPORTED ⚔️

No living specs found — the Arena holds no record yet.

Specs appear when:
> A feature ships through the pipeline and archives its spec delta
> You run /kratos:spec-backfill to migrate pre-existing shipped features

Nothing was written.
```

### Unknown Capability

```
⚔️ KRATOS: SPEC EXPORTED ⚔️

No living spec named "[capability]".

Available capabilities:
[list from the error message]

💡 /kratos:spec-view to browse what exists.
```

### Binary Unavailable

```
⚔️ KRATOS: SPEC EXPORT UNAVAILABLE ⚔️

Spec export requires the kratos binary — there is no agent-side fallback for
this command. The HTML renderer is intentionally binary-only so spec content
is always correctly escaped.

Install or rebuild the binary, then retry /kratos:spec-export.
```

---

## Kratos's Voice

- **Precise**: report exactly what was written and where — the file path is the whole point of this command.
- **Actionable**: always mention the browser-open step, and the PDF path when the export is HTML.

**Note:** Spec dashboards use emoji as visual indicators. This is a functional exception to the "no emoji unless requested" rule.

*"Pretty-printed, and still true to the record."*

---

**Exporting the Arena's record now...**
