#!/usr/bin/env node
/**
 * Kratos CLI Compliance Validator
 *
 * Validates that pipeline agents use the kratos CLI to write status.json
 * instead of directly using Write/Edit tools on the JSON file.
 *
 * For each test task in a run, it:
 *   1. Scans messages.jsonl for direct Write/Edit calls on status.json (VIOLATIONS)
 *   2. Scans messages.jsonl for Bash calls with `kratos pipeline` commands (COMPLIANCE)
 *   3. Checks that status.json exists and is valid JSON (OUTCOME)
 *
 * Usage:
 *   node src/cli-compliance-validator.mjs [results-dir]
 *   node src/cli-compliance-validator.mjs results/2026-04-05T12-00-00-abc123
 */

import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const HARNESS_ROOT = path.resolve(__dirname, "..");

const CLI_COMPLIANCE_TASKS = [
  "cli-compliance-athena",
  "cli-compliance-nemesis",
  "cli-compliance-hephaestus",
  "cli-compliance-apollo",
  "cli-compliance-artemis",
  "cli-compliance-ares",
  "cli-compliance-hera",
  "cli-compliance-hermes",
  "cli-compliance-cassandra",
];

// Ares-specific new-command checks
const ARES_NEW_CMD_TASK = "ares-discover-run";

/**
 * Scan an ares-discover-run messages.jsonl for new-command compliance:
 *   - pipeline discover called
 *   - pipeline get called
 *   - pipeline update --summary called
 *   - no direct Read on status.json
 */
function scanAresNewCmds(messagesJsonlPath) {
  const result = {
    discoverCalled: false,
    getCalled: false,
    summaryCalled: false,
    directStatusReads: [],
    allPipelineCalls: [],
  };

  if (!fs.existsSync(messagesJsonlPath)) return { ...result, error: "messages.jsonl not found" };

  const lines = fs.readFileSync(messagesJsonlPath, "utf8").split("\n").filter(Boolean);

  for (const line of lines) {
    let msg;
    try { msg = JSON.parse(line); } catch { continue; }

    if (msg.type !== "assistant") continue;
    for (const block of msg.message?.content ?? []) {
      if (block.type !== "tool_use") continue;
      const { name: tool, input = {} } = block;
      const cmd = input.command ?? "";

      if (tool === "Bash" && cmd.includes("kratos") && cmd.includes("pipeline")) {
        result.allPipelineCalls.push(cmd.slice(0, 200));
        if (cmd.includes("pipeline discover")) result.discoverCalled = true;
        if (/pipeline\s+get/.test(cmd)) result.getCalled = true;
        if (cmd.includes("--summary")) result.summaryCalled = true;
      }

      if (tool === "Read" && typeof input.file_path === "string" && input.file_path.includes("status.json")) {
        result.directStatusReads.push(input.file_path);
      }
    }
  }

  return result;
}

/**
 * Scan a messages.jsonl file for CLI compliance signals.
 * Returns violations (direct status.json writes) and compliant actions (CLI calls).
 */
function scanMessagesForCompliance(messagesJsonlPath) {
  const violations = [];
  const cliCalls = [];
  const allToolCalls = [];

  if (!fs.existsSync(messagesJsonlPath)) {
    return { violations, cliCalls, allToolCalls, error: "messages.jsonl not found" };
  }

  const lines = fs.readFileSync(messagesJsonlPath, "utf8").split("\n").filter(Boolean);

  for (const line of lines) {
    let msg;
    try {
      msg = JSON.parse(line);
    } catch {
      continue;
    }

    // Check assistant messages for tool_use blocks
    if (msg.type === "assistant") {
      for (const block of msg.message?.content ?? []) {
        if (block.type !== "tool_use") continue;

        const toolName = block.name;
        const input = block.input ?? {};

        allToolCalls.push({ tool: toolName, input: JSON.stringify(input).slice(0, 150) });

        // VIOLATION: Write tool targeting status.json
        if (toolName === "Write" && typeof input.file_path === "string") {
          if (input.file_path.includes("status.json")) {
            violations.push({
              type: "DIRECT_WRITE",
              tool: "Write",
              file: input.file_path,
              snippet: (input.content ?? "").slice(0, 100),
            });
          }
        }

        // VIOLATION: Edit tool targeting status.json
        if (toolName === "Edit" && typeof input.file_path === "string") {
          if (input.file_path.includes("status.json")) {
            violations.push({
              type: "DIRECT_EDIT",
              tool: "Edit",
              file: input.file_path,
              snippet: (input.new_string ?? input.old_string ?? "").slice(0, 100),
            });
          }
        }

        // COMPLIANCE: Bash call with kratos pipeline command
        if (toolName === "Bash" && typeof input.command === "string") {
          if (input.command.includes("kratos") && input.command.includes("pipeline")) {
            cliCalls.push({
              command: input.command.slice(0, 200),
            });
          }
        }
      }
    }

    // Also check user messages (tool results as nested tool_result blocks)
    // These don't contain violations but help correlate CLI call outcomes
  }

  return { violations, cliCalls, allToolCalls };
}

/**
 * Find the status.json for the cli-compliance-test feature in the test project.
 */
function findStatusJson(runDir) {
  // Status.json lives in the shared _test-project used by all runs
  const testProjectDir = path.join(HARNESS_ROOT, "results", "_test-project");
  const statusPath = path.join(testProjectDir, ".claude", "feature", "cli-compliance-test", "status.json");

  if (!fs.existsSync(statusPath)) {
    return { found: false, path: statusPath };
  }

  try {
    const content = fs.readFileSync(statusPath, "utf8");
    const data = JSON.parse(content);
    return { found: true, path: statusPath, data };
  } catch (err) {
    return { found: true, path: statusPath, parseError: err.message };
  }
}

/**
 * Analyze a single task directory for CLI compliance.
 */
function analyzeTaskCompliance(taskDir, taskName) {
  const result = {
    task: taskName,
    passed: false,
    violations: [],
    cliCalls: [],
    statusJson: null,
    notes: [],
  };

  const messagesPath = path.join(taskDir, "messages.jsonl");
  const summaryPath = path.join(taskDir, "summary.json");

  // Read summary for basic task health
  if (fs.existsSync(summaryPath)) {
    try {
      const summary = JSON.parse(fs.readFileSync(summaryPath, "utf8"));
      if (summary.status === "error") {
        result.notes.push(`Task errored: ${summary.errors?.[0]?.message ?? "unknown error"}`);
      }
    } catch {}
  }

  // Scan messages for compliance signals
  const scan = scanMessagesForCompliance(messagesPath);

  if (scan.error) {
    result.notes.push(scan.error);
    return result;
  }

  result.violations = scan.violations;
  result.cliCalls = scan.cliCalls;

  // Determine pass/fail
  const hasViolations = result.violations.length > 0;
  const hasCliCalls = result.cliCalls.length > 0;

  if (hasViolations) {
    result.passed = false;
    result.notes.push(`${result.violations.length} direct status.json write(s) detected — CLI not used`);
  } else if (!hasCliCalls) {
    // No violations but also no CLI calls — agent may have been skipped or not reached this stage
    result.passed = null; // inconclusive
    result.notes.push("No CLI pipeline calls found — agent may not have run in this task");
  } else {
    result.passed = true;
  }

  return result;
}

/**
 * Main validation function — finds the most recent cli-compliance run and validates it.
 */
function validateCliCompliance(resultsDir) {
  console.log("🔍 Kratos CLI Compliance Validation");
  console.log("====================================");
  console.log("Checking that pipeline agents use CLI (not direct writes) for status.json\n");

  if (!fs.existsSync(resultsDir)) {
    console.error(`❌ Results directory not found: ${resultsDir}`);
    return false;
  }

  // Find run directories that contain cli-compliance tasks
  const runDirs = fs
    .readdirSync(resultsDir, { withFileTypes: true })
    .filter((d) => d.isDirectory() && d.name.match(/^\d{4}-\d{2}-\d{2}T/))
    .map((d) => d.name)
    .sort()
    .reverse(); // Most recent first

  let targetRun = null;
  let targetRunDir = null;

  for (const runName of runDirs) {
    const runPath = path.join(resultsDir, runName);
    const taskDirs = fs.readdirSync(runPath, { withFileTypes: true })
      .filter((d) => d.isDirectory() && d.name.startsWith("cli-compliance-"))
      .map((d) => d.name);
    if (taskDirs.length > 0) {
      targetRun = runName;
      targetRunDir = runPath;
      break;
    }
  }

  // Run the ares new-command check regardless of whether cli-compliance runs exist
  console.log("\n🔬 Ares New-Command Check (pipeline discover / get / update --summary)");
  console.log("─".repeat(60));
  let aresNewCmdDir = null;
  for (const runName of runDirs) {
    const candidate = path.join(resultsDir, runName, ARES_NEW_CMD_TASK);
    if (fs.existsSync(candidate)) { aresNewCmdDir = candidate; break; }
  }
  let aresCheckPassed = null;
  if (!aresNewCmdDir) {
    console.log(`  ⏭️  ${ARES_NEW_CMD_TASK} not found in any run — skipping`);
    console.log(`     Run: npm run test:ares-discover`);
  } else {
    const scan = scanAresNewCmds(path.join(aresNewCmdDir, "messages.jsonl"));
    if (scan.error) {
      console.log(`  ⚠️  ${scan.error}`);
    } else {
      const mark = (ok) => ok ? "✅" : "❌";
      console.log(`  ${mark(scan.discoverCalled)} pipeline discover called`);
      console.log(`  ${mark(scan.getCalled)}     pipeline get called`);
      console.log(`  ${mark(scan.summaryCalled)} pipeline update --summary called`);
      if (scan.directStatusReads.length > 0) {
        console.log(`  ❌ Direct Read on status.json detected (${scan.directStatusReads.length}x):`);
        for (const p of scan.directStatusReads) console.log(`       ${p}`);
      } else {
        console.log(`  ✅ No direct Read on status.json`);
      }
      if (scan.allPipelineCalls.length > 0) {
        console.log(`\n  Pipeline calls observed:`);
        for (const c of scan.allPipelineCalls) console.log(`    → ${c.slice(0, 100)}`);
      }
      aresCheckPassed = scan.discoverCalled && scan.getCalled && scan.summaryCalled && scan.directStatusReads.length === 0;
    }
  }

  if (!targetRun) {
    console.log("\n⚠️  No cli-compliance test runs found. Run: npm run test:cli-compliance");
    return aresCheckPassed !== false;
  }

  console.log(`\n📁 Analyzing cli-compliance run: ${targetRun}\n`);

  // Analyze each compliance task
  const results = [];
  let passCount = 0;
  let failCount = 0;
  let inconclusiveCount = 0;

  for (const taskName of CLI_COMPLIANCE_TASKS) {
    const taskDir = path.join(targetRunDir, taskName);

    if (!fs.existsSync(taskDir)) {
      console.log(`  ⏭️  ${taskName.padEnd(30)} SKIPPED (not run)`);
      inconclusiveCount++;
      continue;
    }

    const analysis = analyzeTaskCompliance(taskDir, taskName);
    results.push(analysis);

    const agentName = taskName.replace("cli-compliance-", "").toUpperCase();
    const label = agentName.padEnd(12);

    if (analysis.passed === true) {
      passCount++;
      console.log(`  ✅ ${label}  CLI calls: ${analysis.cliCalls.length}, Violations: 0`);
      for (const call of analysis.cliCalls) {
        console.log(`         → ${call.command.slice(0, 100)}`);
      }
    } else if (analysis.passed === false) {
      failCount++;
      console.log(`  ❌ ${label}  VIOLATIONS: ${analysis.violations.length}`);
      for (const v of analysis.violations) {
        console.log(`         ⚠ ${v.type}: ${v.file}`);
        if (v.snippet) console.log(`           content: ${v.snippet}`);
      }
      if (analysis.cliCalls.length > 0) {
        console.log(`         CLI calls also found: ${analysis.cliCalls.length}`);
      }
    } else {
      inconclusiveCount++;
      console.log(`  ❓ ${label}  INCONCLUSIVE — ${analysis.notes.join("; ")}`);
    }

    for (const note of analysis.notes) {
      if (analysis.passed !== true) console.log(`         note: ${note}`);
    }
  }

  // Check status.json outcome
  console.log("\n📄 Status.json Outcome Check");
  console.log("─".repeat(40));
  const statusCheck = findStatusJson(targetRunDir);
  if (!statusCheck.found) {
    console.log(`  ⚠️  status.json not found at: ${statusCheck.path}`);
    console.log("     (Feature may not have been created — did cli-compliance-athena run?)");
  } else if (statusCheck.parseError) {
    console.log(`  ❌ status.json found but invalid JSON: ${statusCheck.parseError}`);
  } else {
    const stage = statusCheck.data?.stage ?? "?";
    const feature = statusCheck.data?.feature ?? "?";
    const stageCount = Object.keys(statusCheck.data?.pipeline ?? {}).length;
    console.log(`  ✅ status.json exists and is valid JSON`);
    console.log(`     feature: ${feature}, current stage: ${stage}, pipeline keys: ${stageCount}`);

    // Check timestamps look real (not fabricated round numbers)
    const created = statusCheck.data?.created;
    if (created) {
      const d = new Date(created);
      if (d.getSeconds() === 0 && d.getMinutes() % 5 === 0) {
        console.log("  ⚠️  created timestamp looks suspiciously round — may have been manually written");
      } else {
        console.log(`  ✅ created timestamp looks authentic: ${created}`);
      }
    }
  }

  // Summary
  const total = results.length;
  console.log("\n" + "═".repeat(50));
  console.log("CLI COMPLIANCE SUMMARY");
  console.log("═".repeat(50));
  console.log(`Total agents tested : ${total}`);
  console.log(`✅ Passed           : ${passCount}`);
  console.log(`❌ Failed (violations): ${failCount}`);
  console.log(`❓ Inconclusive     : ${inconclusiveCount}`);

  const rate = total > 0 ? Math.round((passCount / total) * 100) : 0;
  console.log(`\n🎯 Compliance rate  : ${rate}%`);

  if (failCount === 0 && passCount > 0) {
    console.log("\n✅ ALL AGENTS USE CLI — No direct status.json writes detected");
  } else if (failCount > 0) {
    console.log("\n❌ VIOLATIONS FOUND — Some agents bypassed the CLI");
  } else {
    console.log("\n⚠️  No conclusive results — run cli-compliance tests first");
  }

  return failCount === 0;
}

// CLI entry point
if (__filename === path.resolve(process.argv[1])) {
  const resultsDir = process.argv[2] || path.join(HARNESS_ROOT, "results");
  const success = validateCliCompliance(resultsDir);
  process.exit(success ? 0 : 1);
}

export { validateCliCompliance, scanMessagesForCompliance };
