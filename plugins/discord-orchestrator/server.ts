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
watch(join(STATE_DIR, "projects.json"), { persistent: true }, () => {
  if (watchDebounce) clearTimeout(watchDebounce);
  watchDebounce = setTimeout(() => {
    console.log("[orchestrator] Config changed, rebuilding channel map");
    bot.rebuildChannelMap();
  }, 200);
});

// Start API server for ops session to call Discord actions
const apiPort = await bot.startApiServer();

// Ops channel handler — uses resume for multi-turn conversation continuity
let opsSessionId: string | null = null;

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

## Config Operations
To register a project: add an entry to projects with name, path (validate path exists with Read on the directory), and empty channels array.
To bind a channel: add the channel ID string to the project's channels array.
To unbind: remove the channel ID from the channels array.
To unregister: delete the project entry.

Always read the file first, modify, then write back. Use the Edit tool for surgical changes.

## Discord API (localhost HTTP)
You can interact with Discord via the orchestrator's API at http://127.0.0.1:${apiPort}. Use Bash with curl:

- **List guilds (servers):**
  curl -s http://127.0.0.1:${apiPort}/guilds

- **List text channels in a guild:**
  curl -s "http://127.0.0.1:${apiPort}/channels?guild_id=GUILD_ID"

- **Create a new text channel:**
  curl -s -X POST http://127.0.0.1:${apiPort}/create-channel -H "Content-Type: application/json" -d '{"guild_id":"GUILD_ID","name":"channel-name"}'
  Optionally pass "category_id" to place it under a category.

After creating a channel, auto-bind it to the project by updating projects.json with the returned channel_id.`;

  const options: Record<string, any> = {
    cwd: STATE_DIR,
    systemPrompt: opsSystemPrompt,
    model: "claude-sonnet-4-6",
    allowedTools: ["Read", "Write", "Edit", "Bash"],
    canUseTool: async (toolName: string, toolInput: unknown) => {
      // Return undefined to allow (avoids ZodError on updatedInput)
      if (toolName === "Read") return undefined;
      // Bash: only allow curl to our local API
      if (toolName === "Bash") {
        const cmd = String((toolInput as any)?.command ?? "");
        if (cmd.includes("127.0.0.1") && cmd.includes(String(apiPort))) {
          return undefined;
        }
        return { behavior: "deny" as const, message: "Bash only allowed for curl to local API" };
      }
      // Write/Edit must target files within STATE_DIR
      const input = toolInput as Record<string, any>;
      const filePath: string = input?.file_path ?? input?.path ?? "";
      if (filePath.startsWith(STATE_DIR)) {
        return undefined;
      }
      return { behavior: "deny" as const, message: `Ops can only write to ${STATE_DIR}` };
    },
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
    // Always rebuild after ops query — fs.watch is unreliable on WSL2
    bot.rebuildChannelMap();
    console.log("[orchestrator] Channel map rebuilt after ops query");
  } catch (err: any) {
    await channel.send(`Ops error: ${err.message}`);
    opsSessionId = null;
  }
});

// Start
const token = loadBotToken();
console.log("[orchestrator] Starting...");
await bot.start(token);
