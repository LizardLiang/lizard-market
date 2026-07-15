#!/usr/bin/env node
'use strict';

// Cross-platform launcher for the kratos binary.
// Used by hooks.json and command files instead of hardcoded "bin/kratos || ~/.kratos/bin/kratos" chains.
// Resolves the correct platform binary, inherits stdio so hook payloads and JSON output pass through,
// and propagates the child exit code (non-zero exits block check --init/--verify gates).
// Missing binary → for `agent load`, fall back to reading the agent .md directly off disk (see
// loadAgentFallback below) so generated launchers never print an empty body. Every other
// subcommand keeps the original silent exit 0 so the pipeline never hard-fails on an optional binary.

const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { resolveBinary } = require('./kratos-bin.cjs');

function toSlashes(p) {
  return p.replace(/\\/g, '/');
}

function pluginRoot() {
  return toSlashes(process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, '..'));
}

// JS fallback for `agent load <name> [--resolve] [--mode=command]` when the
// binary is unavailable. Reads plugins/kratos/agents/<name>.md directly and
// string-replaces <KRATOS_ROOT> with the plugin root. <kratos-bin> is left
// unresolved — agent-protocol.md already tells agents to skip kratos calls
// when no binary path was injected. Protocol-block and --mode=command suffix
// injection are Go-only (embedded FS content); the fallback only serves the
// plain body, whose pointer lines send the agent to agent-protocol.md.
function loadAgentFallback(argv, root) {
  const name = argv[0];
  if (!name) return null;
  const file = name.endsWith('.md') ? name : `${name}.md`;
  const agentPath = path.join(root, 'agents', file);
  let body;
  try {
    body = fs.readFileSync(agentPath, 'utf8');
  } catch {
    return null;
  }
  return body.split('<KRATOS_ROOT>').join(root);
}

const args = process.argv.slice(2);
const root = pluginRoot();
const isAgentLoad = args[0] === 'agent' && args[1] === 'load';

const bin = resolveBinary();
if (!bin) {
  if (isAgentLoad) {
    const out = loadAgentFallback(args.slice(2), root);
    if (out !== null) {
      process.stdout.write(out);
      process.exit(0);
    }
  }
  process.exit(0);
}

const finalArgs = isAgentLoad ? [...args, '--root', root] : args;
const res = spawnSync(bin, finalArgs, { stdio: 'inherit' });
process.exit(res.status === null ? 0 : res.status);
