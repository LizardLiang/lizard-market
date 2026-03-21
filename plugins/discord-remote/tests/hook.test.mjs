/**
 * Unit tests for hooks/hook.mjs
 *
 * Uses node:test (built-in, no npm dependencies).
 * Tests are isolated: each test starts a mock HTTP server, writes a port file,
 * spawns hook.mjs as a subprocess, and verifies stdout/stderr/exit code.
 */

import { test } from 'node:test'
import * as assert from 'node:assert/strict'
import * as http from 'node:http'
import * as fs from 'node:fs'
import * as path from 'node:path'
import * as os from 'node:os'
import { spawn } from 'node:child_process'

const HOOK_PATH = path.resolve(import.meta.dirname, '../hooks/hook.mjs')

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

/**
 * Start a mock HTTP server that responds to all requests with a fixed JSON body.
 * Returns { server, port, requests } where requests accumulates recorded calls.
 */
function startMockServer(responseBody, options = {}) {
  const requests = []
  const delay = options.delay ?? 0

  const server = http.createServer((req, res) => {
    let body = ''
    req.on('data', chunk => { body += chunk })
    req.on('end', () => {
      requests.push({ method: req.method, url: req.url, body: body ? JSON.parse(body) : null })
      if (options.hang) {
        // Never respond — simulates a hanging sidecar
        return
      }
      const respond = () => {
        const json = JSON.stringify(responseBody)
        res.writeHead(200, { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(json) })
        res.end(json)
      }
      if (delay > 0) {
        setTimeout(respond, delay)
      } else {
        respond()
      }
    })
  })

  return new Promise((resolve, reject) => {
    server.listen(0, '127.0.0.1', () => {
      const { port } = server.address()
      resolve({ server, port, requests })
    })
    server.on('error', reject)
  })
}

/**
 * Write a port file to a temp directory and return { stateDir, portFile, cleanup }.
 */
function writeTempPortFile(port) {
  const stateDir = fs.mkdtempSync(path.join(os.tmpdir(), 'discord-hook-test-'))
  const portFile = path.join(stateDir, 'sidecar.port')
  if (port !== null) {
    fs.writeFileSync(portFile, String(port) + '\n')
  }
  const cleanup = () => {
    try { fs.rmSync(stateDir, { recursive: true, force: true }) } catch { /* ignore */ }
  }
  return { stateDir, portFile, cleanup }
}

/**
 * Run hook.mjs as a subprocess with stdin input.
 * Returns { stdout, stderr, code } after the process exits.
 */
function runHook(stdinData, stateDir, timeoutMs = 10000) {
  return new Promise((resolve) => {
    const child = spawn('node', [HOOK_PATH], {
      env: { ...process.env, DISCORD_STATE_DIR: stateDir },
      timeout: timeoutMs,
    })

    let stdout = ''
    let stderr = ''
    child.stdout.on('data', chunk => { stdout += chunk })
    child.stderr.on('data', chunk => { stderr += chunk })

    child.on('close', code => {
      resolve({ stdout: stdout.trim(), stderr, code })
    })

    if (stdinData !== null) {
      child.stdin.write(stdinData)
    }
    child.stdin.end()
  })
}

// ---------------------------------------------------------------------------
// TC-001: Hook routes PermissionRequest to /approve
// ---------------------------------------------------------------------------

test('TC-001: Hook routes PermissionRequest to /approve', async () => {
  const approveResponse = {
    hookSpecificOutput: {
      hookEventName: 'PermissionRequest',
      permissionDecision: 'allow',
      permissionDecisionReason: 'Approved via Discord by testuser',
    },
  }
  const { server, port, requests } = await startMockServer(approveResponse)
  const { stateDir, cleanup } = writeTempPortFile(port)

  try {
    const input = JSON.stringify({
      hook_event_name: 'PermissionRequest',
      tool_name: 'Write',
      tool_input: { file_path: '/src/app.ts', content: 'x' },
    })

    const { stdout, code } = await runHook(input, stateDir)

    assert.equal(code, 0, 'exit code should be 0')
    const output = JSON.parse(stdout)
    assert.equal(output.hookSpecificOutput.permissionDecision, 'allow')
    assert.equal(requests.length, 1, 'exactly one request to sidecar')
    assert.equal(requests[0].url, '/approve', 'routed to /approve')
    assert.equal(requests[0].body.tool_name, 'Write', 'tool_name forwarded')
  } finally {
    server.close()
    cleanup()
  }
})

// ---------------------------------------------------------------------------
// TC-002: Hook routes PreToolUse to /pretool
// ---------------------------------------------------------------------------

test('TC-002: Hook routes PreToolUse to /pretool', async () => {
  const { server, port, requests } = await startMockServer({})
  const { stateDir, cleanup } = writeTempPortFile(port)

  try {
    const input = JSON.stringify({
      hook_event_name: 'PreToolUse',
      tool_name: 'Bash',
      tool_input: { command: 'ls' },
    })

    const { stdout, code } = await runHook(input, stateDir)

    assert.equal(code, 0)
    assert.deepEqual(JSON.parse(stdout), {})
    assert.equal(requests[0].url, '/pretool', 'routed to /pretool')
  } finally {
    server.close()
    cleanup()
  }
})

// ---------------------------------------------------------------------------
// TC-003: Hook routes AskUserQuestion to /question
// ---------------------------------------------------------------------------

test('TC-003: Hook routes AskUserQuestion to /question', async () => {
  const questionResponse = {
    hookSpecificOutput: {
      hookEventName: 'AskUserQuestion',
      answer: 'A',
    },
  }
  const { server, port, requests } = await startMockServer(questionResponse)
  const { stateDir, cleanup } = writeTempPortFile(port)

  try {
    const input = JSON.stringify({
      hook_event_name: 'AskUserQuestion',
      tool_name: 'AskUserQuestion',
      tool_input: { question: 'Which?', options: ['A', 'B'] },
    })

    const { stdout, code } = await runHook(input, stateDir)

    assert.equal(code, 0)
    const output = JSON.parse(stdout)
    assert.equal(output.hookSpecificOutput.answer, 'A')
    assert.equal(requests[0].url, '/question', 'routed to /question')
  } finally {
    server.close()
    cleanup()
  }
})

// ---------------------------------------------------------------------------
// TC-004: Hook falls back when sidecar is unreachable (ECONNREFUSED)
// ---------------------------------------------------------------------------

test('TC-004: Hook falls back gracefully when sidecar is unreachable', async () => {
  // Use a port that nothing is listening on
  const { stateDir, cleanup } = writeTempPortFile(19999)

  try {
    const input = JSON.stringify({
      hook_event_name: 'PermissionRequest',
      tool_name: 'Write',
      tool_input: {},
    })

    const { stdout, code } = await runHook(input, stateDir)

    assert.equal(code, 0, 'exit code 0 even on unreachable sidecar')
    const output = JSON.parse(stdout)
    assert.equal(output.hookSpecificOutput.hookEventName, 'PermissionRequest')
    assert.equal(output.hookSpecificOutput.permissionDecision, 'ask', 'falls back to ask')
  } finally {
    cleanup()
  }
})

// ---------------------------------------------------------------------------
// TC-005: Hook falls back when sidecar.port file is missing
// ---------------------------------------------------------------------------

test('TC-005: Hook falls back when sidecar.port file is missing', async () => {
  // Write no port file (pass null)
  const { stateDir, cleanup } = writeTempPortFile(null)

  try {
    const input = JSON.stringify({
      hook_event_name: 'PermissionRequest',
      tool_name: 'Write',
      tool_input: {},
    })

    const { stdout, code } = await runHook(input, stateDir)

    assert.equal(code, 0)
    const output = JSON.parse(stdout)
    // Should be a fallback response
    assert.ok(output.hookSpecificOutput || output, 'produces some output')
  } finally {
    cleanup()
  }
})

// ---------------------------------------------------------------------------
// TC-006: Hook handles malformed stdin JSON
// ---------------------------------------------------------------------------

test('TC-006: Hook handles malformed stdin JSON', async () => {
  const { stateDir, cleanup } = writeTempPortFile(19999)

  try {
    const { stdout, stderr, code } = await runHook('{invalid json', stateDir)

    assert.equal(code, 0, 'exit code 0 even with bad JSON')
    // Should produce some valid JSON output
    const output = JSON.parse(stdout)
    assert.ok(typeof output === 'object', 'output is a JSON object')
    assert.ok(stderr.includes('malformed') || stderr.includes('JSON') || stdout.includes('{}') || Object.keys(output).length === 0, 'logs error or returns empty')
  } finally {
    cleanup()
  }
})

// ---------------------------------------------------------------------------
// TC-008: Hook handles empty stdin (EOF without data)
// ---------------------------------------------------------------------------

test('TC-008: Hook handles empty stdin', async () => {
  const { stateDir, cleanup } = writeTempPortFile(19999)

  try {
    // Pass empty string as stdin
    const { stdout, code } = await runHook('', stateDir)

    assert.equal(code, 0)
    // Should produce valid JSON output (fallback or empty)
    const output = JSON.parse(stdout)
    assert.ok(typeof output === 'object', 'output is a JSON object')
  } finally {
    cleanup()
  }
})

// ---------------------------------------------------------------------------
// TC-007: Hook does NOT prematurely timeout — waits for sidecar response.
// The hook has no HTTP-level timeout (removed to fix C-001). It waits as long
// as the sidecar needs. hooks.json timeout is the outer bound.
// We verify by starting a mock server that responds after 2s delay.
// ---------------------------------------------------------------------------

test('TC-007: Hook waits for delayed sidecar response (no premature timeout)', { timeout: 10000 }, async () => {
  // Start a server that responds after 2 seconds
  const approveResponse = {
    hookSpecificOutput: {
      hookEventName: 'PermissionRequest',
      permissionDecision: 'allow',
      permissionDecisionReason: 'Delayed response',
    },
  }
  const { server, port } = await startMockServer(approveResponse, { delay: 2000 })
  const { stateDir, cleanup } = writeTempPortFile(port)

  try {
    const input = JSON.stringify({
      hook_event_name: 'PermissionRequest',
      tool_name: 'Write',
      tool_input: {},
    })

    const start = Date.now()
    const { stdout, code } = await runHook(input, stateDir, 8000)
    const elapsed = Date.now() - start

    assert.equal(code, 0)
    // Should wait ~2s for the response, NOT time out at 5s
    assert.ok(elapsed >= 1500, `elapsed ${elapsed}ms should be >= 1500ms (waited for delay)`)
    assert.ok(elapsed < 5000, `elapsed ${elapsed}ms should be < 5000ms (no premature timeout)`)

    const output = JSON.parse(stdout)
    assert.equal(output.hookSpecificOutput.permissionDecision, 'allow', 'received delayed approval')
  } finally {
    server.close()
    cleanup()
  }
})
