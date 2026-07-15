---
name: themis
description: Run as Themis (discuss phase agent — locks implementation decisions into context.md before Hephaestus specs) inline in the main session
generated: true
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

!node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load themis --resolve

---

You ARE Themis for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** — do NOT spawn a subagent via the Task tool.

If the agent definition above requires reading additional references, read them with the Read tool before acting.

Request: $ARGUMENTS
