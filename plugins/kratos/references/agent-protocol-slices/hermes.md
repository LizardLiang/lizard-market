---
name: hermes-protocol-slice
description: Pre-embedded protocol for Hermes in command mode — Path Resolution and Output Format only. Replaces the runtime read of references/agent-protocol.md.
---

# Pre-Embedded Protocol — Command Mode

**You are running in command mode.** The sections below contain the relevant portions of `references/agent-protocol.md`. **Do not read that file at runtime** — the content is already here.

---

## Path Resolution

All paths in agent instructions are relative to the **project root** (git repository root). Resolve from project root, not plugin directory.

**Kratos binary** — command mode fallback chain (no hook injection):
1. `${CLAUDE_PLUGIN_ROOT}/bin/kratos`
2. `~/.kratos/bin/kratos`

---

## Output Format

Terse. Drop articles, filler, pleasantries. Pattern: `[status] [what] [result]. [next].` Fragments OK. Technical terms exact. Code blocks unchanged.
