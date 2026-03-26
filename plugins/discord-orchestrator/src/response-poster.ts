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
      const content = this.buffer.slice(0, 2000);
      this.buffer = this.buffer.slice(2000);

      if (!this.firstMessage) {
        this.firstMessage = await this.channel.send(content);
      } else {
        await this.firstMessage.reply(content);
      }

      if (this.buffer.length < MAX_CHUNK) break;
    }
  }

  async finish(): Promise<void> {
    await this.flush();
    this.buffer = "";
    this.firstMessage = null;
  }
}