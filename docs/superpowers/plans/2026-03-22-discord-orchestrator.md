# Discord Orchestrator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a `discord-orchestrator` plugin that routes Discord channels to different projects, each running an independent Claude Code session via the Agent SDK.

**Architecture:** A standalone Bun process owns a single Discord gateway and spawns Claude Code sessions via the V1 `query()` API from `@anthropic-ai/claude-agent-sdk`. Each project gets one SDK session shared across its bound channels. An ops channel provides natural-language admin via MCP tools.

**Tech Stack:** Bun, TypeScript, discord.js 14, @anthropic-ai/claude-agent-sdk (V1 query API), @modelcontextprotocol/sdk

**Spec:** `docs/superpowers/specs/2026-03-22-discord-orchestrator-design.md`

---

## File Structure

```
plugins/discord-orchestrator/
├── .claude-plugin/
│   └── plugin.json              # Plugin metadata
├── src/
│   ├── types.ts                 # All TypeScript interfaces
│   ├── config.ts                # Read/write projects.json + sessions.json
│   ├── session-manager.ts       # Spawn, track, timeout, resume SDK sessions
│   ├── approval.ts              # PermissionRequest → Discord reaction collector
│   ├── response-poster.ts       # Buffer and post SDK responses to Discord
│   └── bot.ts                   # Discord.js client + message routing
├── server.ts                    # Entry point — starts bot + ops channel wiring
├── package.json
├── tsconfig.json
├── skills/
│   └── configure/SKILL.md       # /discord-orchestrator:configure
└── README.md
```

Note: `ops-tools.ts` (MCP tool handlers) and `.mcp.json` are deferred to a future version. v0.1 ops channel uses direct file editing via SDK session.

---

### Task 1: Plugin Scaffold

**Files:**
- Create: `plugins/discord-orchestrator/.claude-plugin/plugin.json`
- Create: `plugins/discord-orchestrator/package.json`
- Create: `plugins/discord-orchestrator/tsconfig.json`
- Create: `plugins/discord-orchestrator/src/types.ts`

- [ ] **Step 1: Create plugin.json**

```json
{
  "name": "discord-orchestrator",
  "description": "Multi-project Discord orchestrator — routes channels to projects via Claude Agent SDK",
  "version": "0.1.0",
  "author": { "name": "Lizard Liang" }
}
```

- [ ] **Step 2: Create package.json**

```json
{
  "name": "discord-orchestrator",
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "start": "bun install --no-summary && bun server.ts"
  },
  "dependencies": {
    "discord.js": "^14.14.0",
    "@anthropic-ai/claude-agent-sdk": "latest",
    "uuid": "^11.0.0"
  },
  "devDependencies": {
    "bun-types": "latest",
    "@types/node": "^20.0.0",
    "@types/uuid": "^10.0.0"
  }
}
```

- [ ] **Step 3: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "outDir": "dist",
    "rootDir": ".",
    "types": ["bun-types"]
  },
  "include": ["src/**/*.ts", "server.ts"]
}
```

- [ ] **Step 4: Create types.ts**

```typescript
// src/types.ts

export interface ProjectConfig {
  name: string;
  path: string;
  channels: string[]; // Discord channel snowflake IDs
}

export interface OrchestratorConfig {
  projects: Record<string, ProjectConfig>;
  ops_channel: string;
  idle_timeout_ms: number;
}

export interface SessionState {
  sessionId: string;
  lastActiveChannel: string;
  lastActivity: string; // ISO 8601
}

export interface SessionsState {
  [projectName: string]: SessionState;
}

export interface ActiveSession {
  projectName: string;
  sessionId: string;
  lastActiveChannel: string;
  abortController: AbortController;
  idleTimer: ReturnType<typeof setTimeout>;
  autoApproved: Set<string>; // tool names auto-approved for this session
  lastActivityAt: number; // Date.now() — used for idle time display
}
```

- [ ] **Step 5: Install dependencies**

Run: `cd plugins/discord-orchestrator && bun install`
Expected: Lockfile created, node_modules populated

- [ ] **Step 6: Commit**

```bash
git add plugins/discord-orchestrator/.claude-plugin/plugin.json \
       plugins/discord-orchestrator/package.json \
       plugins/discord-orchestrator/tsconfig.json \
       plugins/discord-orchestrator/src/types.ts \
       plugins/discord-orchestrator/bun.lockb
git commit -m "feat(discord-orchestrator): scaffold plugin with types and deps"
```

---

### Task 2: Config Manager

**Files:**
- Create: `plugins/discord-orchestrator/src/config.ts`
- Reference: `plugins/discord-orchestrator/src/types.ts`

- [ ] **Step 1: Write config.ts**

Implements:
- `STATE_DIR` = `~/.discord-orchestrator/`
- `loadConfig(): OrchestratorConfig` — reads `projects.json`, returns defaults if missing
- `saveConfig(config: OrchestratorConfig): void` — writes `projects.json` atomically (write to `.tmp`, rename)
- `loadSessions(): SessionsState` — reads `sessions.json`, returns `{}` if missing
- `saveSessions(sessions: SessionsState): void` — writes `sessions.json` atomically
- `buildChannelMap(config: OrchestratorConfig): Map<string, string>` — reverse lookup: channelId → projectName
- `loadBotToken(): string` — reads `DISCORD_BOT_TOKEN` from `STATE_DIR/.env`
- `ensureStateDir(): void` — creates `~/.discord-orchestrator/` if missing

```typescript
// src/config.ts
import { readFileSync, writeFileSync, mkdirSync, existsSync, renameSync } from "fs";
import { join } from "path";
import { homedir } from "os";
import type { OrchestratorConfig, SessionsState } from "./types";

export const STATE_DIR = join(homedir(), ".discord-orchestrator");
const PROJECTS_FILE = join(STATE_DIR, "projects.json");
const SESSIONS_FILE = join(STATE_DIR, "sessions.json");
const ENV_FILE = join(STATE_DIR, ".env");

const DEFAULT_CONFIG: OrchestratorConfig = {
  projects: {},
  ops_channel: "",
  idle_timeout_ms: 3600000,
};

export function ensureStateDir(): void {
  if (!existsSync(STATE_DIR)) {
    mkdirSync(STATE_DIR, { recursive: true, mode: 0o700 });
  }
}

export function loadConfig(): OrchestratorConfig {
  ensureStateDir();
  if (!existsSync(PROJECTS_FILE)) return { ...DEFAULT_CONFIG };
  const raw = readFileSync(PROJECTS_FILE, "utf-8");
  return { ...DEFAULT_CONFIG, ...JSON.parse(raw) };
}

export function saveConfig(config: OrchestratorConfig): void {
  ensureStateDir();
  const tmp = PROJECTS_FILE + ".tmp";
  writeFileSync(tmp, JSON.stringify(config, null, 2), { mode: 0o600 });
  renameSync(tmp, PROJECTS_FILE);
}

export function loadSessions(): SessionsState {
  ensureStateDir();
  if (!existsSync(SESSIONS_FILE)) return {};
  const raw = readFileSync(SESSIONS_FILE, "utf-8");
  return JSON.parse(raw);
}

export function saveSessions(sessions: SessionsState): void {
  ensureStateDir();
  const tmp = SESSIONS_FILE + ".tmp";
  writeFileSync(tmp, JSON.stringify(sessions, null, 2), { mode: 0o600 });
  renameSync(tmp, SESSIONS_FILE);
}

export function buildChannelMap(config: OrchestratorConfig): Map<string, string> {
  const map = new Map<string, string>();
  for (const [name, project] of Object.entries(config.projects)) {
    for (const channelId of project.channels) {
      map.set(channelId, name);
    }
  }
  return map;
}

export function loadBotToken(): string {
  if (!existsSync(ENV_FILE)) {
    throw new Error(`Bot token not found. Create ${ENV_FILE} with DISCORD_BOT_TOKEN=your-token`);
  }
  const raw = readFileSync(ENV_FILE, "utf-8");
  const match = raw.match(/DISCORD_BOT_TOKEN=(.+)/);
  if (!match) throw new Error(`DISCORD_BOT_TOKEN not found in ${ENV_FILE}`);
  return match[1].trim();
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd plugins/discord-orchestrator && bun build src/config.ts --no-bundle --outdir /dev/null`
Expected: No type errors

- [ ] **Step 3: Commit**

```bash
git add plugins/discord-orchestrator/src/config.ts
git commit -m "feat(discord-orchestrator): add config manager for projects.json and sessions.json"
```

---

### Task 3: Response Poster

**Files:**
- Create: `plugins/discord-orchestrator/src/response-poster.ts`

- [ ] **Step 1: Write response-poster.ts**

Implements the response posting strategy from the spec:
- Collects streamed text into a buffer
- Posts when buffer reaches 1800 chars or stream ends
- Subsequent chunks reply to the first message
- Only posts assistant text blocks, not tool use

```typescript
// src/response-poster.ts
import type { TextChannel, Message } from "discord.js";

const MAX_CHUNK = 1800;

export class ResponsePoster {
  private buffer = "";
  private firstMessage: Message | null = null;
  private channel: TextChannel;

  constructor(channel: TextChannel) {
    this.channel = channel;
  }

  async addText(text: string): Promise<void> {
    this.buffer += text;
    if (this.buffer.length >= MAX_CHUNK) {
      await this.flush();
    }
  }

  async flush(): Promise<void> {
    while (this.buffer.length > 0) {
      const content = this.buffer.slice(0, 2000); // hard cap at Discord limit
      this.buffer = this.buffer.slice(2000);

      if (!this.firstMessage) {
        this.firstMessage = await this.channel.send(content);
      } else {
        await this.firstMessage.reply(content);
      }

      // Only continue flushing if buffer still exceeds threshold
      if (this.buffer.length < MAX_CHUNK) break;
    }
  }

  async finish(): Promise<void> {
    await this.flush();
    this.buffer = "";
    this.firstMessage = null;
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add plugins/discord-orchestrator/src/response-poster.ts
git commit -m "feat(discord-orchestrator): add response poster with Discord message batching"
```

---

### Task 4: Approval System

**Files:**
- Create: `plugins/discord-orchestrator/src/approval.ts`
- Reference: `plugins/discord-remote/src/discord-approver.ts` (for reaction collector pattern)

- [ ] **Step 1: Write approval.ts**

Implements the PermissionRequest → Discord reaction flow:
- `postApprovalAndWait(channel, toolName, toolInput, timeoutMs)` — posts embed, collects reaction, returns allow/deny
- Formats tool name + truncated input into an embed
- Adds ✅ ❌ 🔒 reactions
- Uses `createReactionCollector` with timeout
- Returns `{ decision: "allow" | "deny", autoApprove: boolean }`

```typescript
// src/approval.ts
import type { TextChannel, Message } from "discord.js";

const APPROVE = "✅";
const DENY = "❌";
const ALWAYS = "🔒";
const INPUT_TRUNCATE = 1500;

export interface ApprovalResult {
  decision: "allow" | "deny";
  autoApprove: boolean;
}

function formatToolInput(input: unknown): string {
  const str = typeof input === "string" ? input : JSON.stringify(input, null, 2);
  if (str.length > INPUT_TRUNCATE) {
    return str.slice(0, INPUT_TRUNCATE) + "\n... (truncated)";
  }
  return str;
}

export async function postApprovalAndWait(
  channel: TextChannel,
  toolName: string,
  toolInput: unknown,
  timeoutMs = 60_000,
): Promise<ApprovalResult> {
  const inputStr = formatToolInput(toolInput);

  const msg: Message = await channel.send({
    embeds: [
      {
        title: `🔐 Permission Request: ${toolName}`,
        description: `\`\`\`\n${inputStr}\n\`\`\``,
        color: 0xffa500, // orange
        footer: {
          text: `${APPROVE} Allow  ${DENY} Deny  ${ALWAYS} Always allow this tool | Timeout: ${Math.round(timeoutMs / 1000)}s`,
        },
      },
    ],
  });

  await msg.react(APPROVE);
  await msg.react(DENY);
  await msg.react(ALWAYS);

  return new Promise<ApprovalResult>((resolve) => {
    const collector = msg.createReactionCollector({
      filter: (reaction, user) => {
        if (user.bot) return false;
        const emoji = reaction.emoji.name;
        return emoji === APPROVE || emoji === DENY || emoji === ALWAYS;
      },
      max: 1,
      time: timeoutMs,
    });

    collector.on("collect", (reaction) => {
      const emoji = reaction.emoji.name;
      if (emoji === ALWAYS) {
        resolve({ decision: "allow", autoApprove: true });
      } else if (emoji === APPROVE) {
        resolve({ decision: "allow", autoApprove: false });
      } else {
        resolve({ decision: "deny", autoApprove: false });
      }
    });

    collector.on("end", (collected) => {
      if (collected.size === 0) {
        // Timeout — deny by default
        resolve({ decision: "deny", autoApprove: false });
      }
    });
  });
}
```

- [ ] **Step 2: Commit**

```bash
git add plugins/discord-orchestrator/src/approval.ts
git commit -m "feat(discord-orchestrator): add approval system with Discord reaction collector"
```

---

### Task 5: SDK Verification

**Files:**
- Create: `plugins/discord-orchestrator/scripts/verify-sdk.ts` (temporary, deleted after verification)

Before building the session manager, we must verify the actual SDK API surface. The `@anthropic-ai/claude-agent-sdk` package's exact `query()` signature, stream message shapes, and hook callback format are not guaranteed to match documentation.

- [ ] **Step 1: Write a minimal SDK test script**

```typescript
// scripts/verify-sdk.ts
// Temporary — run once to verify API shapes, then delete
import * as sdk from "@anthropic-ai/claude-agent-sdk";

console.log("SDK exports:", Object.keys(sdk));
console.log("query type:", typeof sdk.query);

// Test basic query
const stream = sdk.query({
  prompt: "Say 'hello' and nothing else",
  options: {
    cwd: process.cwd(),
    model: "claude-haiku-4-5-20251001",
    maxTurns: 1,
  },
});

for await (const msg of stream) {
  console.log("MSG TYPE:", msg.type, "SUBTYPE:", (msg as any).subtype);
  console.log("KEYS:", Object.keys(msg));

  if (msg.type === "system") {
    console.log("SESSION_ID:", (msg as any).session_id);
  }
  if (msg.type === "assistant") {
    console.log("CONTENT:", JSON.stringify((msg as any).message?.content, null, 2));
  }
  if (msg.type === "result") {
    console.log("RESULT:", JSON.stringify(msg, null, 2));
  }
}
```

- [ ] **Step 2: Run the verification**

Run: `cd plugins/discord-orchestrator && bun scripts/verify-sdk.ts`
Expected: Logs showing actual export names, message types, and content shapes.
**Record the actual shapes** — adapt session-manager.ts accordingly in the next task.

- [ ] **Step 3: Test resume capability**

Add to the script: after the first query completes, capture the `session_id` and run a second query with `options.resume`:

```typescript
// Append to verify-sdk.ts for resume test:
const stream2 = sdk.query({
  prompt: "What was my previous message?",
  options: {
    resume: capturedSessionId,
    model: "claude-haiku-4-5-20251001",
    maxTurns: 1,
  },
});
```

Verify that the second query sees the first conversation's context.

- [ ] **Step 4: Test hooks**

Add to the script: pass a `PermissionRequest` hook callback and trigger it by requesting a tool use:

```typescript
const stream3 = sdk.query({
  prompt: "Read the file ./package.json",
  options: {
    cwd: process.cwd(),
    model: "claude-haiku-4-5-20251001",
    hooks: {
      PermissionRequest: [{
        matcher: "",
        hooks: [async (input: any) => {
          console.log("HOOK FIRED — input keys:", Object.keys(input));
          console.log("HOOK INPUT:", JSON.stringify(input, null, 2));
          return {
            hookSpecificOutput: {
              hookEventName: "PermissionRequest",
              permissionDecision: "allow",
              permissionDecisionReason: "test",
            },
          };
        }],
      }],
    },
  },
});
```

Verify that the hook fires and log the exact `input` shape.

- [ ] **Step 5: Delete verification script, commit findings**

Delete `scripts/verify-sdk.ts`. Update `src/types.ts` if the SDK message types differ from what we assumed. Add a comment in `types.ts` documenting the verified API shapes.

```bash
rm -rf plugins/discord-orchestrator/scripts
git add plugins/discord-orchestrator/src/types.ts
git commit -m "chore(discord-orchestrator): verified SDK API shapes, updated types"
```

---

### Task 6: Session Manager

**Files:**
- Create: `plugins/discord-orchestrator/src/session-manager.ts`
- Reference: `plugins/discord-orchestrator/src/types.ts`
- Reference: `plugins/discord-orchestrator/src/config.ts`
- Reference: `plugins/discord-orchestrator/src/approval.ts`
- Reference: `plugins/discord-orchestrator/src/response-poster.ts`

This is the most complex component. Adapt the code below based on SDK verification findings from Task 5.

- [ ] **Step 1: Write session-manager.ts**

Implements:
- `SessionManager` class that holds a `Map<string, ActiveSession>` (keyed by project name)
- `sendMessage(projectName, text, channel)` — sends text to session via `query()`, streams response to Discord
- Each `query()` call uses `resume: sessionId` to maintain conversation continuity (one `query()` per Discord message, SDK handles session persistence internally)
- `closeSession(projectName)` — aborts session, saves state, removes from active map
- `closeAll()` — closes all sessions (for shutdown)
- `resetIdleTimer(projectName)` — resets the 60-min timer
- `getActiveSessions()` — returns session info for status display
- PermissionRequest hook: dynamically looks up `lastActiveChannel` from session map (NOT from closure) to always post to the correct channel

```typescript
// src/session-manager.ts
import { query } from "@anthropic-ai/claude-agent-sdk";
import { v5 as uuidv5 } from "uuid";
import type { Client, TextChannel } from "discord.js";
import type { ActiveSession, ProjectConfig, SessionsState } from "./types";
import { loadSessions, saveSessions } from "./config";
import { postApprovalAndWait } from "./approval";
import { ResponsePoster } from "./response-poster";

const SESSION_NAMESPACE = "6ba7b810-9dad-11d1-80b4-00c04fd430c8";

export class SessionManager {
  private sessions = new Map<string, ActiveSession>();
  private savedSessions: SessionsState;
  private idleTimeoutMs: number;
  private saveQueue: Promise<void> = Promise.resolve();
  // Shared reference so hooks can dynamically resolve channels
  private discordClient: Client | null = null;

  constructor(idleTimeoutMs: number) {
    this.idleTimeoutMs = idleTimeoutMs;
    this.savedSessions = loadSessions();
  }

  setDiscordClient(client: Client): void {
    this.discordClient = client;
  }

  private queueSave(): void {
    this.saveQueue = this.saveQueue.then(() => {
      const state: SessionsState = {};
      for (const [name, session] of this.sessions) {
        state[name] = {
          sessionId: session.sessionId,
          lastActiveChannel: session.lastActiveChannel,
          lastActivity: new Date().toISOString(),
        };
      }
      for (const [name, saved] of Object.entries(this.savedSessions)) {
        if (!state[name]) state[name] = saved;
      }
      saveSessions(state);
    });
  }

  private buildPermissionHook(projectName: string) {
    // Capture `this` (SessionManager) and `projectName` — NOT a specific channel.
    // The hook dynamically looks up lastActiveChannel at invocation time.
    const manager = this;
    return {
      matcher: "",
      hooks: [
        async (input: any) => {
          const session = manager.sessions.get(projectName);
          if (!session || !manager.discordClient) {
            return {
              hookSpecificOutput: {
                hookEventName: "PermissionRequest",
                permissionDecision: "deny",
                permissionDecisionReason: "No active session or Discord client",
              },
            };
          }

          const toolName: string = input?.tool_name ?? "unknown";

          // Check auto-approve set first
          if (session.autoApproved.has(toolName)) {
            return {
              hookSpecificOutput: {
                hookEventName: "PermissionRequest",
                permissionDecision: "allow",
                permissionDecisionReason: `Auto-approved: ${toolName}`,
              },
            };
          }

          // Dynamically resolve the CURRENT active channel
          const channel = await manager.discordClient.channels.fetch(
            session.lastActiveChannel,
          ) as TextChannel;

          const result = await postApprovalAndWait(
            channel,
            toolName,
            input?.tool_input,
          );

          if (result.autoApprove) {
            session.autoApproved.add(toolName);
          }

          return {
            hookSpecificOutput: {
              hookEventName: "PermissionRequest",
              permissionDecision: result.decision,
              permissionDecisionReason:
                result.decision === "allow"
                  ? "Approved via Discord"
                  : "Denied via Discord",
            },
          };
        },
      ],
    };
  }

  async sendMessage(
    projectName: string,
    project: ProjectConfig,
    text: string,
    channel: TextChannel,
  ): Promise<void> {
    let session = this.sessions.get(projectName);

    // Update last active channel (for existing sessions)
    if (session) {
      session.lastActiveChannel = channel.id;
      session.lastActivityAt = Date.now();
      this.resetIdleTimer(projectName);
      this.queueSave();
    }

    const sessionId = uuidv5(projectName, SESSION_NAMESPACE);
    const resumeId = session?.sessionId
      ?? this.savedSessions[projectName]?.sessionId;

    const poster = new ResponsePoster(channel);
    const abortController = session?.abortController ?? new AbortController();

    // Build query options — each message is a new query() call.
    // The SDK maintains conversation continuity via sessionId/resume.
    const options: Record<string, any> = {
      cwd: project.path,
      abortController,
      model: "claude-sonnet-4-6",
      settingSources: ["project"],
      hooks: {
        PermissionRequest: [this.buildPermissionHook(projectName)],
      },
    };

    // Resume existing session or start fresh
    if (resumeId) {
      options.resume = resumeId;
    } else {
      options.sessionId = sessionId;
    }

    // Register session if new
    if (!session) {
      const idleTimer = setTimeout(
        () => this.closeSession(projectName),
        this.idleTimeoutMs,
      );
      session = {
        projectName,
        sessionId,
        lastActiveChannel: channel.id,
        abortController,
        idleTimer,
        autoApproved: new Set<string>(),
        lastActivityAt: Date.now(),
      };
      this.sessions.set(projectName, session);
      this.queueSave();
    }

    try {
      const stream = query({ prompt: text, options });

      for await (const msg of stream) {
        // Capture actual session ID from init message
        if (msg.type === "system" && (msg as any).subtype === "init") {
          const sid = (msg as any).session_id;
          if (sid && session) {
            session.sessionId = sid;
            this.queueSave();
          }
        }

        // Post assistant text to Discord
        if (msg.type === "assistant" && (msg as any).message?.content) {
          for (const block of (msg as any).message.content) {
            if (block.type === "text" && block.text) {
              await poster.addText(block.text);
            }
          }
        }

        // On result, flush remaining text
        if (msg.type === "result") {
          await poster.finish();
        }
      }
    } catch (err: any) {
      if (err.name === "AbortError") return;
      await channel.send(`⚠️ Session error: ${err.message}`);
      this.sessions.delete(projectName);
    }
  }

  private resetIdleTimer(projectName: string): void {
    const session = this.sessions.get(projectName);
    if (!session) return;
    clearTimeout(session.idleTimer);
    session.idleTimer = setTimeout(
      () => this.closeSession(projectName),
      this.idleTimeoutMs,
    );
  }

  async closeSession(projectName: string): Promise<void> {
    const session = this.sessions.get(projectName);
    if (!session) return;

    clearTimeout(session.idleTimer);
    session.abortController.abort();

    this.savedSessions[projectName] = {
      sessionId: session.sessionId,
      lastActiveChannel: session.lastActiveChannel,
      lastActivity: new Date().toISOString(),
    };
    this.sessions.delete(projectName);
    this.queueSave();
  }

  async closeAll(): Promise<void> {
    const names = [...this.sessions.keys()];
    await Promise.all(names.map((n) => this.closeSession(n)));
  }

  getActiveSessions(): Array<{
    projectName: string;
    lastActiveChannel: string;
    idleMinutes: number;
  }> {
    const now = Date.now();
    return [...this.sessions.entries()].map(([name, s]) => ({
      projectName: name,
      lastActiveChannel: s.lastActiveChannel,
      idleMinutes: Math.round((now - s.lastActivityAt) / 60000),
    }));
  }

  hasSession(projectName: string): boolean {
    return this.sessions.has(projectName);
  }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd plugins/discord-orchestrator && bun build src/session-manager.ts --no-bundle --outdir /dev/null`
Expected: No type errors (may have warnings about `any` — acceptable until SDK types are verified)

- [ ] **Step 3: Commit**

```bash
git add plugins/discord-orchestrator/src/session-manager.ts
git commit -m "feat(discord-orchestrator): add session manager with spawn, timeout, resume, and approval"
```

---

### Task 7: Discord Bot (Message Router)

**Files:**
- Create: `plugins/discord-orchestrator/src/bot.ts`

- [ ] **Step 1: Write bot.ts**

The central message router. Handles:
- `messageCreate` event dispatch: ops channel → ops session, mapped channel → project session, unmapped → ignore
- Bot message filtering (ignore self)
- Typing indicator while session processes

```typescript
// src/bot.ts
import {
  Client,
  GatewayIntentBits,
  Partials,
  type TextChannel,
  type Message,
} from "discord.js";
import { loadConfig, buildChannelMap } from "./config";
import { SessionManager } from "./session-manager";
import type { OrchestratorConfig } from "./types";

// Typed callback for ops messages — avoids untyped event emission on Client
export type OpsMessageHandler = (msg: Message) => Promise<void>;

export class OrchestratorBot {
  public client: Client;
  public sessionManager: SessionManager;
  private config: OrchestratorConfig;
  private channelMap: Map<string, string>;
  private opsHandler: OpsMessageHandler | null = null;

  constructor() {
    this.config = loadConfig();
    this.channelMap = buildChannelMap(this.config);
    this.sessionManager = new SessionManager(this.config.idle_timeout_ms);

    this.client = new Client({
      intents: [
        GatewayIntentBits.Guilds,
        GatewayIntentBits.GuildMessages,
        GatewayIntentBits.MessageContent,
        GatewayIntentBits.GuildMessageReactions,
        GatewayIntentBits.DirectMessages,
        GatewayIntentBits.DirectMessageReactions,
      ],
      partials: [Partials.Channel, Partials.Message, Partials.Reaction],
    });

    this.client.on("messageCreate", (msg) => this.handleMessage(msg));
    this.client.on("ready", () => {
      this.sessionManager.setDiscordClient(this.client);
      console.log(`[orchestrator] Bot ready as ${this.client.user?.tag}`);
      console.log(`[orchestrator] ${Object.keys(this.config.projects).length} projects registered`);
      console.log(`[orchestrator] Ops channel: ${this.config.ops_channel || "(not set)"}`);
    });
  }

  /** Register the ops channel message handler (called from server.ts) */
  onOpsMessage(handler: OpsMessageHandler): void {
    this.opsHandler = handler;
  }

  rebuildChannelMap(): void {
    this.config = loadConfig();
    this.channelMap = buildChannelMap(this.config);
  }

  private async handleMessage(msg: Message): Promise<void> {
    if (msg.author.bot) return;

    const channelId = msg.channel.id;

    // Ops channel — delegate to registered handler
    if (channelId === this.config.ops_channel) {
      if (this.opsHandler) await this.opsHandler(msg);
      return;
    }

    // Mapped project channel
    const projectName = this.channelMap.get(channelId);
    if (!projectName) return;

    const project = this.config.projects[projectName];
    if (!project) return;

    const channel = msg.channel as TextChannel;
    await channel.sendTyping();

    try {
      await this.sessionManager.sendMessage(
        projectName,
        project,
        msg.content,
        channel,
      );
    } catch (err: any) {
      console.error(`[orchestrator] Error in ${projectName}:`, err);
      await channel.send(`⚠️ Error: ${err.message}`);
    }
  }

  async start(token: string): Promise<void> {
    await this.client.login(token);
  }

  async shutdown(): Promise<void> {
    await this.sessionManager.closeAll();
    this.client.destroy();
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add plugins/discord-orchestrator/src/bot.ts
git commit -m "feat(discord-orchestrator): add Discord bot with channel-to-project message routing"
```

---

### Task 8: Entry Point + Ops Channel

**Files:**
- Create: `plugins/discord-orchestrator/server.ts`
- Modify: `plugins/discord-orchestrator/src/config.ts` (add `ensureProjectsFile`)

This task wires everything together including the ops channel. For v0.1, the ops channel uses a SDK `query()` session with `systemPrompt` describing the `projects.json` schema — the SDK session edits the file directly. The orchestrator watches `projects.json` for changes and rebuilds the channel map. No MCP server needed for v0.1.

- [ ] **Step 1: Add `ensureProjectsFile` to config.ts**

`fs.watch` throws if the file doesn't exist. Create an empty `projects.json` on first run:

```typescript
// Add to config.ts, call from ensureStateDir():
export function ensureStateDir(): void {
  if (!existsSync(STATE_DIR)) {
    mkdirSync(STATE_DIR, { recursive: true, mode: 0o700 });
  }
  // Ensure projects.json exists so fs.watch doesn't throw
  if (!existsSync(PROJECTS_FILE)) {
    writeFileSync(PROJECTS_FILE, JSON.stringify(DEFAULT_CONFIG, null, 2), { mode: 0o600 });
  }
}
```

- [ ] **Step 2: Write server.ts**

```typescript
// server.ts
import { query } from "@anthropic-ai/claude-agent-sdk";
import { OrchestratorBot } from "./src/bot";
import { loadBotToken, loadConfig, ensureStateDir, STATE_DIR } from "./src/config";
import { ResponsePoster } from "./src/response-poster";
import { watch } from "fs";
import { join } from "path";
import type { TextChannel, Message } from "discord.js";

ensureStateDir();

const bot = new OrchestratorBot();

// Graceful shutdown
const shutdown = async () => {
  console.log("[orchestrator] Shutting down...");
  await bot.shutdown();
  process.exit(0);
};

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
process.on("unhandledRejection", (err) => {
  console.error("[orchestrator] Unhandled rejection:", err);
});

// Watch config for changes from ops session
let watchDebounce: ReturnType<typeof setTimeout> | null = null;
watch(join(STATE_DIR, "projects.json"), { persistent: false }, () => {
  // Debounce — atomic writes trigger multiple events
  if (watchDebounce) clearTimeout(watchDebounce);
  watchDebounce = setTimeout(() => {
    console.log("[orchestrator] Config changed, rebuilding channel map");
    bot.rebuildChannelMap();
  }, 200);
});

// Ops channel handler — uses resume for multi-turn conversation continuity
const OPS_SESSION_NAMESPACE = "6ba7b810-9dad-11d1-80b4-00c04fd430c9"; // different from project namespace
let opsSessionId: string | null = null; // captured from first init message

bot.onOpsMessage(async (msg: Message) => {
  const channel = msg.channel as TextChannel;
  await channel.sendTyping();

  const poster = new ResponsePoster(channel);

  const opsSystemPrompt = `You are the admin assistant for a Discord orchestrator.
You manage projects and channel bindings by editing the config file at ${STATE_DIR}/projects.json.

Current config:
${JSON.stringify(loadConfig(), null, 2)}

The config schema:
- projects: Record<name, { name, path, channels: string[] }>
- ops_channel: string (this channel's ID)
- idle_timeout_ms: number

To register a project: add an entry to projects with name, path (validate path exists with Read on the directory), and empty channels array.
To bind a channel: add the channel ID string to the project's channels array.
To unbind: remove the channel ID from the channels array.
To unregister: delete the project entry.

Always read the file first, modify, then write back. Use the Edit tool for surgical changes.`;

  const options: Record<string, any> = {
    cwd: STATE_DIR,
    systemPrompt: opsSystemPrompt,
    model: "claude-sonnet-4-6",
    allowedTools: ["Read", "Write", "Edit"],
    permissionMode: "bypassPermissions",
    allowDangerouslySkipPermissions: true,
  };

  // Resume previous ops conversation if we have a session ID
  if (opsSessionId) {
    options.resume = opsSessionId;
  }

  try {
    const stream = query({ prompt: msg.content, options });

    for await (const m of stream) {
      // Capture session ID for future resume
      if (m.type === "system" && (m as any).subtype === "init" && (m as any).session_id) {
        opsSessionId = (m as any).session_id;
      }

      if (m.type === "assistant" && (m as any).message?.content) {
        for (const block of (m as any).message.content) {
          if (block.type === "text" && block.text) {
            await poster.addText(block.text);
          }
        }
      }
      if (m.type === "result") {
        await poster.finish();
      }
    }
  } catch (err: any) {
    await channel.send(`⚠️ Ops error: ${err.message}`);
    // Reset ops session on error — next message starts fresh
    opsSessionId = null;
  }
});

// Start
const token = loadBotToken();
console.log("[orchestrator] Starting...");
await bot.start(token);
```

- [ ] **Step 3: Test the bot starts**

Run: `cd plugins/discord-orchestrator && bun server.ts`
Expected: If `.env` exists with token → bot logs in and shows ready message. If no token → clear error message. Ctrl+C → graceful shutdown.

- [ ] **Step 4: Test ops channel**

Send a message in the configured ops channel: "Register a project called test-project at /tmp"
Expected: Bot responds naturally, `projects.json` is updated, channel map rebuilds.

- [ ] **Step 5: Commit**

```bash
git add plugins/discord-orchestrator/server.ts \
       plugins/discord-orchestrator/src/config.ts
git commit -m "feat(discord-orchestrator): add entry point with ops channel and config watch"
```

---

### Task 9: Configure Skill + Marketplace Registration

**Files:**
- Create: `plugins/discord-orchestrator/skills/configure/SKILL.md`
- Modify: `.claude-plugin/marketplace.json`

- [ ] **Step 1: Create configure skill**

```markdown
---
name: configure
description: Set up the Discord orchestrator — save bot token and configure ops channel
user_invocable: true
---

# Discord Orchestrator Setup

Help the user set up the Discord orchestrator.

## Steps

1. Ask for the Discord bot token if not already saved
2. Save it to `~/.discord-orchestrator/.env` as `DISCORD_BOT_TOKEN=<token>` with 0600 permissions
3. Ask for the ops channel ID (the Discord channel where admin commands will go)
4. Update `~/.discord-orchestrator/projects.json` with the `ops_channel` value
5. Confirm setup is complete and explain how to start the bot: `cd plugins/discord-orchestrator && bun server.ts`

## Notes

- The bot token file must have 0600 permissions (owner read/write only)
- Create `~/.discord-orchestrator/` directory if it doesn't exist
- Never display the full bot token back to the user
```

- [ ] **Step 2: Register in marketplace.json**

Read the current marketplace.json, add the discord-orchestrator entry:

```json
{
  "name": "discord-orchestrator",
  "source": "./plugins/discord-orchestrator",
  "description": "Multi-project Discord orchestrator — routes channels to projects via Claude Agent SDK"
}
```

- [ ] **Step 3: Commit**

```bash
git add plugins/discord-orchestrator/skills/configure/SKILL.md \
       .claude-plugin/marketplace.json
git commit -m "feat(discord-orchestrator): add configure skill and marketplace registration"
```

---

### Task 10: End-to-End Integration Test

**Files:** None created — manual testing

- [ ] **Step 1: Ensure token is configured**

Run: `cat ~/.discord-orchestrator/.env`
Expected: Contains `DISCORD_BOT_TOKEN=...`
If missing: Create it manually

- [ ] **Step 2: Configure ops channel**

Ensure `~/.discord-orchestrator/projects.json` has `ops_channel` set to a real Discord channel ID.

- [ ] **Step 3: Start the bot**

Run: `cd plugins/discord-orchestrator && bun server.ts`
Expected: Bot logs in, shows ready message with project count

- [ ] **Step 4: Test ops — register a project**

In the ops channel, type: "Register a project called lizard-market at /home/shotup/programing/ai-agents/lizard-market"
Expected: Bot confirms registration, `projects.json` updated

- [ ] **Step 5: Test ops — bind a channel**

In the ops channel, type: "Bind lizard-market to #test-channel" (use actual channel name/ID)
Expected: Bot confirms binding

- [ ] **Step 6: Test project channel — send a message**

In the bound channel, type: "What files are in this project?"
Expected: Bot spawns a Claude Code session, streams response showing project files

- [ ] **Step 7: Test permission approval**

In the bound channel, type: "Create a file called /tmp/test-orchestrator.txt with the content 'hello'"
Expected: Bot posts a permission approval embed with ✅ ❌ 🔒. Click ✅ → file is created, response posted.

- [ ] **Step 8: Test idle timeout**

Wait 60 minutes (or temporarily set `idle_timeout_ms` to 60000 for testing).
Expected: Session closes, next message spawns a new session (or resumes)

- [ ] **Step 9: Test bot restart resume**

Ctrl+C the bot, then restart with `bun server.ts`.
Send a message in the bound channel.
Expected: Session resumes from prior conversation context.
