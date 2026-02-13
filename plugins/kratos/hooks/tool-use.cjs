#!/usr/bin/env node
/**
 * Kratos Memory - Tool Use Hook
 *
 * Records agent spawns and file changes automatically.
 * Receives tool use data via stdin in JSON format.
 */

const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');

// Global paths
const KRATOS_HOME = path.join(os.homedir(), '.kratos');
const DB_PATH = path.join(KRATOS_HOME, 'memory.db');
const SESSION_FILE = path.join(KRATOS_HOME, 'active-session.json');

// Kratos agent names for detection
const KRATOS_AGENTS = ['metis', 'athena', 'hephaestus', 'apollo', 'artemis', 'ares', 'hermes'];

// Read session data
function getSession() {
  if (!fs.existsSync(SESSION_FILE)) return null;
  try {
    return JSON.parse(fs.readFileSync(SESSION_FILE, 'utf-8'));
  } catch (e) {
    return null;
  }
}

// Find kratos binary
function findKratosBinary() {
  const locations = [
    'kratos', // In PATH
    path.join(__dirname, '..', 'bin', 'kratos'), // Local bin
    path.join(__dirname, '..', 'bin', 'kratos.exe'), // Windows local bin
    path.join(os.homedir(), 'bin', 'kratos'), // User bin
    path.join(os.homedir(), 'bin', 'kratos.exe'), // Windows user bin
  ];

  for (const loc of locations) {
    try {
      execSync(`"${loc}" --version`, { stdio: 'ignore' });
      return loc;
    } catch (e) {}
  }

  return null;
}

// Record agent spawn
function recordAgentSpawn(sessionId, agentName, agentModel, action) {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return false;

  try {
    execSync(
      `"${kratosCmd}" step record-agent "${sessionId}" "${agentName}" "${agentModel}" "${escapeShell(action)}"`,
      {
        stdio: 'ignore',
        env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
      }
    );
    return true;
  } catch (e) {
    return false;
  }
}

// Record file change
function recordFileChange(sessionId, filePath, changeType) {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return false;

  try {
    execSync(
      `"${kratosCmd}" step record-file "${sessionId}" "${changeType}" "${escapeShell(filePath)}"`,
      {
        stdio: 'ignore',
        env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
      }
    );
    return true;
  } catch (e) {
    return false;
  }
}

// Escape shell characters
function escapeShell(str) {
  if (!str) return '';
  return str.replace(/"/g, '\\"').replace(/\n/g, ' ').substring(0, 200);
}

// Detect Kratos agent from Task tool input
function detectAgent(toolInput) {
  const desc = (toolInput?.description || '').toLowerCase();
  const prompt = (toolInput?.prompt || '').toLowerCase();
  const agentType = toolInput?.subagent_type || '';

  for (const agent of KRATOS_AGENTS) {
    if (desc.includes(agent) || prompt.includes(agent) || agentType.includes(agent)) {
      return agent;
    }
  }
  return agentType || 'unknown';
}

// Process tool use
function processToolUse(data) {
  const session = getSession();
  if (!session) return;

  let toolData;
  try {
    toolData = JSON.parse(data);
  } catch (e) {
    return;
  }

  const { tool_name, tool_input } = toolData;
  const sessionId = session.session_id;

  // Record Task tool usage (agent spawns)
  if (tool_name === 'Task') {
    const agent = detectAgent(tool_input);
    const description = tool_input?.description || 'Agent task';
    const model = tool_input?.model || 'sonnet';
    const action = `${description}`;

    recordAgentSpawn(sessionId, agent, model, action);
  }

  // Record file writes
  if (tool_name === 'Write') {
    const filePath = tool_input?.file_path || 'unknown';
    recordFileChange(sessionId, filePath, 'Write');
  }

  // Record file edits
  if (tool_name === 'Edit') {
    const filePath = tool_input?.file_path || 'unknown';
    recordFileChange(sessionId, filePath, 'Edit');
  }
}

// Main - read from stdin
let inputData = '';
process.stdin.setEncoding('utf-8');
process.stdin.on('data', chunk => inputData += chunk);
process.stdin.on('end', () => {
  if (inputData.trim()) {
    processToolUse(inputData);
  }
});

// Handle case where stdin is empty or closed immediately
setTimeout(() => {
  if (!inputData) process.exit(0);
}, 100);
