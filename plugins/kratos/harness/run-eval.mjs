#!/usr/bin/env node
/**
 * Kratos outcome-quality eval runner.
 *
 * Unlike test-harness/ (process compliance: timestamps, CLI usage, spawn
 * patterns), this harness grades DELIVERABLE QUALITY: does the agent's output
 * meet structural standards, cover the PRD, and catch seeded defects?
 *
 * Usage:
 *   node run-eval.mjs --stage 6-test-plan            # one stage
 *   node run-eval.mjs --stage all                     # all supported stages
 *   node run-eval.mjs --stage 9-review --model opus   # model override
 *
 * Per stage: creates an isolated fixture project (mk-fixture.sh seeds golden
 * upstream docs so the agent runs without live prior stages), spawns ONLY the
 * target agent via the Task tool, then runs validate.sh. Stages 8/9 run with
 * HARNESS_SEEDED_GAPS=1 — the fixture implementation deliberately violates
 * AC-3 and AC-5, and reviewers are graded on catching them.
 *
 * Requires: npm i @anthropic-ai/claude-agent-sdk  (or reuse
 * ../../test-harness/node_modules via NODE_PATH).
 */

import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";
import { spawnSync } from "child_process";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const KRATOS_ROOT = path.resolve(__dirname, "..");

// Resolve the SDK from local node_modules or the sibling test-harness install.
let query;
try {
  ({ query } = await import("@anthropic-ai/claude-agent-sdk"));
} catch {
  const alt = path.resolve(KRATOS_ROOT, "../../test-harness/node_modules/@anthropic-ai/claude-agent-sdk/sdk.mjs");
  ({ query } = await import(alt));
}

const STAGES = {
  "1-prd": {
    agent: "kratos:athena",
    deliverable: "prd.md",
    mission: `MISSION: Create PRD
PHASE: CREATE_PRD
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/
ORIGINAL_USER_REQUEST: Add per-token rate limiting to /api/data: 10 requests/minute, return 429 with Retry-After, exempt admin tokens, in-memory only.
GAP_ANALYSIS: User answered: window is rolling 60s; admin tokens come from a config array; no persistence needed; 401s must not consume quota.

Read ${KRATOS_ROOT}/agents/athena.md for the full instruction set before starting.
Create prd.md before completing. Update status.json.`,
  },
  "2-prd-review": {
    agent: "kratos:nemesis",
    deliverable: "prd-challenge.md",
    mission: `MISSION: Review PRD
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/
ORIGINAL_USER_REQUEST: Add per-token rate limiting to /api/data: 10 requests/minute, return 429 with Retry-After, exempt admin tokens, in-memory only.

Read ${KRATOS_ROOT}/agents/nemesis.md for the full instruction set before starting.
Review prd.md and create prd-challenge.md. Update status.json with verdict.`,
  },
  "4-tech-spec": {
    agent: "kratos:hephaestus",
    deliverable: "tech-spec.md",
    mission: `MISSION: Create Tech Spec
PHASE: WRITE_SPEC
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/
DECISIONS_CONTEXT: Approach locked: sliding-window counter in-memory Map, module src/rate-limit.js, admin bypass via exported ADMIN_TOKENS array. No open gray areas.

Read ${KRATOS_ROOT}/agents/hephaestus.md for the full instruction set before starting.
Create tech-spec.md before completing. Update status.json.`,
  },
  "5-spec-review-sa": {
    agent: "kratos:apollo",
    deliverable: "spec-review-sa.md",
    mission: `MISSION: Review Tech Spec (Architecture)
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/

Read ${KRATOS_ROOT}/agents/apollo.md for the full instruction set before starting.
Create spec-review-sa.md. Update status.json.`,
  },
  "6-test-plan": {
    agent: "kratos:artemis",
    deliverable: "test-plan.md",
    mission: `MISSION: Create Test Plan
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/

Read ${KRATOS_ROOT}/agents/artemis.md for the full instruction set before starting.
Create comprehensive test-plan.md. Update status.json.`,
  },
  "7-implementation": {
    agent: "kratos:ares",
    deliverable: "implementation-notes.md",
    mission: `MISSION: Implement Feature
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/

Read ${KRATOS_ROOT}/agents/ares.md for the full instruction set before starting.
Create implementation-notes.md before completing. Update status.json.`,
  },
  "8-prd-alignment": {
    agent: "kratos:hera",
    deliverable: "prd-alignment.md",
    seededGaps: true,
    mission: `MISSION: PRD Alignment Check
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/

Read ${KRATOS_ROOT}/agents/hera.md for the full instruction set before starting.
Verify every acceptance criterion in prd.md is covered by a test and that tests pass. Create prd-alignment.md with verdict. Update status.json.`,
  },
  "9-review": {
    agent: "kratos:hermes",
    deliverable: "code-review.md",
    seededGaps: true,
    mission: `MISSION: Code Review
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/

Read ${KRATOS_ROOT}/agents/hermes.md for the full instruction set before starting.
Create code-review.md with verdict. Update status.json.`,
    secondAgent: {
      agent: "kratos:cassandra",
      mission: `MISSION: Risk Analysis
MODE: pipeline
FEATURE: rate-limit
FOLDER: .claude/feature/rate-limit/

Read ${KRATOS_ROOT}/agents/cassandra.md for the full instruction set before starting.
Create risk-analysis.md with severity-rated findings. Update status.json.`,
    },
  },
};

function parseArgs(argv) {
  const a = { stages: [], model: undefined, keep: false };
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === "--stage" && argv[i + 1]) a.stages = argv[++i].split(",");
    else if (argv[i] === "--model" && argv[i + 1]) a.model = argv[++i];
    else if (argv[i] === "--keep") a.keep = true;
  }
  if (a.stages.includes("all") || a.stages.length === 0) a.stages = Object.keys(STAGES);
  return a;
}

async function runStage(stageKey, model, runDir) {
  const cfg = STAGES[stageKey];
  if (!cfg) throw new Error(`unknown stage ${stageKey}`);
  const projectDir = path.join(runDir, stageKey, "project");
  fs.mkdirSync(projectDir, { recursive: true });

  const fx = spawnSync("bash", [path.join(__dirname, "mk-fixture.sh"), projectDir, stageKey], { stdio: "inherit" });
  if (fx.status !== 0) throw new Error(`fixture setup failed for ${stageKey}`);

  const spawns = [
    `Task(subagent_type: "${cfg.agent}", prompt: ${JSON.stringify(cfg.mission)}, description: "${cfg.agent} eval")`,
    ...(cfg.secondAgent
      ? [`Task(subagent_type: "${cfg.secondAgent.agent}", prompt: ${JSON.stringify(cfg.secondAgent.mission)}, description: "${cfg.secondAgent.agent} eval")`]
      : []),
  ];
  const prompt = `Use the Task tool to spawn exactly ${spawns.length === 1 ? "one subagent" : "these subagents in the same response (parallel)"} and wait for ${spawns.length === 1 ? "it" : "all of them"}:
${spawns.join("\n")}
Do not do the work yourself. Do not spawn any other agent. After ${spawns.length === 1 ? "the subagent returns" : "all subagents return"}, reply DONE.`;

  console.log(`\n▶ ${stageKey} → ${cfg.agent}`);
  const t0 = Date.now();
  const log = fs.createWriteStream(path.join(runDir, stageKey, "messages.jsonl"));
  let cost = 0;
  const stream = query({
    prompt,
    options: {
      cwd: projectDir,
      ...(model ? { model } : {}),
      permissionMode: "bypassPermissions",
      allowDangerouslySkipPermissions: true,
      plugins: [{ type: "local", path: KRATOS_ROOT }],
    },
  });
  for await (const msg of stream) {
    log.write(JSON.stringify(msg) + "\n");
    if (msg.type === "result") cost = msg.total_cost_usd ?? 0;
  }
  log.end();
  const durS = Math.round((Date.now() - t0) / 1000);

  const featureDir = path.join(projectDir, ".claude/feature/rate-limit");
  const v = spawnSync("bash", [path.join(__dirname, "validate.sh"), stageKey, featureDir, projectDir], {
    encoding: "utf8",
    env: { ...process.env, HARNESS_SEEDED_GAPS: cfg.seededGaps ? "1" : "0" },
  });
  console.log(v.stdout);
  if (v.stderr) console.error(v.stderr);

  return {
    stage: stageKey,
    agent: cfg.agent,
    durationSec: durS,
    costUsd: cost,
    failures: v.status,
    report: v.stdout,
  };
}

const args = parseArgs(process.argv);
const runId = new Date().toISOString().replace(/[:.]/g, "-");
const runDir = path.join(__dirname, "results", runId);
fs.mkdirSync(runDir, { recursive: true });

const results = [];
for (const s of args.stages) {
  try {
    results.push(await runStage(s, args.model, runDir));
  } catch (e) {
    results.push({ stage: s, error: String(e) });
  }
}

fs.writeFileSync(path.join(runDir, "report.json"), JSON.stringify(results, null, 2));
console.log("\n══ SUMMARY ══");
for (const r of results) {
  console.log(`${r.stage.padEnd(18)} ${r.error ? "ERROR " + r.error : `${r.failures} failures, ${r.durationSec}s, $${(r.costUsd ?? 0).toFixed(3)}`}`);
}
console.log(`\nreport: ${path.join(runDir, "report.json")}`);
process.exitCode = results.some((r) => r.error || r.failures > 0) ? 1 : 0;
