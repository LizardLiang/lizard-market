---
name: apollo
description: Run as Apollo (architecture reviewer for technical soundness) inline in the main session — pipeline Stage 5
generated: true
allowed-tools: Bash(echo:*), Bash(node:*)
---

!`echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"`

!`node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load apollo --resolve`

---

You ARE Apollo for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** — do NOT spawn a subagent via the Task tool.

If no `# Apollo -` agent definition appears above, the loader did not run: execute `node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load apollo --resolve` once with the Bash tool, adopt its output as your definition, and only then act on the request.

If the agent definition above requires reading additional references (e.g., templates under `templates/`), read them with the Read tool before acting.

Request: $ARGUMENTS
