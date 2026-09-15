#!/usr/bin/env node
/**
 * Kratos Memory - Session End Hook (SessionEnd event)
 *
 * Closes this Claude Code session's ledger row with a one-line summary and
 * removes its ~/.kratos/sessions/<session_id>.json state file.
 *
 * Registered on SessionEnd, not Stop: Stop fires after every assistant turn,
 * so the old wiring ended the session on turn one, deleted the shared state
 * file, and then printed "Kratos: No active session to end" plus the pending
 * spec-delta list on every later turn (9× in one session). SessionEnd is
 * fire-and-forget — hooks.json declares a 2 s timeout for this hook (rounded
 * up from Claude Code's 1.5 s default SessionEnd budget; no fractional
 * per-hook timeout is shown in the docs' examples, and a longer per-hook
 * timeout raises that shared budget to match) — keep this to two quick CLI
 * calls of 600 ms each and print nothing.
 *
 * Kratos calls use spawnSync with an argv array, so the summary text (feature
 * names, cwd basename) never reaches a shell.
 */

const { spawnSync } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');
const { resolveBinary } = require('./kratos-bin.cjs');

const KRATOS_HOME = path.join(os.homedir(), '.kratos');
const DB_PATH = path.join(KRATOS_HOME, 'memory.db');
const SESSIONS_DIR = path.join(KRATOS_HOME, 'sessions');

// Two calls must fit hooks.json's 2 s SessionEnd timeout with node startup — 600 ms each.
const CALL_TIMEOUT_MS = 600;

function runKratos(kratosCmd, args) {
  try {
    const r = spawnSync(kratosCmd, args, {
      encoding: 'utf-8',
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
      stdio: ['ignore', 'pipe', 'ignore'],
      timeout: CALL_TIMEOUT_MS,
    });
    if (r.error || r.status !== 0) return null;
    return r.stdout;
  } catch (e) {
    return null;
  }
}

// Most recently touched in-progress feature under cwd/.claude/feature, for the
// summary line only.
function findActiveFeature(cwd) {
  const featureDir = path.join(cwd, '.claude', 'feature');
  let features;
  try {
    features = fs.readdirSync(featureDir);
  } catch (e) {
    return null;
  }
  let mostRecent = null;
  let mostRecentTime = 0;
  for (const featureName of features) {
    const statusPath = path.join(featureDir, featureName, 'status.json');
    try {
      const stats = fs.statSync(statusPath);
      const statusData = JSON.parse(fs.readFileSync(statusPath, 'utf-8'));
      const featureStatus = statusData.status || (statusData.feature && statusData.feature.status);
      if (featureStatus === 'completed' || featureStatus === 'abandoned') continue;
      if (stats.mtimeMs > mostRecentTime) {
        mostRecentTime = stats.mtimeMs;
        mostRecent = { name: featureName, stage: statusData.current_stage || statusData.stage || 0 };
      }
    } catch (e) {
      // skip invalid status files
    }
  }
  return mostRecent;
}

// Single-line, length-capped summary text (argv-safe; no shell quoting involved).
function clip(str, max) {
  if (!str) return '';
  return String(str).replace(/\r?\n/g, ' ').substring(0, max);
}

function formatDuration(ms) {
  const minutes = Math.floor(ms / 60000);
  if (minutes < 60) return `${minutes} minutes`;
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
}

function endSession(payload) {
  const sessionId = payload && payload.session_id ? String(payload.session_id) : '';
  if (!sessionId) return;
  const cwd = (payload && payload.cwd) || process.cwd();
  const reason = (payload && payload.reason) || 'other';

  const stateFile = path.join(SESSIONS_DIR, `${sessionId}.json`);
  let startedAt = null;
  try {
    startedAt = JSON.parse(fs.readFileSync(stateFile, 'utf-8')).started_at || null;
  } catch (e) {
    // no state file — still close the ledger row if it exists
  }

  const kratosCmd = resolveBinary();
  if (kratosCmd) {
    let stepsSummary = '';
    const stepsRaw = runKratos(kratosCmd, ['step', 'list', sessionId]);
    if (stepsRaw) {
      try {
        const steps = JSON.parse(stepsRaw).steps || [];
        const agents = steps.filter((s) => s.step_type === 'agent_spawn').length;
        stepsSummary = `${steps.length} steps, ${agents} agents spawned`;
      } catch (e) {
        // ignore
      }
    }
    const parts = [`Session in ${path.basename(cwd)}`];
    if (startedAt) parts[0] += ` (${formatDuration(Date.now() - startedAt)})`;
    if (stepsSummary) parts.push(stepsSummary);
    const feature = findActiveFeature(cwd);
    if (feature) parts.push(`Feature: ${feature.name} (stage ${feature.stage})`);
    parts.push(`ended: ${reason}`);
    runKratos(kratosCmd, ['session', 'end', sessionId, clip(parts.join('; '), 500)]);
  }

  try {
    fs.unlinkSync(stateFile);
  } catch (e) {
    // already gone
  }
}

let raw = '';
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
  endSession(payload);
}
process.stdin.setEncoding('utf-8');
process.stdin.on('data', (chunk) => (raw += chunk));
process.stdin.on('end', finish);
process.stdin.on('error', finish);
setTimeout(finish, 200).unref();
