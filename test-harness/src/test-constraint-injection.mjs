#!/usr/bin/env node
/**
 * test-constraint-injection.mjs
 *
 * Measures how many times the terse "Output constraint" sentence gets
 * injected into a Kratos session, across all four known injection channels,
 * and flags waste (the same sentence repeated more times than necessary).
 *
 * Channels (see plugins/kratos/hooks/session-start.cjs,
 * kratos-dev/go/internal/cli/hook.go, plugins/kratos/hooks/path-inject.cjs,
 * kratos-dev/go/internal/cli/agent.go — verified directly, not assumed):
 *   1. SessionStart   — session-start.cjs prints OUTPUT_CONSTRAINT once, unconditionally.
 *   2. UserPromptSubmit — `kratos hook prompt-submit` appends the constraint to
 *      additionalContext once per prompt that matches a kratos/god keyword and
 *      is NOT already a `/kratos:` slash command (those bypass this hook entirely).
 *   3. SubagentStart  — path-inject.cjs emits the constraint TWICE per god spawn:
 *      once as the literal first baseParts entry, and again inside the composed
 *      protocol block (`kratos agent protocol <god> --resolve`), since every
 *      agent lists "output-format" in its protocol_sections frontmatter.
 *   4. `kratos agent load <god> --resolve` — inline command-mode gods get the
 *      constraint once, via the same composed protocol block.
 *
 * MODE A — static accounting (default, zero API spend). Invokes each channel's
 * real emitting process/binary directly and counts the constraint substring in
 * its actual output. Then models a session: 1 SessionStart + P keyword prompts
 * + G god spawns + I inline loads, and reports total/expected-minimum/wasted
 * copies and their approximate token cost. Exits non-zero when wasted copies
 * exceed WASTE_BUDGET.
 *
 * MODE B — live verification (--live). Runs one real SDK session (multiple
 * turns via `resume`) that mixes plain kratos-keyword prompts with a
 * `/kratos:main` spec prompt that spawns a god subagent, then parses the
 * on-disk JSONL transcript(s) for ATTRIBUTED constraint copies — i.e. only
 * copies that arrived as hook-injected context, never occurrences that show
 * up merely because the assistant echoed the sentence or read a file
 * containing it. Real API spend — do not run without deliberately choosing to.
 *
 * Usage:
 *   node src/test-constraint-injection.mjs                     # Mode A, defaults P=5 G=3 I=1
 *   node src/test-constraint-injection.mjs --p 10 --g 5 --i 2  # Mode A, custom session model
 *   node src/test-constraint-injection.mjs --gods ares,athena   # Mode A, custom god set
 *   node src/test-constraint-injection.mjs --live               # Mode B (real API calls)
 *
 * Output: test-harness/results/<run-id>/report.json
 */

import fs from "fs";
import path from "path";
import os from "os";
import crypto from "crypto";
import { fileURLToPath } from "url";
import { spawnSync, execFileSync } from "child_process";
import { createRequire } from "module";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const HARNESS_ROOT = path.resolve(__dirname, "..");
const REPO_ROOT = path.resolve(HARNESS_ROOT, "..");
const KRATOS_PLUGIN_PATH = path.resolve(__dirname, "../../plugins/kratos");

const require = createRequire(import.meta.url);

// ── Constants ────────────────────────────────────────────────────────────────

// Wasted copies allowed before Mode A fails loudly. Set to 0 deliberately —
// channel 3's literal double-injection is real waste we want visible today.
const WASTE_BUDGET = 0;

// Verbatim copy of the sentence injected by every channel (see
// plugins/kratos/hooks/session-start.cjs:28-29, the same text embedded in
// references/agent-protocol.md's "Output Format" section).
const CONSTRAINT_TEXT =
  "**Output constraint:** Terse. Drop articles, filler, pleasantries. Pattern: `[status] [what] [result]. [next].` Fragments OK. Technical terms exact. Code blocks unchanged.";

// Distinctive, stable fragment used to count occurrences — matches the exact
// sentence even if surrounding whitespace/newlines differ between channels.
const CONSTRAINT_MARKER_REGEX = /\*\*Output constraint:\*\*\s*Terse\./g;

// Rough token-length estimate for the sentence. Method: chars / 4 (a common
// English-text heuristic; not a real tokenizer call — avoids pulling in a
// tokenizer dependency for a static-accounting script). Stated explicitly so
// the number is never mistaken for an exact token count.
const TOKEN_CHARS_PER_TOKEN = 4;
const TOKENS_PER_COPY = Math.round(CONSTRAINT_TEXT.length / TOKEN_CHARS_PER_TOKEN);

const DEFAULT_GODS = ["ares", "athena", "hermes", "iris"];

function countConstraint(text) {
  if (typeof text !== "string" || text.length === 0) return 0;
  return (text.match(CONSTRAINT_MARKER_REGEX) || []).length;
}

// ── CLI arg parsing ──────────────────────────────────────────────────────────

function parseArgs(argv) {
  const args = {
    live: false,
    p: 5, // kratos-keyword prompts modeled (UserPromptSubmit, channel 2)
    g: 3, // god spawns modeled (SubagentStart, channel 3)
    i: 1, // inline command-mode god loads modeled (channel 4)
    gods: DEFAULT_GODS,
    model: "claude-sonnet-4-6",
  };
  for (let idx = 2; idx < argv.length; idx++) {
    const a = argv[idx];
    if (a === "--live") args.live = true;
    else if (a === "--p" && argv[idx + 1]) args.p = parseInt(argv[++idx], 10);
    else if (a === "--g" && argv[idx + 1]) args.g = parseInt(argv[++idx], 10);
    else if (a === "--i" && argv[idx + 1]) args.i = parseInt(argv[++idx], 10);
    else if (a === "--gods" && argv[idx + 1]) {
      args.gods = argv[++idx].split(",").map((s) => s.trim()).filter(Boolean);
    } else if (a === "--model" && argv[idx + 1]) args.model = argv[++idx];
  }
  return args;
}

// ── Kratos binary resolution (reuses the plugin's own resolver — same logic
// every hook uses, so Mode A measures exactly what production hooks measure) ─

function resolveKratosBin() {
  try {
    const { resolveBinary } = require(
      path.join(KRATOS_PLUGIN_PATH, "hooks", "kratos-bin.cjs")
    );
    return resolveBinary();
  } catch {
    return null;
  }
}

// ── Mode A: channel invocations ──────────────────────────────────────────────

function runNode(scriptPath, { input } = {}) {
  return spawnSync(process.execPath, [scriptPath], {
    input,
    encoding: "utf-8",
    timeout: 10000,
  });
}

function runKratos(kratosBin, cmdArgs, { input } = {}) {
  return spawnSync(kratosBin, cmdArgs, {
    input,
    encoding: "utf-8",
    timeout: 10000,
  });
}

function parseAdditionalContext(stdout) {
  try {
    const parsed = JSON.parse(stdout ?? "");
    return parsed?.hookSpecificOutput?.additionalContext ?? "";
  } catch {
    return "";
  }
}

// Channel 1 — node plugins/kratos/hooks/session-start.cjs, count in stdout.
function measureSessionStart() {
  const scriptPath = path.join(KRATOS_PLUGIN_PATH, "hooks", "session-start.cjs");
  const res = runNode(scriptPath);
  const stdout = res.stdout ?? "";
  return {
    label: "1. SessionStart (session-start.cjs)",
    detail: "printed once, unconditionally, every session",
    invocation: `node ${path.relative(REPO_ROOT, scriptPath)}`,
    count: countConstraint(stdout),
    raw: stdout,
    stderr: res.stderr ?? "",
    error: res.error ? String(res.error) : null,
  };
}

// Channel 2 — `kratos hook prompt-submit` fed a kratos-keyword prompt on
// stdin, count in the emitted additionalContext.
function measurePromptSubmit(kratosBin, prompt) {
  const res = runKratos(kratosBin, ["hook", "prompt-submit"], {
    input: JSON.stringify({ prompt }),
  });
  const additionalContext = parseAdditionalContext(res.stdout);
  return {
    label: "2. UserPromptSubmit (kratos hook prompt-submit)",
    detail: 'non-slash prompt matching a kratos/god keyword — e.g. "use kratos to build a login feature"',
    invocation: `kratos hook prompt-submit  (stdin prompt: ${JSON.stringify(prompt)})`,
    count: countConstraint(additionalContext),
    raw: additionalContext,
    stderr: res.stderr ?? "",
    error: res.error ? String(res.error) : null,
  };
}

// Channel 3 — node plugins/kratos/hooks/path-inject.cjs with
// {"agent_type":"kratos:<god>"} on stdin, count inside
// hookSpecificOutput.additionalContext. One measurement per god.
function measurePathInject(god) {
  const scriptPath = path.join(KRATOS_PLUGIN_PATH, "hooks", "path-inject.cjs");
  const res = runNode(scriptPath, {
    input: JSON.stringify({ agent_type: `kratos:${god}` }),
  });
  const additionalContext = parseAdditionalContext(res.stdout);
  return {
    god,
    invocation: `node ${path.relative(REPO_ROOT, scriptPath)}  (stdin: {"agent_type":"kratos:${god}"})`,
    count: countConstraint(additionalContext),
    raw: additionalContext,
    stderr: res.stderr ?? "",
    error: res.error ? String(res.error) : null,
  };
}

// Channel 4 — `kratos agent load <god> --resolve`, count in stdout. One
// measurement per god.
function measureAgentLoad(kratosBin, god) {
  const res = runKratos(kratosBin, ["agent", "load", god, "--resolve"]);
  const stdout = res.stdout ?? "";
  return {
    god,
    invocation: `kratos agent load ${god} --resolve`,
    count: countConstraint(stdout),
    raw: stdout,
    stderr: res.stderr ?? "",
    error: res.error ? String(res.error) : null,
  };
}

// ── Mode A: orchestration ────────────────────────────────────────────────────

function padRow(cols, widths) {
  return cols.map((c, idx) => String(c).padEnd(widths[idx])).join(" ");
}

async function runModeA(args) {
  console.log("\nKratos Constraint Injection Audit — MODE A (static accounting, zero API spend)");

  const kratosBin = resolveKratosBin();
  if (!kratosBin) {
    console.error(
      "FATAL: kratos binary not resolvable (checked plugin bin/ and ~/.kratos/bin/). " +
      "Channels 2 and 4 require it — cannot compute Mode A totals."
    );
    return 1;
  }

  console.log(`Kratos bin   : ${kratosBin}`);
  console.log(`Gods tested  : ${args.gods.join(", ")}`);
  console.log(`Session model: P=${args.p} keyword prompts, G=${args.g} god spawns, I=${args.i} inline loads`);

  // Invoke every channel for real.
  const ch1 = measureSessionStart();
  const ch2 = measurePromptSubmit(kratosBin, "use kratos to build a login feature");
  const ch3PerGod = args.gods.map((g) => measurePathInject(g));
  const ch4PerGod = args.gods.map((g) => measureAgentLoad(kratosBin, g));

  // Channels 3/4 should emit the same copy count regardless of which god is
  // loaded (the duplication comes from the shared output-format protocol
  // section, not from any one agent's frontmatter) — verified against
  // ares/athena/hermes/iris. Use the max as the representative
  // copies-per-invocation figure, but warn if any god actually disagrees.
  const ch3Count = ch3PerGod.length ? Math.max(...ch3PerGod.map((r) => r.count)) : 0;
  const ch4Count = ch4PerGod.length ? Math.max(...ch4PerGod.map((r) => r.count)) : 0;
  const ch3Inconsistent = ch3PerGod.some((r) => r.count !== ch3Count);
  const ch4Inconsistent = ch4PerGod.some((r) => r.count !== ch4Count);

  // ── Per-channel table ──
  console.log(`\n${"═".repeat(78)}`);
  console.log("PER-CHANNEL COPY COUNT (measured from real invocation output)");
  console.log(`${"═".repeat(78)}`);
  const widths = [50, 14];
  console.log(padRow(["CHANNEL", "COPIES/INVOC"], widths));
  console.log("─".repeat(78));
  console.log(padRow([ch1.label, ch1.count], widths));
  console.log(`    ${ch1.detail}`);
  console.log(padRow([ch2.label, ch2.count], widths));
  console.log(`    ${ch2.detail}`);
  console.log(padRow(["3. SubagentStart (path-inject.cjs, per god)", ch3Count], widths));
  console.log(`    literal baseParts[0] + composed protocol's output-format section`);
  console.log(`    ${ch3PerGod.map((r) => `${r.god}=${r.count}`).join(" ")}`);
  console.log(padRow(["4. agent load --resolve (inline command-mode)", ch4Count], widths));
  console.log(`    composed protocol's output-format section`);
  console.log(`    ${ch4PerGod.map((r) => `${r.god}=${r.count}`).join(" ")}`);
  if (ch3Inconsistent) console.log("    WARNING: channel 3 copy count differs across gods (see per-god breakdown above)");
  if (ch4Inconsistent) console.log("    WARNING: channel 4 copy count differs across gods (see per-god breakdown above)");

  // ── Modeled session totals ──
  const subtotal1 = ch1.count * 1;
  const subtotal2 = ch2.count * args.p;
  const subtotal3 = ch3Count * args.g;
  const subtotal4 = ch4Count * args.i;
  const total = subtotal1 + subtotal2 + subtotal3 + subtotal4;
  const expectedMin = 1 + args.g + args.i; // 1 main session + G subagents + I inline loads
  const wasted = total - expectedMin;
  const wastedTokens = wasted * TOKENS_PER_COPY;

  console.log(`\n${"═".repeat(78)}`);
  console.log(`MODELED SESSION (1 SessionStart + P=${args.p} prompts + G=${args.g} spawns + I=${args.i} inline loads)`);
  console.log(`${"═".repeat(78)}`);
  const mWidths = [46, 10, 10, 10];
  console.log(padRow(["CHANNEL", "COPIES", "INVOC.", "SUBTOTAL"], mWidths));
  console.log("─".repeat(78));
  console.log(padRow(["1. SessionStart x1", ch1.count, 1, subtotal1], mWidths));
  console.log(padRow([`2. UserPromptSubmit x P (${args.p})`, ch2.count, args.p, subtotal2], mWidths));
  console.log(padRow([`3. SubagentStart x G (${args.g})`, ch3Count, args.g, subtotal3], mWidths));
  console.log(padRow([`4. agent load x I (${args.i})`, ch4Count, args.i, subtotal4], mWidths));
  console.log("─".repeat(78));
  console.log(`TOTAL copies                    : ${total}`);
  console.log(`EXPECTED MINIMUM (1 main + G subagents + I inline loads) : ${expectedMin}`);
  console.log(`WASTED copies                    : ${wasted}`);
  console.log(`Wasted tokens (~${TOKENS_PER_COPY} tok/copy, chars/${TOKEN_CHARS_PER_TOKEN} approx) : ${wastedTokens}`);
  console.log(`WASTE BUDGET                     : ${WASTE_BUDGET}`);

  const exitCode = wasted > WASTE_BUDGET ? 1 : 0;
  console.log(`\nRESULT: ${exitCode === 0 ? "PASS" : "FAIL"} (wasted ${wasted} > budget ${WASTE_BUDGET} = ${wasted > WASTE_BUDGET})`);

  // ── Write results ──
  const runId =
    new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19) +
    "-constraint-audit-" +
    crypto.randomBytes(3).toString("hex");
  const runDir = path.join(HARNESS_ROOT, "results", runId);
  fs.mkdirSync(runDir, { recursive: true });

  const report = {
    mode: "A",
    runId,
    kratosBin,
    gods: args.gods,
    sessionModel: { p: args.p, g: args.g, i: args.i },
    channels: {
      sessionStart: ch1,
      promptSubmit: ch2,
      pathInjectPerGod: ch3PerGod,
      agentLoadPerGod: ch4PerGod,
      representative: { channel3CopiesPerInvocation: ch3Count, channel4CopiesPerInvocation: ch4Count },
    },
    totals: {
      subtotal1,
      subtotal2,
      subtotal3,
      subtotal4,
      total,
      expectedMin,
      wasted,
      wasteBudget: WASTE_BUDGET,
      tokensPerCopy: TOKENS_PER_COPY,
      wastedTokens,
    },
    exitCode,
  };
  fs.writeFileSync(path.join(runDir, "report.json"), JSON.stringify(report, null, 2));
  console.log(`\nFull output: ${runDir}`);

  return exitCode;
}

// ── Mode B: live verification ────────────────────────────────────────────────

// Derives the ~/.claude/projects/<slug> directory name Claude Code uses for a
// given cwd: every path separator, colon, and underscore becomes a dash
// (verified against the real, existing directory
// C--Users-lizard-liang-personal-ai-agents-lizard-market-test-harness-results--test-project
// for cwd .../test-harness/results/_test-project).
function deriveProjectSlug(cwdPath) {
  return cwdPath.replace(/[\\/:_]/g, "-");
}

const HOOK_EVENTS_TO_COUNT = new Set(["SessionStart", "UserPromptSubmit", "SubagentStart"]);

// Attribution rule (CRITICAL — do not naive-grep the transcript): count a
// constraint copy ONLY when it arrived as hook-injected context. In the JSONL
// transcript, every hook invocation Claude Code ran is recorded as its own
// line: { "type": "attachment", "attachment": { "hookEvent": ..., "content":
// ..., "stdout": ... } }. We only look inside those records, and only for the
// three hookEvents that are documented to inject this sentence. We explicitly
// do NOT scan assistant "text" blocks, "tool_result" blocks, or user-typed
// prompt content — those can contain the same substring for reasons that have
// nothing to do with injection (the assistant repeating the phrase, a Read of
// session-start.cjs itself, or — as happened while building this very test —
// an Agent tool_use prompt that quotes the sentence verbatim as spec text). A
// naive whole-file grep on a real session showed 19 hits vs a 2-hit
// attributed baseline; almost all of the extra 17 were exactly these echoes.
function countAttributedConstraintCopies(jsonlPath) {
  if (!fs.existsSync(jsonlPath)) return { total: 0, records: [] };
  const lines = fs.readFileSync(jsonlPath, "utf-8").split("\n").filter(Boolean);
  const records = [];
  for (const line of lines) {
    let obj;
    try {
      obj = JSON.parse(line);
    } catch {
      continue;
    }
    if (obj.type !== "attachment" || !obj.attachment) continue;
    const { hookEvent, content, stdout } = obj.attachment;
    if (!HOOK_EVENTS_TO_COUNT.has(hookEvent)) continue;

    // JSON-emitting hooks (UserPromptSubmit/SubagentStart) leave `content`
    // blank and put the real payload in `stdout` as JSON; plain-text hooks
    // (SessionStart) put the payload directly in `content`. Never read both
    // for the same record — they'd double-count one injected value.
    let text = null;
    try {
      const parsed = JSON.parse(stdout);
      text = parsed?.hookSpecificOutput?.additionalContext ?? null;
    } catch {
      // stdout wasn't JSON — not a hookSpecificOutput-style hook.
    }
    if (text === null) text = content ?? "";

    const n = countConstraint(text);
    if (n > 0) records.push({ hookEvent, hookName: obj.attachment.hookName, count: n });
  }
  return { total: records.reduce((s, r) => s + r.count, 0), records };
}

// Naive whole-file substring count — reported ONLY as a contrast baseline
// alongside the attributed count, never used for pass/fail.
function countNaiveWholeFile(jsonlPath) {
  if (!fs.existsSync(jsonlPath)) return 0;
  return countConstraint(fs.readFileSync(jsonlPath, "utf-8"));
}

async function runModeB(args) {
  console.log("\nKratos Constraint Injection Audit — MODE B (live verification, real API calls)");

  // Dynamic import: Mode A must never even load the SDK module, so a static
  // audit of this file's Mode-A code path shows zero network-capable
  // dependencies pulled in.
  const { query } = await import("@anthropic-ai/claude-agent-sdk");

  const projectDir = path.join(HARNESS_ROOT, "results", "_test-project");
  fs.mkdirSync(projectDir, { recursive: true });
  if (!fs.existsSync(path.join(projectDir, ".git"))) {
    execFileSync("git", ["init"], { cwd: projectDir, stdio: "ignore" });
    execFileSync("git", ["commit", "--allow-empty", "-m", "init"], { cwd: projectDir, stdio: "ignore" });
  }

  const FEATURE_NAME = `constraint-audit-${Date.now()}`;
  // Harness rule for this repo: prompts MUST use an explicit slash command
  // and carry a full spec, or the session no-ops.
  const SPEC_PROMPT =
    `/kratos:main Build a tiny health-check endpoint. Feature name: ${FEATURE_NAME}. ` +
    `Full spec: GET /health returns {"status":"ok"} as JSON, no auth, Node/Express, ` +
    `single file src/health.js, no tests required beyond a curl example in the response.`;

  // Turn 1 spawns a god subagent (channel 3) via the slash command — slash
  // commands bypass channel 2 (hook.go short-circuits on the "/kratos:"
  // prefix). Turns 2-3 are plain language containing a kratos/god keyword,
  // which DOES fire channel 2.
  const turnPrompts = [
    SPEC_PROMPT,
    "kratos, what's the status of that feature? keep going.",
    "ares, please continue implementing it.",
  ];

  let sessionId = null;
  for (const [idx, prompt] of turnPrompts.entries()) {
    const stream = query({
      prompt,
      options: {
        cwd: projectDir,
        model: args.model,
        permissionMode: "bypassPermissions",
        allowDangerouslySkipPermissions: true,
        plugins: [{ type: "local", path: KRATOS_PLUGIN_PATH }],
        ...(sessionId ? { resume: sessionId } : {}),
      },
    });
    for await (const msg of stream) {
      if (msg.type === "system" && msg.subtype === "init" && !sessionId) {
        sessionId = msg.session_id;
      }
    }
    console.log(`  turn ${idx + 1}/${turnPrompts.length} done (session ${sessionId ?? "unknown"})`);
  }

  if (!sessionId) {
    console.error("Could not determine session_id from the init message — aborting Mode B analysis.");
    return 1;
  }

  const slug = deriveProjectSlug(projectDir);
  const transcriptDir = path.join(os.homedir(), ".claude", "projects", slug);
  const mainFile = path.join(transcriptDir, `${sessionId}.jsonl`);
  const agentFiles = fs.existsSync(transcriptDir)
    ? fs
        .readdirSync(transcriptDir)
        .filter((f) => f.startsWith("agent-") && f.endsWith(".jsonl"))
        .map((f) => path.join(transcriptDir, f))
    : [];

  const allFiles = [mainFile, ...agentFiles].filter((f) => fs.existsSync(f));

  console.log(`\nTranscript dir: ${transcriptDir}`);
  console.log(`Main session  : ${path.basename(mainFile)}`);
  console.log(`Subagent files: ${agentFiles.length}`);

  let attributedTotal = 0;
  let naiveTotal = 0;
  const perFile = [];
  for (const f of allFiles) {
    const attributed = countAttributedConstraintCopies(f);
    const naive = countNaiveWholeFile(f);
    attributedTotal += attributed.total;
    naiveTotal += naive;
    perFile.push({ file: path.basename(f), attributed: attributed.total, naive, records: attributed.records });
  }

  console.log(`\n${"═".repeat(78)}`);
  console.log("ATTRIBUTED vs NAIVE COPY COUNT (per transcript file)");
  console.log(`${"═".repeat(78)}`);
  for (const r of perFile) {
    console.log(`${r.file.padEnd(50)} attributed=${r.attributed}  naive=${r.naive}`);
  }
  console.log("─".repeat(78));
  console.log(`TOTAL attributed (hook-injected only): ${attributedTotal}`);
  console.log(`TOTAL naive (whole-file substring)    : ${naiveTotal}`);

  const runId =
    new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19) +
    "-constraint-audit-live-" +
    crypto.randomBytes(3).toString("hex");
  const runDir = path.join(HARNESS_ROOT, "results", runId);
  fs.mkdirSync(runDir, { recursive: true });
  fs.writeFileSync(
    path.join(runDir, "report.json"),
    JSON.stringify(
      { mode: "B", runId, sessionId, projectDir, slug, transcriptDir, perFile, attributedTotal, naiveTotal },
      null,
      2
    )
  );
  console.log(`\nFull output: ${runDir}`);

  return 0;
}

// ── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  const args = parseArgs(process.argv);
  const exitCode = args.live ? await runModeB(args) : await runModeA(args);
  process.exit(exitCode);
}

main().catch((err) => {
  console.error("Fatal:", err);
  process.exit(1);
});
