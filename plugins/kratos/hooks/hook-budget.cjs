'use strict';

// Size budget for hook output. Claude Code caps every hook additionalContext
// string and every plain hook stdout at 10,000 characters
// (https://code.claude.com/docs/en/hooks.md, "JSON output"). Past the cap it
// saves the text to a file and injects only a 2,000-character preview, and
// never asks the model to read the file. The budget leaves 500 characters of
// margin under that cap.
const HOOK_OUTPUT_BUDGET = 9500;

// Protocol sections in the order they are dropped when the output is over
// budget. Output Format is not listed: it carries the output constraint and is
// never dropped.
const DROP_ORDER = [
  'Document Selection',
  'Auto-Discovery',
  'Document Creation',
  'Timestamp Standard',
  'Status Updates via Kratos CLI',
  'Missing Required Input',
  'Artifact Edits',
  'Plain Language',
  'Interactive Questions',
  'Flow Trace',
  'Boundaries',
];

function dropNotice(titles, root) {
  return `Protocol sections cut by the hook cap: ${titles.join(', ')}. Read only those sections in ${root}/references/agent-protocol.md; for them this overrides the "Do NOT read that file" line above.`;
}

// Joins `head` parts (kept whole) and `body` (a composed protocol block) with
// blank lines. While the result exceeds `budget`, removes whole `## ` sections
// of the body in DROP_ORDER and appends one line naming them. When the head
// parts alone exceed `budget`, every droppable section goes and the result is
// still over budget: the function fails open and returns it. Lessons are capped
// at 5 entries of at most 200 characters by `feedback add`, so with the
// one-line head parts that cannot happen in practice.
function fitToBudget(head, body, root, budget = HOOK_OUTPUT_BUDGET) {
  const join = (b, notice) => [...head, b, notice].filter(Boolean).join('\n\n');
  if (!body) return join('', null);
  let text = join(body, null);
  if (text.length <= budget) return text;

  const chunks = body.split('\n## ');
  const dropped = [];
  const titleOf = (chunk) => chunk.split('\n')[0].trim();
  const rebuild = () => {
    const kept = chunks.filter((c) => c !== null).join('\n## ');
    return join(kept, dropped.length ? dropNotice(dropped, root) : null);
  };
  for (const name of DROP_ORDER) {
    const i = chunks.findIndex((c, idx) => idx > 0 && c !== null && titleOf(c).startsWith(name));
    if (i < 0) continue;
    dropped.push(titleOf(chunks[i]));
    chunks[i] = null;
    text = rebuild();
    if (text.length <= budget) break;
  }
  return text;
}

module.exports = { HOOK_OUTPUT_BUDGET, DROP_ORDER, fitToBudget };
