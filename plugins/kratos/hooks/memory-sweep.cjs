#!/usr/bin/env node
'use strict';

/**
 * Kratos Memory - Transcript Sweep Hook (Stop)
 *
 * Once per qualifying session, blocks the final Stop with an instruction that
 * asks Claude to review the whole conversation for (1) durable user facts
 * (preferences, habits, weak spots, corrections, working style), saved via
 * `kratos memory` — the same CLI, dedupe, and 📝 notice Iris uses inline —
 * and (2) corrections the user made to a specific god-agent's finished work,
 * saved per agent via `kratos feedback` and re-injected at that agent's next
 * spawn by path-inject.cjs.
 *
 * This is a session-wide safety net: Iris only sweeps during its own missions,
 * so facts revealed during ordinary (non-Iris) Kratos work would otherwise be
 * lost. Guarded to run at most once per session, to skip sessions where Iris
 * already swept (transcript contains `IRIS COMPLETE`), and to fail open
 * whenever the hook contract, transcript, or binary is unavailable — see
 * hooks/README.md.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const { resolveBinary } = require('./kratos-bin.cjs');

const SWEEP_DIR = path.join(os.homedir(), '.kratos', 'sweeps');
const MIN_USER_MESSAGES = 6;
const MARKER_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000; // 7 days

function markerPath(sessionId) {
  return path.join(SWEEP_DIR, sessionId);
}

// Remove sweep markers older than 7 days so the directory doesn't grow forever.
function pruneOldMarkers() {
  let entries;
  try {
    entries = fs.readdirSync(SWEEP_DIR);
  } catch (e) {
    return;
  }

  const now = Date.now();
  for (const entry of entries) {
    const entryPath = path.join(SWEEP_DIR, entry);
    try {
      const stats = fs.statSync(entryPath);
      if (now - stats.mtimeMs > MARKER_MAX_AGE_MS) {
        fs.unlinkSync(entryPath);
      }
    } catch (e) {
      // Ignore — best-effort cleanup only.
    }
  }
}

// Read the raw transcript once for both the user-message count and the Iris
// skip check below. Returns null when the transcript can't be read — callers
// must treat null as "fail open, allow stop".
function readTranscript(transcriptPath) {
  try {
    return fs.readFileSync(transcriptPath, 'utf-8');
  } catch (e) {
    return null;
  }
}

function buildReason(kratosBin) {
  return 'Two memory sweeps, then stop. '
    + '(1) USER FACTS: Review this conversation for durable user facts (preferences, habits, weak spots, '
    + 'corrections, working style — not project/task facts, never secrets). '
    + 'Project/task/repo facts belong in the project\'s Arena, not memory — when in doubt, save nothing. '
    + `Run \`${kratosBin} memory list\` to dedupe. `
    + `Save at most 3 via \`${kratosBin} memory add "<fact>" --category <preference|habit|weak-spot|context>\`. `
    + 'Use only those four categories. Each fact must be ≤200 characters — write it short the first time. '
    + '(2) AGENT LESSONS: If the user corrected or redirected work a specific Kratos god-agent had just delivered, '
    + `run \`${kratosBin} feedback list --agent <god>\` to dedupe, then save at most 2 via `
    + `\`${kratosBin} feedback add --agent <god> "<lesson>"\`. `
    + 'A lesson is what that agent should do differently next time, ≤200 characters. '
    + 'Only corrections clearly attributable to one agent\'s finished output — general preferences belong in memory, not feedback. '
    + 'If nothing durable in either sweep, save nothing. Then finish with a one-line 📝 note (or nothing) and stop.';
}

function block(reason) {
  process.stdout.write(JSON.stringify({ decision: 'block', reason }));
}

let raw = '';
process.stdin.setEncoding('utf-8');
process.stdin.on('data', (chunk) => raw += chunk);
process.stdin.on('end', () => {
  if (!raw.trim()) return;

  let data;
  try {
    data = JSON.parse(raw);
  } catch (e) {
    return;
  }

  // Loop guard: never re-block a Stop that already fired because of us.
  if (data.stop_hook_active === true) return;

  // Opt-out.
  if (process.env.KRATOS_MEMORY_SWEEP === 'off') return;

  const sessionId = data.session_id;
  if (!sessionId) return;

  // Per-session marker: at most one sweep block per session.
  if (fs.existsSync(markerPath(sessionId))) return;

  // Threshold: skip short sessions. Fail open if the transcript is missing
  // or unreadable — never block blind.
  const transcriptPath = data.transcript_path;
  if (!transcriptPath) return;

  const transcript = readTranscript(transcriptPath);
  if (transcript === null) return;

  let userMessageCount = 0;
  for (const line of transcript.split('\n')) {
    if (line.includes('"type":"user"')) userMessageCount++;
  }
  if (userMessageCount < MIN_USER_MESSAGES) return;

  // Iris sweeps her own missions before IRIS COMPLETE — don't double-sweep.
  if (transcript.includes('IRIS COMPLETE')) return;

  // No CLI, no sweep.
  const kratosBin = resolveBinary();
  if (!kratosBin) return;

  try {
    fs.mkdirSync(SWEEP_DIR, { recursive: true });
    fs.writeFileSync(markerPath(sessionId), String(Date.now()));
  } catch (e) {
    // If we can't write the marker, don't risk an unguarded repeat block.
    return;
  }
  pruneOldMarkers();

  block(buildReason(kratosBin));
});

setTimeout(() => {
  if (!raw) process.exit(0);
}, 100);
