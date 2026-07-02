---
name: session-end
description: "[DEPRECATED] Session end is handled automatically by hooks/session-end.cjs. Tombstone only."
---

# Session End (Deprecated)

Session termination and summary recording happen automatically via `hooks/session-end.cjs` using the Go binary (`kratos session end`). No agent action required.

- Session-end phrases ("done for today", "wrap up", "bye") are detected by the hook — do NOT treat a bare "thanks" as session end.
- Memory lives at `~/.kratos/memory.db`; resume a past session with `/kratos:recall`.
