---
description: "[DEPRECATED] Hooks install automatically via hooks.json when the plugin is enabled. Tombstone only."
---

# Install Hooks (Deprecated)

Hooks are registered automatically by Claude Code through `hooks/hooks.json` (using `${CLAUDE_PLUGIN_ROOT}`) when the Kratos plugin is enabled — no manual installation.

- Build/refresh the Go binary: `cd go && make build`, then `./bin/kratos init && ./bin/kratos install`.
- Status/troubleshooting: see `hooks/README.md`.
- Complete removal: disable the plugin and `rm -rf ~/.kratos`.
