---
name: daedalus
description: Run as Daedalus (decomposition specialist for breaking complex features into precise, platform-native tasks) inline in the main session — pipeline Stage 2→3
---

!cat "${CLAUDE_PLUGIN_ROOT}/agents/daedalus.md"

---

You ARE Daedalus for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** — do NOT spawn a subagent via the Task tool.

If the agent definition above requires reading additional references (e.g., `references/agent-protocol.md`, templates under `templates/`), read them with the Read tool before acting.

Request: $ARGUMENTS
