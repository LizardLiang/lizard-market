#!/usr/bin/env node
/**
 * Kratos Test Harness — main runner
 *
 * Usage:
 *   node src/run.mjs                          # run all 6 tasks
 *   node src/run.mjs --task research-mimir    # run one task
 *   node src/run.mjs --task debug,research-metis    # run two tasks
 *   node src/run.mjs --cwd /path/to/project   # custom working dir
 *   node src/run.mjs --model claude-opus-4-6  # custom model
 *
 * Output: test-harness/results/<run-id>/
 */

import { query } from "@anthropic-ai/claude-agent-sdk";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";
import os from "os";
import crypto from "crypto";
import { execSync } from "child_process";
import { selectTasks } from "./tasks.mjs";
import { TaskLogger } from "./logger.mjs";
import { generateReport } from "./report.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const HARNESS_ROOT = path.resolve(__dirname, "..");
const KRATOS_PLUGIN_PATH = path.resolve(__dirname, "../../"); // plugins/kratos/

// ── CLI arg parsing ──────────────────────────────────────────────────────────

function parseArgs(argv) {
  const args = { tasks: [], cwd: null, model: "claude-sonnet-4-6" };
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === "--task" && argv[i + 1]) {
      args.tasks = argv[++i].split(",").map((s) => s.trim());
    } else if (argv[i] === "--cwd" && argv[i + 1]) {
      args.cwd = argv[++i];
    } else if (argv[i] === "--model" && argv[i + 1]) {
      args.model = argv[++i];
    }
  }
  return args;
}

// ── Run a single task ────────────────────────────────────────────────────────

async function runTask(task, runDir, projectDir, model) {
  const taskDir = path.join(runDir, task.name);
  const logger = new TaskLogger(taskDir, task.name);

  console.log(`\n${"─".repeat(60)}`);
  console.log(`▶ Task: ${task.name} (${task.type})`);
  console.log(`  Prompt: ${task.prompt.slice(0, 80)}...`);
  console.log(`${"─".repeat(60)}`);

  logger.start();

  try {
    const stream = query({
      prompt: task.prompt,
      options: {
        cwd: projectDir,
        model,
        permissionMode: "bypassPermissions",
        allowDangerouslySkipPermissions: true,
        plugins: [
          { type: "local", path: KRATOS_PLUGIN_PATH },
        ],
      },
    });

    let msgCount = 0;
    for await (const msg of stream) {
      logger.record(msg);
      msgCount++;

      // Live progress indicator
      if (msg.type === "assistant") {
        for (const block of msg.message?.content ?? []) {
          if (block.type === "text" && block.text.trim()) {
            process.stdout.write(".");
          } else if (block.type === "tool_use") {
            process.stdout.write(`\n  [tool] ${block.name}\n`);
          }
        }
      } else if (msg.type === "result") {
        process.stdout.write("\n");
      }
    }

    const summary = logger.finish();
    console.log(`✓ Done — ${summary.durationSec}s, ${msgCount} messages, ${summary.agentCount} agents spawned`);
    return summary;
  } catch (err) {
    console.error(`✗ Error in task "${task.name}": ${err.message}`);
    const summary = logger.finish(err);
    return summary;
  }
}

// ── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  const args = parseArgs(process.argv);
  const tasks = selectTasks(args.tasks);

  if (tasks.length === 0) {
    console.error("No tasks matched. Available: implementation, debug, research-metis, research-mimir, research-clio, brainstorming");
    process.exit(1);
  }

  // Create run directory
  const runId = new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19) +
    "-" + crypto.randomBytes(3).toString("hex");
  const runDir = path.join(HARNESS_ROOT, "results", runId);
  fs.mkdirSync(runDir, { recursive: true });

  // Read Kratos plugin version
  let kratosVersion = "unknown";
  try {
    const pluginJson = JSON.parse(
      fs.readFileSync(path.join(KRATOS_PLUGIN_PATH, ".claude-plugin", "plugin.json"), "utf8")
    );
    kratosVersion = pluginJson.version ?? "unknown";
  } catch {}

  // Ensure test project exists
  let projectDir;
  try {
    if (args.cwd) {
      projectDir = args.cwd;
    } else {
      projectDir = path.join(HARNESS_ROOT, "results", "_test-project");
      fs.mkdirSync(projectDir, { recursive: true });
      if (!fs.existsSync(path.join(projectDir, ".git"))) {
        execSync("git init", { cwd: projectDir, stdio: "ignore" });
        execSync('git commit --allow-empty -m "init"', { cwd: projectDir, stdio: "ignore" });
      }
    }
  } catch (err) {
    console.error("Failed to set up test project:", err.message);
    process.exit(1);
  }

  // Write run metadata
  const runMeta = {
    runId,
    kratosPluginPath: KRATOS_PLUGIN_PATH,
    kratosVersion,
    model: args.model,
    projectDir,
    tasksRequested: tasks.map((t) => t.name),
    startedAt: new Date().toISOString(),
  };
  fs.writeFileSync(path.join(runDir, "run-meta.json"), JSON.stringify(runMeta, null, 2));

  console.log(`\nKratos Test Harness`);
  console.log(`Run ID : ${runId}`);
  console.log(`Plugin : ${KRATOS_PLUGIN_PATH} (v${kratosVersion})`);
  console.log(`Model  : ${args.model}`);
  console.log(`Project: ${projectDir}`);
  console.log(`Tasks  : ${tasks.map((t) => t.name).join(", ")}`);
  console.log(`Output : ${runDir}`);

  // Run tasks sequentially
  const summaries = [];
  for (const task of tasks) {
    const summary = await runTask(task, runDir, projectDir, args.model);
    summaries.push(summary);
  }

  // Generate cross-task report
  const report = generateReport(runDir, tasks.map((t) => t.name), runMeta);

  // Print summary table
  console.log(`\n${"═".repeat(60)}`);
  console.log("RESULTS");
  console.log(`${"═".repeat(60)}`);
  console.log(
    `${"Task".padEnd(18)} ${"Status".padEnd(9)} ${"Duration".padEnd(10)} ${"Agents".padEnd(8)} ${"Tokens"}`
  );
  console.log("─".repeat(60));
  for (const row of report.comparison) {
    const status = row.status === "success" ? "✓ ok    " : "✗ error ";
    console.log(
      `${row.task.padEnd(18)} ${status.padEnd(9)} ${String(row.durationSec + "s").padEnd(10)} ${String(row.agentsSpawned).padEnd(8)} ${row.inputTokens}+${row.outputTokens}`
    );
  }
  console.log("─".repeat(60));
  console.log(`Health: ${report.health.rating.toUpperCase()}`);
  if (report.health.notes.length > 0) {
    for (const note of report.health.notes) {
      console.log(`  ⚠ ${note}`);
    }
  }
  console.log(`\nFull output: ${runDir}`);
}

main().catch((err) => {
  console.error("Fatal:", err);
  process.exit(1);
});
