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
 */

const { execSync, spawn } = require("child_process");
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
const MAX_MEMORIES = 15;

// Output constraint injected into every session (verbatim from references/agent-protocol.md).
const OUTPUT_CONSTRAINT =
  "\n**Output constraint:** Two registers.\n" +
  "- Status updates (mid-turn): terse. `[status] [what] [result]. [next].` Fragments OK. Never a bare `[what]:` — always carry the result. No arrow chains.\n" +
  "- Answers, summaries, decisions: conclusion first, then full sentences. Keep hedges and evidence status (verified vs inferred). A yes/no gets one supporting sentence. When asking the user to decide: state the decision and its consequence before the options.\n" +
  "Both: no filler, no pleasantries. Technical terms exact. Code blocks unchanged.\n";

function ensureDir() {
  fs.mkdirSync(SESSIONS_DIR, { recursive: true });
}

const findKratosBinary = resolveBinary;

function runKratos(args) {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return null;
  try {
    return execSync(`"${kratosCmd}" ${args}`, {
      encoding: "utf-8",
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
      stdio: ["ignore", "pipe", "ignore"],
    });
  } catch (e) {
    return null;
  }
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
  return runKratos("init") !== null;
}

// Stored user memories (preferences/habits) — read-side of the memory sweep.
// Newest first, capped, fetched with --limit so the store's growth never bloats
// the hook (the unbounded list is 60+ KB today).
function formatMemories() {
  const raw = runKratos(`memory list --limit ${MAX_MEMORIES}`);
  if (!raw) return null;
  try {
    const data = JSON.parse(raw);
    if (!data.memories || data.memories.length === 0) return null;

    const total = typeof data.total === "number" ? data.total : data.memories.length;
    const shown = data.memories.slice(0, MAX_MEMORIES);
    const older = total - shown.length;

    const lines = ["", "## Stored user preferences"];
    for (const m of shown) {
      const cat = m.category ? ` [${m.category}]` : "";
      lines.push(`- ${m.text}${cat}`);
    }
    if (older > 0) {
      lines.push(`(+${older} older — run \`kratos memory list --limit 50\` for more)`);
    }
    lines.push("");
    return lines.join("\n");
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

// Copy kratos binary to ~/.kratos/bin/ so agents use a single fixed path
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
    return;
  }

  let needsCopy = !fs.existsSync(targetPath);
  if (!needsCopy) {
    needsCopy = fs.statSync(srcPath).mtimeMs > fs.statSync(targetPath).mtimeMs;
  }
  if (needsCopy) {
    fs.mkdirSync(targetDir, { recursive: true });
    fs.copyFileSync(srcPath, targetPath);
    if (!isWin) fs.chmodSync(targetPath, 0o755);
  }
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

  const raw = runKratos(`session start "${cwd}" --session-id "${sessionId}"`);
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
  ensureBinary();
  pruneSessionFiles();

  // Always inject the output constraint, regardless of session source.
  console.log(OUTPUT_CONSTRAINT);

  const kratosBin = findKratosBinary();
  if (kratosBin) {
    console.log(`KRATOS_BIN: ${kratosBin}`);
  }
  const memoriesMsg = formatMemories();
  if (memoriesMsg) {
    console.log(memoriesMsg);
  }

  for (const line of [formatHandoffNotice(cwd), formatPendingSpecDeltas(cwd), formatDraftPlans(cwd)]) {
    if (line) console.log(line);
  }

  registerSession(sessionId, cwd, source);
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
