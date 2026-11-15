# Artifact Edit Protocol — documents, diagrams, decks

Applies whenever a mission changes a design document, diagram, image export, or slide deck
(`.md`, `.drawio`, `.svg`, `.png` exports, `.pptx`, `.docx`). The injected **Artifact Edits**
protocol section is the short form; this file is the detail. The rules come from one week of
corrections on NETZERO design work: wrong page / wrong file / wrong section eight times,
"can you view your outcome, it is ugly", six near-identical "this is self-talking" trims.

## 1. Resolve the target before you touch it

1. Identify **file, page/slide, and section** exactly. Pages and slides get both the name and the
   1-based index (`page 9 "AI 單據辨識"`, `slide 12`). The draw.io desktop CLI `-p` flag is
   1-based; python-pptx `slides[i]` is 0-based — say which convention you used.
2. If the reference could match more than one thing — sibling files (`VITALNetZero-AICopilot.drawio`
   vs `VITALNetZero-AICopilot-sequence.drawio`), a heading that appears twice, "the table" when
   there are three — echo your resolution in one line and stop for confirmation. A wrong-target
   edit costs a revert plus the user's trust; a one-line echo costs nothing.
3. Echo the resolved target in one line before the first edit, even when unambiguous:
   `Target: docs/x.drawio · page 4 "方案 A" (index 4) · block "Web API"`.

## 2. Look at your output before you report it

"Done" without looking is not done.

| Artifact | Render | Then |
|----------|--------|------|
| `.drawio` | `drawio --export --format png --page-index <1-based> --output <png> <file>` (desktop CLI; on this user's box `-p` is 1-based) | Read the PNG; check the changed element is present, readable, not overlapping |
| `.pptx` | PowerPoint COM `Slides(i).Export(path, "PNG", w, h)` (no LibreOffice on this box) | Read the PNG for each changed slide |
| `.svg` / `.png` | Open the file (Read renders images) | Check size, legibility, no clipped text |
| `.md` | Read the changed section | Check headings, tables, embedded image paths resolve |

- Set the Bash timeout to at least 5 minutes for draw.io and PowerPoint exports; the default 2
  minutes has killed exports mid-run.
- Tools that need a running desktop app (PowerPoint COM, the Obsidian CLI) get a process precheck
  before the first call; if the app is not running, say so once and ask, do not retry blind.
- A file that will not open (duplicate page IDs after a merge, malformed XML) is a failed edit —
  fix it before reporting.

## 3. Keep linked artifacts in sync, in the same turn

When the project links them — a `.drawio` exported to `.png` embedded in a `.md` that is also
placed on a `.pptx` slide — one request updates the whole chain: `.drawio → .png → .md → .pptx`.
Finish the chain before reporting; do not wait for "update the ppt too". State which links you
updated and which you deliberately did not.

Hand-edited decks are patched in place, never rebuilt from a generator script, unless the user
asks for a rebuild.

## 4. Artifact register — write only the delta

Text inside the artifact is read cold by a reviewer who did not see this conversation.

- Write exactly what was asked. No extra cross-references ("see §5.4"), no rationale asides, no
  status markers (`⚠ in-plan`, `TODO`), no commentary about the edit itself.
- No self-talk: a sentence that explains your own reasoning or hedges about the content does not
  belong in the document.
- Match the document's existing register, terminology, and heading style; one term per concept.
- Plain-language rules (the injected section) govern sentence shape.
- Do not "improve" nearby prose while you are there; log a suggestion in chat instead.

## 5. Report

The WORK result names the resolved targets, what changed in each, the render you looked at,
and the landed commit. Offer what to check in one line. Nothing else.
