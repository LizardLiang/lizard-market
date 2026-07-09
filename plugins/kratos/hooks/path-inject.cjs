#!/usr/bin/env node
'use strict';

// Kratos SubagentStart hook — resolves the kratos binary path and the plugin
// root for the current platform/install, and injects both for agents that
// reference <kratos-bin> / <KRATOS_ROOT>. Root injection fires even when the
// binary is unavailable (subagents still need <KRATOS_ROOT> resolved via
// Read, independent of kratos-bin availability). Emits nothing (silent exit
// 0) if neither is available.

const path = require('path');
const { resolveBinary } = require('./kratos-bin.cjs');

function toSlashes(p) {
  return p.replace(/\\/g, '/');
}

function pluginRoot() {
  return toSlashes(process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, '..'));
}

const kratosPath = resolveBinary();
const root = pluginRoot();

const parts = [];

if (kratosPath) {
  parts.push(
    `**Kratos binary resolved:** \`${kratosPath}\`\n\n` +
    `Use the path directly in every Bash command that calls kratos:\n` +
    `\`\`\`bash\n` +
    `'${kratosPath}' <subcommand>\n` +
    `\`\`\`\n` +
    `Wherever agent instructions show \`<kratos-bin>\`, use \`'${kratosPath}'\`.`
  );
}

if (root) {
  parts.push(
    `**Kratos plugin root:** \`${root}\` — wherever instructions show \`<KRATOS_ROOT>\`, use this path.`
  );
}

if (parts.length > 0) {
  process.stdout.write(JSON.stringify({
    hookSpecificOutput: {
      hookEventName: 'SubagentStart',
      additionalContext: parts.join('\n\n'),
    },
  }));
}
