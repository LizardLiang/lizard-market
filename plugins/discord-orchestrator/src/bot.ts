import {
  Client,
  ChannelType,
  GatewayIntentBits,
  Partials,
  type TextChannel,
  type Message,
  type Guild,
} from "discord.js";
import { loadConfig, buildChannelMap } from "./config";
import { SessionManager } from "./session-manager";
import type { OrchestratorConfig } from "./types";

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

    // Instant acknowledgement so the user knows the bot received the message
    const isNewSession = !this.sessionManager.hasSession(projectName);
    const ack = isNewSession
      ? `Working on it... (starting session for **${projectName}**)`
      : `Working on it...`;
    const ackMsg = await channel.send(ack);

    try {
      await this.sessionManager.sendMessage(
        projectName,
        project,
        msg.content,
        channel,
      );
      // Delete the ack after the real response is posted
      await ackMsg.delete().catch(() => {});
    } catch (err: any) {
      console.error(`[orchestrator] Error in ${projectName}:`, err);
      await ackMsg.edit(`Error: ${err.message}`);
    }
  }

  /**
   * Start a localhost-only HTTP API for the ops SDK session to call.
   * Returns the port number so it can be passed to the ops system prompt.
   */
  async startApiServer(): Promise<number> {
    const server = Bun.serve({
      hostname: "127.0.0.1",
      port: 0, // random available port
      fetch: async (req) => {
        const url = new URL(req.url);

        if (req.method === "POST" && url.pathname === "/create-channel") {
          try {
            const body = await req.json() as {
              guild_id: string;
              name: string;
              project_name?: string;
              category_id?: string;
            };

            const guild: Guild = await this.client.guilds.fetch(body.guild_id);
            const channel = await guild.channels.create({
              name: body.name,
              type: ChannelType.GuildText,
              parent: body.category_id || undefined,
            });

            return Response.json({
              ok: true,
              channel_id: channel.id,
              channel_name: channel.name,
            });
          } catch (err: any) {
            return Response.json({ ok: false, error: err.message }, { status: 400 });
          }
        }

        if (req.method === "GET" && url.pathname === "/guilds") {
          try {
            const guilds = this.client.guilds.cache.map((g) => ({
              id: g.id,
              name: g.name,
            }));
            return Response.json({ ok: true, guilds });
          } catch (err: any) {
            return Response.json({ ok: false, error: err.message }, { status: 500 });
          }
        }

        if (req.method === "GET" && url.pathname === "/channels") {
          try {
            const guildId = url.searchParams.get("guild_id");
            if (!guildId) return Response.json({ ok: false, error: "guild_id required" }, { status: 400 });
            const guild = await this.client.guilds.fetch(guildId);
            const channels = (await guild.channels.fetch())
              .filter((c) => c?.type === ChannelType.GuildText)
              .map((c) => ({ id: c!.id, name: c!.name }));
            return Response.json({ ok: true, channels });
          } catch (err: any) {
            return Response.json({ ok: false, error: err.message }, { status: 500 });
          }
        }

        if (req.method === "POST" && url.pathname === "/clear-session") {
          try {
            const body = await req.json() as { project_name?: string };
            if (!body.project_name) {
              return Response.json({ ok: false, error: "project_name is required" }, { status: 400 });
            }
            const project_name = body.project_name;
            await this.sessionManager.clearSession(project_name);
            return Response.json({ ok: true, project_name });
          } catch (err: any) {
            return Response.json({ ok: false, error: err.message }, { status: 400 });
          }
        }

        return new Response("Not found", { status: 404 });
      },
    });

    const port = server.port!;
    console.log(`[orchestrator] API server on http://127.0.0.1:${port}`);
    return port;
  }

  async start(token: string): Promise<void> {
    await this.client.login(token);
  }

  async shutdown(): Promise<void> {
    await this.sessionManager.closeAll();
    this.client.destroy();
  }
}
