#!/usr/bin/env node
/**
 * Kratos Memory - Tool Use Hook (PostToolUse)
 *
 * Records agent spawns and project file changes against the Claude Code
 * session id carried in the hook payload. The session row is created on
 * demand by the CLI, so recording never depends on a state file.
 *
 * Only files under the session's cwd are recorded. Pipeline deliverables under
 * .claude/feature/ and Arena writes under .claude/.Arena/ count as project work
 * (agents no longer run `step record-file` themselves — this hook is the only
 * recorder). The agent scratchpad (%TEMP%/claude), .claude/tmp/, other .claude/
 * bookkeeping, .kratos/ and .git/ are skipped — before this filter 13 of 15
 * weekly "file_modify" steps were scratchpad writes.
 *
 * Every kratos call goes through spawnSync with an argv array: payload text
 * (agent descriptions, file paths) never reaches a shell.
 */

const { spawnSync } = require('child_process');
const path = require('path');
const os = require('os');
const { resolveBinary } = require('./kratos-bin.cjs');

const KRATOS_HOME = path.join(os.homedir(), '.kratos');
const DB_PATH = path.join(KRATOS_HOME, 'memory.db');

// Kratos god names for detection (subagent_type "kratos:<god>" or a description mention)
const KRATOS_AGENTS = [
  'metis', 'athena', 'nemesis', 'daedalus', 'hephaestus', 'apollo', 'artemis', 'ares', 'hera',
  'hermes', 'cassandra', 'clio', 'mimir', 'hades', 'odysseus', 'prometheus', 'themis', 'ananke', 'iris',
];

const findKratosBinary = resolveBinary;

function run(args) {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return false;
  try {
    const r = spawnSync(kratosCmd, args, {
      stdio: 'ignore',
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
      timeout: 3000,
    });
    return !r.error && r.status === 0;
  } catch (e) {
    return false;
  }
}

// Single-line, length-capped argument text (argv-safe; no shell quoting involved).
function clip(str, max) {
  if (!str) return '';
  return String(str).replace(/\r?\n/g, ' ').substring(0, max);
}

function toSlashes(p) {
  return String(p || '').replace(/\\/g, '/');
}

// Detect the god from an Agent/Task tool input
function detectAgent(toolInput) {
  const agentType = String((toolInput && toolInput.subagent_type) || '');
  const m = agentType.match(/^kratos:([a-z-]+)$/i);
  if (m) return m[1].toLowerCase();
  const desc = String((toolInput && toolInput.description) || '').toLowerCase();
  for (const agent of KRATOS_AGENTS) {
    if (desc.includes(agent)) return agent;
  }
  return agentType || 'unknown';
}

// A file is project work when it sits under cwd and outside bookkeeping dirs.
// Feature deliverables (.claude/feature/) and Arena shards (.claude/.Arena/) are
// project work; the rest of .claude/ (tmp/, settings, plans) is not.
function isProjectFile(filePath, cwd) {
  const file = toSlashes(filePath).toLowerCase();
  const root = toSlashes(cwd).toLowerCase().replace(/\/+$/, '');
  if (!file || !root) return false;
  if (!file.startsWith(root + '/')) return false;
  const rel = file.slice(root.length + 1);
  if (rel.startsWith('.kratos/') || rel.startsWith('.git/')) return false;
  if (rel.startsWith('.claude/')) {
    if (rel.startsWith('.claude/tmp/')) return false;
    if (!rel.startsWith('.claude/feature/') && !rel.startsWith('.claude/.arena/')) return false;
  }
  if (/(^|\/)(temp|tmp)\/claude\//.test(file)) return false;
  return true;
}

function processToolUse(data) {
  let toolData;
  try {
    toolData = JSON.parse(data);
  } catch (e) {
    return;
  }

  const sessionId = toolData.session_id ? String(toolData.session_id) : '';
  if (!sessionId) return;
  const cwd = toolData.cwd || process.cwd();
  const { tool_name, tool_input } = toolData;
  const projectArgs = ['--project', clip(cwd, 200)];

  // Agent spawns (the Agent tool; older harnesses call it Task)
  if (tool_name === 'Agent' || tool_name === 'Task') {
    const agent = detectAgent(tool_input);
    const action = (tool_input && tool_input.description) || 'Agent task';
    const model = (tool_input && tool_input.model) || 'default';
    run(['step', 'record-agent', sessionId, clip(agent, 200), clip(model, 200), clip(action, 200), ...projectArgs]);
    return;
  }

  // Project file writes and edits
  if (tool_name === 'Write' || tool_name === 'Edit' || tool_name === 'MultiEdit') {
    const filePath = tool_input && tool_input.file_path;
    if (!isProjectFile(filePath, cwd)) return;
    const rel = toSlashes(filePath).slice(toSlashes(cwd).replace(/\/+$/, '').length + 1);
    run(['step', 'record-file', sessionId, tool_name, clip(rel, 200), ...projectArgs]);
  }
}

// Only read stdin when run as a hook. Required as a module, this file is the
// reference implementation of isProjectFile: the Go edit gate ports that rule
// and a shared fixture test (TestGateProjectFileMatchesJS) runs both over the
// same inputs so the two copies cannot drift.
if (require.main === module) {
  let inputData = '';
  process.stdin.setEncoding('utf-8');
  process.stdin.on('data', (chunk) => (inputData += chunk));
  process.stdin.on('end', () => {
    if (inputData.trim()) processToolUse(inputData);
  });

  setTimeout(() => {
    if (!inputData) process.exit(0);
  }, 100);
} else {
  module.exports = { isProjectFile, detectAgent, toSlashes };
}
