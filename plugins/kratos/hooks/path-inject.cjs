#!/usr/bin/env node
/**
 * Kratos SubagentStart hook — resolves the kratos binary path for the
 * current platform and injects it as KRATOS_BIN.
 *
 * Search order: ${CLAUDE_PLUGIN_ROOT}/bin/kratos, then the user home .kratos/bin.
 * On Windows, checks both `kratos.exe` and `kratos`.
 * All returned paths use forward slashes so they work in bash on every platform.
 * Emits nothing (silent exit 0) if binary is not found.
 */

const fs = require('fs');
const path = require('path');

const isWindows = process.platform === 'win32';

function toSlashes(p) {
  return p.replace(/\\/g, '/');
}

function tryBinary(dir, name) {
  if (isWindows) {
    const withExt = path.join(dir, name + '.exe');
    if (fs.existsSync(withExt)) return toSlashes(withExt);
  }
  const plain = path.join(dir, name);
  if (fs.existsSync(plain)) return toSlashes(plain);
  return null;
}

function findKratos() {
  const pluginRoot = process.env.CLAUDE_PLUGIN_ROOT;
  if (pluginRoot) {
    const found = tryBinary(path.join(pluginRoot, 'bin'), 'kratos');
    if (found) return found;
  }
  const home = process.env.HOME || process.env.USERPROFILE;
  if (home) {
    const found = tryBinary(path.join(home, '.kratos', 'bin'), 'kratos');
    if (found) return found;
  }
  return null;
}

const kratosPath = findKratos();
if (kratosPath) {
  const msg =
    `**Kratos binary resolved:** \`${kratosPath}\`\n\n` +
    `Set \`KRATOS_BIN\` at the start of **every** Bash command that calls kratos:\n` +
    `\`\`\`bash\n` +
    `KRATOS_BIN='${kratosPath}'; "$KRATOS_BIN" <subcommand>\n` +
    `\`\`\`\n` +
    `Agent instructions use \`"$KRATOS_BIN"\` as the placeholder — ` +
    `the variable must be set inline because each Bash call runs in a fresh shell.`;
  process.stdout.write(JSON.stringify({
    hookSpecificOutput: {
      hookEventName: 'SubagentStart',
      additionalContext: msg,
    },
  }));
}
