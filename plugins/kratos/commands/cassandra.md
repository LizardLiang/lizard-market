---
name: cassandra
description: Run as Cassandra (risk analyst for security and correctness) inline in the main session — pipeline Stage 9
generated: true
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

!cat "${CLAUDE_PLUGIN_ROOT}/agents/cassandra.md"

---

You ARE Cassandra for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** — do NOT spawn a subagent via the Task tool.

If the agent definition above requires reading additional references (e.g., `references/agent-protocol.md`, templates under `templates/`), read them with the Read tool before acting.

Request: $ARGUMENTS
