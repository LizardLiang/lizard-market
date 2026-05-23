#!/usr/bin/env node
/**
 * test-inline-commands.mjs
 *
 * Verifies the new per-agent inline commands (/kratos:<agent>).
 *
 * Key assertions per test:
 *   1. agentCount === 0  — the main session handled the request; no Task tool fired
 *   2. persona keywords in response text — the agent's .md loaded correctly via !cat
 *   3. query() completed with status "success"
 *
 * Usage:
 *   node src/test-inline-commands.mjs
 *   node src/test-inline-commands.mjs --agent athena,hermes   # run specific agents
 *   node src/test-inline-commands.mjs --model claude-sonnet-4-6
 */

import { query } from "@anthropic-ai/claude-agent-sdk";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";
import { execFileSync } from "child_process";
import crypto from "crypto";
import { TaskLogger } from "./logger.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const HARNESS_ROOT = path.resolve(__dirname, "..");
const KRATOS_PLUGIN_PATH = path.resolve(__dirname, "../../plugins/kratos");

// ---------------------------------------------------------------------------
// Test definitions
// Prompt is a lightweight "introduce yourself" so we can verify persona loaded
// without doing expensive real work.  personaKeywords are case-insensitive
// strings expected somewhere in the assistant response text.
// ---------------------------------------------------------------------------

const ALL_TESTS = [
  {
    name: "athena-inline",
    prompt: "/kratos:athena introduce yourself: who are you and what can you help with?",
    personaKeywords: ["athena", "prd", "requirements"],
  },
  {
    name: "hermes-inline",
    prompt: "/kratos:hermes introduce yourself: who are you and what can you help with?",
    personaKeywords: ["hermes", "review"],
  },
  {
    name: "hades-inline",
    prompt: "/kratos:hades introduce yourself: who are you and what can you help with?",
    personaKeywords: ["hades", "debug"],
  },
  {
    name: "ares-inline",
    prompt: "/kratos:ares introduce yourself: who are you and what can you help with?",
    personaKeywords: ["ares", "implement"],
  },
  {
    name: "metis-inline",
    prompt: "/kratos:metis introduce yourself: who are you and what can you help with?",
    personaKeywords: ["metis", "research"],
  },
  {
    name: "clio-inline",
    prompt: "/kratos:clio introduce yourself: who are you and what can you help with?",
    personaKeywords: ["clio", "git"],
  },
  {
    name: "mimir-inline",
    prompt: "/kratos:mimir introduce yourself: who are you and what can you help with?",
    personaKeywords: ["mimir", "research"],
  },
  {
    name: "nemesis-inline",
    prompt: "/kratos:nemesis introduce yourself: who are you and what can you help with?",
    personaKeywords: ["nemesis", "prd"],
  },
  {
    name: "apollo-inline",
    prompt: "/kratos:apollo introduce yourself: who are you and what can you help with?",
    personaKeywords: ["apollo", "architecture"],
  },
  {
    name: "artemis-inline",
    prompt: "/kratos:artemis introduce yourself: who are you and what can you help with?",
    personaKeywords: ["artemis", "test"],
  },
  {
    name: "cassandra-inline",
    prompt: "/kratos:cassandra introduce yourself: who are you and what can you help with?",
    personaKeywords: ["cassandra", "risk"],
  },
  {
    name: "daedalus-inline",
    prompt: "/kratos:daedalus introduce yourself: who are you and what can you help with?",
    personaKeywords: ["daedalus", "decompos"],
  },
  {
    name: "hephaestus-inline",
    prompt: "/kratos:hephaestus introduce yourself: who are you and what can you help with?",
    personaKeywords: ["hephaestus", "spec"],
  },
  {
    name: "hera-inline",
    prompt: "/kratos:hera introduce yourself: who are you and what can you help with?",
    personaKeywords: ["hera", "prd"],
  },
  {
    name: "prometheus-inline",
    prompt: "/kratos:prometheus introduce yourself: who are you and what can you help with?",
    personaKeywords: ["prometheus", "plan"],
  },
  {
    name: "themis-inline",
    prompt: "/kratos:themis introduce yourself: who are you and what can you help with?",
    personaKeywords: ["themis", "discuss"],
  },
  {
    name: "ananke-inline",
    prompt: "/kratos:ananke introduce yourself: who are you and what can you help with?",
    personaKeywords: ["ananke", "todo"],
  },
];

// ---------------------------------------------------------------------------
// Arg parsing
// ---------------------------------------------------------------------------

function parseArgs(argv) {
  const args = { agents: [], model: "claude-sonnet-4-6" };
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === "--agent" && argv[i + 1]) {
      args.agents = argv[++i].split(",").map((s) => s.trim());
    } else if (argv[i] === "--model" && argv[i + 1]) {
      args.model = argv[++i];
    }
  }
  return args;
}

// ---------------------------------------------------------------------------
// Run one test
// ---------------------------------------------------------------------------

async function runTest(test, outDir, projectDir, model) {
  const logger = new TaskLogger(outDir, test.name);
  logger.start();

  let responseText = "";

  try {
    const stream = query({
      prompt: test.prompt,
      options: {
        cwd: projectDir,
        model,
        permissionMode: "bypassPermissions",
        allowDangerouslySkipPermissions: true,
        plugins: [{ type: "local", path: KRATOS_PLUGIN_PATH }],
      },
    });

    for await (const msg of stream) {
      logger.record(msg);
      if (msg.type === "assistant") {
        for (const block of msg.message?.content ?? []) {
          if (block.type === "text") responseText += block.text + "\n";
          if (block.type === "tool_use") process.stdout.write(`  [tool] ${block.name}\n`);
        }
      }
    }

    const summary = logger.finish();
    return { summary, responseText, error: null };
  } catch (err) {
    const hadSuccess = logger.messages.some(
      (m) => m.type === "result" && m.subtype === "success"
    );
    if (hadSuccess) {
      const summary = logger.finish();
      return { summary, responseText, error: null };
    }
    const summary = logger.finish(err);
    return { summary, responseText, error: err };
  }
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

function assertTest(test, summary, responseText) {
  const failures = [];

  if (summary.status !== "success") {
    failures.push(`query() failed: ${summary.errors?.[0]?.message ?? "unknown error"}`);
  }

  if (summary.agentCount !== 0) {
    failures.push(
      `expected 0 subagents, got ${summary.agentCount}: [${summary.agentsSpawned.join(", ")}]`
    );
  }

  const lower = responseText.toLowerCase();
  for (const kw of test.personaKeywords) {
    if (!lower.includes(kw.toLowerCase())) {
      failures.push(`persona keyword missing in response: "${kw}"`);
    }
  }

  return failures;
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main() {
  const args = parseArgs(process.argv);

  const tests =
    args.agents.length > 0
      ? ALL_TESTS.filter((t) =>
          args.agents.some((a) => t.name.startsWith(a + "-") || t.name === a)
        )
      : ALL_TESTS;

  if (tests.length === 0) {
    console.error(
      "No tests matched. Available agents:",
      ALL_TESTS.map((t) => t.name.replace("-inline", "")).join(", ")
    );
    process.exit(1);
  }

  // Read plugin version
  let kratosVersion = "unknown";
  try {
    const pj = JSON.parse(
      fs.readFileSync(
        path.join(KRATOS_PLUGIN_PATH, ".claude-plugin", "plugin.json"),
        "utf8"
      )
    );
    kratosVersion = pj.version ?? "unknown";
  } catch {}

  // Prepare output directory
  const runId =
    new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19) +
    "-inline-" +
    crypto.randomBytes(3).toString("hex");
  const runDir = path.join(HARNESS_ROOT, "results", runId);
  fs.mkdirSync(runDir, { recursive: true });

  // Prepare minimal project dir — must be a git repo for the plugin to work
  const projectDir = path.join(HARNESS_ROOT, "results", "_test-project");
  fs.mkdirSync(projectDir, { recursive: true });
  if (!fs.existsSync(path.join(projectDir, ".git"))) {
    execFileSync("git", ["init"], { cwd: projectDir, stdio: "ignore" });
    execFileSync("git", ["commit", "--allow-empty", "-m", "init"], {
      cwd: projectDir,
      stdio: "ignore",
    });
  }

  console.log(`\nKratos Inline Command Tests`);
  console.log(`Plugin : ${KRATOS_PLUGIN_PATH} (v${kratosVersion})`);
  console.log(`Model  : ${args.model}`);
  console.log(`Tests  : ${tests.length}`);
  console.log(`Output : ${runDir}\n`);

  const results = [];

  for (const test of tests) {
    process.stdout.write(`▶ ${test.name.padEnd(22)}`);
    const taskDir = path.join(runDir, test.name);
    const { summary, responseText, error } = await runTest(
      test,
      taskDir,
      projectDir,
      args.model
    );

    const failures = error
      ? [`query() threw: ${error.message}`]
      : assertTest(test, summary, responseText);

    const passed = failures.length === 0;
    const mark = passed ? "✓" : "✗";
    const agents =
      summary.agentCount === 0 ? "no-subagent" : `${summary.agentCount} subagent(s)`;
    console.log(`${mark}  ${agents}  ${summary.durationSec}s`);

    if (!passed) {
      for (const f of failures) console.log(`     ✗ ${f}`);
    }

    results.push({
      name: test.name,
      passed,
      failures,
      durationSec: summary.durationSec,
      agentCount: summary.agentCount,
    });
  }

  // Summary
  const passedCount = results.filter((r) => r.passed).length;
  const failedCount = results.length - passedCount;
  console.log(`\n${"─".repeat(50)}`);
  console.log(
    `${passedCount}/${results.length} passed${failedCount > 0 ? `, ${failedCount} FAILED` : ""}`
  );

  fs.writeFileSync(
    path.join(runDir, "results.json"),
    JSON.stringify({ runId, kratosVersion, model: args.model, results }, null, 2)
  );
  console.log(`Results: ${path.join(runDir, "results.json")}`);

  if (failedCount > 0) process.exit(1);
}

main().catch((err) => {
  console.error("Fatal:", err);
  process.exit(1);
});
