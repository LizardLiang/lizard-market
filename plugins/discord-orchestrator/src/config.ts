import { readFileSync, writeFileSync, mkdirSync, existsSync, renameSync } from "fs";
import { join } from "path";
import { homedir } from "os";
import type { OrchestratorConfig, SessionsState } from "./types";

export const STATE_DIR = join(homedir(), ".discord-orchestrator");
const PROJECTS_FILE = join(STATE_DIR, "projects.json");
const SESSIONS_FILE = join(STATE_DIR, "sessions.json");
const ENV_FILE = join(STATE_DIR, ".env");

const DEFAULT_CONFIG: OrchestratorConfig = {
  projects: {},
  ops_channel: "",
  idle_timeout_ms: 3600000,
};

export function ensureStateDir(): void {
  if (!existsSync(STATE_DIR)) {
    mkdirSync(STATE_DIR, { recursive: true, mode: 0o700 });
  }
  // Ensure projects.json exists so fs.watch doesn't throw
  if (!existsSync(PROJECTS_FILE)) {
    writeFileSync(PROJECTS_FILE, JSON.stringify(DEFAULT_CONFIG, null, 2), { mode: 0o600 });
  }
}

export function loadConfig(): OrchestratorConfig {
  ensureStateDir();
  if (!existsSync(PROJECTS_FILE)) return { ...DEFAULT_CONFIG };
  const raw = readFileSync(PROJECTS_FILE, "utf-8");
  return { ...DEFAULT_CONFIG, ...JSON.parse(raw) };
}

export function saveConfig(config: OrchestratorConfig): void {
  ensureStateDir();
  const tmp = PROJECTS_FILE + ".tmp";
  writeFileSync(tmp, JSON.stringify(config, null, 2), { mode: 0o600 });
  renameSync(tmp, PROJECTS_FILE);
}

export function loadSessions(): SessionsState {
  ensureStateDir();
  if (!existsSync(SESSIONS_FILE)) return {};
  const raw = readFileSync(SESSIONS_FILE, "utf-8");
  return JSON.parse(raw);
}

export function saveSessions(sessions: SessionsState): void {
  ensureStateDir();
  const tmp = SESSIONS_FILE + ".tmp";
  writeFileSync(tmp, JSON.stringify(sessions, null, 2), { mode: 0o600 });
  renameSync(tmp, SESSIONS_FILE);
}

export function buildChannelMap(config: OrchestratorConfig): Map<string, string> {
  const map = new Map<string, string>();
  for (const [name, project] of Object.entries(config.projects)) {
    for (const channelId of project.channels) {
      map.set(channelId, name);
    }
  }
  return map;
}

export function loadBotToken(): string {
  if (!existsSync(ENV_FILE)) {
    throw new Error(`Bot token not found. Create ${ENV_FILE} with DISCORD_BOT_TOKEN=your-token`);
  }
  const raw = readFileSync(ENV_FILE, "utf-8");
  const match = raw.match(/DISCORD_BOT_TOKEN=(.+)/);
  if (!match) throw new Error(`DISCORD_BOT_TOKEN not found in ${ENV_FILE}`);
  return match[1].trim();
}