/**
 * Configuration loading for discord-remote plugin.
 *
 * Reads remote-config.json from STATE_DIR. All fields have defaults —
 * a missing config file is not an error. Config is loaded once at startup
 * and reloaded on SIGHUP (Unix only; on Windows it reloads per-request).
 */

import { readFileSync, writeFileSync, mkdirSync } from 'fs'
import { randomBytes } from 'crypto'
import { join } from 'path'
import { homedir } from 'os'
import type { RemoteConfig } from './types.js'

export const STATE_DIR =
  process.env.DISCORD_STATE_DIR ?? join(homedir(), '.claude', 'channels', 'discord')

const CONFIG_FILE = join(STATE_DIR, 'remote-config.json')

const SECRET_FILE = join(STATE_DIR, 'sidecar.secret')

/** Generate or read the shared secret for sidecar authentication. */
function getOrCreateSecret(): string {
  try {
    return readFileSync(SECRET_FILE, 'utf8').trim()
  } catch {
    const secret = randomBytes(32).toString('hex')
    try {
      mkdirSync(STATE_DIR, { recursive: true, mode: 0o700 })
      writeFileSync(SECRET_FILE, secret + '\n', { mode: 0o600 })
    } catch { /* proceed with in-memory secret */ }
    return secret
  }
}

const DEFAULT_CONFIG: RemoteConfig = {
  sidecar: {
    port: 19275,
    host: '127.0.0.1',
    secret: '', // populated at runtime
  },
  timeout: {
    approval_ms: 60000,
    question_ms: 120000,
  },
  defaults: {
    permission_fallback: 'ask',
    question_fallback: 'skip',
  },
  deny_patterns: [],
  reactions: {
    approve: '\u2705',  // checkmark
    deny: '\u274c',     // X
    always: '\ud83d\udd12', // lock
  },
}

function mergeConfig(partial: Partial<RemoteConfig>): RemoteConfig {
  return {
    sidecar: {
      ...DEFAULT_CONFIG.sidecar,
      ...partial.sidecar,
      // Always override host to localhost for security regardless of config
      host: '127.0.0.1',
      // Secret is generated once and persisted — never overridden by config file
      secret: getOrCreateSecret(),
    },
    timeout: {
      ...DEFAULT_CONFIG.timeout,
      ...partial.timeout,
    },
    defaults: {
      ...DEFAULT_CONFIG.defaults,
      ...partial.defaults,
    },
    deny_patterns: partial.deny_patterns ?? DEFAULT_CONFIG.deny_patterns,
    reactions: {
      ...DEFAULT_CONFIG.reactions,
      ...partial.reactions,
    },
  }
}

export function loadConfig(): RemoteConfig {
  try {
    const raw = readFileSync(CONFIG_FILE, 'utf8')
    const parsed = JSON.parse(raw) as Partial<RemoteConfig>
    return mergeConfig(parsed)
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
      // Config file doesn't exist yet — use defaults, write them for reference
      try {
        mkdirSync(STATE_DIR, { recursive: true, mode: 0o700 })
        writeFileSync(CONFIG_FILE, JSON.stringify(DEFAULT_CONFIG, null, 2) + '\n', { mode: 0o600 })
      } catch {
        // If we can't write defaults, just proceed with in-memory defaults
      }
      return { ...DEFAULT_CONFIG }
    }
    // Malformed JSON — log and use defaults
    process.stderr.write(`discord-remote: remote-config.json is malformed, using defaults: ${err}\n`)
    return { ...DEFAULT_CONFIG }
  }
}

// ---------------------------------------------------------------------------
// Live config holder with SIGHUP reload support
// ---------------------------------------------------------------------------

let _config: RemoteConfig = loadConfig()

export function getConfig(): RemoteConfig {
  return _config
}

export function reloadConfig(): void {
  _config = loadConfig()
  process.stderr.write('discord-remote: config reloaded\n')
}

// On Unix, reload config on SIGHUP. On Windows, SIGHUP is not available.
// Windows users get config reload on each sidecar request (cheap enough at
// the expected request volume — see tech-spec section 10 Open Questions).
if (process.platform !== 'win32') {
  process.on('SIGHUP', reloadConfig)
}