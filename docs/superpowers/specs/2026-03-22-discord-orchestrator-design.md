# Discord Orchestrator Plugin — Design Spec

**Date:** 2026-03-22
**Status:** Approved
**Plugin name:** `discord-orchestrator`

## Problem

The current `discord-remote` plugin supports only a single Claude Code session bound to one project. We need a system where different Discord channels route messages to different projects, each running its own independent Claude Code session.

## Solution

A standalone Bun process that owns a single Discord gateway connection and spawns Claude Code sessions via the Claude Agent SDK (`@anthropic-ai/claude-agent-sdk`). Each channel maps to a registered project. **Sessions are per-project, not per-channel** — if multiple channels are bound to the same project, they share one SDK session. Messages from any bound channel are sent to the session; responses are posted back to the originating channel.

## Architecture

```
Discord Bot (single Bun process)
  │
  ├── discord.js client (one bot token, one gateway)
  ├── Config Manager (reads/writes projects.json)
  ├── Session Manager (spawn, track, timeout, resume)
  │
  ├── Mapped channels → SDK sessions (one per project)
  │     └── PermissionRequest hook callback (in-process, posts to same channel)
  │
  ├── Ops channel → SDK session with MCP tools for config management
  │
  └── Idle monitor → closes sessions after 60 min silence
```

### Key Architectural Decisions

- **No sidecar.** SDK hooks are in-process callbacks with closure access to the Discord client. No HTTP IPC needed.
- **No MCP server for project channels.** The orchestrator spawns SDK sessions, not the other way around.
- **MCP server only for ops.** The ops channel's Claude session gets MCP tools for managing projects and sessions.
- **Standalone process.** The orchestrator is NOT spawned by Claude Code — it runs independently and spawns Claude Code sessions.

## Data Model

### projects.json (`~/.discord-orchestrator/projects.json`)

```json
{
  "projects": {
    "lizard-market": {
      "name": "lizard-market",
      "path": "/home/shotup/programing/ai-agents/lizard-market",
      "channels": ["1234567890123456"]
    }
  },
  "ops_channel": "5555666677778888",
  "idle_timeout_ms": 3600000
}
```

- Projects keyed by human-readable name
- One project can have multiple channels
- Channel-to-project is a reverse lookup derived at startup

### sessions.json (`~/.discord-orchestrator/sessions.json`)

```json
{
  "lizard-market": {
    "sessionId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "lastActiveChannel": "1234567890123456",
    "lastActivity": "2026-03-22T10:30:00Z"
  }
}
```

- Keyed by **project name** (one session per project, shared across all its channels)
- `lastActiveChannel` tracks where the most recent message came from (for approval routing)
- Ephemeral — written on session state change, read on restart for resume
- Stores session IDs for SDK resume capability
- Writes are serialized via an async queue (single Bun process, but concurrent timers can race)

### State files (`~/.discord-orchestrator/`)

| File | Purpose |
|------|---------|
| `.env` | `DISCORD_BOT_TOKEN=...` (0600 permissions) |
| `projects.json` | Project registry + channel bindings |
| `sessions.json` | Active session IDs for resume |

## Message Flows

### Flow 1: Normal Conversation

1. Discord message arrives in a mapped channel
2. `bot.ts` looks up `channelId` → project name via reverse lookup
3. `session-manager.ts` checks for existing session; if none, spawns via `unstable_v2_createSession({ cwd: project.path, sessionId: deterministic UUID })`
4. If a prior `sessionId` exists in `sessions.json`, uses `resume` instead of fresh spawn
5. Calls `session.send(messageText)`
6. Streams response chunks → posts to the same Discord channel
7. Resets 60-minute idle timer

### Flow 2: Tool Permission Approval

1. SDK session needs tool approval → fires `PermissionRequest` hook callback
2. Callback (in-process) posts an embed to the project's Discord channel showing tool name + truncated input
3. Adds ✅ ❌ 🔒 reactions
4. `ReactionCollector` waits for an allowed user's reaction
5. ✅ → allow, ❌ → deny, 🔒 → allow + auto-approve this tool for the session lifetime
6. Returns decision to SDK → session continues or halts

### Flow 3: Ops Channel (Natural Language Admin)

1. Message arrives in the configured `ops_channel`
2. Routed to a dedicated Claude session whose `cwd` is the orchestrator plugin directory
3. This session runs with `allowedTools` restricted to the ops MCP tools + `Read` (no filesystem writes, no Bash). No `PermissionRequest` hook — unlisted tools are silently denied.
4. Claude interprets natural language → calls appropriate MCP tool
5. Tool handler modifies `projects.json` or manages sessions
6. Response posted to ops channel

### Response Posting Strategy

SDK sessions stream response chunks. The orchestrator batches them into Discord messages:

1. Collect streamed text into a buffer
2. When the buffer reaches 1800 characters (under Discord's 2000 limit) or the stream ends, post the buffer as a message
3. If the response is still streaming after the first post, continue collecting into a new buffer and post subsequent messages as replies to the first
4. Tool use blocks (Read, Bash output, etc.) are not posted — only the assistant's text responses
5. If the full response fits in one message, send it as a single message (most common case)

## Ops MCP Tools

| Tool | Description |
|------|-------------|
| `register_project` | Register a project (name + absolute path). Validates path exists. |
| `unregister_project` | Remove a project and all its channel bindings. |
| `bind_channel` | Map a Discord channel to a registered project. |
| `unbind_channel` | Remove a channel-to-project mapping. |
| `list_projects` | List all registered projects, their paths, and bound channels. |
| `list_sessions` | List active sessions with idle times. |
| `kill_session` | Force-close a specific project's session. |

## Session Lifecycle

### Spawn on Message
- Sessions are created lazily when the first message arrives in a mapped channel
- No pre-spawning on bot startup

### Idle Timeout
- 60 minutes of no messages → session closes
- Session ID saved to `sessions.json` before closing
- Timer resets on every message

### Lazy Resume on Restart
- On bot restart, reads `sessions.json` to know which channels had active sessions
- Does NOT immediately resume — waits for next message in that channel
- When message arrives, uses `resume: savedSessionId` for conversation continuity
- If resume fails (session expired), spawns fresh

### Session Spawning Configuration

**Session ID generation:** Uses UUID v5 with a fixed namespace (`6ba7b810-9dad-11d1-80b4-00c04fd430c8` — the URL namespace) and the project name as input. This produces a deterministic, valid UUID for each project that is stable across restarts.

```typescript
import { v5 as uuidv5 } from "uuid";

const SESSION_NAMESPACE = "6ba7b810-9dad-11d1-80b4-00c04fd430c8";
const sessionId = uuidv5(project.name, SESSION_NAMESPACE);

const session = unstable_v2_createSession({
  cwd: project.path,
  sessionId,
  hooks: {
    PermissionRequest: [{
      hooks: [async (input, toolUseId, { signal }) => {
        // originChannel is tracked by session-manager per message
        const channel = await discordClient.channels.fetch(originChannelId);
        const approved = await postApprovalAndWait(channel, input);
        return {
          hookSpecificOutput: {
            hookEventName: "PermissionRequest",
            permissionDecision: approved ? "allow" : "deny",
            permissionDecisionReason: approved
              ? "Approved via Discord reaction"
              : "Denied via Discord reaction",
          }
        };
      }]
    }]
  }
});
```

**Session teardown:** Each session is paired with an `AbortController`. Idle timeout calls `controller.abort()` for clean shutdown before saving session state.

**Auto-approve (🔒):** Maintained as an in-memory `Set<string>` per session, keyed by tool name only (e.g., `"Read"`, `"Glob"`). When a `PermissionRequest` fires, the set is checked first — if the tool name is present, the hook returns `allow` immediately without posting to Discord. The set is never persisted; it dies with the session.

**Note on V2 API:** The `unstable_v2_createSession` API is marked unstable. If its constructor signature does not accept `cwd`/`hooks`/`sessionId` directly, we fall back to V1 `query()` with equivalent options. The permission and lifecycle logic remains the same regardless of which API surface is used.

## Plugin File Structure

```
plugins/discord-orchestrator/
├── .claude-plugin/
│   └── plugin.json
├── .mcp.json                    # MCP tools for ops session
├── src/
│   ├── bot.ts                   # Discord.js client + message routing
│   ├── session-manager.ts       # Spawn, track, timeout, resume SDK sessions
│   ├── config.ts                # Read/write projects.json
│   ├── approval.ts              # PermissionRequest → Discord reaction flow
│   ├── ops-tools.ts             # MCP tool handlers
│   └── types.ts                 # TypeScript interfaces
├── server.ts                    # Entry point — starts bot + MCP server for ops
├── package.json
├── skills/
│   └── configure/SKILL.md       # Initial setup skill (bot token, ops channel)
└── README.md
```

## Dependencies

```json
{
  "dependencies": {
    "discord.js": "^14.14.0",
    "@anthropic-ai/claude-agent-sdk": "latest",
    "@modelcontextprotocol/sdk": "^1.0.0"
  },
  "devDependencies": {
    "bun-types": "latest",
    "@types/node": "^20.0.0"
  }
}
```

## Security Considerations (Stage 1)

- Bot token stored in `.env` with 0600 permissions
- `projects.json` validated — project paths must exist on filesystem
- Permission approval only accepted from users in the project's channel (Discord-level, not yet role-based)
- Auto-approve (🔒) scoped to session lifetime only — never persisted
- Stage 2 (deferred): role-based access, per-project permissions, audit trail

## Out of Scope

- User-based security / role system (Stage 2)
- Multiple bot tokens
- Cross-project message routing
- File attachment handling (can be added later)
- Discord application slash commands (using natural language ops instead)
