#!/usr/bin/env node
/**
 * test-orchestrator-qa.mjs
 *
 * Verifies the orchestrator-owns-interaction architecture:
 *
 * TEST 1 — Gap analysis in Kratos (not Athena)
 *   - Athena is spawned with PHASE: CREATE_PRD (not PHASE: GAP_ANALYSIS)
 *   - AskUserQuestion is called BEFORE the Athena spawn (by Kratos) if clarification needed,
 *     or skipped entirely if the request is already clear
 *
 * TEST 2 — Hephaestus gate, no Arena
 *   - Metis is spawned by Kratos BEFORE Hephaestus (not inside Hephaestus)
 *   - Hephaestus first spawn has PHASE: ANALYZE in prompt → writes tech-spec-proposal.md
 *   - Hephaestus second spawn has PHASE: WRITE_SPEC in prompt → writes tech-spec.md
 *
 * Usage:
 *   node src/test-orchestrator-qa.mjs
 *   node src/test-orchestrator-qa.mjs --test gap     # run only gap analysis test
 *   node src/test-orchestrator-qa.mjs --test heph    # run only hephaestus gate test
 *   node src/test-orchestrator-qa.mjs --model claude-opus-4-7
 *   node src/test-orchestrator-qa.mjs --keep-tmp
 *   node src/test-orchestrator-qa.mjs --debug
 */

import { query } from "@anthropic-ai/claude-agent-sdk";
import fs from "fs";
import path from "path";
import os from "os";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const KRATOS_PLUGIN_PATH = path.resolve(__dirname, "../../plugins/kratos");

// ── CLI args ──────────────────────────────────────────────────────────────────

const argv = process.argv.slice(2);
const MODEL = argv.find((_, i) => argv[i - 1] === "--model") ?? "claude-sonnet-4-6";
const KEEP_TMP = argv.includes("--keep-tmp");
const DEBUG = argv.includes("--debug");
const FILTER = argv.find((_, i) => argv[i - 1] === "--test") ?? null; // "gap" | "heph"
const TIMEOUT_MS = 12 * 60 * 1000;

// ── Fixtures ──────────────────────────────────────────────────────────────────

function makePrd(featureName) {
  const now = new Date().toISOString().slice(0, 10);
  return `# PRD — ${featureName}

## Document Info
| Field | Value |
|-------|-------|
| Feature | ${featureName} |
| Status | Approved |
| Date | ${now} |

## 0. Original Request
> Build a minimal Go HTTP health-check endpoint that returns {"status":"ok"} on GET /health.
> Single file main.go, stdlib only, listen on PORT env var (default 8080).

## 1. Executive Summary
A single-binary Go HTTP server with one endpoint: GET /health → 200 {"status":"ok"}.

## 2. Requirements

### P0
| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| FR-001 | GET /health returns 200 | status code == 200, body == {"status":"ok"} |
| FR-002 | PORT env configures port | Listening on PORT, default 8080 |
| FR-003 | Stdlib only | go.mod has no external deps |

## 3. Out of Scope
- Authentication
- TLS
- Any other endpoints
`;
}

function makePrdChallenge(featureName) {
  return `# PRD Adversarial Review — ${featureName}

## Verdict: APPROVED

No blocking findings. Scope is minimal and clear.
`;
}

function makeDecisions(featureName) {
  return `# Decisions Log — ${featureName}

## Product Decisions (Athena)
- Single endpoint: simplest health-check pattern. Rejected: multi-endpoint — out of scope.

## Intent Alignment (Athena)
Original ask: Build a minimal Go HTTP health-check endpoint
Restatement: A stdlib-only Go HTTP server that returns {"status":"ok"} on GET /health
Alignment: confirmed
`;
}

function makeStatusJson(featureName, currentStage = "4-tech-spec") {
  const now = new Date().toISOString();
  return {
    feature: featureName,
    created: now,
    updated: now,
    stage: currentStage,
    pipeline_status: "in-progress",
    pipeline: {
      "1-prd":          { status: "complete", started: now, completed: now, document: "prd.md", summary: "Minimal health-check endpoint, GET /health, stdlib only." },
      "2-prd-review":   { status: "complete", started: now, completed: now, verdict: "approved", document: "prd-challenge.md" },
      "3-decomposition":{ status: "skipped" },
      "4-tech-spec":    { status: "pending" },
      "5-spec-review-sa":{ status: "pending" },
      "6-test-plan":    { status: "pending" },
      "7-implementation":{ status: "pending" },
      "8-prd-alignment":{ status: "pending" },
      "9-review":       { status: "pending" },
    },
  };
}

// Minimal Go project for Metis to scan
const GO_MOD = `module github.com/lizard/health-check\n\ngo 1.22\n`;
const MAIN_GO = `package main\n\nimport "net/http"\n\nfunc main() {\n\thttp.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {})\n\thttp.ListenAndServe(":8080", nil)\n}\n`;

function createHephFixture(featureName) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "kratos-heph-qa-"));
  const featureDir = path.join(tmpDir, ".claude", "feature", featureName);
  fs.mkdirSync(featureDir, { recursive: true });

  fs.writeFileSync(path.join(featureDir, "prd.md"), makePrd(featureName));
  fs.writeFileSync(path.join(featureDir, "prd-challenge.md"), makePrdChallenge(featureName));
  fs.writeFileSync(path.join(featureDir, "decisions.md"), makeDecisions(featureName));
  fs.writeFileSync(path.join(featureDir, "status.json"), JSON.stringify(makeStatusJson(featureName), null, 2));

  // Seed a minimal Go project so Metis has real code to scan
  fs.writeFileSync(path.join(tmpDir, "go.mod"), GO_MOD);
  fs.writeFileSync(path.join(tmpDir, "main.go"), MAIN_GO);

  return tmpDir;
}

function createGapFixture() {
  return fs.mkdtempSync(path.join(os.tmpdir(), "kratos-gap-qa-"));
}

// ── Event stream helpers ───────────────────────────────────────────────────────

function collectEvents(stream, handlers, timeoutMs) {
  return Promise.race([
    (async () => {
      for await (const msg of stream) {
        handlers.onMessage?.(msg);

        if (msg.type === "assistant") {
          for (const block of msg.message?.content ?? []) {
            if (block.type === "text") {
              handlers.onText?.(block.text ?? "");
            }
            if (block.type === "tool_use") {
              handlers.onToolUse?.(block);
              if (DEBUG) {
                const input = JSON.stringify(block.input ?? {});
                process.stdout.write(`  [tool] ${block.name}(${input.slice(0, 120)})\n`);
              }
            }
          }
        }
        if (msg.type === "result") {
          handlers.onResult?.(msg);
        }
      }
    })(),
    new Promise((_, reject) =>
      setTimeout(() => reject(new Error(`TIMEOUT after ${timeoutMs / 1000}s`)), timeoutMs)
    ),
  ]);
}

// ── Test 1: Gap analysis runs in Kratos ───────────────────────────────────────

async function testGapAnalysis() {
  console.log("\n" + "═".repeat(60));
  console.log("TEST 1: Gap analysis in orchestrator (not Athena)");
  console.log("═".repeat(60));

  const tmpDir = createGapFixture();
  console.log(`Fixture: ${tmpDir}`);

  // Fully-specified request → ambiguity should be ≤ 0.10 → no AskUserQuestion needed
  const PROMPT = `/kratos:main build a minimal Go HTTP health-check endpoint.
Feature name: qa-gap-test.
Requirements:
- GET /health returns 200 with body {"status":"ok"}
- Single file main.go, stdlib only (no external dependencies)
- Listen on PORT env var, default 8080
- On startup, log "listening on :<port>" to stderr`;

  const results = {
    athenaSpawns: [],         // all Athena Task calls: { phase, promptSnippet }
    askUserQuestionCalls: [], // AskUserQuestion tool calls before Athena
    prdCreated: false,
    errored: null,
  };

  let athenaSpawnCount = 0;

  const stream = query({
    prompt: PROMPT,
    options: {
      cwd: tmpDir,
      model: MODEL,
      permissionMode: "bypassPermissions",
      allowDangerouslySkipPermissions: true,
      plugins: [{ type: "local", path: KRATOS_PLUGIN_PATH }],
    },
  });

  try {
    await collectEvents(stream, {
      onToolUse(block) {
        if (block.name === "Task") {
          const sub = block.input?.subagent_type ?? "";
          const prompt = block.input?.prompt ?? "";
          if (sub === "kratos:athena") {
            athenaSpawnCount++;
            const phase = prompt.match(/PHASE:\s*(\S+)/)?.[1] ?? "UNKNOWN";
            results.athenaSpawns.push({ phase, promptSnippet: prompt.slice(0, 200) });
            process.stdout.write(`  → Athena spawn #${athenaSpawnCount}: PHASE=${phase}\n`);
          }
        }
        if (block.name === "AskUserQuestion") {
          results.askUserQuestionCalls.push({
            beforeAthena: athenaSpawnCount === 0,
            question: block.input?.questions?.[0]?.question?.slice(0, 80) ?? "(no question)",
          });
          process.stdout.write(`  → AskUserQuestion called (beforeAthena=${athenaSpawnCount === 0})\n`);
        }
        if (block.name === "Write" || block.name === "Edit") {
          const fp = block.input?.file_path ?? "";
          if (fp.endsWith("prd.md")) {
            results.prdCreated = true;
            process.stdout.write(`  → prd.md written\n`);
          }
        }
      },
      onText(text) {
        if (text.includes("prd.md") && text.includes("created")) {
          results.prdCreated = true;
        }
      },
    }, TIMEOUT_MS);
  } catch (err) {
    results.errored = err.message;
  }

  // ── Assertions ──────────────────────────────────────────────────────────────

  const checks = [];

  // 1. Athena was spawned at least once
  checks.push({
    name: "Athena spawned",
    pass: results.athenaSpawns.length > 0,
    detail: `spawned ${results.athenaSpawns.length} time(s)`,
  });

  // 2. Athena was NOT spawned with PHASE: GAP_ANALYSIS
  const hadGapAnalysisPhase = results.athenaSpawns.some(s => s.phase === "GAP_ANALYSIS");
  checks.push({
    name: "Athena NOT spawned with PHASE: GAP_ANALYSIS",
    pass: !hadGapAnalysisPhase,
    detail: hadGapAnalysisPhase ? "FAIL: GAP_ANALYSIS phase found" : "No GAP_ANALYSIS spawn",
  });

  // 3. Athena was spawned with PHASE: CREATE_PRD
  const hadCreatePrd = results.athenaSpawns.some(s => s.phase === "CREATE_PRD");
  checks.push({
    name: "Athena spawned with PHASE: CREATE_PRD",
    pass: hadCreatePrd,
    detail: `Phases seen: ${results.athenaSpawns.map(s => s.phase).join(", ") || "none"}`,
  });

  // 4. prd.md was created
  const prdPath = path.join(tmpDir, ".claude", "feature", "qa-gap-test", "prd.md");
  const prdOnDisk = fs.existsSync(prdPath);
  checks.push({
    name: "prd.md written to disk",
    pass: prdOnDisk || results.prdCreated,
    detail: prdOnDisk ? "found on disk" : "not found on disk",
  });

  // 5. AskUserQuestion calls (if any) happened before Athena spawn
  const askAfterAthena = results.askUserQuestionCalls.filter(c => !c.beforeAthena);
  checks.push({
    name: "AskUserQuestion only called before Athena (if at all)",
    pass: askAfterAthena.length === 0,
    detail: `${results.askUserQuestionCalls.length} total AQU calls, ${askAfterAthena.length} after Athena`,
  });

  if (!KEEP_TMP) fs.rmSync(tmpDir, { recursive: true, force: true });
  return reportChecks("TEST 1", checks, results.errored);
}

// ── Test 2: Hephaestus gate — Metis spawned by Kratos ─────────────────────────

async function testHephaestusGate() {
  console.log("\n" + "═".repeat(60));
  console.log("TEST 2: Hephaestus gate — Metis spawned by Kratos (no Arena)");
  console.log("═".repeat(60));

  const FEATURE = "qa-heph-gate-test";
  const tmpDir = createHephFixture(FEATURE);
  console.log(`Fixture: ${tmpDir}`);

  const events = []; // ordered list of notable events for ordering checks
  const results = {
    hephaestusSpawns: [],  // { phase, index }
    metisSpawns: [],       // { spawnedBy, index }  spawnedBy = "kratos" | "hephaestus" | "unknown"
    techSpecProposalCreated: false,
    techSpecCreated: false,
    errored: null,
  };

  let eventIndex = 0;
  // Track the most recent Hephaestus spawn so we can attribute nested Metis calls
  let insideHephaestus = false;

  const stream = query({
    prompt: `/kratos:main the ${FEATURE} feature needs a tech spec. Advance to Stage 4.`,
    options: {
      cwd: tmpDir,
      model: MODEL,
      permissionMode: "bypassPermissions",
      allowDangerouslySkipPermissions: true,
      plugins: [{ type: "local", path: KRATOS_PLUGIN_PATH }],
    },
  });

  try {
    await collectEvents(stream, {
      onMessage(msg) {
        // Heuristic: tool_use from the top-level agent vs inside a subagent
        // The SDK surfaces all messages; we use event ordering to determine nesting
      },
      onToolUse(block) {
        eventIndex++;
        if (block.name === "Task") {
          const sub = block.input?.subagent_type ?? "";
          const prompt = block.input?.prompt ?? "";

          if (sub === "kratos:hephaestus") {
            const phase = prompt.match(/PHASE:\s*(\S+)/)?.[1] ?? "UNKNOWN";
            insideHephaestus = true;
            results.hephaestusSpawns.push({ phase, index: eventIndex });
            process.stdout.write(`  → Hephaestus spawn #${results.hephaestusSpawns.length}: PHASE=${phase} (event ${eventIndex})\n`);
          }

          if (sub === "kratos:metis") {
            // If this Metis spawn appears BEFORE any Hephaestus spawn, it was by Kratos
            const beforeAnyHeph = results.hephaestusSpawns.length === 0;
            results.metisSpawns.push({ spawnedBeforeHephaestus: beforeAnyHeph, index: eventIndex });
            process.stdout.write(`  → Metis spawn (event ${eventIndex}, beforeHephaestus=${beforeAnyHeph})\n`);
          }
        }

        if (block.name === "Write" || block.name === "Edit") {
          const fp = block.input?.file_path ?? "";
          if (fp.endsWith("tech-spec-proposal.md")) {
            results.techSpecProposalCreated = true;
            process.stdout.write(`  → tech-spec-proposal.md written\n`);
          }
          if (fp.endsWith("tech-spec.md")) {
            results.techSpecCreated = true;
            process.stdout.write(`  → tech-spec.md written\n`);
          }
        }
      },
    }, TIMEOUT_MS);
  } catch (err) {
    results.errored = err.message;
  }

  // ── Assertions ──────────────────────────────────────────────────────────────

  const checks = [];

  // 1. Metis was spawned
  checks.push({
    name: "Metis spawned (no Arena → scan required)",
    pass: results.metisSpawns.length > 0,
    detail: `${results.metisSpawns.length} Metis spawn(s)`,
  });

  // 2. Metis was spawned BEFORE the first Hephaestus spawn (by Kratos, not Hephaestus)
  const metisBeforeHeph = results.metisSpawns.some(m => m.spawnedBeforeHephaestus);
  checks.push({
    name: "Metis spawned by Kratos (before Hephaestus ANALYZE)",
    pass: metisBeforeHeph,
    detail: metisBeforeHeph
      ? `Metis event index ${results.metisSpawns[0]?.index}, first Hephaestus at ${results.hephaestusSpawns[0]?.index}`
      : "Metis not seen before Hephaestus",
  });

  // 3. Hephaestus spawned with PHASE: ANALYZE
  const analyzeSpawn = results.hephaestusSpawns.find(s => s.phase === "ANALYZE");
  checks.push({
    name: "Hephaestus spawned with PHASE: ANALYZE",
    pass: !!analyzeSpawn,
    detail: `Phases seen: ${results.hephaestusSpawns.map(s => s.phase).join(", ") || "none"}`,
  });

  // 4. tech-spec-proposal.md created
  const proposalPath = path.join(tmpDir, ".claude", "feature", FEATURE, "tech-spec-proposal.md");
  const proposalOnDisk = fs.existsSync(proposalPath);
  checks.push({
    name: "tech-spec-proposal.md written",
    pass: proposalOnDisk || results.techSpecProposalCreated,
    detail: proposalOnDisk ? "found on disk" : "not found on disk",
  });

  // 5. Hephaestus spawned with PHASE: WRITE_SPEC
  const writeSpecSpawn = results.hephaestusSpawns.find(s => s.phase === "WRITE_SPEC");
  checks.push({
    name: "Hephaestus spawned with PHASE: WRITE_SPEC",
    pass: !!writeSpecSpawn,
    detail: writeSpecSpawn ? `at event ${writeSpecSpawn.index}` : "not seen",
  });

  // 6. tech-spec.md created
  const specPath = path.join(tmpDir, ".claude", "feature", FEATURE, "tech-spec.md");
  const specOnDisk = fs.existsSync(specPath);
  checks.push({
    name: "tech-spec.md written",
    pass: specOnDisk || results.techSpecCreated,
    detail: specOnDisk ? "found on disk" : "not found on disk",
  });

  // 7. Hephaestus spawned exactly twice (ANALYZE + WRITE_SPEC)
  checks.push({
    name: "Hephaestus spawned exactly 2 times",
    pass: results.hephaestusSpawns.length === 2,
    detail: `spawned ${results.hephaestusSpawns.length} time(s)`,
  });

  if (!KEEP_TMP) fs.rmSync(tmpDir, { recursive: true, force: true });
  return reportChecks("TEST 2", checks, results.errored);
}

// ── Reporter ──────────────────────────────────────────────────────────────────

function reportChecks(label, checks, errored) {
  console.log(`\n${label} Results:`);
  let passed = 0;
  for (const c of checks) {
    const icon = c.pass ? "✓" : "✗";
    console.log(`  ${icon} ${c.name} — ${c.detail}`);
    if (c.pass) passed++;
  }
  if (errored) {
    console.log(`\n  ⚠ Error: ${errored}`);
  }
  const allPassed = passed === checks.length && !errored;
  console.log(`\n  ${allPassed ? "PASS" : "FAIL"} (${passed}/${checks.length} checks)`);
  return allPassed;
}

// ── Main ──────────────────────────────────────────────────────────────────────

async function main() {
  console.log(`Model: ${MODEL}`);
  console.log(`Plugin: ${KRATOS_PLUGIN_PATH}`);

  const results = [];

  if (!FILTER || FILTER === "gap") {
    results.push(await testGapAnalysis());
  }
  if (!FILTER || FILTER === "heph") {
    results.push(await testHephaestusGate());
  }

  console.log("\n" + "═".repeat(60));
  const allPassed = results.every(Boolean);
  console.log(`Overall: ${allPassed ? "ALL PASS ✓" : "SOME FAILED ✗"}`);
  process.exit(allPassed ? 0 : 1);
}

main().catch(err => {
  console.error("Fatal:", err);
  process.exit(1);
});
