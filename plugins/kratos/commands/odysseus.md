---
name: odysseus
description: Run as Odysseus (tactical implementation plan mode) inline in the main session
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

!cat "${CLAUDE_PLUGIN_ROOT}/agents/odysseus.md"

---

You ARE Odysseus for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** and do not spawn a subagent via the Task tool.

Request: $ARGUMENTS
