#!/usr/bin/env node
/**
 * Kratos Memory - Session Start Hook
 *
 * Registers this Claude Code session in the memory ledger, keyed by the
 * harness session_id from the hook payload, and injects the small amount of
 * context every session needs: the output constraint, the resolved binary
 * path, stored user preferences, and one-line pointers to a fresh handoff,
 * pending spec deltas, or unfinished plan drafts.
 *
 * Session state lives in ~/.kratos/sessions/<session_id>.json — one file per
 * Claude Code session. The previous single ~/.kratos/active-session.json was
 * shared by every concurrent window: sessions in other projects ended each
 * other, resume pointers named the wrong project, and `step record-agent`
 * failed with FOREIGN KEY errors (2026-09 transcript review).
 *
 * Budget: hooks.json gives SessionStart 5000 ms. Every spawn below carries a
 * timeout and the serial sum stays under ~4000 ms (version 800 + memory list
 * 1500 + init 500 + session start 800); the plugin-bin → ~/.kratos/bin copy
 * runs last so it can never starve the calls. Every kratos call uses spawnSync
 * with an argv array — payload text never reaches a shell.
 */

const { execFileSync, spawn, spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");
const { resolveBinary, platformBinaryName } = require("./kratos-bin.cjs");

// Global paths
const KRATOS_HOME = path.join(os.homedir(), ".kratos");
const DB_PATH = path.join(KRATOS_HOME, "memory.db");
const SESSIONS_DIR = path.join(KRATOS_HOME, "sessions");
const LEGACY_SESSION_FILE = path.join(KRATOS_HOME, "active-session.json");
const SESSION_FILE_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000; // 7 days
const REMINDER_MAX_AGE_MS = 14 * 24 * 60 * 60 * 1000; // 14 days
const MAX_MEMORIES = 8;

// Output constraint injected into every session (verbatim from references/agent-protocol.md).
const OUTPUT_CONSTRAINT =
  "\n**Output constraint:** Two registers.\n" +
  "- Status updates (mid-turn): terse. `[status] [what] [result]. [next].` Fragments OK. Never a bare `[what]:` — always carry the result. No arrow chains.\n" +
  "- Answers, summaries, decisions: conclusion first, then full sentences. Keep hedges and evidence status (verified vs inferred). A yes/no gets one supporting sentence. When asking the user to decide: state the decision and its consequence before the options.\n" +
  "Both: no filler, no pleasantries. Technical terms exact. Code blocks unchanged.\n" +
  "A message the human typed always gets an answer; `No response requested` is only for harness task notifications.\n";

// The pre-plugin installer registered copies of the hooks in ~/.claude/settings.json;
// when they are still there every event runs twice and the stale copy errors
// (`recall --project`, "active session already exists"). Point at the fix once.
function formatLegacyHooksWarning() {
  const settingsFile = path.join(os.homedir(), ".claude", "settings.json");
  let text;
  try {
    text = fs.readFileSync(settingsFile, "utf-8");
  } catch (e) {
    return null;
  }
  if (!/hooks[\\\/]+kratos[\\\/]/.test(text)) return null;
  return "Kratos: legacy hook copies are still registered in ~/.claude/settings.json and run alongside the plugin's — run `kratos install` once to remove them.";
}

function ensureDir() {
  fs.mkdirSync(SESSIONS_DIR, { recursive: true });
}

const findKratosBinary = resolveBinary;

// Per-call budgets (ms). Serial sum must stay under the 5000 ms hook timeout
// with room for node startup.
const VERSION_TIMEOUT_MS = 800;
const MEMORY_LIST_TIMEOUT_MS = 1500;
const INIT_TIMEOUT_MS = 500;
const SESSION_START_TIMEOUT_MS = 800;

// Runs the kratos binary with an argv array; stdout on success, null otherwise.
function runKratos(args, timeoutMs) {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return null;
  try {
    const r = spawnSync(kratosCmd, args, {
      encoding: "utf-8",
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
      stdio: ["ignore", "pipe", "ignore"],
      timeout: timeoutMs,
    });
    if (r.error || r.status !== 0) return null;
    return r.stdout;
  } catch (e) {
    return null;
  }
}

function firstLine(text) {
  const line = String(text || "").split(/\r?\n/).find((l) => l.trim()) || "";
  return line.trim().slice(0, 160);
}

// Like runKratos, but keeps stderr so a failure can name its cause.
// Returns { out, err }; out is null on any failure.
function runKratosCapture(args, timeoutMs) {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return { out: null, err: "kratos binary not found" };
  try {
    const r = spawnSync(kratosCmd, args, {
      encoding: "utf-8",
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
      stdio: ["ignore", "pipe", "pipe"],
      timeout: timeoutMs,
    });
    if (r.error || r.status !== 0 || !r.stdout) {
      return { out: null, err: firstLine(r.stderr) || firstLine(r.error && r.error.message) || `exit ${r.status}` };
    }
    return { out: r.stdout, err: firstLine(r.stderr) };
  } catch (e) {
    return { out: null, err: firstLine(e && e.message) };
  }
}

// plugin.json version without a leading "v", or null when unreadable.
function pluginVersion() {
  try {
    const pluginRoot = process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, "..");
    const manifest = JSON.parse(fs.readFileSync(path.join(pluginRoot, ".claude-plugin", "plugin.json"), "utf-8"));
    return manifest.version ? String(manifest.version).replace(/^v/, "") : null;
  } catch (e) {
    return null;
  }
}

// `<bin> --version` prints "kratos version v2.108.0"; returns "2.108.0" or null.
function binaryVersion(bin) {
  try {
    const out = execFileSync(bin, ["--version"], {
      encoding: "utf-8",
      stdio: ["ignore", "pipe", "ignore"],
      timeout: VERSION_TIMEOUT_MS,
    });
    const match = out.match(/v?(\d+\.\d+\.\d+)/);
    return match ? match[1] : null;
  } catch (e) {
    return null;
  }
}

// A stale binary silently lacks flags newer agents call (2026-09 review: a
// v2.1.0 binary under a v2.108.0 plugin). One line; ensureBinary already
// spawned the refresh when the binary is a release download.
function formatVersionMismatch(bin, refreshing) {
  const have = binaryVersion(bin);
  const want = pluginVersion();
  if (!have || !want || have === want) return null;
  const action = refreshing ? "refreshing in background" : "rebuild: cd kratos-dev/go && make build";
  return `Kratos: binary v${have} ≠ plugin v${want} — ${action}`;
}

// Initialize database if needed
function initDb() {
  if (fs.existsSync(DB_PATH)) return true;
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) {
    console.error(
      "Kratos binary not found yet. Downloading in the background to ~/.kratos/bin/ - " +
        "retry shortly, or build from source: cd kratos-dev/go && make build",
    );
    return false;
  }
  return runKratos(["init"], INIT_TIMEOUT_MS) !== null;
}

function toSlashes(p) {
  return String(p || "").replace(/\\/g, "/");
}

// Same canonical form the CLI stores in user_memories.project.
function normalizeProject(p) {
  let out = toSlashes(p).replace(/\/+$/, "");
  if (process.platform === "win32") out = out.toLowerCase();
  return out;
}

// Stored user memories — read-side of the memory sweep, ranked for THIS
// project: facts scoped to the current project first, then global
// preferences / habits / weak spots, then global context; other projects'
// scoped facts are never shown. The old newest-15-of-everything injection put
// the same list in every project and 0-2 of 15 items were relevant (2026-09).
//
// A failed list with an existing DB returns one "memory unavailable" line
// instead of nothing: silent null hid a broken binary for weeks (2026-09).
function formatMemories(cwd) {
  const { out, err } = runKratosCapture(["memory", "list", "--limit", "80"], MEMORY_LIST_TIMEOUT_MS);
  const unavailable = (reason) => (fs.existsSync(DB_PATH) ? `Kratos: memory unavailable (${reason})` : null);
  if (!out) return unavailable(err || "no output");
  let data;
  try {
    data = JSON.parse(out);
  } catch (e) {
    return unavailable(err || "unreadable output");
  }
  try {
    if (!data.memories || data.memories.length === 0) return null;
    const total = typeof data.total === "number" ? data.total : data.memories.length;
    const here = normalizeProject(cwd);

    const scoped = [];
    const globalPrefs = [];
    const globalContext = [];
    for (const m of data.memories) {
      if (m.project) {
        if (normalizeProject(m.project) === here) scoped.push(m);
        continue;
      }
      if (m.category === "context") globalContext.push(m);
      else globalPrefs.push(m);
    }
    const shown = [...scoped, ...globalPrefs, ...globalContext].slice(0, MAX_MEMORIES);
    if (shown.length === 0) return null;

    const lines = ["", "## Stored user preferences"];
    for (const m of shown) {
      const tag = m.project ? ` [${m.category || "context"} · this project]` : m.category ? ` [${m.category}]` : "";
      lines.push(`- ${m.text}${tag}`);
    }
    const more = total - shown.length;
    if (more > 0) {
      lines.push(`(+${more} more — \`kratos memory list --limit 50\` or \`--project "${toSlashes(cwd)}"\`)`);
    }
    lines.push("");
    return lines.join("\n");
  } catch (e) {
    return null;
  }
}

// After a compaction the model has lost its working targets; print the head of
// a fresh handoff.md (the memory sweep keeps it current) so files, pages and
// next steps survive the boundary.
function formatHandoffHead(cwd) {
  try {
    const handoffPath = path.join(cwd, ".claude", ".Arena", "handoff.md");
    const stats = fs.statSync(handoffPath);
    if (Date.now() - stats.mtimeMs >= SESSION_FILE_MAX_AGE_MS) return null;
    const lines = fs.readFileSync(handoffPath, "utf-8").split(/\r?\n/).filter((l) => l.trim()).slice(0, 12);
    if (lines.length === 0) return null;
    return ["Kratos: current targets after compaction (.claude/.Arena/handoff.md):", ...lines.map((l) => "  " + l)].join("\n");
  } catch (e) {
    return null;
  }
}

// Session handoff written by /kratos:wrap — .claude/.Arena/handoff.md. Content
// is injected on demand by the Go UserPromptSubmit hook when a resume phrase
// appears; this only prints a one-line pointer for a fresh file.
function formatHandoffNotice(cwd) {
  try {
    const handoffPath = path.join(cwd, ".claude", ".Arena", "handoff.md");
    const stats = fs.statSync(handoffPath);
    const age = Date.now() - stats.mtimeMs;
    if (age >= SESSION_FILE_MAX_AGE_MS) return null;
    return `Kratos: handoff from last session (${formatTimeAgo(stats.mtimeMs)}) — say "continue" or /kratos:recall to load it`;
  } catch (e) {
    return null;
  }
}

// Pending spec deltas: .claude/feature/*/spec-delta/*.md that are not under
// archived/. One line, once per session, only while the newest delta is fresh
// — stale deltas from abandoned features stop nagging on their own.
function formatPendingSpecDeltas(cwd) {
  const featureRoot = path.join(cwd, ".claude", "feature");
  let features;
  try {
    features = fs.readdirSync(featureRoot);
  } catch (e) {
    return null;
  }
  const pending = [];
  let newest = 0;
  for (const name of features) {
    const deltaDir = path.join(featureRoot, name, "spec-delta");
    let entries;
    try {
      entries = fs.readdirSync(deltaDir);
    } catch (e) {
      continue;
    }
    for (const entry of entries) {
      if (!entry.endsWith(".md")) continue;
      try {
        const stats = fs.statSync(path.join(deltaDir, entry));
        if (!stats.isFile()) continue;
        pending.push(name);
        if (stats.mtimeMs > newest) newest = stats.mtimeMs;
      } catch (e) {
        // unreadable entry — skip
      }
    }
  }
  if (pending.length === 0) return null;
  if (Date.now() - newest >= REMINDER_MAX_AGE_MS) return null;
  const unique = [...new Set(pending)];
  const shown = unique.slice(0, 3).join(", ") + (unique.length > 3 ? ", …" : "");
  return `Kratos: ${pending.length} un-archived spec delta(s) (${shown}) — /kratos:spec-archive <feature> when the work has landed`;
}

// Count entries under a plan's `## Locked Decisions` heading, so the reminder
// can say how much answered-question work is sitting in the draft.
function countLockedDecisions(body) {
  const section = body.split(/^##\s+Locked Decisions\s*$/m)[1];
  if (!section) return 0;
  const untilNextHeading = section.split(/^##\s+/m)[0];
  return (untilNextHeading.match(/^\s*-\s+\*\*/gm) || []).length;
}

// Unfinished tactical plans (status: draft) hold answers the user already
// gave. One line, same freshness gate as the deltas.
function formatDraftPlans(cwd) {
  const planDir = path.join(cwd, ".claude", ".Arena", "tactical-plans");
  let names;
  try {
    names = fs.readdirSync(planDir);
  } catch (e) {
    return null;
  }
  const drafts = [];
  for (const name of names) {
    if (!name.endsWith(".md")) continue;
    const full = path.join(planDir, name);
    try {
      const stats = fs.statSync(full);
      if (Date.now() - stats.mtimeMs >= REMINDER_MAX_AGE_MS) continue;
      const body = fs.readFileSync(full, "utf-8");
      if (!/^---\r?\n(?:.*\r?\n)*?status:\s*draft\b/m.test(body.slice(0, 512))) continue;
      const n = countLockedDecisions(body);
      drafts.push(`${name} (${n === 1 ? "1 locked decision" : `${n} locked decisions`})`);
    } catch (e) {
      // skip unreadable plan
    }
  }
  if (drafts.length === 0) return null;
  return `Kratos: ${drafts.length} unfinished plan draft(s): ${drafts.join(", ")} — /kratos:plan <task> resumes it`;
}

// Format time ago
function formatTimeAgo(timestampMs) {
  if (!timestampMs) return "unknown";
  const diffMs = Date.now() - timestampMs;
  const diffMin = diffMs / 60000;
  const diffHour = diffMin / 60;
  const diffDay = diffHour / 24;
  if (diffMin < 1) return "just now";
  if (diffMin < 60) return `${Math.floor(diffMin)} minutes ago`;
  if (diffHour < 24) return `${Math.floor(diffHour)} hours ago`;
  if (diffDay < 7) return `${Math.floor(diffDay)} days ago`;
  return `${Math.floor(diffDay / 7)} weeks ago`;
}

// Keep ~/.kratos/bin/ in sync with the plugin binary so agents use a single
// fixed path. Returns { refreshing, copy }: refreshing is true when a background
// release download was spawned; copy is a function performing the plugin-bin →
// ~/.kratos/bin copy, or null when the target is already current. The caller
// runs copy() last so a slow ~10MB copy never starves the CLI calls.
function ensureBinary() {
  const targetDir = path.join(KRATOS_HOME, "bin");
  const isWin = process.platform === "win32";
  const targetName = isWin ? "kratos.exe" : "kratos";
  const targetPath = path.join(targetDir, targetName);

  const pluginRoot = process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, "..");
  const srcPath = path.join(pluginRoot, "bin", platformBinaryName());
  if (!fs.existsSync(srcPath)) {
    // No plugin-local binary (release install) — background download; the
    // SessionStart budget cannot fit a ~10MB fetch inline.
    try {
      const child = spawn(process.execPath, [path.join(__dirname, "ensure-binary.cjs")], {
        detached: true,
        stdio: "ignore",
      });
      child.unref();
    } catch (e) {
      // best-effort
    }
    return { refreshing: true, copy: null };
  }

  let needsCopy = !fs.existsSync(targetPath);
  if (!needsCopy) {
    needsCopy = fs.statSync(srcPath).mtimeMs > fs.statSync(targetPath).mtimeMs;
  }
  if (!needsCopy) return { refreshing: false, copy: null };
  return {
    refreshing: false,
    copy: () => {
      fs.mkdirSync(targetDir, { recursive: true });
      fs.copyFileSync(srcPath, targetPath);
      if (!isWin) fs.chmodSync(targetPath, 0o755);
    },
  };
}

// Remove per-session state files older than 7 days, plus the legacy shared
// file, so the directory never grows without bound.
function pruneSessionFiles() {
  try {
    if (fs.existsSync(LEGACY_SESSION_FILE)) fs.unlinkSync(LEGACY_SESSION_FILE);
  } catch (e) {
    // ignore
  }
  let entries;
  try {
    entries = fs.readdirSync(SESSIONS_DIR);
  } catch (e) {
    return;
  }
  const now = Date.now();
  for (const entry of entries) {
    const full = path.join(SESSIONS_DIR, entry);
    try {
      if (now - fs.statSync(full).mtimeMs > SESSION_FILE_MAX_AGE_MS) fs.unlinkSync(full);
    } catch (e) {
      // ignore
    }
  }
}

// Register the session in the ledger (idempotent on session_id) and write the
// per-session state file other hooks read.
function registerSession(sessionId, cwd, source) {
  if (!sessionId) return;
  if (!initDb()) return;

  const raw = runKratos(["session", "start", cwd, "--session-id", sessionId], SESSION_START_TIMEOUT_MS);
  let created = false;
  if (raw) {
    try {
      created = JSON.parse(raw).created === true;
    } catch (e) {
      // keep going — the state file is still useful
    }
  }

  const stateFile = path.join(SESSIONS_DIR, `${sessionId}.json`);
  let startedAt = Date.now();
  try {
    const prev = JSON.parse(fs.readFileSync(stateFile, "utf-8"));
    if (prev && prev.started_at) startedAt = prev.started_at;
  } catch (e) {
    // fresh file
  }
  fs.writeFileSync(
    stateFile,
    JSON.stringify({ session_id: sessionId, project: path.basename(cwd), cwd, started_at: startedAt, source: source || "startup" }, null, 2),
  );

  if (created && (source === "startup" || source === "clear" || !source)) {
    console.log(`Kratos: session ${sessionId.slice(0, 8)} started`);
  }
}

function main(payload) {
  const cwd = (payload && payload.cwd) || process.cwd();
  const sessionId = payload && payload.session_id ? String(payload.session_id) : "";
  const source = payload && payload.source ? String(payload.source) : "";

  ensureDir();
  let refreshing = false;
  let pendingCopy = null;
  try {
    const bin = ensureBinary();
    refreshing = bin.refreshing;
    pendingCopy = bin.copy;
  } catch (e) {
    // a failed binary check must not block session start
  }
  pruneSessionFiles();

  // Always inject the output constraint, regardless of session source.
  console.log(OUTPUT_CONSTRAINT);

  const kratosBin = findKratosBinary();
  if (kratosBin) {
    console.log(`KRATOS_BIN: ${kratosBin}`);
    const mismatch = formatVersionMismatch(kratosBin, refreshing);
    if (mismatch) console.log(mismatch);
  }
  const memoriesMsg = formatMemories(cwd);
  if (memoriesMsg) {
    console.log(memoriesMsg);
  }

  const handoffLine = source === "compact" ? formatHandoffHead(cwd) || formatHandoffNotice(cwd) : formatHandoffNotice(cwd);
  for (const line of [handoffLine, formatPendingSpecDeltas(cwd), formatDraftPlans(cwd), formatLegacyHooksWarning()]) {
    if (line) console.log(line);
  }

  registerSession(sessionId, cwd, source);

  // Copy last: resolveBinary() prefers the plugin-local binary, so nothing
  // above depends on the ~/.kratos/bin copy being fresh.
  if (pendingCopy) {
    try {
      pendingCopy();
    } catch (e) {
      // a failed copy must not block session start
    }
  }
}

// Read the hook payload from stdin (session_id, cwd, source). Older harnesses
// send nothing — fall back to process.cwd() and skip session registration.
let raw = "";
let done = false;
function finish() {
  if (done) return;
  done = true;
  let payload = null;
  if (raw.trim()) {
    try {
      payload = JSON.parse(raw);
    } catch (e) {
      payload = null;
    }
  }
  main(payload);
}
process.stdin.setEncoding("utf-8");
process.stdin.on("data", (chunk) => (raw += chunk));
process.stdin.on("end", finish);
process.stdin.on("error", finish);
setTimeout(finish, 300).unref();
