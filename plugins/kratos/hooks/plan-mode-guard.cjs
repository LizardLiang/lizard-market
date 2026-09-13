#!/usr/bin/env node
'use strict';

/**
 * Best-effort Plan Mode guard for Odysseus.
 *
 * Claude hook payloads are allowed to evolve. This hook only enforces when the
 * payload identifies the active agent as Odysseus; otherwise it fails open so
 * normal Write/Edit/Bash usage by other Kratos agents is not broken.
 */

const path = require('path');

const PLAN_ROOT_PARTS = ['.claude', '.Arena', 'tactical-plans'];
// Odysseus authors a pending spec delta in step 4. It is a planning artifact,
// not source, so it belongs in the write allowlist alongside tactical plans.
// Exactly one segment after spec-delta/, so spec-delta/archived/*.md — where
// `kratos spec archive` moves promoted deltas — stays denied.
const SPEC_DELTA_RE = /(^|\/)\.claude\/feature\/[^/]+\/spec-delta\/[^/]+\.md$/i;
const READ_ONLY_COMMANDS = [
  /^git\s+(status|diff|show|log|branch|rev-parse|ls-files)\b/i,
  // `git -C <path> <verb>` is how Odysseus inspects a sibling repo without cd.
  /^git\s+-C\s+(?:"[^"]*"|'[^']*'|\S+)\s+(status|diff|show|log|branch|rev-parse|ls-files)\b/i,
  // `sed -n 'a,bp'` prints a line range; `sed -i` is caught by the deny heuristics first.
  /^sed\s+-n\b/i,
  /^(head|tail|wc|which|stat|file)\b/i,
  /^command\s+-v\b/i,
  /^(ls|dir|pwd)\b/i,
  /^(cat|type)\b/i,
  /^(find|grep|rg)\b/i,
  /^Get-(ChildItem|Content|Location)\b/i,
  /^Select-String\b/i,
  /^Test-Path\b/i
];

// Read-only kratos subcommands Odysseus is instructed to run: slug mint and
// timestamp (step 2), template fetch and delta self-validation (step 4), draft
// discovery (step 1). Mutating subcommands — spec archive, pipeline update,
// session start, init, install — are deliberately absent and stay denied.
const KRATOS_READ_ONLY_SUBCOMMAND = /^(?:slug|now|version|--version|template\s+get|spec\s+(?:validate|list)|agent\s+(?:load|protocol)|session\s+active|pipeline\s+(?:get|status|next|discover)|feedback\s+list|memory\s+list|profile\s+list|todo\s+list)\b/i;

// Any shell metacharacter disqualifies the command. This is what makes it safe
// to check the kratos allowlist *before* the generic deny heuristics: without it,
// `kratos slug -d "x" && rm -rf build` matches the allowlist prefix and is
// allowed outright.
const SHELL_META_RE = /[;&|<>`$]/;

function output(decision, reason) {
  process.stdout.write(JSON.stringify({
    hookSpecificOutput: {
      hookEventName: 'PreToolUse',
      permissionDecision: decision,
      permissionDecisionReason: reason
    }
  }));
}

function isOdysseus(data) {
  const haystack = [
    data.agent_type,
    data.subagent_type,
    data.agent,
    data.tool_input?.agent_type,
    data.tool_input?.subagent_type
  ].filter(Boolean).join(' ').toLowerCase();

  return haystack.includes('odysseus') || haystack.includes('kratos:odysseus');
}

function normalizeFilePath(filePath) {
  return String(filePath || '')
    .replace(/\\/g, '/')
    .replace(/\/+/g, '/');
}

function isPlanPath(filePath) {
  const normalized = normalizeFilePath(filePath);
  const required = PLAN_ROOT_PARTS.join('/');
  return normalized.includes(required + '/') && normalized.endsWith('.md');
}

// `.claude/feature/<slug>/spec-delta/<capability>.md` only — the rest of the
// feature dir (prd.md, status.json, tech-spec.md) stays denied.
function isSpecDeltaPath(filePath) {
  return SPEC_DELTA_RE.test(normalizeFilePath(filePath));
}

// `<maybe-quoted path to>/kratos[.exe] <read-only subcommand> ...` with no shell
// metacharacters anywhere, so argument text (task titles) is inert by construction.
//
// Checked before the generic heuristics below, because those scan the whole
// command string and would false-deny `kratos slug --dated "move the sidebar"`
// on their `\bmove\b` pattern.
// The agent protocol itself recommends the timestamp fallback
// `TS=$(<kratos-bin> now 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)`. Those
// decorations are inert, so they are stripped before the allowlist check: a
// VAR=$( ... ) wrapper, `2>/dev/null`, and a trailing `|| date ...`. Anything
// else that leaves a shell metacharacter behind is still rejected. Before this,
// the guard denied the plugin's own `kratos now` / `slug --dated` /
// `session active` calls 12 times in one week.
function stripSafeFallbacks(command) {
  let c = String(command || '').trim();
  const wrap = c.match(/^[A-Za-z_][A-Za-z0-9_]*=\$\(([\s\S]*)\)$/);
  if (wrap) c = wrap[1].trim();
  c = c.replace(/\s*2>\s*\/dev\/null/g, '');
  c = c.replace(/\s*\|\|\s*date\b[^;&|$`]*$/, '');
  return c.trim();
}

function isReadOnlyKratosCommand(command) {
  const trimmed = stripSafeFallbacks(command);
  if (!trimmed || SHELL_META_RE.test(trimmed)) return false;

  const m = trimmed.match(/^(?:"([^"]+)"|'([^']+)'|(\S+))\s+([\s\S]+)$/);
  if (!m) return false;

  const bin = normalizeFilePath(m[1] || m[2] || m[3]);
  const base = bin.slice(bin.lastIndexOf('/') + 1).toLowerCase();
  if (base !== 'kratos' && base !== 'kratos.exe') return false;

  return KRATOS_READ_ONLY_SUBCOMMAND.test(m[4].trim());
}

function isReadOnlyCommand(command) {
  const trimmed = String(command || '').trim();
  if (!trimmed) return true;

  if (/[;&|]\s*(rm|del|erase|mv|move|cp|copy|mkdir|rmdir|npm\s+install|pnpm\s+install|yarn\s+add|bun\s+add)\b/i.test(trimmed)) {
    return false;
  }
  if (/(^|\s)(>|>>|Set-Content|Add-Content|Out-File|Remove-Item|Move-Item|Copy-Item|New-Item)\b/i.test(trimmed)) {
    return false;
  }
  // The optional `-C <path>` keeps `git -C /repo checkout main` denied here
  // rather than relying on the allowlist missing it.
  if (/\b(rm|del|erase|mv|move|mkdir|rmdir|npm\s+install|pnpm\s+install|yarn\s+add|bun\s+add|git\s+(?:-C\s+(?:"[^"]*"|'[^']*'|\S+)\s+)?(push|commit|reset|checkout|switch|merge|rebase|pull))\b/i.test(trimmed)) {
    return false;
  }
  // In-place sed edits files: `-i`, `-i.bak`, a combined `-ni`, or `--in-place`.
  if (/^sed\b.*\s(-[A-Za-z]*i|--in-place)/i.test(trimmed)) {
    return false;
  }

  return READ_ONLY_COMMANDS.some((pattern) => pattern.test(trimmed));
}

let raw = '';
process.stdin.setEncoding('utf-8');
process.stdin.on('data', (chunk) => raw += chunk);
process.stdin.on('end', () => {
  if (!raw.trim()) return;

  let data;
  try {
    data = JSON.parse(raw);
  } catch (_) {
    return;
  }

  if (!isOdysseus(data)) return;

  const toolName = data.tool_name;
  const input = data.tool_input || {};

  if (toolName === 'Write' || toolName === 'Edit' || toolName === 'MultiEdit') {
    const filePath = input.file_path || input.path || input.filePath;
    if (isPlanPath(filePath)) {
      output('allow', 'Odysseus may write tactical plan markdown files.');
      return;
    }
    if (isSpecDeltaPath(filePath)) {
      output('allow', 'Odysseus may write the pending spec delta.');
      return;
    }
    output('deny', 'Odysseus plan mode may only write .claude/.Arena/tactical-plans/*.md and .claude/feature/<slug>/spec-delta/*.md.');
    return;
  }

  if (toolName === 'Bash') {
    if (isReadOnlyKratosCommand(input.command)) {
      output('allow', 'Odysseus may run read-only kratos subcommands.');
      return;
    }
    if (isReadOnlyCommand(input.command)) {
      output('allow', 'Odysseus may run read-only inspection commands.');
      return;
    }
    output('deny', 'Odysseus plan mode may only run read-only inspection commands.');
  }
});

setTimeout(() => {
  if (!raw) process.exit(0);
}, 100);
