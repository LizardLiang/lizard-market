import { query } from "@anthropic-ai/claude-agent-sdk";
import { v5 as uuidv5 } from "uuid";
import type { Client, TextChannel } from "discord.js";
import type { ActiveSession, ProjectConfig, SessionsState } from "./types";
import { homedir } from "os";
import { loadSessions, saveSessions } from "./config";
import { postApprovalAndWait } from "./approval";
import { ResponsePoster } from "./response-poster";

const SESSION_NAMESPACE = "6ba7b810-9dad-11d1-80b4-00c04fd430c8";

export class SessionManager {
  private sessions = new Map<string, ActiveSession>();
  private savedSessions: SessionsState;
  private idleTimeoutMs: number;
  private saveQueue: Promise<void> = Promise.resolve();
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
      // Keep closed sessions for resume
      for (const [name, saved] of Object.entries(this.savedSessions)) {
        if (!state[name]) state[name] = saved;
      }
      saveSessions(state);
    });
  }

  /**
   * Build the canUseTool callback for a project session.
   *
   * Captures `this` (SessionManager) and `projectName` — NOT a specific channel.
   * Dynamically looks up lastActiveChannel at invocation time so approval
   * always goes to the correct channel even if the user switches channels.
   */
  private buildCanUseTool(projectName: string) {
    const manager = this;
    return async (toolName: string, toolInput: unknown) => {
      const session = manager.sessions.get(projectName);
      if (!session || !manager.discordClient) {
        return { behavior: "deny" as const, message: "No active session or Discord client" };
      }

      // Auto-approved tools: return undefined to pass through to default behavior
      // (returning { behavior: "allow" } causes ZodError on updatedInput)
      if (session.autoApproved.has(toolName)) {
        return undefined;
      }

      // Dynamically resolve the CURRENT active channel
      let channel: TextChannel;
      try {
        channel = await manager.discordClient.channels.fetch(
          session.lastActiveChannel,
        ) as TextChannel;
      } catch {
        return { behavior: "deny" as const, message: "Channel unavailable" };
      }

      const result = await postApprovalAndWait(channel, toolName, toolInput);

      if (result.autoApprove) {
        session.autoApproved.add(toolName);
      }

      if (result.decision === "allow") {
        return undefined; // pass through — avoids ZodError on updatedInput
      }
      return { behavior: "deny" as const, message: "Denied via Discord" };
    };
  }

  async sendMessage(
    projectName: string,
    project: ProjectConfig,
    text: string,
    channel: TextChannel,
  ): Promise<void> {
    let session = this.sessions.get(projectName);

    // Update last active channel for existing sessions
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

    // Build query options
    // Don't load settingSources — project/user hooks (Kratos, discord-remote)
    // have PreToolUse handlers that output incorrectly in SDK mode, causing
    // ZodError on updatedInput. Instead: bypass permissions and let our
    // canUseTool callback handle all approval via Discord.
    const options: Record<string, any> = {
      cwd: project.path,
      abortController,
      model: "claude-sonnet-4-6",
      permissionMode: "bypassPermissions",
      allowDangerouslySkipPermissions: true,
      canUseTool: this.buildCanUseTool(projectName),
      // Load plugins via local path — SDK only supports type: "local"
      plugins: [
        { type: "local", path: homedir() + "/.claude/plugins/marketplaces/claude-plugins-official/external_plugins/linear" },
        { type: "local", path: homedir() + "/.claude/plugins/cache/lizard-plugins/kratos/2.29.0" },
        { type: "local", path: homedir() + "/.claude/plugins/cache/claude-plugins-official/frontend-design/b10b583de281" },
      ],
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
        sendLock: Promise.resolve(),
      };
      this.sessions.set(projectName, session);
      // Don't queueSave here — wait for init message to capture real session ID
    }

    // Serialize sends per project — wait for any in-flight query to finish
    const prevLock = session.sendLock;
    let unlock: (() => void) | undefined;
    session.sendLock = new Promise<void>((resolve) => { unlock = resolve; });
    await prevLock;

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
      await channel.send(`Session error: ${err.message}`);
      this.sessions.delete(projectName);
    } finally {
      unlock!();
      this.resetIdleTimer(projectName);
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

  async clearSession(projectName: string): Promise<void> {
    await this.closeSession(projectName);
    delete this.savedSessions[projectName];
    this.queueSave();
  }
}
