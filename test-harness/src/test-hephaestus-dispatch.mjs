#!/usr/bin/env node
/**
 * test-hephaestus-dispatch.mjs
 *
 * Verifies that after the dispatch-protocol removal, Hephaestus:
 *   1. Does NOT emit DISPATCH_TO or HEPHAESTUS_DIRECTIVE_RESULT blocks
 *   2. Calls kratos:metis directly via the Task tool (CODEBASE_SCAN)
 *   3. Is only spawned ONCE by Kratos (old pattern spawned it twice)
 *
 * Usage:
 *   node src/test-hephaestus-dispatch.mjs
 *   node src/test-hephaestus-dispatch.mjs --model claude-opus-4-7
 *   node src/test-hephaestus-dispatch.mjs --keep-tmp   # keep temp dir for inspection
 */

import { query } from "@anthropic-ai/claude-agent-sdk";
import fs from "fs";
import path from "path";
import os from "os";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const KRATOS_PLUGIN_PATH = path.resolve(__dirname, "../../plugins/kratos");

// ── CLI args ─────────────────────────────────────────────────────────────────

const args = process.argv.slice(2);
const modelArg = args.find((_, i) => args[i - 1] === "--model") ?? null;
const MODEL = modelArg ?? "claude-sonnet-4-6";
const KEEP_TMP = args.includes("--keep-tmp");
const DEBUG = args.includes("--debug");
const TIMEOUT_MS = 10 * 60 * 1000; // 10 minutes

// ── Seed data ─────────────────────────────────────────────────────────────────

const PRD_CONTENT = `# Product Requirements Document (PRD)

## Document Info
| Field | Value |
|-------|-------|
| **Feature** | dispatch-test |
| **Author** | Athena (PM Agent) |
| **Status** | Approved |
| **Date** | ${new Date().toISOString().slice(0, 10)} |

## 0. Original Request

**Verbatim user ask:**
> Build a CLI tool in Go named \`dispatch-test\` that reads a list of file paths from stdin and prints a SHA-256 checksum for each file, one per line.

**Athena's one-sentence restatement:**
A Go CLI that accepts file paths via stdin and outputs SHA-256 checksums.

**Scope confirmation:** This PRD addresses the request above.

## 1. Executive Summary
A lightweight Go CLI that reads newline-separated file paths from stdin and prints SHA-256 checksums to stdout. No external dependencies; stdlib only.

## 2. Problem Statement

### Current Situation
Users must shell out to sha256sum or openssl — not always available cross-platform.

### Target Users
| Persona | Description | Primary Need |
|---------|-------------|--------------|
| Developer | Cross-platform Go developer | Portable checksum tool |

### Pain Points
1. sha256sum unavailable on macOS by default
2. No stdlib-only Go alternative exists in the project

## 3. Goals & Success Metrics

### Out of Scope
- File writing, renaming, or deletion
- Hash algorithms other than SHA-256

## 4. Requirements

### P0 - Must Have
| ID | Requirement | User Story | Acceptance Criteria |
|----|-------------|------------|---------------------|
| FR-001 | Read file paths from stdin | As a developer, I want to pipe paths via stdin | Given valid paths, when run, then SHA-256 per line |
| FR-002 | Output checksum per line | As a developer, I want one line per file | Given N inputs, then N output lines in same order |
| FR-003 | Exit code 1 on unreadable file | As a developer, I want errors surfaced | Given a missing file, then stderr message + exit 1 |

## 5. Technical Constraints
- Go stdlib only (no external packages)
- Target: Linux, macOS, Windows
`;

const DECISIONS_CONTENT = `# Decisions Log — dispatch-test

## Product Decisions (Athena — PRD Creation)

- Stdin-based input: simplest pipe-compatible interface. Rejected: flag-based list — more friction.

## Intent Alignment (Athena)

Original ask: Build a Go CLI that reads file paths from stdin and prints SHA-256 checksums
Restatement: A stdlib-only Go CLI that accepts file paths via stdin and outputs SHA-256 checksums per line
Alignment: confirmed
`;

const PRD_CHALLENGE_CONTENT = `# PRD Adversarial Review — dispatch-test

## Reviewer
Nemesis (Devil's Advocate + User Advocate)

## Verdict: APPROVED

## Executive Summary
PRD is clear and well-bounded. No BLOCKING findings.

## Findings
None blocking. FR-003 error handling coverage acceptable.
`;

function buildStatusJson() {
  const now = new Date().toISOString();
  return {
    feature: "dispatch-test",
    created: now,
    updated: now,
    stage: "4-tech-spec",
    pipeline_status: "in-progress",
    mode: "normal",
    implementation_mode: null,
    pipeline: {
      "1-prd": {
        status: "complete",
        agent: "athena",
        started: now,
        completed: now,
        documents: ["prd.md"],
        gap_analysis_rounds: 0,
      },
      "2-prd-review": {
        status: "complete",
        agents: ["nemesis"],
        started: now,
        completed: now,
        documents: ["prd-challenge.md"],
        nemesis_verdict: "approved",
        verdict: "approved",
      },
      "3-decomposition": {
        status: "skipped",
        agent: "daedalus",
        started: null,
        completed: null,
        documents: [],
        output_targets: [],
      },
      "4-tech-spec": {
        status: "pending",
        agent: "hephaestus",
        started: null,
        completed: null,
        documents: [],
        based_on_prd_version: null,
        summary: null,
      },
      "5-spec-review-sa": { status: "pending" },
      "6-test-plan": { status: "pending" },
      "7-implementation": { status: "pending" },
      "8-prd-alignment": { status: "pending" },
      "9-review": { status: "pending" },
    },
  };
}

// ── Go project seed ───────────────────────────────────────────────────────────
// Gives Hephaestus an existing codebase to scan — otherwise he correctly
// skips Metis (nothing to discover in an empty project).

const GO_MOD = `module github.com/lizard/dispatch-test

go 1.22
`;

const MAIN_GO = `package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/lizard/dispatch-test/internal/runner"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	r := runner.New(os.Stdout, os.Stderr)
	if err := r.Run(scanner); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`;

const RUNNER_GO = `// Package runner provides the core processing logic.
package runner

import (
	"bufio"
	"io"
)

// Runner processes lines from the scanner and writes results.
type Runner struct {
	out io.Writer
	err io.Writer
}

// New creates a Runner.
func New(out, err io.Writer) *Runner {
	return &Runner{out: out, err: err}
}

// Run iterates scanner lines and calls processLine for each.
func (r *Runner) Run(scanner *bufio.Scanner) error {
	hadError := false
	for scanner.Scan() {
		line := scanner.Text()
		if err := r.processLine(line); err != nil {
			hadError = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if hadError {
		return fmt.Errorf("one or more lines failed")
	}
	return nil
}

func (r *Runner) processLine(line string) error {
	// TODO: implementation goes here
	return nil
}
`;

const RUNNER_TEST_GO = `package runner_test

import (
	"bytes"
	"bufio"
	"strings"
	"testing"

	"github.com/lizard/dispatch-test/internal/runner"
)

func TestRunEmpty(t *testing.T) {
	var out, errBuf bytes.Buffer
	r := runner.New(&out, &errBuf)
	scanner := bufio.NewScanner(strings.NewReader(""))
	if err := r.Run(scanner); err != nil {
		t.Fatal(err)
	}
}
`;

// ── Temp project setup ────────────────────────────────────────────────────────

function createTempProject() {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "kratos-heph-test-"));
  const featureDir = path.join(tmpDir, ".claude", "feature", "dispatch-test");
  fs.mkdirSync(featureDir, { recursive: true });

  // Feature artifacts (stages 1-2 done)
  fs.writeFileSync(path.join(featureDir, "prd.md"), PRD_CONTENT);
  fs.writeFileSync(path.join(featureDir, "decisions.md"), DECISIONS_CONTENT);
  fs.writeFileSync(path.join(featureDir, "prd-challenge.md"), PRD_CHALLENGE_CONTENT);
  fs.writeFileSync(
    path.join(featureDir, "status.json"),
    JSON.stringify(buildStatusJson(), null, 2)
  );

  // Seed Go project — gives Hephaestus real code to scan via Metis
  fs.writeFileSync(path.join(tmpDir, "go.mod"), GO_MOD);
  fs.writeFileSync(path.join(tmpDir, "main.go"), MAIN_GO);
  const internalDir = path.join(tmpDir, "internal", "runner");
  fs.mkdirSync(internalDir, { recursive: true });
  fs.writeFileSync(path.join(internalDir, "runner.go"), RUNNER_GO);
  fs.writeFileSync(path.join(internalDir, "runner_test.go"), RUNNER_TEST_GO);

  return tmpDir;
}

// ── Main test ─────────────────────────────────────────────────────────────────

async function main() {
  const tmpDir = createTempProject();
  console.log(`Temp project: ${tmpDir}`);
  console.log(`Model: ${MODEL}`);
  console.log(`Plugin: ${KRATOS_PLUGIN_PATH}\n`);

  // Tracking state
  const violations = [];       // dispatch protocol fragments found in text
  let metisDirectCallSeen = false;   // Task(kratos:metis) with CODEBASE_SCAN
  let hephaestusSpawnCount = 0;      // how many times Kratos spawned Hephaestus
  let metisSpawnCount = 0;           // total Metis spawns (any spawner)
  let totalMessages = 0;

  const stream = query({
    prompt:
      "/kratos:main the dispatch-test feature needs a tech spec. Advance to Stage 4.",
    options: {
      cwd: tmpDir,
      model: MODEL,
      permissionMode: "bypassPermissions",
      allowDangerouslySkipPermissions: true,
      plugins: [{ type: "local", path: KRATOS_PLUGIN_PATH }],
    },
  });

  let resolved = false;
  const timeoutPromise = new Promise((_, reject) =>
    setTimeout(() => {
      if (!resolved) reject(new Error("TIMEOUT — AskUserQuestion likely stalled the agent (expected)"));
    }, TIMEOUT_MS)
  );

  try {
    await Promise.race([
      (async () => {
        for await (const msg of stream) {
          totalMessages++;

          if (msg.type === "assistant") {
            for (const block of msg.message?.content ?? []) {
              // Check for dispatch protocol violation in text output
              if (block.type === "text") {
                const text = block.text ?? "";
                if (text.includes("DISPATCH_TO:") || text.includes("DISPATCH_TO :")) {
                  violations.push(`[DISPATCH_TO] in text: "${text.slice(0, 120).replace(/\n/g, " ")}..."`);
                }
                if (text.includes("HEPHAESTUS_DIRECTIVE_RESULT")) {
                  violations.push(`[HEPHAESTUS_DIRECTIVE_RESULT] in text: "${text.slice(0, 120).replace(/\n/g, " ")}..."`);
                }
                if (text.includes("DISPATCH_PHASE") || text.includes("DISPATCH_RETURN_TO")) {
                  violations.push(`[DISPATCH_PHASE/RETURN_TO] in text: "${text.slice(0, 120).replace(/\n/g, " ")}..."`);
                }
              }

              // Log all tool calls in debug mode
              if (DEBUG && block.type === "tool_use") {
                process.stdout.write(`  [tool] ${block.name}(${JSON.stringify(block.input ?? {}).slice(0, 120)})\n`);
              }

              // Track Task tool invocations
              if (block.type === "tool_use" && block.name === "Task") {
                const sub = block.input?.subagent_type ?? "";
                const prompt = block.input?.prompt ?? "";

                if (sub === "kratos:hephaestus") {
                  hephaestusSpawnCount++;
                  process.stdout.write(`  → Hephaestus spawn #${hephaestusSpawnCount} detected\n`);
                }

                if (sub === "kratos:metis") {
                  metisSpawnCount++;
                  // Hephaestus's direct call will contain CODEBASE_SCAN
                  if (prompt.includes("CODEBASE_SCAN") || prompt.includes("METIS_SEARCH_DIRECTIVE")) {
                    metisDirectCallSeen = true;
                    process.stdout.write(`  → Metis CODEBASE_SCAN Task call detected (direct from Hephaestus)\n`);
                  }
                }
              }
            }
          }

          // Once we have confirmed the Metis direct call, we've seen enough
          if (metisDirectCallSeen) {
            process.stdout.write("\n✓ Metis direct call confirmed — collecting remaining messages...\n");
            // Continue a bit longer to catch any re-spawn of Hephaestus by Kratos
          }
        }
        resolved = true;
      })(),
      timeoutPromise,
    ]);
  } catch (err) {
    if (err.message.startsWith("TIMEOUT")) {
      console.log(`\nNote: ${err.message}`);
    } else {
      throw err;
    }
  }

  // ── Assertions ──────────────────────────────────────────────────────────────

  const pass = [];
  const fail = [];

  // 1. No dispatch protocol fragments in text
  if (violations.length === 0) {
    pass.push("No DISPATCH_TO / HEPHAESTUS_DIRECTIVE_RESULT / DISPATCH_PHASE in any text output");
  } else {
    for (const v of violations) fail.push(`Dispatch protocol fragment found — ${v}`);
  }

  // 2. Metis was either called directly OR the skip was explicitly documented in decisions.md
  //    Silent skips (neither calling Metis nor documenting the skip) are treated as bugs.
  if (metisSpawnCount > 0) {
    if (metisDirectCallSeen) {
      pass.push(`Metis spawned ${metisSpawnCount}x — all via direct Task call from Hephaestus (no orchestrator dispatch)`);
    } else {
      fail.push(
        `Metis spawned ${metisSpawnCount}x but NOT via a direct CODEBASE_SCAN Task — ` +
        `old orchestrator-dispatch pattern may still be active`
      );
    }
  } else {
    // Metis not called — require an explicit skip decision in decisions.md
    const decisionsPath = path.join(
      tmpDir, ".claude", "feature", "dispatch-test", "decisions.md"
    );
    let decisionsContent = "";
    try { decisionsContent = fs.readFileSync(decisionsPath, "utf8"); } catch { /* file may not exist */ }

    const hasExplicitSkip =
      decisionsContent.includes("Metis scan skipped") ||
      decisionsContent.includes("Codebase Scan Decision");

    if (hasExplicitSkip) {
      pass.push(`Metis not called — explicit skip decision documented in decisions.md (valid Path B)`);
    } else {
      fail.push(
        `Metis not called AND no skip decision in decisions.md — ` +
        `silent skip is not permitted (Hephaestus must call Metis OR document the skip explicitly)`
      );
    }
  }

  // 3. Hephaestus spawned at most once from Kratos
  //    Old pattern: spawned twice (Phase 0 sonnet → Phase 1 opus after Metis)
  if (hephaestusSpawnCount <= 1) {
    pass.push(`Hephaestus spawned ${hephaestusSpawnCount}x from Kratos (old pattern: 2x) — single-spawn confirmed`);
  } else {
    fail.push(
      `Hephaestus spawned ${hephaestusSpawnCount}x — old multi-spawn dispatch pattern still active`
    );
  }


  // ── Report ──────────────────────────────────────────────────────────────────

  console.log("\n" + "═".repeat(64));
  console.log("HEPHAESTUS DISPATCH TEST — RESULTS");
  console.log("═".repeat(64));
  console.log(`Messages collected: ${totalMessages}`);
  console.log(`Hephaestus spawns (from Kratos): ${hephaestusSpawnCount}`);
  console.log(`Metis spawns (any): ${metisSpawnCount}`);
  console.log(`Metis direct call (CODEBASE_SCAN): ${metisDirectCallSeen ? "YES" : "NO"}`);
  console.log(`Dispatch violations: ${violations.length}`);
  console.log("");

  for (const p of pass) console.log(`  ✓ ${p}`);
  for (const f of fail) console.log(`  ✗ ${f}`);

  const allPassed = fail.length === 0;
  console.log("");
  console.log(
    allPassed
      ? "RESULT: ✓ ALL TESTS PASSED — dispatch protocol successfully removed"
      : `RESULT: ✗ ${fail.length} TEST(S) FAILED`
  );
  console.log("═".repeat(64));

  // ── Cleanup ──────────────────────────────────────────────────────────────────

  if (KEEP_TMP) {
    console.log(`\nTemp dir preserved: ${tmpDir}`);
  } else {
    try {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    } catch {
      // Windows EBUSY: briefly locked after process exit — ignore
    }
  }

  process.exit(allPassed ? 0 : 1);
}

main().catch((err) => {
  console.error("\nFatal test error:", err);
  process.exit(1);
});
