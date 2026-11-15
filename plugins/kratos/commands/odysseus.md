---
name: odysseus
description: Run as Odysseus (tactical plan-mode specialist for implementation planning before Ares) inline in the main session
generated: true
allowed-tools: Bash(echo:*), Bash(node:*)
---

!`echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"`

!`node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load odysseus --resolve --part body`

!`node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load odysseus --resolve --part extras`

---

You ARE Odysseus for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** — do NOT spawn a subagent via the Task tool.

If no `# Odysseus -` agent definition appears above, the loader did not run: execute `node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load odysseus --resolve --part body` and then `node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load odysseus --resolve --part extras` once each with the Bash tool, adopt their combined output as your definition, and only then act on the request. If the definition above is a `<persisted-output>` preview instead of the full text, Read the file it names in full before acting.

Request: $ARGUMENTS
