import fs from "fs";
import path from "path";

/**
 * TaskLogger captures every message from a query() stream and writes:
 *   - messages.jsonl  — raw messages, one JSON object per line
 *   - transcript.md   — human-readable, timestamped transcript
 *   - summary.json    — aggregated metrics for performance analysis
 */
export class TaskLogger {
  constructor(outDir, taskName) {
    this.outDir = outDir;
    this.taskName = taskName;
    this.startTime = null;
    this.endTime = null;
    this.messages = [];
    this.errors = [];

    fs.mkdirSync(outDir, { recursive: true });

    this._jsonlStream = fs.createWriteStream(
      path.join(outDir, "messages.jsonl"),
      { flags: "a" }
    );
    this._transcriptLines = [];
  }

  /** Call before starting the query() loop */
  start() {
    this.startTime = new Date();
    this._transcriptLines.push(
      `# Kratos Task: ${this.taskName}`,
      `**Started:** ${this.startTime.toISOString()}`,
      ""
    );
  }

  /** Record one message from the stream */
  record(msg) {
    const ts = new Date().toISOString();
    const tagged = { _ts: ts, ...msg };
    this.messages.push(tagged);

    // Write raw to JSONL
    this._jsonlStream.write(JSON.stringify(tagged) + "\n");

    // Format for transcript
    const line = this._formatMessage(ts, msg);
    if (line) this._transcriptLines.push(line, "");
  }

  /** Call after the query() loop ends (success or error) */
  finish(error = null) {
    this.endTime = new Date();
    const durationMs = this.endTime - this.startTime;

    if (error) {
      this.errors.push({ message: error.message, stack: error.stack });
      this._transcriptLines.push(
        `---`,
        `**ERROR:** ${error.message}`,
        ""
      );
    }

    this._transcriptLines.push(
      `---`,
      `**Finished:** ${this.endTime.toISOString()}`,
      `**Duration:** ${(durationMs / 1000).toFixed(1)}s`,
      ""
    );

    this._jsonlStream.end();

    // Write transcript
    fs.writeFileSync(
      path.join(this.outDir, "transcript.md"),
      this._transcriptLines.join("\n")
    );

    // Build and write summary
    const summary = this._buildSummary(durationMs, error);
    fs.writeFileSync(
      path.join(this.outDir, "summary.json"),
      JSON.stringify(summary, null, 2)
    );

    return summary;
  }

  // ── Private helpers ──────────────────────────────────────────────────────

  _formatMessage(ts, msg) {
    const time = ts.slice(11, 23); // HH:MM:SS.mmm

    switch (msg.type) {
      case "system": {
        const sub = msg.subtype ?? "";
        if (sub === "init") {
          return `**[${time}] SYSTEM/init** — session_id: \`${msg.session_id ?? "?"}\``;
        }
        return `**[${time}] SYSTEM/${sub}**\n\`\`\`json\n${JSON.stringify(msg, null, 2)}\n\`\`\``;
      }

      case "assistant": {
        const parts = [];
        for (const block of msg.message?.content ?? []) {
          if (block.type === "text") {
            parts.push(`**[${time}] ASSISTANT**\n${block.text.trim()}`);
          } else if (block.type === "tool_use") {
            const inputSnippet = JSON.stringify(block.input ?? {}).slice(0, 200);
            parts.push(
              `**[${time}] TOOL USE** \`${block.name}\` (id: ${block.id})\n\`\`\`json\n${inputSnippet}\n\`\`\``
            );
          } else if (block.type === "thinking") {
            const snippet = (block.thinking ?? "").slice(0, 500);
            parts.push(
              `**[${time}] THINKING**\n> ${snippet.replace(/\n/g, "\n> ")}${block.thinking?.length > 500 ? "\n> *(truncated)*" : ""}`
            );
          }
        }
        return parts.join("\n\n") || null;
      }

      case "tool": {
        // Tool result messages
        const content = typeof msg.content === "string"
          ? msg.content.slice(0, 300)
          : JSON.stringify(msg.content ?? {}).slice(0, 300);
        return `**[${time}] TOOL RESULT** \`${msg.tool_name ?? "?"}\`\n\`\`\`\n${content}\n\`\`\``;
      }

      case "result": {
        const cost = msg.usage
          ? `input=${msg.usage.input_tokens} output=${msg.usage.output_tokens}`
          : "n/a";
        return `**[${time}] RESULT** — subtype: \`${msg.subtype ?? "?"}\`, tokens: ${cost}`;
      }

      case "user": {
        // Tool results come back as user messages in SDK
        const content = msg.message?.content ?? [];
        const parts = [];
        for (const block of content) {
          if (block.type === "tool_result") {
            const snippet = (
              Array.isArray(block.content)
                ? block.content.map((c) => c.text ?? "").join("")
                : String(block.content ?? "")
            ).slice(0, 300);
            parts.push(`**[${time}] TOOL RESULT** (id: ${block.tool_use_id})\n\`\`\`\n${snippet}\n\`\`\``);
          }
        }
        return parts.join("\n\n") || null;
      }

      default:
        return `**[${time}] ${msg.type?.toUpperCase() ?? "UNKNOWN"}**\n\`\`\`json\n${JSON.stringify(msg, null, 2).slice(0, 400)}\n\`\`\``;
    }
  }

  _buildSummary(durationMs, error) {
    const counts = {
      total: this.messages.length,
      system: 0,
      assistant: 0,
      tool_use: 0,
      tool_result: 0,
      thinking: 0,
      result: 0,
      other: 0,
    };

    let sessionId = null;
    // Per-message token accumulation (covers all agents in the stream)
    let inputTokens = 0;
    let cacheCreateTokens = 0;
    let cacheReadTokens = 0;
    let outputTokens = 0;
    const agentsSpawned = new Set();

    for (const msg of this.messages) {
      switch (msg.type) {
        case "system":
          counts.system++;
          if (msg.subtype === "init") sessionId = msg.session_id ?? null;
          break;
        case "assistant": {
          counts.assistant++;
          // Sum usage from every assistant turn (includes subagents)
          const u = msg.message?.usage ?? {};
          inputTokens += u.input_tokens ?? 0;
          cacheCreateTokens += u.cache_creation_input_tokens ?? 0;
          cacheReadTokens += u.cache_read_input_tokens ?? 0;
          outputTokens += u.output_tokens ?? 0;
          for (const block of msg.message?.content ?? []) {
            if (block.type === "tool_use") {
              counts.tool_use++;
              if (block.name === "Task" && block.input?.subagent_type) {
                agentsSpawned.add(block.input.subagent_type);
              }
            } else if (block.type === "thinking") {
              counts.thinking++;
            }
          }
          break;
        }
        case "user":
          for (const block of msg.message?.content ?? []) {
            if (block.type === "tool_result") counts.tool_result++;
          }
          break;
        case "result":
          counts.result++;
          break;
        default:
          counts.other++;
      }
    }

    const totalInput = inputTokens + cacheCreateTokens + cacheReadTokens;

    return {
      task: this.taskName,
      startedAt: this.startTime?.toISOString(),
      finishedAt: this.endTime?.toISOString(),
      durationMs,
      durationSec: +(durationMs / 1000).toFixed(1),
      sessionId,
      messageCounts: counts,
      tokens: {
        input: totalInput,
        inputRaw: inputTokens,
        cacheCreate: cacheCreateTokens,
        cacheRead: cacheReadTokens,
        output: outputTokens,
        total: totalInput + outputTokens,
      },
      agentsSpawned: [...agentsSpawned],
      agentCount: agentsSpawned.size,
      errors: this.errors,
      status: error ? "error" : "success",
    };
  }
}
