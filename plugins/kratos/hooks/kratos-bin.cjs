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
//   3. ~/.kratos/bin/kratos[.exe]                      (populated by ensureBinary's plugin-bin copy,
//                                                        or by ensure-binary.cjs's release download)
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

// ---- Kratos state directory -------------------------------------------------
// Project directory that holds feature/, .Arena/ and tmp/. Same precedence and
// validation as the Go binary (internal/cli/statedir.go): env KRATOS_STATE_DIR,
// then ~/.kratos/config.json "state_dir", then ".claude".

const DEFAULT_STATE_DIR = '.claude';

function validStateDirName(v) {
  return typeof v === 'string' && v.length >= 2 && v[0] === '.' && v !== '..' && !/[\/\\]/.test(v) && !v.includes(String.fromCharCode(0));
}

function configPath() {
  return path.join(os.homedir(), '.kratos', 'config.json');
}

function readConfig() {
  try {
    const parsed = JSON.parse(fs.readFileSync(configPath(), 'utf-8'));
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null;
  } catch (e) {
    return null;
  }
}

// Returns { dir, source } where source is 'env', '~/.kratos/config.json' or 'default'.
function stateDirInfo() {
  const env = process.env.KRATOS_STATE_DIR;
  if (env) {
    if (validStateDirName(env)) return { dir: env, source: 'KRATOS_STATE_DIR' };
    process.stderr.write("kratos: ignoring invalid KRATOS_STATE_DIR '" + env + "'\n");
    return { dir: DEFAULT_STATE_DIR, source: 'default' };
  }
  const cfg = readConfig();
  if (cfg && cfg.state_dir) {
    if (validStateDirName(cfg.state_dir)) return { dir: cfg.state_dir, source: '~/.kratos/config.json' };
    process.stderr.write("kratos: ignoring invalid state_dir '" + cfg.state_dir + "' in ~/.kratos/config.json\n");
  }
  return { dir: DEFAULT_STATE_DIR, source: 'default' };
}

function stateDir() {
  return stateDirInfo().dir;
}

// Merges <pluginRoot>/kratos.config.json "state_dir" into ~/.kratos/config.json
// when it is valid and differs. Writes a temp file, then renames. A corrupt
// existing config is left alone. Returns true when the file was written.
function syncStateDirConfig(pluginRoot) {
  try {
    const shipped = JSON.parse(fs.readFileSync(path.join(pluginRoot, 'kratos.config.json'), 'utf-8'));
    if (!shipped || !validStateDirName(shipped.state_dir)) return false;
    const target = configPath();
    let current = {};
    if (fs.existsSync(target)) {
      current = readConfig();
      if (!current) return false;
    }
    if (current.state_dir === shipped.state_dir) return false;
    fs.mkdirSync(path.dirname(target), { recursive: true });
    const tmp = target + '.' + process.pid + '.tmp';
    fs.writeFileSync(tmp, JSON.stringify({ ...current, state_dir: shipped.state_dir }, null, 2) + '\n');
    fs.renameSync(tmp, target);
    return true;
  } catch (e) {
    return false;
  }
}

// ---- Binary replacement -----------------------------------------------------
// Windows lets a running .exe be renamed but never deleted or overwritten:
// copyFileSync/renameSync onto ~/.kratos/bin/kratos.exe fail with EBUSY/EPERM
// whenever another session's hook is executing it (measured 2026-11-09 on a
// running kratos.exe: rename OK, unlink EPERM, unlink OK once it exited). Linux
// refuses the overwrite too (ETXTBSY). So an update renames the live file
// into `<bin>/stale/`, places the new file at the fixed path, then deletes the
// stale one — at once when nothing runs it, otherwise at a later SessionStart
// sweep, which ignores the EPERM of a file still held by another process and
// waits for the next start. The Windows call that marks a file for deletion
// (MoveFileEx MOVEFILE_DELAY_UNTIL_REBOOT) needs administrator rights, so the
// sweep stands in for it.

const STALE_DIR_NAME = 'stale';

function staleDirFor(binDir) {
  return path.join(binDir, STALE_DIR_NAME);
}

// Deletes every file in `<binDir>/stale/` that is no longer in use and removes
// the folder once empty. Returns the number removed; a file still executing
// stays for the next sweep. No stale folder → nothing to do.
function sweepStaleBinaries(binDir) {
  const staleDir = staleDirFor(binDir);
  let names;
  try {
    names = fs.readdirSync(staleDir);
  } catch (e) {
    return 0; // no stale folder — the common case
  }
  let removed = 0;
  let left = 0;
  for (const name of names) {
    try {
      fs.unlinkSync(path.join(staleDir, name));
      removed++;
    } catch (e) {
      left++; // still mapped by a running process — next sweep
    }
  }
  if (left === 0) {
    try {
      fs.rmdirSync(staleDir);
    } catch (e) {
      // best-effort tidy-up
    }
  }
  return removed;
}

// Replaces the binary at targetPath. place(targetPath) must put the new file
// there (rename or copy). The old file is moved into `<bin>/stale/` first so
// place() never hits a running executable; if place() throws, the old file is
// moved back so the fixed path never goes empty. Returns the stale path when
// the old binary is still in use and could not be deleted yet, else null.
function replaceBinary(targetPath, place) {
  const dir = path.dirname(targetPath);
  const base = path.basename(targetPath);
  sweepStaleBinaries(dir);
  let aside = null;
  if (fs.existsSync(targetPath)) {
    const staleDir = staleDirFor(dir);
    fs.mkdirSync(staleDir, { recursive: true });
    aside = path.join(staleDir, `${base}.${Date.now()}-${process.pid}`);
    fs.renameSync(targetPath, aside);
  }
  try {
    place(targetPath);
  } catch (e) {
    if (aside) {
      try {
        fs.renameSync(aside, targetPath);
      } catch (e2) {
        // the old binary stays in stale/; the next sweep removes it
      }
    }
    throw e;
  }
  if (aside) {
    try {
      fs.unlinkSync(aside);
      aside = null;
      sweepStaleBinaries(dir); // removes the now-empty stale folder
    } catch (e) {
      // in use by another session's hook — swept at a later SessionStart
    }
  }
  return aside;
}

module.exports = {
  resolveBinary,
  platformBinaryName,
  stateDir,
  stateDirInfo,
  syncStateDirConfig,
  validStateDirName,
  DEFAULT_STATE_DIR,
  replaceBinary,
  sweepStaleBinaries,
  STALE_DIR_NAME,
};
