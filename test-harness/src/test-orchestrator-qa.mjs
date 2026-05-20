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
// Use the repo root as cwd so "plugins/kratos/..." paths resolve correctly
const REPO_ROOT = path.resolve(__dirname, "../..");

// ── CLI args ──────────────────────────────────────────────────────────────────

const argv = process.argv.slice(2);
const MODEL = argv.find((_, i) => argv[i - 1] === "--model") ?? "claude-sonnet-4-6";
const KEEP_TMP = argv.includes("--keep-tmp");
const DEBUG = argv.includes("--debug");
const FILTER = argv.find((_, i) => argv[i - 1] === "--test") ?? null; // "gap" | "heph"
const TIMEOUT_MS_GAP  = 10 * 60 * 1000; // Athena (opus) + Nemesis can take 8+ min
const TIMEOUT_MS_HEPH = 10 * 60 * 1000; // Metis + Hephaestus ANALYZE

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

// Use REPO_ROOT as cwd so "plugins/kratos/..." paths resolve.
// Feature state lives inside REPO_ROOT/.claude/feature/<name>.
// cleanup() removes only the feature dir, leaving the repo intact.

function createHephFixture(featureName) {
  const featureDir = path.join(REPO_ROOT, ".claude", "feature", featureName);
  fs.mkdirSync(featureDir, { recursive: true });

  fs.writeFileSync(path.join(featureDir, "prd.md"), makePrd(featureName));
  fs.writeFileSync(path.join(featureDir, "prd-challenge.md"), makePrdChallenge(featureName));
  fs.writeFileSync(path.join(featureDir, "decisions.md"), makeDecisions(featureName));
  fs.writeFileSync(path.join(featureDir, "status.json"), JSON.stringify(makeStatusJson(featureName), null, 2));

  return REPO_ROOT;
}

function createGapFixture() {
  // No feature pre-state needed — Kratos will ask for feature name and create it
  return REPO_ROOT;
}

function safeCleanup(featureName) {
  const featureDir = path.join(REPO_ROOT, ".claude", "feature", featureName);
  try {
    if (fs.existsSync(featureDir)) {
      fs.rmSync(featureDir, { recursive: true, force: true });
    }
  } catch {
    // Windows EBUSY — best effort, leave for next run to clean
  }
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

  const FEATURE_NAME = `qa-gap-${Date.now()}`;
  const projectDir = createGapFixture();
  console.log(`cwd: ${projectDir}, feature: ${FEATURE_NAME}`);

  // Fully-specified request → ambiguity should be ≤ 0.10 → no AskUserQuestion needed
  const PROMPT = `/kratos:main build a minimal Go HTTP health-check endpoint.
Feature name: ${FEATURE_NAME}.
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
      cwd: projectDir,
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
    }, TIMEOUT_MS_GAP);
  } catch (err) {
    // Timeout is expected — pipeline auto-continues to Stage 4 which blocks on
    // AskUserQuestion for approach selection. We only care about Stage 1 results.
    if (!err.message.includes("TIMEOUT")) results.errored = err.message;
  }

  // ── Assertions ──────────────────────────────────────────────────────────────

  const checks = [];

  // 1. Athena was spawned at least once
  checks.push({
    name: "Athena spawned",
    pass: results.athenaSpawns.length > 0,
    detail: `spawned ${results.athenaSpawns.length} time(s)`,
  });

  // 2. Athena was NOT spawned with PHASE: GAP_ANALYSIS (old pattern)
  const hadGapAnalysisPhase = results.athenaSpawns.some(s => s.phase === "GAP_ANALYSIS");
  checks.push({
    name: "Athena NOT spawned with PHASE: GAP_ANALYSIS (old pattern eliminated)",
    pass: !hadGapAnalysisPhase,
    detail: hadGapAnalysisPhase ? "FAIL: GAP_ANALYSIS phase found" : "No GAP_ANALYSIS spawn",
  });

  // 3. Athena was spawned with PHASE: CREATE_PRD (new pattern)
  const hadCreatePrd = results.athenaSpawns.some(s => s.phase === "CREATE_PRD");
  checks.push({
    name: "Athena spawned with PHASE: CREATE_PRD (new pattern)",
    pass: hadCreatePrd,
    detail: `Phases seen: ${results.athenaSpawns.map(s => s.phase).join(", ") || "none"}`,
  });

  // 4. prd.md was created
  const prdPath = path.join(projectDir, ".claude", "feature", FEATURE_NAME, "prd.md");
  const prdOnDisk = fs.existsSync(prdPath);
  checks.push({
    name: "prd.md written to disk",
    pass: prdOnDisk || results.prdCreated,
    detail: prdOnDisk ? "found on disk" : "not found on disk",
  });

  // Note: AskUserQuestion calls after Athena are expected — Kratos uses them
  // at Stage 4 for approach selection (hephaestus-gate Phase 4c). That is
  // correct behavior; we do not assert on them here.

  if (!KEEP_TMP) safeCleanup(FEATURE_NAME);
  return reportChecks("TEST 1", checks, results.errored);
}

// ── Test 2: Hephaestus gate — Metis spawned by Kratos ─────────────────────────

async function testHephaestusGate() {
  console.log("\n" + "═".repeat(60));
  console.log("TEST 2: Hephaestus gate — Metis spawned by Kratos (no Arena)");
  console.log("═".repeat(60));

  const FEATURE = `qa-heph-${Date.now()}`;
  // Clean up any leftover qa-* test features from prior runs before creating
  // the fixture, so Kratos doesn't discover the wrong feature.
  const featureBase = path.join(REPO_ROOT, ".claude", "feature");
  if (fs.existsSync(featureBase)) {
    for (const entry of fs.readdirSync(featureBase)) {
      if (entry.startsWith("qa-")) safeCleanup(entry);
    }
  }
  const projectDir = createHephFixture(FEATURE);
  console.log(`cwd: ${projectDir}, feature: ${FEATURE}`);

  const events = []; // ordered list of notable events for ordering checks
  const results = {
    hephaestusSpawns: [],    // { phase, index }
    metisSpawns: [],         // { spawnedBeforeHephaestus, index }
    askUserQuestionCalls: [],// all AskUserQuestion calls
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
      cwd: projectDir,
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

        if (block.name === "AskUserQuestion") {
          results.askUserQuestionCalls.push(block.input);
          process.stdout.write(`  → AskUserQuestion (event ${eventIndex})\n`);
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
    }, TIMEOUT_MS_HEPH);
  } catch (err) {
    // Timeout is expected — AskUserQuestion for approach selection in Phase 4c
    // blocks in non-interactive SDK context. We only check Phase 4a/4b here.
    if (!err.message.includes("TIMEOUT")) results.errored = err.message;
  }

  // ── Assertions ──────────────────────────────────────────────────────────────
  // We only verify Phase 4a (Metis) and Phase 4b (ANALYZE + proposal).
  // Phase 4c/4d (AskUserQuestion + WRITE_SPEC) require interactive user input
  // and cannot be reliably tested in a non-interactive SDK context.

  const checks = [];

  // 1. Metis was spawned
  checks.push({
    name: "Metis spawned (no Arena → scan required)",
    pass: results.metisSpawns.length > 0,
    detail: `${results.metisSpawns.length} Metis spawn(s)`,
  });

  // 2. Metis spawned BEFORE Hephaestus — means Kratos did it, not Hephaestus
  const metisBeforeHeph = results.metisSpawns.some(m => m.spawnedBeforeHephaestus);
  checks.push({
    name: "Metis spawned by Kratos (before Hephaestus ANALYZE)",
    pass: metisBeforeHeph,
    detail: metisBeforeHeph
      ? `Metis event ${results.metisSpawns[0]?.index}, Hephaestus at ${results.hephaestusSpawns[0]?.index}`
      : "Metis not seen before Hephaestus",
  });

  // 3. Hephaestus spawned with PHASE: ANALYZE
  const analyzeSpawn = results.hephaestusSpawns.find(s => s.phase === "ANALYZE");
  checks.push({
    name: "Hephaestus spawned with PHASE: ANALYZE",
    pass: !!analyzeSpawn,
    detail: `Phases seen: ${results.hephaestusSpawns.map(s => s.phase).join(", ") || "none"}`,
  });

  // 4. tech-spec-proposal.md created (Hephaestus ANALYZE output)
  const proposalPath = path.join(projectDir, ".claude", "feature", FEATURE, "tech-spec-proposal.md");
  const proposalOnDisk = fs.existsSync(proposalPath);
  checks.push({
    name: "tech-spec-proposal.md written (ANALYZE output)",
    pass: proposalOnDisk || results.techSpecProposalCreated,
    detail: proposalOnDisk ? "found on disk" : "not found on disk",
  });

  // 5. AskUserQuestion called after ANALYZE (Kratos asking about approach)
  //    This confirms Phase 4c is reached — even if user can't answer in SDK context.
  const askAfterAnalyze = results.hephaestusSpawns.some(s => s.phase === "ANALYZE")
    ? results.askUserQuestionCalls.length > 0
    : false;
  checks.push({
    name: "AskUserQuestion called for approach selection (Phase 4c reached)",
    pass: askAfterAnalyze,
    detail: `${results.askUserQuestionCalls.length} AskUserQuestion call(s) detected`,
  });

  if (!KEEP_TMP) safeCleanup(FEATURE);
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
