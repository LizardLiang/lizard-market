#!/usr/bin/env node
'use strict';

/**
 * Kratos Memory - Transcript Sweep Hook (Stop)
 *
 * Periodically injects a one-sentence instruction via
 * `hookSpecificOutput.additionalContext` (no `decision` field, so no red
 * "Stop hook error" styling — see issue #4) pointing Claude at
 * references/memory-sweep.md: (1) durable user facts saved via
 * `kratos memory`, (2) corrections to a god-agent's finished work saved via
 * `kratos feedback` and re-injected at that agent's next spawn.
 *
 * Cadence. Stop fires after every assistant turn, so the hook keeps a small
 * per-session marker (~/.kratos/sweeps/<session_id>.json) with the transcript
 * byte offset it has already scanned plus message counters. Each run reads
 * only the new tail, and a sweep is emitted once enough human messages AND
 * assistant turns have accumulated since the previous sweep — then the
 * counters reset and the next sweep arms again. The previous design swept
 * once, on the first Stop after six "user" lines (tool results included), so
 * every long session was swept ~5% in and never again; every later correction
 * was lost (2026-09 transcript review).
 *
 * Skips: a Stop that is itself a re-invocation (stop_hook_active), the opt-out
 * env var, an inline sweep already run in the new tail (Iris prints
 * `IRIS COMPLETE`, /kratos:wrap prints `KRATOS WRAP COMPLETE` — both reset the
 * counters), missing binary or protocol file. Every failure fails open.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const { resolveBinary } = require('./kratos-bin.cjs');

const SWEEP_DIR = path.join(os.homedir(), '.kratos', 'sweeps');
const MARKER_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000; // 7 days

// A sweep arms when BOTH thresholds are met since the last sweep (or session start).
const MIN_HUMAN_MESSAGES = 10;
const MIN_ASSISTANT_TURNS = 25;
const MAX_SWEEPS_PER_SESSION = 8;

function markerPath(sessionId) {
  return path.join(SWEEP_DIR, `${sessionId}.json`);
}

function legacyMarkerPath(sessionId) {
  return path.join(SWEEP_DIR, sessionId);
}

// Remove markers older than 7 days so the directory doesn't grow forever.
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
      if (now - fs.statSync(entryPath).mtimeMs > MARKER_MAX_AGE_MS) fs.unlinkSync(entryPath);
    } catch (e) {
      // best-effort cleanup only
    }
  }
}

function readMarker(sessionId, transcriptSize) {
  try {
    const m = JSON.parse(fs.readFileSync(markerPath(sessionId), 'utf-8'));
    if (m && typeof m.offset === 'number') return m;
  } catch (e) {
    // no marker yet
  }
  // A legacy 13-byte epoch marker means one sweep already ran at an unknown
  // point: start fresh from the current end so nothing is re-swept.
  if (fs.existsSync(legacyMarkerPath(sessionId))) {
    try { fs.unlinkSync(legacyMarkerPath(sessionId)); } catch (e) { /* ignore */ }
    return { offset: transcriptSize, human: 0, assistant: 0, sweeps: 1 };
  }
  return { offset: 0, human: 0, assistant: 0, sweeps: 0 };
}

function writeMarker(sessionId, marker) {
  fs.mkdirSync(SWEEP_DIR, { recursive: true });
  fs.writeFileSync(markerPath(sessionId), JSON.stringify(marker));
}

// Read the transcript from offset to the end. Returns null when unreadable.
function readTail(transcriptPath, offset) {
  let fd;
  try {
    const size = fs.statSync(transcriptPath).size;
    if (size < offset) offset = 0; // rotated/rewritten transcript
    const length = size - offset;
    if (length <= 0) return { text: '', size };
    fd = fs.openSync(transcriptPath, 'r');
    const buf = Buffer.alloc(length);
    fs.readSync(fd, buf, 0, length, offset);
    return { text: buf.toString('utf-8'), size };
  } catch (e) {
    return null;
  } finally {
    if (fd !== undefined) {
      try { fs.closeSync(fd); } catch (e) { /* ignore */ }
    }
  }
}

// Count human prompts and assistant turns in a transcript chunk. A "user" line
// that carries a tool_result, a slash-command echo, or hook metadata is not a
// human message.
function countMessages(text) {
  let human = 0;
  let assistant = 0;
  for (const line of text.split('\n')) {
    if (line.includes('"type":"assistant"')) {
      assistant++;
      continue;
    }
    if (!line.includes('"type":"user"')) continue;
    if (line.includes('"tool_result"') || line.includes('"isMeta":true') || line.includes('<command-name>')) continue;
    human++;
  }
  return { human, assistant };
}

// Returns the one-line sweep instruction, or null when the protocol file is
// missing (partial install).
function buildInstruction(kratosBin) {
  const pluginRoot = process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, '..');
  const protocol = path.join(pluginRoot, 'references', 'memory-sweep.md');
  if (!fs.existsSync(protocol)) return null;
  return `Silently run the memory sweep in "${protocol.replace(/\\/g, '/')}" `
    + `with <kratos-bin> = ${kratosBin} — no narration; finish with exactly one line: `
    + '`Kratos: swept <n>` or `Kratos: nothing to sweep`.';
}

function quietSweep(instruction) {
  process.stdout.write(JSON.stringify({
    hookSpecificOutput: {
      hookEventName: 'Stop',
      additionalContext: instruction,
    },
  }));
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

  // Loop guard: never re-emit on a Stop that already fired because of us.
  if (data.stop_hook_active === true) return;

  // Opt-out.
  if (process.env.KRATOS_MEMORY_SWEEP === 'off') return;

  const sessionId = data.session_id;
  const transcriptPath = data.transcript_path;
  if (!sessionId || !transcriptPath) return;

  let size;
  try {
    size = fs.statSync(transcriptPath).size;
  } catch (e) {
    return; // fail open
  }

  const marker = readMarker(sessionId, size);
  const tail = readTail(transcriptPath, marker.offset);
  if (tail === null) return;

  const counts = countMessages(tail.text);
  marker.offset = tail.size;
  marker.human += counts.human;
  marker.assistant += counts.assistant;

  // An inline sweep ran in this stretch (Iris mission, /kratos:wrap): the
  // counters restart from here.
  if (tail.text.includes('IRIS COMPLETE') || tail.text.includes('KRATOS WRAP COMPLETE')) {
    marker.human = 0;
    marker.assistant = 0;
  }

  const armed = marker.human >= MIN_HUMAN_MESSAGES
    && marker.assistant >= MIN_ASSISTANT_TURNS
    && marker.sweeps < MAX_SWEEPS_PER_SESSION;

  if (!armed) {
    try { writeMarker(sessionId, marker); } catch (e) { /* fail open */ }
    return;
  }

  const kratosBin = resolveBinary();
  const instruction = kratosBin ? buildInstruction(kratosBin) : null;
  if (!instruction) {
    try { writeMarker(sessionId, marker); } catch (e) { /* fail open */ }
    return;
  }

  marker.human = 0;
  marker.assistant = 0;
  marker.sweeps += 1;
  try {
    writeMarker(sessionId, marker);
  } catch (e) {
    // If we can't persist the marker, don't risk an unguarded repeat emission.
    return;
  }
  pruneOldMarkers();

  quietSweep(instruction);
});

setTimeout(() => {
  if (!raw) process.exit(0);
}, 100);
