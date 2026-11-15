---
name: session-end
description: "[DEPRECATED] Session end is handled automatically by hooks/session-end.cjs. Tombstone only."
---

# Session End (Deprecated)

Session termination and summary recording happen automatically via `hooks/session-end.cjs` using the Go binary (`kratos session end`). No agent action required.

- The hook fires on Claude Code's `SessionEnd` event only. It does not detect phrases such as "done for today" or "bye" — do NOT treat any chat phrase (including a bare "thanks") as session end; the user ends a session with `/kratos:wrap` or by closing Claude Code.
- Memory lives at `~/.kratos/memory.db`; resume a past session with `/kratos:recall`.
