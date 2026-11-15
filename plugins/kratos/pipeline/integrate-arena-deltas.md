---
name: integrate-arena-deltas
description: "[SUPERSEDED] Replaced by the spec-lifecycle feature (kratos spec archive / /kratos:spec-archive). This file is kept for reference only."
---

# Integrate Arena Deltas Command (Superseded)

This command is superseded by the living-spec lifecycle: `.claude/.Arena/specs/<capability>/spec.md` + `.claude/feature/<name>/spec-delta/<capability>.md`, promoted with the `kratos spec` CLI verbs. See `references/arena-protocol.md` § Behavioral Specs for the model, and `commands/spec-archive.md` for the manual promotion command.

**Use instead:**

```bash
# Preview a feature's pending changes
kratos spec diff <feature-name>

# Validate before promoting
kratos spec validate <feature-name>

# Promote into the living spec (RENAMED → REMOVED → MODIFIED → ADDED merge order; conflicts block)
kratos spec archive <feature-name>

# Or via slash command, anytime — works even if Hera never ran
/kratos:spec-archive <feature-name>
```

`kratos spec list --changes` shows every un-archived pending delta across all features — the safety-net view that replaces manually tracking `arena-deltas.md` files. `kratos spec backfill` migrates pre-existing (pre-spec-lifecycle) shipped features into living specs.

This file is retained for historical reference only; do not build new tooling against the `arena-deltas.md` format described below. Ask before deleting this file outright — it was left in place deliberately pending confirmation.
