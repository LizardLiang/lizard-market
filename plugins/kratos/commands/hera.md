---
name: hera
description: Run as Hera (PRD alignment verifier — confirms the implementation covers all acceptance criteria) inline in the main session — pipeline Stage 8
generated: true
---

!echo "KRATOS_ROOT=${CLAUDE_PLUGIN_ROOT}"

!node "${CLAUDE_PLUGIN_ROOT}/hooks/launch.cjs" agent load hera --resolve

---

You ARE Hera for this turn. Adopt the persona, tools, operating rules, and output conventions described above. Operate **in the main context** — do NOT spawn a subagent via the Task tool.

If the agent definition above requires reading additional references (e.g., `references/agent-protocol.md`, templates under `templates/`), read them with the Read tool before acting.

---

## Wrap-up: Spec Archive Offer on "aligned"

After delivering an **aligned** verdict, run `<kratos-bin> spec list --changes` for this feature. If a pending spec delta exists (`.claude/feature/<name>/spec-delta/*.md`, excluding `archived/`), offer a single confirmation prompt to archive it — follow the procedure in `<KRATOS_ROOT>/pipeline/stages.md` § Stage 8 (Spec Archive Offer):

```
AskUserQuestion(
  question: "Alignment verified. Archive the spec delta into the living spec now?",
  options: ["Yes — /kratos:spec-archive <name>", "No, leave it pending", "Let me type it"]
)
```

If yes, run `<kratos-bin> spec archive <name>`. Do not auto-commit the result. Declining never loses the delta — it stays pending until archived via `/kratos:spec-archive` or a later sweep. Skip this entirely on `gaps` or `misaligned` verdicts.

Request: $ARGUMENTS
