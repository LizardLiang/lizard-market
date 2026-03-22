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
        color: 0xffa500,
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
        resolve({ decision: "deny", autoApprove: false });
      }
    });
  });
}