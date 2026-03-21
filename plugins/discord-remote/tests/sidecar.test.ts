/**
 * API tests for src/sidecar.ts HTTP endpoints.
 *
 * Uses bun:test. Starts a real HTTP sidecar on a random port with mocked
 * Discord interactions. Tests each endpoint with real HTTP requests.
 */

import { describe, it, expect, beforeEach, afterEach, mock } from 'bun:test'
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { RemoteConfig } from '../src/types.js'

// ---------------------------------------------------------------------------
// Test config
// ---------------------------------------------------------------------------

const BASE_CONFIG: RemoteConfig = {
  sidecar: { port: 0, host: '127.0.0.1', secret: '' }, // port 0, no secret for tests
  timeout: { approval_ms: 200, question_ms: 200 },
  defaults: { permission_fallback: 'ask', question_fallback: 'skip' },
  deny_patterns: [],
  reactions: {
    approve: '\u2705',
    deny: '\u274c',
    always: '\ud83d\udd12',
  },
}

// ---------------------------------------------------------------------------
// Mock Discord approver module
// ---------------------------------------------------------------------------

// We mock the discord-approver module to avoid real Discord calls.
// The mock is set up before each test to control what the approver returns.

type ApproverMock = {
  sendApprovalRequest: ReturnType<typeof mock>
  sendQuestionMultiChoice: ReturnType<typeof mock>
  sendQuestionFreeText: ReturnType<typeof mock>
}

// ---------------------------------------------------------------------------
// Filesystem setup
// ---------------------------------------------------------------------------

const PAIRED_USER_ID = 'user123'
let stateDir: string

function setupStateDir(): void {
  stateDir = mkdtempSync(join(tmpdir(), 'discord-sidecar-test-'))
  writeFileSync(join(stateDir, 'access.json'), JSON.stringify({ allowFrom: [PAIRED_USER_ID] }))
  process.env.DISCORD_STATE_DIR = stateDir
}

function teardownStateDir(): void {
  try { rmSync(stateDir, { recursive: true, force: true }) } catch { /* ignore */ }
  delete process.env.DISCORD_STATE_DIR
}

// ---------------------------------------------------------------------------
// HTTP helper
// ---------------------------------------------------------------------------

async function httpPost(port: number, path: string, body: unknown): Promise<{ status: number; body: unknown }> {
  const response = await fetch(`http://127.0.0.1:${port}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const json = await response.json()
  return { status: response.status, body: json }
}

async function httpGet(port: number, path: string): Promise<{ status: number; body: unknown }> {
  const response = await fetch(`http://127.0.0.1:${port}${path}`)
  const json = await response.json()
  return { status: response.status, body: json }
}

// ---------------------------------------------------------------------------
// Sidecar factory with mock Discord client
// ---------------------------------------------------------------------------

async function startTestSidecar(config: RemoteConfig, mockApprover: {
  sendApprovalRequest?: (client: unknown, config: RemoteConfig, toolName: string, toolInput: Record<string, unknown>, permissionSuggestions?: Record<string, unknown>) => Promise<unknown>
  sendQuestionMultiChoice?: (client: unknown, config: RemoteConfig, question: string, options: string[]) => Promise<unknown>
  sendQuestionFreeText?: (client: unknown, config: RemoteConfig, question: string) => Promise<unknown>
} = {}) {
  // We need to inject mocks into the sidecar. Since the sidecar imports discord-approver
  // at module load time, we test sidecar by calling createSidecar with a mock client.
  // The discord-approver functions are imported inside route handlers, so we use
  // a test-specific sidecar wrapper that intercepts the Discord calls.

  // Instead of full module mocking, we create an integration-style test that
  // uses the real sidecar but with a mock Discord client (which never sends real messages).
  // The sendApprovalRequest etc. in discord-approver will throw because there's no
  // access.json with a real paired user — we handle this by writing access.json.

  // For true unit isolation, we test the sidecar's HTTP layer by importing createSidecar
  // and providing a mock client. The discord-approver functions wrap the client calls.

  const { createSidecar } = await import(`../src/sidecar.js?t=${Date.now()}`)

  // Create a mock Discord client
  const mockClient = {
    isReady: () => true,
    users: {
      fetch: async (id: string) => ({
        id,
        username: 'testuser',
        createDM: async () => {
          let channelCollectCb: ((msg: unknown) => void) | null = null
          let channelEndCb: ((c: { size: number }) => void) | null = null

          const dmChannel = {
          createMessageCollector: (opts: { time: number }) => {
            setTimeout(() => {
              if (channelEndCb) channelEndCb({ size: 0 })
            }, opts.time)

            return {
              on: (event: string, cb: unknown) => {
                if (event === 'collect') channelCollectCb = cb as typeof channelCollectCb
                if (event === 'end') channelEndCb = cb as typeof channelEndCb
              },
            }
          },
          send: async (text: string) => {
            const reactCalls: string[] = []
            let reactionResolver: ((collection: Map<string, unknown>) => void) | null = null

            const message = {
              react: async (emoji: string) => { reactCalls.push(emoji) },
              awaitReactions: (opts: { time: number }) => {
                return new Promise<Map<string, unknown>>((resolve) => {
                  reactionResolver = resolve
                  // Auto-resolve after timeout or via mock
                  const mockResolve = mockApprover.sendApprovalRequest
                  if (mockResolve) {
                    // The mock will handle resolution
                  } else {
                    setTimeout(() => resolve(new Map()), opts.time)
                  }
                })
              },
              channel: {
                createMessageCollector: (opts: { time: number }) => {
                  let collectCb: ((msg: unknown) => void) | null = null
                  let endCb: ((c: { size: number }) => void) | null = null

                  setTimeout(() => {
                    if (endCb) endCb({ size: 0 })
                  }, opts.time)

                  return {
                    on: (event: string, cb: unknown) => {
                      if (event === 'collect') collectCb = cb as typeof collectCb
                      if (event === 'end') endCb = cb as typeof endCb
                    },
                  }
                },
              },
              _reactCalls: reactCalls,
              _resolveReaction: (emoji: string, userId: string, username: string) => {
                if (reactionResolver) {
                  const collection = new Map()
                  collection.set(emoji, {
                    emoji: { name: emoji },
                    users: { cache: { find: (fn: (u: { id: string }) => boolean) => {
                      const u = { id: userId, username }
                      return fn(u) ? u : undefined
                    } } },
                  })
                  reactionResolver(collection)
                  reactionResolver = null
                }
              },
            }
            message.channel = dmChannel
            return message
          },
          }
          return dmChannel
        },
      }),
    },
  }

  const sidecar = createSidecar(mockClient, config)
  await sidecar.start()

  return { sidecar, port: sidecar.port(), client: mockClient }
}

// ---------------------------------------------------------------------------
// TC-030: GET /health responds within 100ms
// ---------------------------------------------------------------------------

describe('GET /health', () => {
  let sidecar: Awaited<ReturnType<typeof startTestSidecar>>['sidecar']
  let port: number

  beforeEach(async () => {
    setupStateDir()
    const result = await startTestSidecar(BASE_CONFIG)
    sidecar = result.sidecar
    port = result.port
  })

  afterEach(async () => {
    await sidecar.stop()
    teardownStateDir()
  })

  it('TC-030: responds with 200 OK within 100ms', async () => {
    const start = Date.now()
    const { status, body } = await httpGet(port, '/health')
    const elapsed = Date.now() - start

    expect(status).toBe(200)
    expect(elapsed).toBeLessThan(100)

    const b = body as Record<string, unknown>
    expect(b.status).toBe('ok')
    expect(typeof b.uptime_ms).toBe('number')
    expect(b.pending_requests).toBe(0)
  })

  it('health reports gateway status from client.isReady()', async () => {
    const { body } = await httpGet(port, '/health')
    const b = body as Record<string, unknown>
    expect(b.gateway).toBe('connected')
  })
})

// ---------------------------------------------------------------------------
// TC-024/TC-025: POST /pretool
// ---------------------------------------------------------------------------

describe('POST /pretool', () => {
  let sidecar: Awaited<ReturnType<typeof startTestSidecar>>['sidecar']
  let port: number

  beforeEach(async () => {
    setupStateDir()
    const config: RemoteConfig = { ...BASE_CONFIG, deny_patterns: ['rm -rf /'] }
    const result = await startTestSidecar(config)
    sidecar = result.sidecar
    port = result.port
  })

  afterEach(async () => {
    await sidecar.stop()
    teardownStateDir()
  })

  it('TC-024: blocks tool matching deny pattern', async () => {
    const { status, body } = await httpPost(port, '/pretool', {
      hook_event_name: 'PreToolUse',
      tool_name: 'Bash',
      tool_input: { command: 'rm -rf /tmp/data' },
    })

    expect(status).toBe(200)
    const b = body as Record<string, unknown>
    const out = b.hookSpecificOutput as Record<string, unknown>
    expect(out.permissionDecision).toBe('block')
    expect(String(out.reason)).toContain('rm -rf /')
  })

  it('TC-025: passes through non-matching tool with empty object', async () => {
    const { status, body } = await httpPost(port, '/pretool', {
      hook_event_name: 'PreToolUse',
      tool_name: 'Write',
      tool_input: { file_path: '/src/app.ts', content: 'x' },
    })

    expect(status).toBe(200)
    expect(body).toEqual({})
  })

  it('TC-026: returns 400 for missing tool_name', async () => {
    const { status } = await httpPost(port, '/pretool', {
      hook_event_name: 'PreToolUse',
      tool_input: { command: 'ls' },
    })
    expect(status).toBe(400)
  })
})

// ---------------------------------------------------------------------------
// TC-020/TC-021/TC-022/TC-023: POST /approve
// ---------------------------------------------------------------------------

describe('POST /approve', () => {
  let sidecar: Awaited<ReturnType<typeof startTestSidecar>>['sidecar']
  let port: number

  beforeEach(async () => {
    setupStateDir()
    const result = await startTestSidecar(BASE_CONFIG)
    sidecar = result.sidecar
    port = result.port
  })

  afterEach(async () => {
    await sidecar.stop()
    teardownStateDir()
  })

  it('TC-023: returns "ask" fallback when approval times out (short timeout)', async () => {
    const shortConfig: RemoteConfig = { ...BASE_CONFIG, timeout: { approval_ms: 100, question_ms: 100 } }
    const { sidecar: s, port: p } = await startTestSidecar(shortConfig)
    setupStateDir() // already set up, but ensure access.json

    try {
      const { status, body } = await httpPost(p, '/approve', {
        hook_event_name: 'PermissionRequest',
        tool_name: 'Write',
        tool_input: { file_path: '/src/app.ts' },
      })

      expect(status).toBe(200)
      const out = (body as Record<string, unknown>).hookSpecificOutput as Record<string, unknown>
      expect(out.permissionDecision).toBe('ask')
      expect(String(out.permissionDecisionReason)).toContain('Timed out')
    } finally {
      await s.stop()
    }
  })

  it('returns 400 for missing tool_name', async () => {
    const { status } = await httpPost(port, '/approve', {
      hook_event_name: 'PermissionRequest',
      tool_input: {},
    })
    expect(status).toBe(400)
  })

  it('returns 400 for invalid JSON body', async () => {
    const response = await fetch(`http://127.0.0.1:${port}/approve`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{invalid json',
    })
    expect(response.status).toBe(400)
  })

  it('returns 404 for unknown routes', async () => {
    const response = await fetch(`http://127.0.0.1:${port}/unknown`)
    expect(response.status).toBe(404)
  })
})

// ---------------------------------------------------------------------------
// TC-027/TC-028: POST /question
// ---------------------------------------------------------------------------

describe('POST /question', () => {
  let sidecar: Awaited<ReturnType<typeof startTestSidecar>>['sidecar']
  let port: number

  beforeEach(async () => {
    setupStateDir()
    const config: RemoteConfig = { ...BASE_CONFIG, timeout: { approval_ms: 100, question_ms: 100 } }
    const result = await startTestSidecar(config)
    sidecar = result.sidecar
    port = result.port
  })

  afterEach(async () => {
    await sidecar.stop()
    teardownStateDir()
  })

  it('TC-029: returns empty answer when question times out', async () => {
    const { status, body } = await httpPost(port, '/question', {
      hook_event_name: 'AskUserQuestion',
      tool_name: 'AskUserQuestion',
      tool_input: { question: 'What is your name?' },
    })

    expect(status).toBe(200)
    const out = (body as Record<string, unknown>).hookSpecificOutput as Record<string, unknown>
    expect(out.hookEventName).toBe('AskUserQuestion')
    expect(out.answer).toBe('')
  })

  it('TC-027: handles multiple-choice question (timeout = empty answer)', async () => {
    const { status, body } = await httpPost(port, '/question', {
      hook_event_name: 'AskUserQuestion',
      tool_name: 'AskUserQuestion',
      tool_input: { question: 'Which framework?', options: ['React', 'Vue', 'Svelte'] },
    })

    expect(status).toBe(200)
    const out = (body as Record<string, unknown>).hookSpecificOutput as Record<string, unknown>
    expect(out.hookEventName).toBe('AskUserQuestion')
  })
})
