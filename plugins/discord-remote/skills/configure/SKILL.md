---
name: configure
description: Set up discord-remote — save the bot token, configure approval settings, review sidecar status, and manage deny patterns. Use when the user pastes a Discord bot token, asks to configure discord-remote, asks about remote approval setup, or wants to check sidecar status.
user-invocable: true
allowed-tools:
  - Read
  - Write
  - Bash(ls *)
  - Bash(mkdir *)
  - Bash(curl *)
---

# /discord-remote:configure — Discord Remote Setup

Sets up and configures the discord-remote plugin. Writes the bot token to
`~/.claude/channels/discord/.env` and manages `remote-config.json` for
approval settings, timeouts, and deny patterns.

Arguments passed: `$ARGUMENTS`

---

## Dispatch on arguments

### No args — status and guidance

Read state files and give the user a complete picture:

1. **Token** — check `~/.claude/channels/discord/.env` for
   `DISCORD_BOT_TOKEN`. Show set/not-set; if set, show first 6 chars masked.

2. **Sidecar** — check if `~/.claude/channels/discord/sidecar.port` exists.
   If it does, read the port and try `curl -s http://127.0.0.1:<port>/health`
   to confirm the sidecar is alive. Show: running/not-running, port, pending
   request count if running.

3. **Remote config** — read `~/.claude/channels/discord/remote-config.json`
   (missing = defaults). Show:
   - Approval timeout (default 60s)
   - Question timeout (default 120s)
   - Permission fallback (default "ask")
   - Deny patterns count and list

4. **Access** — read `~/.claude/channels/discord/access.json` (missing =
   defaults). Show: dmPolicy, allowFrom count (this is the user who receives
   approval DMs).

5. **What next** — end with a concrete next step based on state:
   - No token → `"Run /discord-remote:configure <token> with your bot token."`
   - Token set, no paired user → `"DM your bot on Discord. It replies with
     a code; approve with /discord-remote:access pair <code>."`
   - Token set, paired user, sidecar not running → `"The MCP server is not
     running. Start a Claude Code session to activate it."`
   - Everything ready → `"Ready. Tool approval requests will be forwarded
     to Discord."`

### `<token>` — save bot token

1. Treat `$ARGUMENTS` as the token (trim whitespace).
2. `mkdir -p ~/.claude/channels/discord`
3. Read existing `.env` if present; update/add the `DISCORD_BOT_TOKEN=` line,
   preserve other keys. Write back, no quotes around the value.
4. `chmod 600 ~/.claude/channels/discord/.env` — the token is a credential.
5. Confirm, then show the no-args status so the user sees where they stand.

### `timeout <approval_ms> [question_ms]` — configure timeouts

1. Read `remote-config.json` (create with defaults if missing).
2. Update `timeout.approval_ms` to the first argument.
3. If second argument provided, update `timeout.question_ms`.
4. Write back. Confirm with human-readable durations (e.g., "60 seconds").

### `fallback <ask|allow|deny>` — configure permission fallback

1. Validate the value is one of `ask`, `allow`, `deny`.
2. Read `remote-config.json`, update `defaults.permission_fallback`, write back.
3. Explain the choice:
   - `ask` (default): falls back to terminal prompt — nothing breaks if
     Discord is unreachable
   - `allow`: automatically allows if no Discord response — use only if you
     trust all tool invocations
   - `deny`: automatically denies if no Discord response — safe but blocks
     progress if Discord is down

### `deny add <pattern>` — add a deny pattern

1. Read `remote-config.json`.
2. Add the pattern string to `deny_patterns` (dedupe).
3. Write back. Confirm with the full pattern list.
4. Explain: patterns are case-sensitive substrings matched against
   tool_name + JSON-stringified tool_input. The sidecar blocks immediately
   without contacting Discord when a match is found.

### `deny rm <pattern>` — remove a deny pattern

1. Read `remote-config.json`.
2. Filter out the exact pattern string from `deny_patterns`.
3. Write back. Confirm with the updated list.

### `deny list` — list deny patterns

1. Read `remote-config.json`. Show all deny patterns, numbered.
2. If empty, say so.

### `clear` — remove the bot token

Delete the `DISCORD_BOT_TOKEN=` line (or the file if that's the only line).

---

## Note on coexistence with the official Discord plugin

If the official `discord` plugin is also installed, both plugins share the
same bot token and `access.json`. This is intentional — they share access
control so you don't have to pair twice. The two plugins will send separate
DMs for their respective operations (channel messaging vs. approval requests),
which may appear duplicated if both are active simultaneously.

The discord-remote plugin uses a different sidecar port file
(`sidecar.port`) that the official plugin does not write or read.

---

## Implementation notes

- The channels dir might not exist if the server hasn't run yet. Missing
  file = not configured, not an error.
- The server reads `.env` once at boot. Token changes need a session restart
  or `/reload-plugins`. Say so after saving.
- `remote-config.json` is read on each sidecar request (Windows) or at
  startup + SIGHUP (Unix). Changes take effect on next request after SIGHUP,
  or immediately on Windows. Say so after editing.
- Pretty-print JSON (2-space indent) for hand-editability.