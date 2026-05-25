#!/usr/bin/env node
'use strict';

// Kratos SubagentStart hook — resolves the kratos binary path for the current platform
// and injects it for agents that reference <kratos-bin>.
// Emits nothing (silent exit 0) if binary is not found.

const { resolveBinary } = require('./kratos-bin.cjs');

const kratosPath = resolveBinary();
if (kratosPath) {
  const msg =
    `**Kratos binary resolved:** \`${kratosPath}\`\n\n` +
    `Use the path directly in every Bash command that calls kratos:\n` +
    `\`\`\`bash\n` +
    `'${kratosPath}' <subcommand>\n` +
    `\`\`\`\n` +
    `Wherever agent instructions show \`<kratos-bin>\`, use \`'${kratosPath}'\`.`;
  process.stdout.write(JSON.stringify({
    hookSpecificOutput: {
      hookEventName: 'SubagentStart',
      additionalContext: msg,
    },
  }));
}
