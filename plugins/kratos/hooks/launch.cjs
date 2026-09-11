#!/usr/bin/env node
'use strict';

// Cross-platform launcher for the kratos binary.
// Used by hooks.json and command files instead of hardcoded "bin/kratos || ~/.kratos/bin/kratos" chains.
// Resolves the correct platform binary, inherits stdio so hook payloads and JSON output pass through,
// and propagates the child exit code (non-zero exits block check --init/--verify gates).
//
// `agent load` gets two extras:
//   - Missing binary → fall back to reading the agent .md directly off disk (loadAgentFallback) so
//     generated launchers never print an empty body. Every other subcommand keeps the silent exit 0
//     so the pipeline never hard-fails on an optional binary.
//   - Stale binary (older than 2.108, rejects `--part`) → re-run the body line without `--part` so
//     the launcher degrades to the single full load instead of an empty definition; the extras line
//     prints nothing rather than repeating the body.

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

// parseLoadArgs reads `<name> [--resolve] [--root <dir>] [--mode=command] [--part body|extras]`.
function parseLoadArgs(argv) {
  const out = { name: '', part: '', mode: '' };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith('--part=')) out.part = a.slice('--part='.length);
    else if (a === '--part') out.part = argv[++i] || '';
    else if (a.startsWith('--mode=')) out.mode = a.slice('--mode='.length);
    else if (a === '--mode') out.mode = argv[++i] || '';
    else if (a === '--root') i++;
    else if (a.startsWith('--')) continue;
    else if (!out.name) out.name = a;
  }
  return out;
}

// withoutPart strips `--part <v>` / `--part=<v>` so a pre-2.108 binary accepts the call.
function withoutPart(argv) {
  const out = [];
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--part') { i++; continue; }
    if (argv[i].startsWith('--part=')) continue;
    out.push(argv[i]);
  }
  return out;
}

function readPluginFile(root, rel) {
  try {
    return fs.readFileSync(path.join(root, rel), 'utf8');
  } catch {
    return null;
  }
}

// JS fallback for `agent load` when the binary is unavailable. Reads
// plugins/kratos/agents/<name>.md directly and string-replaces <KRATOS_ROOT>
// with the plugin root. <kratos-bin> is left unresolved — agent-protocol.md
// already tells agents to skip kratos calls when no binary path was injected.
// Protocol-block and lessons injection are Go-only (embedded FS + SQLite);
// this path serves the plain body (`--part body`, or no --part) and, for
// `--part extras --mode=command`, the command-mode suffix. Empty extras is
// '' (exit 0), never an error. Returns null only when the agent file is missing.
function loadAgentFallback(argv, root) {
  const { name, part, mode } = parseLoadArgs(argv);
  if (!name) return null;
  const file = name.endsWith('.md') ? name : `${name}.md`;
  const body = readPluginFile(root, path.join('agents', file));
  if (body === null) return null;
  const resolved = (s) => s.split('<KRATOS_ROOT>').join(root);
  const suffix = mode === 'command' ? readPluginFile(root, path.join('command-mode-suffix', file)) : null;
  if (part === 'body') return resolved(body);
  if (part === 'extras') return suffix ? resolved(suffix) : '';
  return resolved(suffix ? `${body}\n---\n\n${suffix}` : body);
}

function main() {
  const args = process.argv.slice(2);
  const root = pluginRoot();
  const isAgentLoad = args[0] === 'agent' && args[1] === 'load';

  const bin = resolveBinary();
  if (!bin) {
    if (isAgentLoad) {
      const out = loadAgentFallback(args.slice(2), root);
      if (out !== null) {
        process.stdout.write(out);
        process.exitCode = 0;
        return;
      }
    }
    process.exitCode = 0;
    return;
  }

  if (!isAgentLoad) {
    const res = spawnSync(bin, args, { stdio: 'inherit' });
    process.exitCode = res.status === null ? 0 : res.status;
    return;
  }

  const finalArgs = [...args, '--root', root];
  const { part } = parseLoadArgs(args.slice(2));
  if (!part) {
    const res = spawnSync(bin, finalArgs, { stdio: 'inherit' });
    process.exitCode = res.status === null ? 0 : res.status;
    return;
  }

  // Capture stderr only to recognize a stale binary; everything else passes through.
  let res = spawnSync(bin, finalArgs, { stdio: ['inherit', 'inherit', 'pipe'] });
  const stderr = res.stderr ? String(res.stderr) : '';
  if (res.status !== 0 && /unknown flag: --part/.test(stderr)) {
    if (part === 'extras') {
      process.exitCode = 0; // the body line already printed the whole definition
      return;
    }
    res = spawnSync(bin, withoutPart(finalArgs), { stdio: 'inherit' });
  } else if (stderr) {
    process.stderr.write(stderr);
  }
  process.exitCode = res.status === null ? 0 : res.status;
}

if (require.main === module) {
  main();
} else {
  module.exports = { loadAgentFallback, parseLoadArgs, withoutPart };
}
