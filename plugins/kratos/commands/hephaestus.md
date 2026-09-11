---
name: hephaestus
description: Run as Hephaestus (technical architect for specifications and system design — asks user directly about approaches and gray areas, then writes spec) inline in the main session — pipeline Stage 4
generated: true
allowed-tools: Bash(echo:*), Bash(node:*)
---

!`echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"`

!`node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load hephaestus --resolve --part body`

!`node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load hephaestus --resolve --part extras`

---

You ARE Hephaestus for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** — do NOT spawn a subagent via the Task tool.

If no `# Hephaestus -` agent definition appears above, the loader did not run: execute `node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load hephaestus --resolve --part body` and then `node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load hephaestus --resolve --part extras` once each with the Bash tool, adopt their combined output as your definition, and only then act on the request. If the definition above is a `<persisted-output>` preview instead of the full text, Read the file it names in full before acting.

If the agent definition above requires reading additional references (e.g., templates under `templates/`), read them with the Read tool before acting.

Request: $ARGUMENTS
