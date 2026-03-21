/**
 * HTTP sidecar server for discord-remote plugin.
 *
 * Co-hosted with the MCP stdio server in the same Bun process. Listens on
 * localhost only (security). Handles three hook event types via HTTP endpoints:
 *   POST /approve  — PermissionRequest hooks
 *   POST /pretool  — PreToolUse hooks (deny pattern check)
 *   POST /question — AskUserQuestion hooks
 *   GET  /health   — Health check
 *
 * Port is written to STATE_DIR/sidecar.port at startup and deleted on shutdown
 * so hook scripts can discover where to POST.
 *
 * Uses long-poll pattern: HTTP response is held open until the Discord
 * interaction completes or times out. On shutdown, all pending requests
 * are resolved with fallback responses.
 */

import * as http from 'http'
import { writeFileSync, unlinkSync, mkdirSync } from 'fs'
import { join } from 'path'
import type { Client } from 'discord.js'
import type {
  RemoteConfig,
  HookRequest,
  HookResponse,
  PendingRequest,
  PermissionResponse,
  QuestionResponse,
} from './types.js'
import { STATE_DIR, getConfig, loadConfig } from './config.js'
import { matchDenyPattern } from './deny-patterns.js'
import {
  sendApprovalRequest,
  sendQuestionMultiChoice,
  sendQuestionFreeText,
} from './discord-approver.js'

const PORT_FILE = join(STATE_DIR, 'sidecar.port')
const PPID_FILE = join(STATE_DIR, 'sidecar.ppid')
const MAX_PORT_ATTEMPTS = 10

// ---------------------------------------------------------------------------
// In-memory pending request map
// ---------------------------------------------------------------------------

const pendingRequests = new Map<string, PendingRequest>()

// ---------------------------------------------------------------------------
// Request body parsing
// ---------------------------------------------------------------------------

async function parseBody(req: http.IncomingMessage): Promise<unknown> {
  return new Promise((resolve, reject) => {
    let data = ''
    req.on('data', (chunk: string | Buffer) => {
      data += chunk
      if (data.length > 100_000) {
        reject(new Error('Request body too large'))
      }
    })
    req.on('end', () => {
      try {
        resolve(JSON.parse(data))
      } catch {
        reject(new Error('Invalid JSON'))
      }
    })
    req.on('error', reject)
  })
}

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

function sendJson(res: http.ServerResponse, status: number, body: unknown): void {
  const json = JSON.stringify(body)
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(json),
  })
  res.end(json)
}

function sendError(res: http.ServerResponse, status: number, message: string): void {
  sendJson(res, status, { error: message })
}

// ---------------------------------------------------------------------------
// Route handlers
// ---------------------------------------------------------------------------

async function handleApprove(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  client: Client,
  cfg: RemoteConfig,
): Promise<void> {
  let body: HookRequest
  try {
    body = (await parseBody(req)) as HookRequest
  } catch {
    sendError(res, 400, 'Invalid request body')
    return
  }

  if (!body.tool_name) {
    sendError(res, 400, 'Missing tool_name')
    return
  }

  try {
    // sendApprovalRequest blocks until user reacts or timeout fires
    const response: PermissionResponse = await sendApprovalRequest(
      client,
      cfg,
      body.tool_name,
      body.tool_input ?? {},
      body.permission_suggestions,
    )
    sendJson(res, 200, response)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    process.stderr.write(`discord-remote: /approve error: ${message}\n`)
    sendError(res, 500, message)
  }
}

async function handlePreTool(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  cfg: RemoteConfig,
): Promise<void> {
  let body: HookRequest
  try {
    body = (await parseBody(req)) as HookRequest
  } catch {
    sendError(res, 400, 'Invalid request body')
    return
  }

  if (!body.tool_name) {
    sendError(res, 400, 'Missing tool_name')
    return
  }

  const matchedPattern = matchDenyPattern(
    body.tool_name,
    body.tool_input ?? {},
    cfg.deny_patterns,
  )

  if (matchedPattern !== null) {
    sendJson(res, 200, {
      hookSpecificOutput: {
        hookEventName: 'PreToolUse',
        permissionDecision: 'block',
        reason: `Matched deny pattern: ${matchedPattern}`,
      },
    })
  } else {
    // Empty object = pass-through (no opinion)
    sendJson(res, 200, {})
  }
}

async function handleQuestion(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  client: Client,
  cfg: RemoteConfig,
): Promise<void> {
  let body: HookRequest
  try {
    body = (await parseBody(req)) as HookRequest
  } catch {
    sendError(res, 400, 'Invalid request body')
    return
  }

  const input = body.tool_input ?? {}
  const question = (input.question as string) ?? JSON.stringify(input)
  const options = input.options as string[] | undefined

  try {
    let response: QuestionResponse
    if (Array.isArray(options) && options.length > 0) {
      response = await sendQuestionMultiChoice(client, cfg, question, options)
    } else {
      response = await sendQuestionFreeText(client, cfg, question)
    }
    sendJson(res, 200, response)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    process.stderr.write(`discord-remote: /question error: ${message}\n`)
    sendError(res, 500, message)
  }
}

function handleHealth(res: http.ServerResponse, startTime: number, client: Client): void {
  const isConnected = client.isReady()
  sendJson(res, 200, {
    status: 'ok',
    gateway: isConnected ? 'connected' : 'disconnected',
    pending_requests: pendingRequests.size,
    uptime_ms: Date.now() - startTime,
  })
}

// ---------------------------------------------------------------------------
// Port discovery: write actual port to file
// ---------------------------------------------------------------------------

function writePortFile(port: number): void {
  try {
    mkdirSync(STATE_DIR, { recursive: true, mode: 0o700 })
    writeFileSync(PORT_FILE, String(port) + '\n', { mode: 0o600 })
    // Write the parent PID (Claude Code's PID) so hook scripts can check
    // if they belong to the same session. The MCP server is a child process
    // of Claude Code, and hooks are also children of the same Claude Code.
    writeFileSync(PPID_FILE, String(process.ppid) + '\n', { mode: 0o600 })
  } catch (err) {
    process.stderr.write(`discord-remote: failed to write sidecar.port: ${err}\n`)
  }
}

function deletePortFile(): void {
  try { unlinkSync(PORT_FILE) } catch {}
  try { unlinkSync(PPID_FILE) } catch {}
}

// ---------------------------------------------------------------------------
// Server factory
// ---------------------------------------------------------------------------

export type Sidecar = {
  start: () => Promise<void>
  stop: () => Promise<void>
  port: () => number
}

export function createSidecar(client: Client, config: RemoteConfig): Sidecar {
  let server: http.Server | null = null
  let actualPort = config.sidecar.port
  const startTime = Date.now()
  let shuttingDown = false

  async function start(): Promise<void> {
    server = http.createServer(async (req, res) => {
      if (shuttingDown) {
        sendError(res, 503, 'Sidecar shutting down')
        return
      }

      const url = req.url ?? '/'
      const method = req.method ?? 'GET'

      try {
        // On Windows, reload config per-request (no SIGHUP). On Unix, use the
        // config passed at construction time (updated by SIGHUP via the module-level holder).
        const currentConfig: RemoteConfig = process.platform === 'win32' ? loadConfig() : config

        // Validate shared secret on all POST endpoints (H-001 fix)
        if (method === 'POST' && currentConfig.sidecar.secret) {
          const authHeader = req.headers['authorization'] ?? ''
          const expected = `Bearer ${currentConfig.sidecar.secret}`
          if (authHeader !== expected) {
            sendError(res, 403, 'Invalid or missing authorization')
            return
          }
        }

        if (method === 'GET' && url === '/health') {
          handleHealth(res, startTime, client)
        } else if (method === 'POST' && url === '/approve') {
          await handleApprove(req, res, client, currentConfig)
        } else if (method === 'POST' && url === '/pretool') {
          await handlePreTool(req, res, currentConfig)
        } else if (method === 'POST' && url === '/question') {
          await handleQuestion(req, res, client, currentConfig)
        } else {
          sendError(res, 404, `Not found: ${method} ${url}`)
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err)
        process.stderr.write(`discord-remote: unhandled route error: ${message}\n`)
        if (!res.headersSent) {
          sendError(res, 500, 'Internal server error')
        }
      }
    })

    // Try to bind to the configured port, incrementing up to MAX_PORT_ATTEMPTS times
    actualPort = await new Promise<number>((resolve, reject) => {
      let attempts = 0
      const tryBind = (port: number): void => {
        server!.listen(port, config.sidecar.host, () => {
          // Use server.address() to get the actual port (handles port 0 OS assignment)
          const addr = server!.address() as { port: number } | null
          resolve(addr?.port ?? port)
        })
        server!.once('error', (err: NodeJS.ErrnoException) => {
          if (err.code === 'EADDRINUSE' && attempts < MAX_PORT_ATTEMPTS) {
            attempts++
            server!.removeAllListeners('error')
            tryBind(port + 1)
          } else {
            reject(new Error(`Failed to bind sidecar on port ${port}: ${err.message}`))
          }
        })
      }
      tryBind(config.sidecar.port)
    })

    writePortFile(actualPort)
    process.stderr.write(`discord-remote: sidecar listening on ${config.sidecar.host}:${actualPort}\n`)
  }

  async function stop(): Promise<void> {
    shuttingDown = true

    // Resolve all pending requests with fallback before closing
    for (const [id, pending] of pendingRequests) {
      const fallbackConfig = getConfig()
      const fallback = fallbackConfig.defaults.permission_fallback
      if (pending.type === 'permission') {
        pending.resolve({
          hookSpecificOutput: {
            hookEventName: 'PermissionRequest',
            decision: {
              behavior: fallback,
              reason: 'Sidecar shutting down',
            },
          },
        } as HookResponse)
      } else {
        pending.resolve({
          hookSpecificOutput: {
            hookEventName: 'AskUserQuestion',
            answer: '',
          },
        } as HookResponse)
      }
      clearTimeout(pending.timer)
      pendingRequests.delete(id)
    }

    deletePortFile()

    if (server) {
      await new Promise<void>((resolve) => {
        server!.close(() => resolve())
      })
    }
  }

  function port(): number {
    return actualPort
  }

  return { start, stop, port }
}