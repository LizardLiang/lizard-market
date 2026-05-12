#!/usr/bin/env node
/**
 * Kratos SubagentStart hook — injects the resolved kratos binary path
 * into the subagent's context so agents don't rely on the hardcoded
 * ~/.kratos/bin/kratos path.
 *
 * Resolves: ${CLAUDE_PLUGIN_ROOT}/bin/kratos first, then ~/.kratos/bin/kratos.
 * Emits nothing (silent exit 0) if neither is found.
 */

const fs = require('fs');
const path = require('path');

function findKratos() {
  const pluginRoot = process.env.CLAUDE_PLUGIN_ROOT;
  if (pluginRoot) {
    const candidate = path.join(pluginRoot, 'bin', 'kratos');
    if (fs.existsSync(candidate)) return candidate;
  }
  const home = process.env.HOME || process.env.USERPROFILE;
  if (home) {
    const candidate = path.join(home, '.kratos', 'bin', 'kratos');
    if (fs.existsSync(candidate)) return candidate;
  }
  return null;
}

const kratosPath = findKratos();
if (kratosPath) {
  const msg =
    `**Kratos binary path:** \`${kratosPath}\`\n\n` +
    `When any instruction tells you to run \`~/.kratos/bin/kratos <subcommand>\`, ` +
    `use the absolute path above instead. This path is resolved at runtime and ` +
    `works regardless of whether \`kratos install\` has been run.`;
  console.log(JSON.stringify({
    hookSpecificOutput: {
      hookEventName: 'SubagentStart',
      additionalContext: msg,
    },
  }));
}
