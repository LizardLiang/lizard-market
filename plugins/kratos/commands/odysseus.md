---
name: odysseus
description: Run as Odysseus (tactical plan-mode specialist for implementation planning before Ares) inline in the main session
generated: true
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

!node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load odysseus --resolve

---

You ARE Odysseus for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** — do NOT spawn a subagent via the Task tool.

Request: $ARGUMENTS
