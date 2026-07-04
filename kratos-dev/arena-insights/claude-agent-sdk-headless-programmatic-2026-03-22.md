# Claude Agent SDK — Headless & Programmatic Use Research

## Metadata
| Field | Value |
|-------|-------|
| **Researched** | 2026-03-22 |
| **TTL** | 14 days |
| **Query** | Claude Code headless mode, Agent SDK, non-interactive permissions, hooks in SDK mode, MCP/plugins in SDK mode, concurrent sessions, Discord bot orchestrator design, unstable_v2_createSession exact API signature |
| **Researcher** | Mimir |
| **Cache Until** | 2026-04-05 |

## Summary

The Claude Agent SDK (`@anthropic-ai/claude-agent-sdk`) ships two APIs. V1 (`query()`) is stable and covers single-turn and multi-turn via async generators with the full `Options` config object — accepting `cwd`, `hooks`, `sessionId`, `allowedTools`, `plugins`, `mcpServers`, `resume`, `abortController`, and ~35 more options. V2 (`unstable_v2_createSession`) is a preview with a simplified send/stream pattern for multi-turn conversations; its constructor currently only formally documents `model: string` with "additional options supported" — it does NOT expose the full `Options` surface (no `cwd`, `hooks`, etc.) in the current preview spec. The `session.send()` method accepts either a plain string or an `SDKUserMessage` object. `session.stream()` yields `SDKMessage` union items identified by a `type` discriminant. Session resume uses `unstable_v2_resumeSession(sessionId, opts)`. Cleanup uses `session.close()` or `await using`.

## Key Findings

### 1. V2 API — `unstable_v2_createSession`

**Source**: https://platform.claude.com/docs/en/agent-sdk/typescript-v2-preview

**Constructor signature:**

```typescript
function unstable_v2_createSession(options: {
  model: string;
  // "Additional options supported" per docs — no further fields formally specified
}): SDKSession;
```

**CRITICAL NOTE:** The V2 preview API only formally documents `model` as the constructor parameter. The docs state "Additional options supported" without listing them. Based on CHANGELOG entries and usage patterns, the intent is that `cwd`, `permissionMode`, `allowDangerouslySkipPermissions`, `sessionId`, `plugins`, `mcpServers`, `settingSources`, `hooks`, `abortController` etc. from V1 `Options` should also work — but they are NOT guaranteed stable or documented in V2. If you need those options, use V1 `query()` or pass them experimentally.

**`SDKSession` interface (exact):**

```typescript
interface SDKSession {
  readonly sessionId: string;
  send(message: string | SDKUserMessage): Promise<void>;
  stream(): AsyncGenerator<SDKMessage, void>;
  close(): void;
}
```

**`session.send()` — accepts:**
- Plain string: `await session.send("Hello!")`
- Or an `SDKUserMessage` object (full structured user message)

**`session.stream()` — yields `SDKMessage` union. Key message types by `msg.type`:**

| `msg.type` | Sub-discriminant | Description |
|-----------|-----------------|-------------|
| `"assistant"` | — | Claude's response. `msg.message.content` is a `BetaMessage` content array |
| `"user"` | — | Echoed user message (`SDKUserMessage`) |
| `"result"` | `msg.subtype: "success"` | Final result. Has `msg.result: string`, `msg.total_cost_usd`, `msg.num_turns`, `msg.usage` |
| `"result"` | `msg.subtype: "error_max_turns" \| "error_during_execution" \| "error_max_budget_usd" \| "error_max_structured_output_retries"` | Error result. Has `msg.errors: string[]` instead of `msg.result` |
| `"system"` | `msg.subtype: "init"` | Init event. Has `msg.session_id`, `msg.cwd`, `msg.model`, `msg.tools`, `msg.plugins`, etc. |
| `"system"` | `msg.subtype: "compact_boundary"` | Conversation compaction occurred |
| `"stream_event"` | — | Raw streaming event (only when `includePartialMessages: true`) |

**Extracting text from assistant messages:**

```typescript
if (msg.type === "assistant") {
  const text = msg.message.content
    .filter((block) => block.type === "text")
    .map((block) => block.text)
    .join("");
  console.log(text);
}
```

**Getting session ID from any message:**

```typescript
let sessionId: string | undefined;
for await (const msg of session.stream()) {
  sessionId = msg.session_id; // present on all message types
}
```

### 2. V2 Resume — `unstable_v2_resumeSession`

```typescript
function unstable_v2_resumeSession(
  sessionId: string,
  options: {
    model: string;
    // Additional options supported
  }
): SDKSession;
```

Usage pattern:

```typescript
import {
  unstable_v2_createSession,
  unstable_v2_resumeSession,
  type SDKMessage
} from "@anthropic-ai/claude-agent-sdk";

// First session — capture ID
const session = unstable_v2_createSession({ model: "claude-sonnet-4-6" });
await session.send("Remember this number: 42");
let sessionId: string | undefined;
for await (const msg of session.stream()) {
  sessionId = msg.session_id;
}
session.close();

// Later — resume
await using resumed = unstable_v2_resumeSession(sessionId!, { model: "claude-sonnet-4-6" });
await resumed.send("What number did I ask you to remember?");
for await (const msg of resumed.stream()) {
  if (msg.type === "assistant") { /* handle */ }
}
```

### 3. V2 Close / Abort

**Manual close:**
```typescript
const session = unstable_v2_createSession({ model: "claude-sonnet-4-6" });
// ... use ...
session.close(); // terminates underlying process, cleans up resources
```

**Automatic close via `await using` (TypeScript 5.2+):**
```typescript
await using session = unstable_v2_createSession({ model: "claude-sonnet-4-6" });
// session.close() called automatically when block exits
```

**AbortController:** The V2 `SDKSession` interface does NOT expose `AbortController` directly. `AbortController` is a V1 `Options` field. In V2, use `session.close()` for cancellation. If mid-stream abort is needed, break out of the `for await` loop and then call `session.close()`.

### 4. V2 One-Shot — `unstable_v2_prompt`

```typescript
function unstable_v2_prompt(
  prompt: string,
  options: {
    model: string;
    // Additional options supported
  }
): Promise<SDKResultMessage>;
```

Returns the final `SDKResultMessage` directly (no streaming). Check `result.subtype === "success"` before using `result.result`.

### 5. V1 `query()` — Full Options Reference

**Source**: https://platform.claude.com/docs/en/agent-sdk/typescript

```typescript
function query({
  prompt,
  options
}: {
  prompt: string | AsyncIterable<SDKUserMessage>;
  options?: Options;
}): Query;
```

**Complete `Options` interface:**

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `abortController` | `AbortController` | new AbortController() | Cancel mid-session |
| `additionalDirectories` | `string[]` | `[]` | Extra dirs Claude can access |
| `agent` | `string` | undefined | Agent name for main thread |
| `agents` | `Record<string, AgentDefinition>` | undefined | Programmatic subagents |
| `allowDangerouslySkipPermissions` | `boolean` | `false` | Required for `bypassPermissions` mode |
| `allowedTools` | `string[]` | `[]` | Pre-approve tools (does not restrict; unlisted fall to permissionMode) |
| `betas` | `SdkBeta[]` | `[]` | Beta features (e.g., `['context-1m-2025-08-07']`) |
| `canUseTool` | `CanUseTool` | undefined | Custom permission callback |
| `continue` | `boolean` | `false` | Continue most recent session |
| `cwd` | `string` | `process.cwd()` | Working directory per session |
| `debug` | `boolean` | `false` | Debug mode |
| `debugFile` | `string` | undefined | Write debug logs to file |
| `disallowedTools` | `string[]` | `[]` | Always deny — beats everything including bypassPermissions |
| `effort` | `'low' \| 'medium' \| 'high' \| 'max'` | `'high'` | Thinking depth |
| `enableFileCheckpointing` | `boolean` | `false` | Enable file rewind support |
| `env` | `Record<string, string \| undefined>` | `process.env` | Per-session env vars |
| `executable` | `'bun' \| 'deno' \| 'node'` | auto | JS runtime |
| `executableArgs` | `string[]` | `[]` | Args to runtime |
| `fallbackModel` | `string` | undefined | Model if primary fails |
| `forkSession` | `boolean` | `false` | Fork to new ID when resuming |
| `hooks` | `Partial<Record<HookEvent, HookCallbackMatcher[]>>` | `{}` | In-process hook callbacks |
| `includePartialMessages` | `boolean` | `false` | Emit stream_event messages |
| `maxBudgetUsd` | `number` | undefined | USD cost cap |
| `maxThinkingTokens` | `number` | undefined | Deprecated: use `thinking` |
| `maxTurns` | `number` | undefined | Max agentic turns |
| `mcpServers` | `Record<string, McpServerConfig>` | `{}` | MCP servers |
| `model` | `string` | CLI default | Model name |
| `outputFormat` | `{ type: 'json_schema', schema: JSONSchema }` | undefined | Structured output schema |
| `permissionMode` | `PermissionMode` | `'default'` | Permission behavior |
| `permissionPromptToolName` | `string` | undefined | MCP tool for permission prompts |
| `persistSession` | `boolean` | `true` | Disk persistence (false = no resume) |
| `plugins` | `SdkPluginConfig[]` | `[]` | Local plugins |
| `promptSuggestions` | `boolean` | `false` | Emit next-prompt predictions |
| `resume` | `string` | undefined | Session ID to resume |
| `resumeSessionAt` | `string` | undefined | Resume at specific message UUID |
| `sandbox` | `SandboxSettings` | undefined | Sandbox config |
| `sessionId` | `string` | auto-generated UUID | Bring-your-own session UUID |
| `settingSources` | `SettingSource[]` | `[]` | Load filesystem settings (must include `'project'` for CLAUDE.md) |
| `spawnClaudeCodeProcess` | function | undefined | Custom process spawn (containers/VMs) |
| `stderr` | `(data: string) => void` | undefined | Stderr callback |
| `strictMcpConfig` | `boolean` | `false` | Strict MCP validation |
| `systemPrompt` | `string \| { type: 'preset'; preset: 'claude_code'; append?: string }` | undefined | System prompt. Use preset form to get CLAUDE.md loaded |
| `thinking` | `ThinkingConfig` | `{ type: 'adaptive' }` | Thinking behavior |
| `toolConfig` | `ToolConfig` | undefined | Built-in tool config |
| `tools` | `string[] \| { type: 'preset'; preset: 'claude_code' }` | undefined | Tool set |

**`Query` object** (what `query()` returns) — extends `AsyncGenerator<SDKMessage, void>` and adds:

```typescript
interface Query extends AsyncGenerator<SDKMessage, void> {
  interrupt(): Promise<void>;
  rewindFiles(userMessageId: string, options?: { dryRun?: boolean }): Promise<RewindFilesResult>;
  setPermissionMode(mode: PermissionMode): Promise<void>;
  setModel(model?: string): Promise<void>;
  initializationResult(): Promise<SDKControlInitializeResponse>;
  supportedCommands(): Promise<SlashCommand[]>;
  supportedModels(): Promise<ModelInfo[]>;
  supportedAgents(): Promise<AgentInfo[]>;
  mcpServerStatus(): Promise<McpServerStatus[]>;
  accountInfo(): Promise<AccountInfo>;
  reconnectMcpServer(serverName: string): Promise<void>;
  toggleMcpServer(serverName: string, enabled: boolean): Promise<void>;
  setMcpServers(servers: Record<string, McpServerConfig>): Promise<McpSetServersResult>;
  streamInput(stream: AsyncIterable<SDKUserMessage>): Promise<void>;
  stopTask(taskId: string): Promise<void>;
  close(): void;
}
```

**V1 session resume pattern:**

```typescript
let sessionId: string | undefined;

// Turn 1 — capture session ID from system init message
for await (const message of query({
  prompt: "Read auth.ts",
  options: { allowedTools: ["Read", "Glob"] }
})) {
  if (message.type === "system" && message.subtype === "init") {
    sessionId = message.session_id;
  }
}

// Turn 2 — resume with full context
for await (const message of query({
  prompt: "Now find all callers",
  options: { resume: sessionId }
})) {
  if ("result" in message) console.log(message.result);
}
```

**V1 AbortController pattern:**

```typescript
const abort = new AbortController();
setTimeout(() => abort.abort(), 30_000); // 30s timeout

for await (const message of query({
  prompt: "Do long work",
  options: { abortController: abort }
})) { ... }
```

### 6. Full `SDKMessage` Union

```typescript
type SDKMessage =
  | SDKAssistantMessage       // type: "assistant"
  | SDKUserMessage            // type: "user"
  | SDKUserMessageReplay      // type: "user", isReplay: true
  | SDKResultMessage          // type: "result", subtype: "success" | "error_*"
  | SDKSystemMessage          // type: "system", subtype: "init"
  | SDKPartialAssistantMessage // type: "stream_event" (requires includePartialMessages)
  | SDKCompactBoundaryMessage  // type: "system", subtype: "compact_boundary"
  | SDKStatusMessage
  | SDKHookStartedMessage
  | SDKHookProgressMessage
  | SDKHookResponseMessage
  | SDKToolProgressMessage
  | SDKAuthStatusMessage
  | SDKTaskNotificationMessage
  | SDKTaskStartedMessage
  | SDKTaskProgressMessage
  | SDKFilesPersistedEvent
  | SDKToolUseSummaryMessage
  | SDKRateLimitEvent
  | SDKPromptSuggestionMessage;
```

All messages have `session_id: string` and `uuid: UUID` fields.

**`SDKResultMessage` shape (success):**

```typescript
{
  type: "result";
  subtype: "success";
  uuid: UUID;
  session_id: string;
  duration_ms: number;
  duration_api_ms: number;
  is_error: boolean;
  num_turns: number;
  result: string;           // The final text result
  stop_reason: string | null;
  total_cost_usd: number;
  usage: NonNullableUsage;
  modelUsage: { [modelName: string]: ModelUsage };
  permission_denials: SDKPermissionDenial[];
  structured_output?: unknown;
}
```

**`SDKSystemMessage` (init) shape:**

```typescript
{
  type: "system";
  subtype: "init";
  uuid: UUID;
  session_id: string;       // Capture this for resume
  agents?: string[];
  apiKeySource: ApiKeySource;
  betas?: string[];
  claude_code_version: string;
  cwd: string;
  tools: string[];
  mcp_servers: { name: string; status: string; }[];
  model: string;
  permissionMode: PermissionMode;
  slash_commands: string[];
  output_style: string;
  skills: string[];
  plugins: { name: string; path: string; }[];
}
```

### 7. Permission Modes

```typescript
type PermissionMode =
  | "default"           // Standard — falls through to canUseTool or interactive
  | "acceptEdits"       // Auto-accept file edits + filesystem commands
  | "bypassPermissions" // All tools auto-approved (hooks still run and can block)
  | "plan"              // No execution — planning only
  | "dontAsk";          // Deny anything not in allowedTools; no prompt (TypeScript only)
```

### 8. Hooks in SDK Mode

Available hook events: `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `Notification`, `UserPromptSubmit`, `SessionStart`, `SessionEnd`, `Stop`, `SubagentStart`, `SubagentStop`, `PreCompact`, `PermissionRequest`, `Setup`, `TeammateIdle`, `TaskCompleted`, `ConfigChange`, `WorktreeCreate`, `WorktreeRemove`

`PermissionRequest` hook for Discord-style remote approval:

```typescript
hooks: {
  PermissionRequest: [{
    hooks: [async (input, toolUseId, { signal }) => {
      return {
        hookSpecificOutput: {
          hookEventName: "PermissionRequest",
          permissionDecision: "allow", // or "deny" or "ask"
          permissionDecisionReason: "Auto-approved by Discord bot"
        }
      };
    }]
  }]
}
```

## Recommendations

For a Discord bot orchestrating multi-turn Claude sessions per channel:

1. **Use V1 `query()` with `cwd`, `plugins`, `hooks`, `sessionId`, `resume`** — V2 does not formally expose these in the constructor yet. V1 gives full control and is production-stable.

2. **Capture `session_id` from the `system/init` message** on first turn, store it keyed by Discord channel ID, and pass as `resume: storedId` on follow-up turns.

3. **V2 is the right direction** for new code written after V2 stabilizes, because `session.send()` / `session.stream()` is cleaner than managing input generators. Monitor CHANGELOG for when `cwd`, `hooks`, `plugins` are formally added to V2 constructor.

4. **`await using` requires TypeScript 5.2+** — fall back to `session.close()` in a `finally` block if not available.

5. **Session resume for bot restarts** — store `session_id` (UUID string visible on every message) in your persistence layer. Use `resume: storedId` in V1 or `unstable_v2_resumeSession(storedId, opts)` in V2 to reconnect after bot restart.

## Sources Consulted

- https://platform.claude.com/docs/en/agent-sdk/overview — SDK overview
- https://platform.claude.com/docs/en/agent-sdk/typescript — Full V1 TypeScript API reference (Options table, Query interface, all SDKMessage types)
- https://platform.claude.com/docs/en/agent-sdk/typescript-v2-preview — V2 preview API (createSession, resumeSession, prompt, SDKSession interface)
- https://github.com/anthropics/claude-agent-sdk-typescript/blob/main/CHANGELOG.md — Version history confirming unstable_v2_* functions, session.close() fix (v0.2.51), stream() subagent fix (v0.2.45)

## Related Topics

- `canUseTool` callback — alternative to `PermissionRequest` hook for programmatic approval
- `forkSession` — branch a session to explore different paths (V1 only)
- `enableFileCheckpointing` + `rewindFiles()` — revert file changes to a prior turn
- `spawnClaudeCodeProcess` — run Claude Code in containers or remote VMs
- `settingSources: ["project"]` — required to load CLAUDE.md and hooks.json from project disk