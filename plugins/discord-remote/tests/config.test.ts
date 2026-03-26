/**
 * Unit tests for src/config.ts
 *
 * Uses bun:test. Tests loadConfig() with filesystem isolation using temp directories.
 * Each test writes its own config file or leaves it absent.
 */

import { describe, it, expect, beforeEach, afterEach } from 'bun:test'
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

// We need to import loadConfig with a custom STATE_DIR.
// Since config.ts reads STATE_DIR at module-load time (via process.env.DISCORD_STATE_DIR),
// we test by spawning isolated Bun processes OR by importing the module directly with
// the env variable set before import. The cleanest approach for unit tests is to
// use dynamic import with env override per test.

// Helper: write a config file and return the temp dir
function setupConfigDir(content: string | null): { stateDir: string; configPath: string } {
  const stateDir = mkdtempSync(join(tmpdir(), 'discord-config-test-'))
  const configPath = join(stateDir, 'remote-config.json')
  if (content !== null) {
    writeFileSync(configPath, content, { mode: 0o600 })
  }
  return { stateDir, configPath }
}

// Helper: import loadConfig with a specific DISCORD_STATE_DIR
// We use a fresh dynamic import by busting the cache with a unique URL parameter
async function importLoadConfig(stateDir: string): Promise<{
  loadConfig: () => import('../src/types.js').RemoteConfig
}> {
  // Set env before dynamic import (Bun uses process.env at module eval time)
  const originalStateDir = process.env.DISCORD_STATE_DIR
  process.env.DISCORD_STATE_DIR = stateDir
  try {
    // Use a cache-busting query string to force re-evaluation
    const mod = await import(`../src/config.js?stateDir=${encodeURIComponent(stateDir)}`)
    return mod
  } finally {
    if (originalStateDir === undefined) {
      delete process.env.DISCORD_STATE_DIR
    } else {
      process.env.DISCORD_STATE_DIR = originalStateDir
    }
  }
}

describe('loadConfig', () => {
  let stateDir: string
  let cleanup: () => void

  afterEach(() => {
    cleanup?.()
  })

  // -------------------------------------------------------------------------
  // TC-014: Config loader applies defaults for missing fields
  // -------------------------------------------------------------------------

  it('TC-014: applies defaults for missing optional fields', async () => {
    const setup = setupConfigDir(JSON.stringify({ sidecar: { port: 19275 } }))
    stateDir = setup.stateDir
    cleanup = () => rmSync(stateDir, { recursive: true, force: true })

    const { loadConfig } = await importLoadConfig(stateDir)
    const config = loadConfig()

    expect(config.timeout.approval_ms).toBe(60000)
    expect(config.timeout.question_ms).toBe(120000)
    expect(config.defaults.permission_fallback).toBe('ask')
    expect(config.defaults.question_fallback).toBe('skip')
    expect(config.deny_patterns).toEqual([])
    expect(config.reactions.approve).toBeTruthy() // checkmark emoji
  })

  // -------------------------------------------------------------------------
  // TC-015: Config loader handles malformed JSON with fallback to defaults
  // -------------------------------------------------------------------------

  it('TC-015: handles malformed JSON by returning defaults', async () => {
    const setup = setupConfigDir('{bad json')
    stateDir = setup.stateDir
    cleanup = () => rmSync(stateDir, { recursive: true, force: true })

    const { loadConfig } = await importLoadConfig(stateDir)
    // Should not throw — returns defaults
    const config = loadConfig()

    expect(config.timeout.approval_ms).toBe(60000)
    expect(config.sidecar.port).toBe(19275)
  })

  // -------------------------------------------------------------------------
  // TC-016: Config loader handles missing config file with all defaults
  // -------------------------------------------------------------------------

  it('TC-016: returns full defaults when config file does not exist', async () => {
    const setup = setupConfigDir(null) // no file written
    stateDir = setup.stateDir
    cleanup = () => rmSync(stateDir, { recursive: true, force: true })

    const { loadConfig } = await importLoadConfig(stateDir)
    const config = loadConfig()

    expect(config.sidecar.port).toBe(19275)
    expect(config.timeout.approval_ms).toBe(60000)
    expect(config.defaults.permission_fallback).toBe('ask')
  })

  // -------------------------------------------------------------------------
  // Additional: host is always overridden to 127.0.0.1
  // -------------------------------------------------------------------------

  it('overrides sidecar.host to 127.0.0.1 regardless of config value', async () => {
    const setup = setupConfigDir(JSON.stringify({ sidecar: { port: 19275, host: '0.0.0.0' } }))
    stateDir = setup.stateDir
    cleanup = () => rmSync(stateDir, { recursive: true, force: true })

    const { loadConfig } = await importLoadConfig(stateDir)
    const config = loadConfig()

    expect(config.sidecar.host).toBe('127.0.0.1')
  })

  // -------------------------------------------------------------------------
  // Additional: deny_patterns loaded from config
  // -------------------------------------------------------------------------

  it('loads deny_patterns from config file', async () => {
    const setup = setupConfigDir(JSON.stringify({ deny_patterns: ['rm -rf /', 'DROP TABLE'] }))
    stateDir = setup.stateDir
    cleanup = () => rmSync(stateDir, { recursive: true, force: true })

    const { loadConfig } = await importLoadConfig(stateDir)
    const config = loadConfig()

    expect(config.deny_patterns).toEqual(['rm -rf /', 'DROP TABLE'])
  })

  // -------------------------------------------------------------------------
  // Additional: partial timeout config merges with defaults
  // -------------------------------------------------------------------------

  it('merges partial timeout config with defaults', async () => {
    const setup = setupConfigDir(JSON.stringify({ timeout: { approval_ms: 30000 } }))
    stateDir = setup.stateDir
    cleanup = () => rmSync(stateDir, { recursive: true, force: true })

    const { loadConfig } = await importLoadConfig(stateDir)
    const config = loadConfig()

    expect(config.timeout.approval_ms).toBe(30000)
    expect(config.timeout.question_ms).toBe(120000) // default
  })
})
