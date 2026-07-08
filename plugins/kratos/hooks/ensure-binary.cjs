#!/usr/bin/env node
'use strict';

// Kratos Memory - Binary Downloader
//
// Downloads the platform kratos binary from GitHub Release assets into
// ~/.kratos/bin/ when no plugin-local binary is available. Zero-dependency
// (https, crypto, fs, path, os, child_process only) so it never adds a
// package.json to the hooks/ directory.
//
// Dual-use: `require('./ensure-binary.cjs').ensureBinary()` for programmatic
// use, or `node ensure-binary.cjs` for the detached background spawn from
// session-start.cjs. Every failure path is a silent exit 0 — the binary is
// optional and this script must never surface an error to the caller.

const https = require('https');
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const os = require('os');
const { URL } = require('url');
const { execSync } = require('child_process');
const { platformBinaryName } = require('./kratos-bin.cjs');

const isWindows = process.platform === 'win32';

// Repo hosting the release assets. Assets live on lizard-market (the release
// workflow needs the Go source and only runs there) — not the runtime-only
// LizardLiang/kratos mirror. Override via env for pre-release / test runs.
const REPO = process.env.KRATOS_DOWNLOAD_REPO || 'LizardLiang/lizard-market';
const BASE_URL =
  process.env.KRATOS_DOWNLOAD_BASE_URL ||
  `https://github.com/${REPO}/releases/download`;

const THROTTLE_MS = 6 * 60 * 60 * 1000; // 6h between failed attempts
const STALE_LOCK_MS = 15 * 60 * 1000; // 15min before a lock is considered abandoned
const MAX_REDIRECTS = 5;

const KRATOS_HOME = path.join(os.homedir(), '.kratos');
const BIN_DIR = path.join(KRATOS_HOME, 'bin');
const VERSION_MARKER = path.join(BIN_DIR, '.version');
const LOCK_FILE = path.join(BIN_DIR, '.download.lock');
const LAST_ATTEMPT_FILE = path.join(BIN_DIR, '.last-download-attempt');
const TMP_FILE = path.join(BIN_DIR, '.kratos.download.tmp');

// Reads the plugin's own manifest to find the version we want installed.
function pluginVersion() {
  const pluginRoot = process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, '..');
  const manifestPath = path.join(pluginRoot, '.claude-plugin', 'plugin.json');
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf-8'));
  if (!manifest.version) throw new Error('plugin.json missing version');
  return manifest.version;
}

// The raw GitHub Release asset filename (kratos-release.yml step 1) always
// carries the OS-arch suffix, even on Windows — unlike platformBinaryName()
// from kratos-bin.cjs, which returns the bare "kratos.exe" for Windows
// because that's the plugin-local bin/ convention (single supported arch).
// The two must stay separate or the downloader 404s on every Windows install.
function releaseAssetName() {
  if (isWindows) return 'kratos-windows-amd64.exe';
  if (process.platform === 'darwin') {
    return `kratos-darwin-${os.arch() === 'arm64' ? 'arm64' : 'amd64'}`;
  }
  return `kratos-linux-${os.arch() === 'arm64' ? 'arm64' : 'amd64'}`;
}

// Dev builds keep precedence — if the plugin ships its own bin/, never download.
function hasPluginLocalBinary() {
  const pluginRoot = process.env.CLAUDE_PLUGIN_ROOT || path.join(__dirname, '..');
  const pluginBin = path.join(pluginRoot, 'bin');
  const genericName = isWindows ? 'kratos.exe' : 'kratos';
  return (
    fs.existsSync(path.join(pluginBin, platformBinaryName())) ||
    fs.existsSync(path.join(pluginBin, genericName))
  );
}

function recordAttempt() {
  try {
    fs.mkdirSync(BIN_DIR, { recursive: true });
    fs.writeFileSync(LAST_ATTEMPT_FILE, String(Date.now()));
  } catch (e) {
    // best-effort marker — never let this throw
  }
}

function throttled() {
  try {
    const raw = fs.readFileSync(LAST_ATTEMPT_FILE, 'utf-8');
    const last = parseInt(raw, 10);
    if (!Number.isNaN(last) && Date.now() - last < THROTTLE_MS) return true;
  } catch (e) {
    // no marker yet — not throttled
  }
  return false;
}

// fs.openSync(..., 'wx') fails atomically if the lock already exists — the
// concurrency primitive. A lock older than STALE_LOCK_MS is treated as
// abandoned (crashed session) and taken over.
function acquireLock(attemptsLeft = 3) {
  try {
    fs.mkdirSync(BIN_DIR, { recursive: true });
    const fd = fs.openSync(LOCK_FILE, 'wx');
    fs.writeSync(fd, `${process.pid}\n${Date.now()}\n`);
    fs.closeSync(fd);
    return true;
  } catch (e) {
    if (e.code !== 'EEXIST' || attemptsLeft <= 0) return false;
    try {
      const stat = fs.statSync(LOCK_FILE);
      if (Date.now() - stat.mtimeMs > STALE_LOCK_MS) {
        fs.unlinkSync(LOCK_FILE);
        return acquireLock(attemptsLeft - 1);
      }
    } catch (e2) {
      // lock vanished mid-check (another process finished) — retry once
      return acquireLock(attemptsLeft - 1);
    }
    return false;
  }
}

function releaseLock() {
  try {
    fs.unlinkSync(LOCK_FILE);
  } catch (e) {
    // best-effort
  }
}

// Hand-rolled redirect following: GitHub 302s release asset URLs to
// release-assets.githubusercontent.com. Capped at MAX_REDIRECTS hops.
function httpGetFollow(url, redirectsLeft, onResponse, onError) {
  https
    .get(url, { headers: { 'User-Agent': 'kratos-ensure-binary' } }, (res) => {
      if (
        res.statusCode >= 300 &&
        res.statusCode < 400 &&
        res.headers.location
      ) {
        res.resume();
        if (redirectsLeft <= 0) {
          onError(new Error('too many redirects'));
          return;
        }
        const nextUrl = new URL(res.headers.location, url).href;
        httpGetFollow(nextUrl, redirectsLeft - 1, onResponse, onError);
        return;
      }
      if (res.statusCode !== 200) {
        res.resume();
        onError(new Error(`unexpected status ${res.statusCode} for ${url}`));
        return;
      }
      onResponse(res);
    })
    .on('error', onError);
}

function downloadToFile(url, destPath) {
  return new Promise((resolve, reject) => {
    httpGetFollow(
      url,
      MAX_REDIRECTS,
      (res) => {
        const out = fs.createWriteStream(destPath);
        res.pipe(out);
        out.on('finish', () => out.close(() => resolve()));
        out.on('error', reject);
        res.on('error', reject);
      },
      reject,
    );
  });
}

function downloadToString(url) {
  return new Promise((resolve, reject) => {
    httpGetFollow(
      url,
      MAX_REDIRECTS,
      (res) => {
        let data = '';
        res.setEncoding('utf-8');
        res.on('data', (chunk) => {
          data += chunk;
        });
        res.on('end', () => resolve(data));
        res.on('error', reject);
      },
      reject,
    );
  });
}

function sha256File(filePath) {
  const buf = fs.readFileSync(filePath);
  return crypto.createHash('sha256').update(buf).digest('hex');
}

// checksums.txt lines look like "<sha256>  <filename>" (sha256sum format).
function parseChecksum(checksumsText, filename) {
  const lines = checksumsText.split('\n');
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const parts = trimmed.split(/\s+/);
    if (parts.length >= 2 && parts[1].replace(/^\*/, '') === filename) {
      return parts[0];
    }
  }
  return null;
}

// Existing installs with a markerless binary get the marker seeded from one
// cheap `--version` exec instead of a forced re-download.
function seedMarkerFromExisting(targetPath) {
  try {
    const out = execSync(`"${targetPath}" --version`, { encoding: 'utf-8' });
    const match = out.trim().match(/(\d+\.\d+\.\d+)/);
    if (match) {
      fs.writeFileSync(VERSION_MARKER, match[1]);
      return match[1];
    }
  } catch (e) {
    // corrupt or unreadable binary — treat as no version, fall through to download
  }
  return null;
}

// Entry point. Every failure path degrades to a silent no-op — the binary is
// optional and this must never surface an error to a caller (SessionStart's
// detached spawn, or a direct `require`).
async function ensureBinary() {
  try {
    if (hasPluginLocalBinary()) return;

    const wantedVersion = pluginVersion();
    const assetName = releaseAssetName();
    const genericName = isWindows ? 'kratos.exe' : 'kratos';
    const targetPath = path.join(BIN_DIR, genericName);

    let currentVersion = null;
    if (fs.existsSync(VERSION_MARKER)) {
      currentVersion = fs.readFileSync(VERSION_MARKER, 'utf-8').trim();
    } else if (fs.existsSync(targetPath)) {
      currentVersion = seedMarkerFromExisting(targetPath);
    }

    if (currentVersion === wantedVersion) return; // already up to date

    if (throttled()) return;

    if (!acquireLock()) return; // another session is downloading, or lock busy

    try {
      const assetUrl = `${BASE_URL}/v${wantedVersion}/${assetName}`;
      const checksumsUrl = `${BASE_URL}/v${wantedVersion}/checksums.txt`;

      await downloadToFile(assetUrl, TMP_FILE);
      const checksumsText = await downloadToString(checksumsUrl);
      const expectedSha = parseChecksum(checksumsText, assetName);
      if (!expectedSha) {
        throw new Error(`checksum entry not found for ${assetName}`);
      }

      const actualSha = sha256File(TMP_FILE);
      if (actualSha !== expectedSha) {
        throw new Error('checksum mismatch');
      }

      try {
        fs.renameSync(TMP_FILE, targetPath);
      } catch (renameErr) {
        // Windows: binary currently executing in another session's hook.
        // Leave temp file for the next attempt; exit silently.
        if (renameErr.code === 'EBUSY' || renameErr.code === 'EPERM') {
          recordAttempt();
          return;
        }
        throw renameErr;
      }

      if (!isWindows) {
        fs.chmodSync(targetPath, 0o755);
      }

      fs.writeFileSync(VERSION_MARKER, wantedVersion);
    } catch (e) {
      try {
        if (fs.existsSync(TMP_FILE)) fs.unlinkSync(TMP_FILE);
      } catch (cleanupErr) {
        // best-effort
      }
      recordAttempt();
    } finally {
      releaseLock();
    }
  } catch (outerErr) {
    // Absolute last resort — never throw, never break SessionStart.
  }
}

module.exports = { ensureBinary };

if (require.main === module) {
  ensureBinary().catch(() => {});
}
