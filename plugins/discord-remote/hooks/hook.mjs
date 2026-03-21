#!/usr/bin/env node
/**
 * Universal hook script for discord-remote plugin.
 *
 * Handles all three Claude Code hook event types:
 *   - PermissionRequest  -> POST /approve
 *   - PreToolUse         -> POST /pretool
 *   - AskUserQuestion    -> POST /question
 *
 * Uses only Node.js built-in modules (http, fs, path, os) — no npm
 * dependencies. This ensures cross-platform compatibility on any system
 * where Node.js is installed (Windows, macOS, Linux).
 *
 * Port discovery: reads STATE_DIR/sidecar.port to find where the sidecar
 * is listening. Falls back gracefully if the sidecar is unreachable.
 *
 * Fallback behavior:
 *   - Sidecar unreachable (ECONNREFUSED) -> write fallback JSON to stdout
 *   - HTTP timeout (5s) -> write fallback JSON to stdout
 *   - Malformed stdin -> log to stderr, write fallback JSON to stdout
 *   - Unknown hook_event_name -> write empty JSON to stdout (pass-through)
 */

import * as http from 'http'
import * as fs from 'fs'
import * as path from 'path'
import * as os from 'os'

// ---------------------------------------------------------------------------
// State directory and port file location
// ---------------------------------------------------------------------------

const STATE_DIR = process.env.DISCORD_STATE_DIR
  ?? path.join(os.homedir(), '.claude', 'channels', 'discord')

const PORT_FILE = path.join(STATE_DIR, 'sidecar.port')
const SECRET_FILE = path.join(STATE_DIR, 'sidecar.secret')

/**
 * Read the shared secret for sidecar authentication.
 * Returns empty string if the file doesn't exist (sidecar will skip auth check).
 */
function readSecret() {
  try {
    return fs.readFileSync(SECRET_FILE, 'utf8').trim()
  } catch {
    return ''
  }
}

// No HTTP-level timeout constant needed. The sidecar manages its own
// approval/question timeouts (configurable, default 60s/120s), and hooks.json
// defines the outer bound that kills the hook process (120s/180s).
// Connection failures (ECONNREFUSED) resolve immediately via the error handler.

// ---------------------------------------------------------------------------
// Fallback responses for when sidecar is unreachable
// ---------------------------------------------------------------------------

/**
 * Build a fallback response for PermissionRequest when sidecar is down.
 * Falls back to "ask" — Claude Code will prompt the terminal as normal.
 */
function permissionFallback() {
  return {
    hookSpecificOutput: {
      hookEventName: 'PermissionRequest',
      decision: {
        behavior: 'ask',
        reason: 'discord-remote sidecar unreachable, falling back to terminal',
      },
    },
  }
}

/**
 * Build a fallback response for PreToolUse when sidecar is down.
 * Falls back to empty object (pass-through — no block decision).
 */
function preToolFallback() {
  return {}
}

/**
 * Build a fallback response for AskUserQuestion when sidecar is down.
 * Falls back to empty answer — Claude Code will handle the empty response.
 */
function questionFallback() {
  return {
    hookSpecificOutput: {
      hookEventName: 'AskUserQuestion',
      answer: '',
    },
  }
}

// ---------------------------------------------------------------------------
// Port discovery
// ---------------------------------------------------------------------------

/**
 * Read the sidecar port from the port file.
 * Returns null if the file doesn't exist or can't be read.
 */
function readSidecarPort() {
  try {
    const content = fs.readFileSync(PORT_FILE, 'utf8').trim()
    const port = parseInt(content, 10)
    if (isNaN(port) || port < 1 || port > 65535) return null
    return port
  } catch {
    return null
  }
}

// ---------------------------------------------------------------------------
// HTTP POST to sidecar
// ---------------------------------------------------------------------------

/**
 * POST JSON data to the sidecar and return the parsed response.
 * Returns null on connection error or timeout.
 */
function postToSidecar(port, endpoint, body) {
  return new Promise((resolve) => {
    const data = JSON.stringify(body)
    const secret = readSecret()

    const headers = {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(data),
    }
    if (secret) {
      headers['Authorization'] = `Bearer ${secret}`
    }

    const options = {
      hostname: '127.0.0.1',
      port,
      path: endpoint,
      method: 'POST',
      headers,
    }

    let resolved = false

    const req = http.request(options, (res) => {
      let responseData = ''
      res.on('data', (chunk) => { responseData += chunk })
      res.on('end', () => {
        if (resolved) return
        resolved = true
        try {
          resolve(JSON.parse(responseData))
        } catch {
          process.stderr.write(`discord-remote hook: invalid JSON response from sidecar\n`)
          resolve(null)
        }
      })
    })

    req.on('error', (err) => {
      if (resolved) return
      resolved = true
      // ECONNREFUSED means sidecar isn't running — fall back silently
      if (err.code !== 'ECONNREFUSED') {
        process.stderr.write(`discord-remote hook: sidecar request error: ${err.message}\n`)
      }
      resolve(null)
    })

    // No HTTP-level timeout. The sidecar long-polls for 60-120s waiting for
    // the user to react on Discord — any HTTP timeout shorter than that kills
    // the request prematurely. The hooks.json timeout (120s/180s) is the outer
    // bound that kills the hook process if the sidecar never responds.
    // Connection failures (ECONNREFUSED) are caught by the 'error' handler above.

    req.write(data)
    req.end()
  })
}

// ---------------------------------------------------------------------------
// Main hook handling
// ---------------------------------------------------------------------------

/**
 * Route the hook event to the correct sidecar endpoint.
 * Returns the endpoint path and fallback response for the event type.
 */
function routeEvent(hookEventName) {
  switch (hookEventName) {
    case 'PermissionRequest':
      return { endpoint: '/approve', fallback: permissionFallback() }
    case 'PreToolUse':
      return { endpoint: '/pretool', fallback: preToolFallback() }
    case 'AskUserQuestion':
      return { endpoint: '/question', fallback: questionFallback() }
    default:
      return null
  }
}

async function main() {
  // Read stdin (Claude Code writes JSON hook data here)
  let stdinData = ''
  try {
    for await (const chunk of process.stdin) {
      stdinData += chunk
    }
  } catch (err) {
    process.stderr.write(`discord-remote hook: stdin read error: ${err}\n`)
    process.stdout.write(JSON.stringify(permissionFallback()) + '\n')
    process.exit(0)
  }

  // Parse stdin JSON
  let hookData
  try {
    hookData = JSON.parse(stdinData)
  } catch {
    process.stderr.write(`discord-remote hook: malformed stdin JSON\n`)
    // Unknown event type — write pass-through
    process.stdout.write(JSON.stringify({}) + '\n')
    process.exit(0)
  }

  const hookEventName = hookData.hook_event_name
  if (!hookEventName) {
    process.stderr.write(`discord-remote hook: missing hook_event_name in stdin\n`)
    process.stdout.write(JSON.stringify({}) + '\n')
    process.exit(0)
  }

  // Route to correct endpoint
  const route = routeEvent(hookEventName)
  if (!route) {
    // Unknown hook type — pass through (empty object = no opinion)
    process.stdout.write(JSON.stringify({}) + '\n')
    process.exit(0)
  }

  // Discover sidecar port
  const port = readSidecarPort()
  if (!port) {
    process.stderr.write(`discord-remote hook: cannot read sidecar port from ${PORT_FILE}\n`)
    process.stdout.write(JSON.stringify(route.fallback) + '\n')
    process.exit(0)
  }

  // POST to sidecar (long-poll — waits until user responds or timeout)
  const response = await postToSidecar(port, route.endpoint, hookData)

  if (response === null) {
    // Sidecar unreachable or errored — use fallback
    process.stdout.write(JSON.stringify(route.fallback) + '\n')
  } else {
    process.stdout.write(JSON.stringify(response) + '\n')
  }

  process.exit(0)
}

main().catch((err) => {
  process.stderr.write(`discord-remote hook: unhandled error: ${err}\n`)
  process.stdout.write(JSON.stringify({}) + '\n')
  process.exit(0)
})