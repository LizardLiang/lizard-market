#!/usr/bin/env node
/**
 * Kratos Memory - Session Start Hook
 *
 * Automatically starts a memory session when Claude Code session begins.
 * Uses global database at ~/.kratos/memory.db
 * 
 * NEW: Injects detailed context if there's an incomplete feature from last session.
 */

const { execSync, spawn } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');

// Global paths
const KRATOS_HOME = path.join(os.homedir(), '.kratos');
const DB_PATH = path.join(KRATOS_HOME, 'memory.db');
const SESSION_FILE = path.join(KRATOS_HOME, 'active-session.json');
const SCHEMA_PATH = path.join(__dirname, '..', 'memory', 'schema.sql');

// Get project name from current working directory
const cwd = process.cwd();
const projectName = path.basename(cwd);

// Ensure .kratos directory exists
function ensureDir() {
  if (!fs.existsSync(KRATOS_HOME)) {
    fs.mkdirSync(KRATOS_HOME, { recursive: true });
  }
}

// Check if Python is available
function getPythonCmd() {
  try {
    const { getPythonCmd: getCmd } = require('./check-python.cjs');
    return getCmd();
  } catch (e) {
    // Fallback: try common commands
    for (const cmd of ['python3', 'python']) {
      try {
        execSync(`${cmd} --version`, { stdio: 'ignore' });
        return cmd;
      } catch (e) {}
    }
    return null;
  }
}

// Initialize database if needed
function initDb() {
  if (fs.existsSync(DB_PATH)) return true;

  const pythonCmd = getPythonCmd();
  if (!pythonCmd) return false;

  try {
    const pythonScript = path.join(__dirname, '..', 'memory', 'kratos_memory.py');
    execSync(`${pythonCmd} "${pythonScript}" init`, {
      stdio: 'ignore',
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
    });
    return true;
  } catch (e) {
    console.error('Failed to init DB:', e.message);
    return false;
  }
}

// Generate UUID v4
function uuid() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
}

// Start session using Python script
function startSession() {
  const pythonCmd = getPythonCmd();
  if (!pythonCmd) return null;

  try {
    const pythonScript = path.join(__dirname, '..', 'memory', 'kratos_memory.py');
    const result = execSync(
      `${pythonCmd} "${pythonScript}" session start "${projectName}"`,
      {
        encoding: 'utf-8',
        env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
      }
    );
    return JSON.parse(result).session_id;
  } catch (e) {
    console.error('Failed to start session:', e.message);
    return null;
  }
}

// Get last session info for context injection
function getLastSessionInfo() {
  const pythonCmd = getPythonCmd();
  if (!pythonCmd) return null;

  try {
    const pythonScript = path.join(__dirname, '..', 'memory', 'kratos_memory.py');
    const result = execSync(
      `${pythonCmd} "${pythonScript}" last-session "${projectName}"`,
      {
        encoding: 'utf-8',
        env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
      }
    );
    return JSON.parse(result);
  } catch (e) {
    return null;
  }
}

// Format time ago
function formatTimeAgo(timestampMs) {
  if (!timestampMs) return 'unknown';
  
  const diffMs = Date.now() - timestampMs;
  const diffMin = diffMs / 60000;
  const diffHour = diffMin / 60;
  const diffDay = diffHour / 24;
  
  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return `${Math.floor(diffMin)} minutes ago`;
  if (diffHour < 24) return `${Math.floor(diffHour)} hours ago`;
  if (diffDay < 7) return `${Math.floor(diffDay)} days ago`;
  return `${Math.floor(diffDay / 7)} weeks ago`;
}

// Format detailed context message for injection
function formatContextMessage(info) {
  if (!info || !info.feature_name) return null;
  if (info.feature_status === 'completed') return null;
  
  const timeAgo = formatTimeAgo(info.started_at);
  const stage = info.current_stage || 0;
  const stageName = info.stage_name || 'Unknown';
  const nextAgent = info.next_agent || 'Unknown';
  const nextStageName = info.next_stage_name || 'Unknown';
  
  // Build the context box
  const lines = [
    '',
    '+----------------------------------------------------------------------+',
    '|  KRATOS MEMORY: Last session detected                                |',
    '+----------------------------------------------------------------------+',
    `|  Feature: ${(info.feature_name || '').padEnd(56)}|`,
    `|  Stage: ${stage}/8 (${stageName})`.padEnd(71) + '|',
    `|  Last active: ${timeAgo}`.padEnd(71) + '|',
    '|                                                                      |'
  ];
  
  // Add last actions
  if (info.last_actions && info.last_actions.length > 0) {
    lines.push('|  Last actions:                                                       |');
    for (const action of info.last_actions.slice(-3)) {
      const truncated = action.length > 60 ? action.substring(0, 57) + '...' : action;
      lines.push(`|  - ${truncated}`.padEnd(71) + '|');
    }
    lines.push('|                                                                      |');
  }
  
  // Add recommendation
  if (info.next_stage !== null && info.next_stage !== undefined) {
    const rec = `Continue with Stage ${info.next_stage} (${nextAgent} - ${nextStageName})?`;
    lines.push(`|  Recommendation: ${rec}`.padEnd(71) + '|');
    lines.push('|  Say "continue" or "/kratos" to resume                               |');
  }
  
  lines.push('+----------------------------------------------------------------------+');
  lines.push('');
  
  return lines.join('\n');
}

// Main
function main() {
  ensureDir();

  // Check for existing active session
  if (fs.existsSync(SESSION_FILE)) {
    try {
      const existing = JSON.parse(fs.readFileSync(SESSION_FILE, 'utf-8'));
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
    started_at: Date.now()
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
