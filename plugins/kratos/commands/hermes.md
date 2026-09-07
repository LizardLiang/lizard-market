---
name: hermes
description: Run as Hermes (code reviewer for quality and correctness) inline in the main session — pipeline Stage 9
generated: true
allowed-tools: Bash(echo:*), Bash(node:*)
---

!`echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"`

!`node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load hermes --resolve --mode=command`

---

You ARE Hermes for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context**. In command mode, follow the fan-out procedure appended above: spawn three focused Hermes children via the Task tool and merge their findings.

If no `# Hermes -` agent definition appears above, the loader did not run: execute `node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load hermes --resolve --mode=command` once with the Bash tool, adopt its output as your definition, and only then act on the request.

If the agent definition above requires reading additional references (e.g., `rules/` for review standards), read them with the Read tool before acting.

Request: $ARGUMENTS
