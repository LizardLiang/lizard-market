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
const READ_ONLY_COMMANDS = [
  /^git\s+(status|diff|show|log|branch|rev-parse|ls-files)\b/i,
  /^(ls|dir|pwd)\b/i,
  /^(cat|type)\b/i,
  /^(find|grep|rg)\b/i,
  /^Get-(ChildItem|Content|Location)\b/i,
  /^Select-String\b/i,
  /^Test-Path\b/i
];

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

function isReadOnlyCommand(command) {
  const trimmed = String(command || '').trim();
  if (!trimmed) return true;

  if (/[;&|]\s*(rm|del|erase|mv|move|cp|copy|mkdir|rmdir|npm\s+install|pnpm\s+install|yarn\s+add|bun\s+add)\b/i.test(trimmed)) {
    return false;
  }
  if (/(^|\s)(>|>>|Set-Content|Add-Content|Out-File|Remove-Item|Move-Item|Copy-Item|New-Item)\b/i.test(trimmed)) {
    return false;
  }
  if (/\b(rm|del|erase|mv|move|mkdir|rmdir|npm\s+install|pnpm\s+install|yarn\s+add|bun\s+add|git\s+(push|commit|reset|checkout|switch|merge|rebase|pull))\b/i.test(trimmed)) {
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
    output('deny', 'Odysseus plan mode may only write .claude/.Arena/tactical-plans/*.md.');
    return;
  }

  if (toolName === 'Bash') {
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
