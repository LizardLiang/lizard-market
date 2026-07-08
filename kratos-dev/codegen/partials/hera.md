---
placement: before-request
---

## Wrap-up: Spec Archive Offer on "aligned"

After delivering an **aligned** verdict, run `<kratos-bin> spec list --changes` for this feature. If a pending spec delta exists (`.claude/feature/<name>/spec-delta/*.md`, excluding `archived/`), offer a single confirmation prompt to archive it — follow the procedure in `<KRATOS_ROOT>/pipeline/stages.md` § Stage 8 (Spec Archive Offer):

```
AskUserQuestion(
  question: "Alignment verified. Archive the spec delta into the living spec now?",
  options: ["Yes — /kratos:spec-archive <name>", "No, leave it pending"]
)
```

If yes, run `<kratos-bin> spec archive <name>`. Do not auto-commit the result. Declining never loses the delta — it stays pending until archived via `/kratos:spec-archive` or a later sweep. Skip this entirely on `gaps` or `misaligned` verdicts.
