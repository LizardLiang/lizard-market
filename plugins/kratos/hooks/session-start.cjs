#!/usr/bin/env node
/**
 * Kratos Memory - Session Start Hook
 *
 * Automatically starts a memory session when Claude Code session begins.
 * Uses global database at ~/.kratos/memory.db
 *
 * NEW: Injects detailed context if there's an incomplete feature from last session.
 */

const { execSync, spawn } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");
const { resolveBinary, platformBinaryName } = require("./kratos-bin.cjs");

// Global paths
const KRATOS_HOME = path.join(os.homedir(), ".kratos");
const DB_PATH = path.join(KRATOS_HOME, "memory.db");
const SESSION_FILE = path.join(KRATOS_HOME, "active-session.json");
const SCHEMA_PATH = path.join(__dirname, "..", "memory", "schema.sql");

// Get project name from current working directory
const cwd = process.cwd();
const projectName = path.basename(cwd);

// Output constraint injected into every session (verbatim from references/agent-protocol.md).
const OUTPUT_CONSTRAINT =
  "\n**Output constraint:** Terse. Drop articles, filler, pleasantries. Pattern: `[status] [what] [result]. [next].` Fragments OK. Technical terms exact. Code blocks unchanged.\n";

// Ensure .kratos directory exists
function ensureDir() {
  if (!fs.existsSync(KRATOS_HOME)) {
    fs.mkdirSync(KRATOS_HOME, { recursive: true });
  }
}

const findKratosBinary = resolveBinary;

// Initialize database if needed
function initDb() {
  if (fs.existsSync(DB_PATH)) return true;

  const kratosCmd = findKratosBinary();
  if (!kratosCmd) {
    console.error(
      "Kratos binary not found yet. Downloading in the background to ~/.kratos/bin/ - " +
        "retry shortly, or build from source: cd go && go build -o ../bin/kratos ./cmd/kratos",
    );
    return false;
  }

  try {
    execSync(`"${kratosCmd}" init`, {
      stdio: "ignore",
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
    });
    return true;
  } catch (e) {
    console.error("Failed to init DB:", e.message);
    return false;
  }
}

// Generate UUID v4
function uuid() {
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    return (c === "x" ? r : (r & 0x3) | 0x8).toString(16);
  });
}

// Start session using Go CLI
function startSession() {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return null;

  try {
    const result = execSync(`"${kratosCmd}" session start "${cwd}"`, {
      encoding: "utf-8",
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
    });
    return JSON.parse(result).session_id;
  } catch (e) {
    console.error("Failed to start session:", e.message);
    return null;
  }
}

// Get last session info for context injection
function getLastSessionInfo() {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return null;

  try {
    const result = execSync(`"${kratosCmd}" recall --project "${cwd}"`, {
      encoding: "utf-8",
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
    });
    const data = JSON.parse(result);
    return data.last_session || null;
  } catch (e) {
    return null;
  }
}

// Stored user memories (preferences/habits) — read-side of the memory sweep.
// Injected every session so the main session and inline command-mode gods
// actually consume what the Stop-hook sweep saved. Silent on any failure.
function formatMemories() {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return null;

  try {
    const result = execSync(`"${kratosCmd}" memory list`, {
      encoding: "utf-8",
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
    });
    const data = JSON.parse(result);
    if (!data.memories || data.memories.length === 0) return null;

    const lines = ["", "## Stored user preferences"];
    for (const m of data.memories) {
      const cat = m.category ? ` [${m.category}]` : "";
      lines.push(`- ${m.text}${cat}`);
    }
    lines.push("");
    return lines.join("\n");
  } catch (e) {
    return null;
  }
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

// Format detailed context message for injection
function formatContextMessage(info) {
  if (!info || !info.feature_name) return null;
  if (info.feature_status === "completed") return null;

  const timeAgo = formatTimeAgo(info.started_at);
  const stage = info.current_stage || 0;
  const stageName = info.stage_name || "Unknown";
  const nextAgent = info.next_agent || "Unknown";
  const nextStageName = info.next_stage_name || "Unknown";

  // Build the context box
  const lines = [
    "",
    "+----------------------------------------------------------------------+",
    "|  KRATOS MEMORY: Last session detected                                |",
    "+----------------------------------------------------------------------+",
    `|  Feature: ${(info.feature_name || "").padEnd(56)}|`,
    `|  Stage: ${stage}/8 (${stageName})`.padEnd(71) + "|",
    `|  Last active: ${timeAgo}`.padEnd(71) + "|",
    "|                                                                      |",
  ];

  // Add last actions
  if (info.last_actions && info.last_actions.length > 0) {
    lines.push(
      "|  Last actions:                                                       |",
    );
    for (const action of info.last_actions.slice(-3)) {
      const truncated =
        action.length > 60 ? action.substring(0, 57) + "..." : action;
      lines.push(`|  - ${truncated}`.padEnd(71) + "|");
    }
    lines.push(
      "|                                                                      |",
    );
  }

  // Add recommendation
  if (info.next_stage !== null && info.next_stage !== undefined) {
    const rec = `Continue with Stage ${info.next_stage} (${nextAgent} - ${nextStageName})?`;
    lines.push(`|  Recommendation: ${rec}`.padEnd(71) + "|");
    lines.push(
      '|  Say "continue" or "/kratos" to resume                               |',
      '|  Tip: /kratos:recall <path> to view past sessions                    |',
    );
  }

  lines.push(
    "+----------------------------------------------------------------------+",
  );
  lines.push("");

  return lines.join("\n");
}

// Copy kratos binary to ~/.kratos/bin/ so agents use a single fixed path
function ensureBinary() {
  const targetDir = path.join(KRATOS_HOME, "bin");
  const isWin = process.platform === "win32";
  const targetName = isWin ? "kratos.exe" : "kratos";
  const targetPath = path.join(targetDir, targetName);

  // Determine source binary from plugin bin/ directory
  const pluginRoot =
    process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, "..");
  const srcDir = path.join(pluginRoot, "bin");

  const srcName = platformBinaryName();

  const srcPath = path.join(srcDir, srcName);
  if (!fs.existsSync(srcPath)) {
    // No plugin-local binary (release install, not a dev checkout) - fall
    // back to a background download. SessionStart hooks have a 5s timeout
    // and a ~10MB download cannot fit inline, so spawn detached and return
    // immediately; ensure-binary.cjs degrades silently on any failure.
    try {
      const ensureBinaryScript = path.join(__dirname, "ensure-binary.cjs");
      const child = spawn(process.execPath, [ensureBinaryScript], {
        detached: true,
        stdio: "ignore",
      });
      child.unref();
    } catch (e) {
      // best-effort - never block session start on the downloader
    }
    return;
  }

  // Copy if target missing or source is newer
  let needsCopy = !fs.existsSync(targetPath);
  if (!needsCopy) {
    const srcMtime = fs.statSync(srcPath).mtimeMs;
    const tgtMtime = fs.statSync(targetPath).mtimeMs;
    needsCopy = srcMtime > tgtMtime;
  }

  if (needsCopy) {
    fs.mkdirSync(targetDir, { recursive: true });
    fs.copyFileSync(srcPath, targetPath);
    if (!isWin) {
      fs.chmodSync(targetPath, 0o755);
    }
  }


}

// Main
function main() {
  ensureDir();
  ensureBinary();

  // Always inject the output constraint, regardless of session resume/init path.
  console.log(OUTPUT_CONSTRAINT);

  // Always inject the resolved binary path so inline gods never hunt for it
  // (template get / spec validate silently got skipped when the binary wasn't
  // found, producing prose spec deltas), plus stored user preferences.
  const kratosBin = findKratosBinary();
  if (kratosBin) {
    console.log(`KRATOS_BIN: ${kratosBin}`);
  }
  const memoriesMsg = formatMemories();
  if (memoriesMsg) {
    console.log(memoriesMsg);
  }

  // Check for existing active session
  if (fs.existsSync(SESSION_FILE)) {
    try {
      const existing = JSON.parse(fs.readFileSync(SESSION_FILE, "utf-8"));
      // Session from same project and less than 1 hour old? Reuse it
      const age = Date.now() - existing.started_at;
      if (existing.project === projectName && age < 3600000) {
        console.log(`Kratos: Resuming session ${existing.session_id}`);
        return;
      }
    } catch (e) {
      // Invalid session file, continue to create new
    }
  }

  if (!initDb()) return;

  // Get last session info BEFORE starting new session
  const lastSessionInfo = getLastSessionInfo();

  const sessionId = startSession();
  if (!sessionId) return;

  // Save session info
  const sessionData = {
    session_id: sessionId,
    project: projectName,
    cwd: cwd,
    started_at: Date.now(),
  };

  fs.writeFileSync(SESSION_FILE, JSON.stringify(sessionData, null, 2));

  console.log(`Kratos: Memory session started - ${sessionId}`);

  // Inject context if there's an incomplete feature
  if (lastSessionInfo && lastSessionInfo.feature_name) {
    const contextMsg = formatContextMessage(lastSessionInfo);
    if (contextMsg) {
      console.log(contextMsg);
    }
  }
}

main();
