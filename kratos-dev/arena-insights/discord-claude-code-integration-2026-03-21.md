# Discord / Chat Platform Claude Code Integration Research

## Metadata
| Field | Value |
|-------|-------|
| **Researched** | 2026-03-21 |
| **TTL** | 30 days |
| **Query** | Claude Code Telegram/Discord plugin architecture, permission model, MCP channels protocol, headless mode, remote control for building Discord integration |
| **Researcher** | Mimir |
| **Cache Until** | 2026-04-20 |

## Summary

Anthropic shipped **Claude Code Channels** on March 20, 2026 as a research preview (v2.1.80+). This is the official answer to remote chat-platform control. Channels are MCP servers that push events into a running local Claude Code session using a `notifications/claude/channel` notification method over stdio transport. Telegram and Discord are both officially supported via the `anthropics/claude-plugins-official` repo. The protocol is well-documented and custom channels can be built with nothing more than `@modelcontextprotocol/sdk` and Bun/Node. All permissions remain as-is unless `--dangerously-skip-permissions` (alias for `bypassPermissions` mode) is used.

For unattended/background use there is no official daemon mode yet (feature request open at issue #30447), but the `claude remote-control` server mode is the closest official approach. Community projects bridge the gap via PTY injection or tmux `send-keys`.

## Key Findings

### Approach 1: Official Channels Protocol (Recommended)

**Source**: https://code.claude.com/docs/en/channels and https://code.claude.com/docs/en/channels-reference

**How it works**:
1. A channel is a standard MCP server spawned by Claude Code as a subprocess over stdio.
2. It declares capability `{ experimental: { 'claude/channel': {} } }` in its `Server` constructor.
3. It emits `notifications/claude/channel` with `{ content, meta }` to push events into the session.
4. Events arrive in Claude's context wrapped as `<channel source="name" ...attr>body</channel>` XML tags.
5. For two-way (chat bridge), the server also exposes a `reply` MCP tool that Claude calls to send messages back.

**MCP server constructor (TypeScript/Bun)**:
```ts
const mcp = new Server(
  { name: 'discord', version: '1.0.0' },
  {
    capabilities: {
      experimental: { 'claude/channel': {} },  // marks this as a channel
      tools: {},                                 // needed for reply tool
    },
    instructions: 'Messages arrive as <channel source="discord" user_id="..." channel_id="...">. Reply using the reply tool, passing channel_id.',
  },
)
await mcp.connect(new StdioServerTransport())
```

**Pushing an event**:
```ts
await mcp.notification({
  method: 'notifications/claude/channel',
  params: {
    content: 'user message text here',
    meta: { user_id: '123456', channel_id: '987654', username: 'alice' },
    // meta keys must be letters/digits/underscores only — hyphens are dropped silently
  },
})
```

**Result in Claude's context**:
```
<channel source="discord" user_id="123456" channel_id="987654" username="alice">
user message text here
</channel>
```

**Reply tool registration**:
```ts
mcp.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: [{
    name: 'reply',
    description: 'Send a message back to Discord',
    inputSchema: {
      type: 'object',
      properties: {
        channel_id: { type: 'string' },
        text: { type: 'string' },
      },
      required: ['channel_id', 'text'],
    },
  }],
}))

mcp.setRequestHandler(CallToolRequestSchema, async req => {
  if (req.params.name === 'reply') {
    const { channel_id, text } = req.params.arguments
    await discordClient.channels.cache.get(channel_id).send(text)
    return { content: [{ type: 'text', text: 'sent' }] }
  }
})
```

**Launching with the channel**:
```bash
# Official plugin (requires being on Anthropic allowlist):
claude --channels plugin:discord@claude-plugins-official

# Custom / development channel (bypasses allowlist for testing):
claude --dangerously-load-development-channels server:discord
# where "discord" matches the key in .mcp.json mcpServers
```

**Security - sender allowlist**:
```ts
const allowed = new Set(loadAllowlist())  // stored in ~/.claude/channels/discord/.env or access.json
// in message handler, before emitting:
if (!allowed.has(message.author.id)) return  // drop silently
await mcp.notification({ ... })
// Gate on sender identity (message.author.id), NOT channel id
```

**Pros**:
- Official, first-class support; Anthropic-maintained protocol
- Discord plugin already exists at `anthropics/claude-plugins-official`
- Full local filesystem, MCP tools, project context available
- Pairing flow built-in for user authorization
- Survives interruptions; session reconnects automatically

**Cons**:
- Requires `--channels` flag each session launch (or set `remoteControlAllSessions: true`)
- Research preview: allowlist changes only for approved plugins; dev work needs `--dangerously-load-development-channels`
- Requires claude.ai login (not API key)
- Requires Bun runtime
- Permission prompts still fire unless `--dangerously-skip-permissions` used

---

### Approach 2: Existing Discord MCP Servers (Read/Send Only)

**Source**: https://github.com/v-3/discordmcp (189 stars), https://github.com/barryyip0625/mcp-discord (75 stars)

These are standard (non-channel) MCP servers that expose Discord read/send tools to Claude. Claude polls them on-demand; they do NOT push events into the session.

**v-3/discordmcp tools**: `send-message`, `read-messages`
**barryyip0625/mcp-discord tools**: 40+ tools including channel management, webhooks, role management, member operations. Dual transport: stdio and HTTP (`/mcp` endpoint).

**Pattern**:
```json
// .mcp.json
{
  "mcpServers": {
    "discord": {
      "command": "node",
      "args": ["./build/index.js"],
      "env": { "DISCORD_TOKEN": "your-bot-token" }
    }
  }
}
```

**Pros**: Works today without research-preview restrictions; rich Discord API coverage
**Cons**: Pull-only (Claude queries when asked, not event-driven); cannot receive real-time messages from users; not a chat bridge

---

### Approach 3: Community Hook-Based Injection (JessyTsui/Claude-Code-Remote)

**Source**: https://github.com/JessyTsui/Claude-Code-Remote

Uses Claude Code's `Stop`/`SubagentStop` hooks to send notifications, and injects replies via PTY simulation or tmux `send-keys`.

**Architecture**:
1. Stop hook fires `node claude-hook-notify.js completed` when Claude finishes a task
2. Notification sent to Discord/Telegram/email
3. User replies to notification
4. Webhook server receives reply, extracts command text, injects into Claude session via PTY or tmux

**PTY injection**:
```js
// Simulates keyboard input into the running Claude terminal process
pty.write('your command here\n')
```

**tmux injection**:
```bash
tmux send-keys -t session_name "your command here" Enter
```

**Pros**: Works without Channels research preview access; daemon-friendly via tmux; no Bun required
**Cons**: Fragile (PTY simulation can break); requires tmux for headless; not officially supported

---

### Approach 4: RichardAtCT/claude-code-telegram (2174 stars)

**Source**: https://github.com/RichardAtCT/claude-code-telegram

Not a Channels-protocol integration — uses the Anthropic SDK directly or Claude Code CLI as subprocess. Pre-dates the official Channels feature.

**Architecture**: Telegram bot receives message → validates whitelist (`ALLOWED_USERS`) → calls Anthropic SDK or shells out to `claude` CLI → returns result to Telegram.

**Pros**: Mature, 2174 stars, works today
**Cons**: Not event-driven into a running session; starts fresh SDK calls per message; does not use the local file context of a running session

---

## Permission System Reference

### Permission Modes (set in settings.json `defaultMode` or via CLI)

| Mode | Behavior |
|------|----------|
| `default` | Prompts on first use of each tool |
| `acceptEdits` | Auto-accepts file edits for the session |
| `plan` | Read-only; no file writes or command execution |
| `dontAsk` | Auto-denies unless pre-approved via `/permissions` |
| `bypassPermissions` | Skips all prompts (alias: `--dangerously-skip-permissions`) |

### Pre-approving tools in settings.json
```json
{
  "permissions": {
    "allow": [
      "Bash(npm run *)",
      "Bash(git commit *)",
      "Read",
      "Edit"
    ],
    "deny": [
      "Bash(git push *)"
    ]
  }
}
```

### CLI flags for non-interactive use
```bash
# Basic headless (non-interactive, single prompt):
claude -p "your task here"

# With pre-approved tools:
claude -p "run tests and fix failures" --allowedTools "Bash,Read,Edit"

# Structured JSON output:
claude -p "summarize project" --output-format json

# Streaming JSON:
claude -p "write a poem" --output-format stream-json --verbose --include-partial-messages

# Continue previous session:
claude -p "continue" --continue
claude -p "continue specific session" --resume "$SESSION_ID"

# Full bypass (use in containers only):
claude -p "do everything" --dangerously-skip-permissions

# Fine-grained Bash allow:
claude -p "create commit" --allowedTools "Bash(git diff *),Bash(git commit *),Bash(git status *)"
```

**Note**: `--allowedTools` may be ignored with `bypassPermissions`; use `--disallowedTools` for reliable blocklisting in all modes.

### MCP tool permission syntax
```json
{ "permissions": { "allow": ["mcp__discord", "mcp__discord__reply"] } }
```

---

## Remote Control Reference

### Official remote-control server mode
```bash
# Headless server, waits for remote connections from claude.ai or Claude mobile:
claude remote-control --name "My Project"

# Interactive session also accessible remotely:
claude --remote-control "My Project"

# Multiple concurrent sessions via git worktrees:
claude remote-control --spawn worktree --capacity 10
```

**Limitations**: terminal must stay open; no official daemon mode (feature request: issue #30447); requires claude.ai login.

---

## Official Discord Plugin Tools (from `anthropics/claude-plugins-official`)

| Tool | Description |
|------|-------------|
| `reply` | Send text to Discord; supports `reply_to` (message ID) for threading; file attachments up to 25MB, 10 files max |
| `react` | Add emoji reaction (Unicode or custom emoji syntax) |
| `edit_message` | Modify a previously sent bot message |
| `fetch_messages` | Retrieve up to 100 recent channel messages with IDs |
| `download_attachment` | Download files from messages to `~/.claude/channels/discord/inbox/` |

Attachments are NOT auto-downloaded; Claude must explicitly call `download_attachment`.

---

## Recommendations

Based on this project's context (Kratos — a Claude Code plugin system with hook infrastructure and an existing plugin architecture):

1. **Use the official Channels protocol** — Implement as a proper channel plugin following the `notifications/claude/channel` spec. The official Discord plugin source at `anthropics/claude-plugins-official` can be referenced directly. This integrates with Kratos's existing plugin system naturally.

2. **For unattended operation** — Run `claude remote-control` or `claude --channels` inside a tmux session. Until the official daemon feature lands (issue #30447), tmux is the standard workaround. JessyTsui/Claude-Code-Remote's tmux injection pattern is a viable fallback.

3. **For permission pre-approval** — Commit a `.claude/settings.json` with an `allow` list covering the expected tool surface, and use `acceptEdits` as the `defaultMode` for sessions where file editing is routine. Reserve `bypassPermissions` for fully containerized/sandboxed environments.

4. **Sender security** — Always gate on `message.author.id` (not `channel.id`) before emitting channel notifications. Pair users via the code-exchange flow identical to the official Telegram/Discord plugin implementations.

---

## Sources Consulted

- https://code.claude.com/docs/en/channels — Official Channels feature docs (research preview, March 20 2026)
- https://code.claude.com/docs/en/channels-reference — Full channel protocol spec, notification format, reply tool pattern
- https://code.claude.com/docs/en/permissions — Complete permission system and mode reference
- https://code.claude.com/docs/en/headless — Programmatic/headless CLI reference (`-p` flag, `--allowedTools`, output formats)
- https://code.claude.com/docs/en/remote-control — Remote Control session architecture and limitations
- https://github.com/anthropics/claude-plugins-official — Official Telegram and Discord plugin source
- https://github.com/RichardAtCT/claude-code-telegram — Community Telegram bot (2174 stars, SDK-based)
- https://github.com/v-3/discordmcp — Discord MCP server, pull-based (189 stars)
- https://github.com/barryyip0625/mcp-discord — Discord MCP server, 40+ tools, dual transport (75 stars)
- https://github.com/JessyTsui/Claude-Code-Remote — Hook + PTY/tmux injection approach for remote Discord/Telegram/email control
- https://venturebeat.com/orchestration/anthropic-just-shipped-an-openclaw-killer-called-claude-code-channels — Announcement coverage
- https://github.com/anthropics/claude-code/issues/30447 — Feature request: headless daemon mode for remote-control

## Related Topics

- MCP plugin packaging and marketplace submission — for getting a custom Discord channel onto the official allowlist
- Claude Code sandboxing — for safe `bypassPermissions` use in containers
- Claude Code hooks — Kratos already has hook infrastructure that could drive `Stop` notifications to Discord