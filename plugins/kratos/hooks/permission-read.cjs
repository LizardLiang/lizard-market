#!/usr/bin/env node
'use strict';

/**
 * Scoped Read PermissionRequest hook.
 *
 * Auto-allows a Read permission request only when the requested file path
 * resolves under CLAUDE_PLUGIN_ROOT or ~/.kratos/. Every other path is left
 * unhandled so Claude Code's normal permission prompt applies unchanged.
 * Fails open (exit 0, no output) on empty/garbage stdin, a missing file
 * path, or an unset CLAUDE_PLUGIN_ROOT — same discipline as plan-mode-guard.cjs.
 */

const path = require('path');
const os = require('os');

function output() {
  process.stdout.write(JSON.stringify({
    hookSpecificOutput: {
      hookEventName: 'PermissionRequest',
      permissionDecision: 'allow',
      permissionDecisionReason: 'Kratos plugin file (scoped auto-allow)'
    }
  }));
}

function normalize(candidate) {
  let resolved;
  try {
    resolved = path.resolve(String(candidate || ''));
  } catch (_) {
    return '';
  }
  resolved = resolved.replace(/\\/g, '/');
  if (process.platform === 'win32') {
    resolved = resolved.toLowerCase();
  }
  return resolved;
}

function isUnderAllowedRoot(filePath, allowedRoots) {
  const normalizedFile = normalize(filePath);
  if (!normalizedFile) return false;

  return allowedRoots.some((root) => {
    const normalizedRoot = normalize(root);
    if (!normalizedRoot) return false;
    return normalizedFile === normalizedRoot || normalizedFile.startsWith(normalizedRoot + '/');
  });
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

  const input = data.tool_input || {};
  const filePath = input.file_path || input.path || input.filePath;
  if (!filePath) return;

  const allowedRoots = [];
  if (process.env.CLAUDE_PLUGIN_ROOT) {
    allowedRoots.push(process.env.CLAUDE_PLUGIN_ROOT);
  }
  allowedRoots.push(path.join(os.homedir(), '.kratos'));

  if (isUnderAllowedRoot(filePath, allowedRoots)) {
    output();
  }
});

setTimeout(() => {
  if (!raw) process.exit(0);
}, 100);
