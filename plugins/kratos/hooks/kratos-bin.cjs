#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const os = require('os');

const isWindows = process.platform === 'win32';

// Returns the platform-specific binary name inside plugin bin/.
// Used to pick the right prebuilt artifact: kratos.exe, kratos-darwin-arm64, etc.
function platformBinaryName() {
  if (isWindows) return 'kratos.exe';
  if (process.platform === 'darwin') {
    return `kratos-darwin-${os.arch() === 'arm64' ? 'arm64' : 'amd64'}`;
  }
  return `kratos-linux-${os.arch() === 'arm64' ? 'arm64' : 'amd64'}`;
}

function toSlashes(p) {
  return p.replace(/\\/g, '/');
}

// Returns the absolute path to the kratos binary, or null if not found.
// Search order:
//   1. ${CLAUDE_PLUGIN_ROOT}/bin/<platform-specific>  (kratos.exe / kratos-darwin-arm64 / …)
//   2. ${CLAUDE_PLUGIN_ROOT}/bin/kratos[.exe]          (generic local build)
//   3. ~/.kratos/bin/kratos[.exe]                      (populated by ensureBinary at SessionStart)
// Returns null when none of these paths exist — callers must treat null as "binary unavailable".
// All returned paths use forward slashes so they work in bash on every platform.
function resolveBinary() {
  const pluginRoot = process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, '..');
  const pluginBin = path.join(pluginRoot, 'bin');
  const home = os.homedir();
  const genericName = isWindows ? 'kratos.exe' : 'kratos';

  const candidates = [
    path.join(pluginBin, platformBinaryName()),
    path.join(pluginBin, genericName),
    path.join(home, '.kratos', 'bin', genericName),
  ];

  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) return toSlashes(candidate);
  }

  return null;
}

module.exports = { resolveBinary, platformBinaryName };
