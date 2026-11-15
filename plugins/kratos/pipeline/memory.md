---
name: memory
description: "[DEPRECATED] Memory is handled by the Go binary and hooks (.cjs files). Tombstone only."
---

# Kratos Memory (Deprecated)

Session/step memory is recorded automatically by the Go binary and Claude Code hooks — no agent action required.

- **Binary**: `${CLAUDE_PLUGIN_ROOT}/bin/kratos` (fallback `~/.kratos/bin/kratos`); implementation in `go/internal/cli/` and `go/internal/db/`.
- **Hooks**: `hooks/hooks.json` + `.cjs` implementations record session start/end and agent steps.
- **Database**: `~/.kratos/memory.db` (SQLite, WAL). Schema: `go/internal/db/schema.sql`.
- **Recall**: use `/kratos:recall` or `kratos recall`.

The old Python/Rust implementations under `memory/` are legacy and unused at runtime.
