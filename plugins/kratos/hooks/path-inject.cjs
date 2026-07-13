#!/usr/bin/env node
'use strict';

// Kratos SubagentStart hook — resolves the kratos binary path and the plugin
// root for the current platform/install, and injects both for agents that
// reference <kratos-bin> / <KRATOS_ROOT>. Root injection fires even when the
// binary is unavailable (subagents still need <KRATOS_ROOT> resolved via
// Read, independent of kratos-bin availability). Also reads the SubagentStart
// payload from stdin for the spawning agent's name and injects that agent's
// stored feedback lessons (≤5, current-project first) — fail-open: any error
// just drops the lessons part. Emits nothing (silent exit 0) if no part is
// available.

const path = require('path');
const os = require('os');
const { execSync } = require('child_process');
const { resolveBinary } = require('./kratos-bin.cjs');

const DB_PATH = path.join(os.homedir(), '.kratos', 'memory.db');
const MAX_LESSONS = 5;

function toSlashes(p) {
  return p.replace(/\\/g, '/');
}

function pluginRoot() {
  return toSlashes(process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, '..'));
}

// Lessons saved from past user corrections of this god (memory-sweep.cjs
// capture side). Current-project lessons sort first via --prefer-project.
function lessonsPart(kratosPath, agentType) {
  if (!kratosPath || typeof agentType !== 'string' || !agentType.startsWith('kratos:')) return null;
  const god = agentType.slice('kratos:'.length).toLowerCase();
  if (!/^[a-z][a-z-]*$/.test(god)) return null;
  try {
    const out = execSync(`"${kratosPath}" feedback list --agent ${god} --limit ${MAX_LESSONS} --prefer-project`, {
      encoding: 'utf-8',
      timeout: 2000,
      env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH },
    });
    const data = JSON.parse(out);
    if (!Array.isArray(data.feedback) || data.feedback.length === 0) return null;
    const lines = [`**Lessons from past user corrections of ${god}** — apply them to this task:`];
    for (const f of data.feedback.slice(0, MAX_LESSONS)) lines.push(`- ${f.lesson}`);
    return lines.join('\n');
  } catch (e) {
    return null; // fail-open: no lessons part
  }
}

const kratosPath = resolveBinary();
const root = pluginRoot();

const baseParts = [];

if (kratosPath) {
  baseParts.push(
    `**Kratos binary resolved:** \`${kratosPath}\`\n\n` +
    `Use the path directly in every Bash command that calls kratos:\n` +
    `\`\`\`bash\n` +
    `'${kratosPath}' <subcommand>\n` +
    `\`\`\`\n` +
    `Wherever agent instructions show \`<kratos-bin>\`, use \`'${kratosPath}'\`.`
  );
}

if (root) {
  baseParts.push(
    `**Kratos plugin root:** \`${root}\` — wherever instructions show \`<KRATOS_ROOT>\`, use this path.`
  );
}

let emitted = false;
function emit(parts) {
  if (emitted) return;
  emitted = true;
  if (parts.length > 0) {
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: {
        hookEventName: 'SubagentStart',
        additionalContext: parts.join('\n\n'),
      },
    }));
  }
  // Release stdin so the process exits naturally once stdout flushes —
  // process.exit() here can truncate piped output on Windows.
  process.stdin.destroy();
}

let raw = '';
process.stdin.setEncoding('utf-8');
process.stdin.on('data', (chunk) => raw += chunk);
process.stdin.on('end', () => {
  let agentType = null;
  try {
    agentType = JSON.parse(raw).agent_type;
  } catch (e) {
    // Malformed/empty payload — inject base parts only.
  }
  const lessons = lessonsPart(kratosPath, agentType);
  emit(lessons ? baseParts.concat(lessons) : baseParts);
});

// If no stdin arrives (older harness, manual invocation), still inject the
// base parts before the 3s hook timeout.
setTimeout(() => emit(baseParts), 1500).unref();
