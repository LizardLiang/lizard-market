---
name: athena-protocol-slice
description: Pre-embedded protocol for Athena in command mode — Path Resolution, Auto-Discovery, Timestamps, Status Updates, Output Format. Replaces the runtime read of references/agent-protocol.md.
---

# Pre-Embedded Protocol — Command Mode

**You are running in command mode.** The sections below contain the relevant portions of `references/agent-protocol.md`. **Do not read that file at runtime** — the content is already here.

---

## Path Resolution

All paths in agent instructions are relative to the **project root** (git repository root). Resolve from project root, not plugin directory.

Templates: `'<kratos-path>' template get <template-name>` (omit the `.md` extension).

**Kratos binary** — command mode fallback chain (no hook injection):
1. `${CLAUDE_PLUGIN_ROOT}/bin/kratos`
2. `~/.kratos/bin/kratos`
3. If unavailable, skip all `<kratos-bin>` calls and edit files directly.

---

## Auto-Discovery

Find the active feature before starting:

```
Glob: .claude/feature/*/status.json
```

Then read pipeline state:
```bash
<kratos-bin> pipeline get --feature FEATURE_NAME
```

In command mode there may be no active feature — follow the feature-name derivation in the Command-Mode section below if none is found.

---

## Timestamp Standard

Never write `<ISO-timestamp>` placeholders. Always use a real timestamp.

```bash
TS=$(<kratos-bin> now 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
```

---

## Status Updates via Kratos CLI

Two-step process for authentic timestamps:

```bash
# Step 1: Mark started
<kratos-bin> pipeline update --feature FEATURE_NAME --stage STAGE_NUMBER --status in-progress

# Step 2: Mark complete
<kratos-bin> pipeline update --feature FEATURE_NAME --stage STAGE_NUMBER --status complete --document DOC_NAME
```

If the CLI is unavailable, fall back to editing `status.json` directly using the timestamp from `<kratos-bin> now`.

---

## Output Format

When completing PRD work (not during gap analysis): terse. Pattern: `[status] [what] [result]. [next].` Fragments OK.
