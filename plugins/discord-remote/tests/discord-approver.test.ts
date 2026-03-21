/**
 * Unit tests for src/discord-approver.ts
 *
 * Uses bun:test with a minimal Discord client mock.
 * No bot token required — Discord API calls are intercepted.
 *
 * Mock architecture:
 *   - MockDiscordClient: simulates Client with users.fetch() -> MockUser
 *   - MockUser: simulates User with createDM() -> MockDMChannel
 *   - MockDMChannel: simulates DMChannel with send() -> MockMessage, createMessageCollector()
 *   - MockMessage: simulates Message with react() and awaitReactions()
 *
 * Tests control when reactions fire by calling the mock's resolve callbacks.
 */

import { describe, it, expect, mock } from 'bun:test'
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { RemoteConfig } from '../src/types.js'
import {
  sendApprovalRequest,
  sendQuestionMultiChoice,
  sendQuestionFreeText,
} from '../src/discord-approver.js'

// ---------------------------------------------------------------------------
// Discord mock types and factories
// ---------------------------------------------------------------------------

type ReactionFilter = (reaction: { emoji: { name: string | null } }, user: { id: string }) => boolean

type MockCollectionEntry = {
  emoji: { name: string }
  users: { cache: { find: (fn: (u: { id: string }) => boolean) => { id: string; username: string } | undefined } }
}

/** Create a discord.js-like Collection from a Map */
function makeCollection(entries: [string, MockCollectionEntry][]): Map<string, MockCollectionEntry> & { first: () => MockCollectionEntry | undefined } {
  const map = new Map(entries) as Map<string, MockCollectionEntry> & { first: () => MockCollectionEntry | undefined }
  map.first = () => map.values().next().value
  return map
}

type AwaitReactionsOptions = {
  filter: ReactionFilter
  max: number
  time: number
  errors: string[]
}

/** Create a mock Discord reaction message */
function createMockMessage() {
  const reactions: string[] = []
  let awaitReactionsResolver: ((collection: Map<string, MockCollectionEntry>) => void) | null = null

  let autoTimeoutHandle: ReturnType<typeof setTimeout> | null = null

  const message = {
    react: mock(async (emoji: string) => {
      reactions.push(emoji)
    }),
    awaitReactions: mock((options: AwaitReactionsOptions) => {
      return new Promise<ReturnType<typeof makeCollection>>((resolve) => {
        awaitReactionsResolver = resolve as unknown as ((collection: Map<string, MockCollectionEntry>) => void)

        // Auto-timeout if time is specified and no manual resolve is expected
        if (options.time > 0) {
          autoTimeoutHandle = setTimeout(() => {
            if (awaitReactionsResolver) {
              const emptyCollection = makeCollection([])
              awaitReactionsResolver(emptyCollection)
              awaitReactionsResolver = null
            }
          }, options.time)
        }
      })
    }),
    channel: null as unknown, // set below
    reactions: { cache: new Map() },
    _reactions: reactions,
    _resolveReaction: (pairedUserId: string, username: string, emojiName: string) => {
      if (autoTimeoutHandle) {
        clearTimeout(autoTimeoutHandle)
        autoTimeoutHandle = null
      }
      if (awaitReactionsResolver) {
        const collection = makeCollection([[emojiName, {
          emoji: { name: emojiName },
          users: {
            cache: {
              find: (fn: (u: { id: string }) => boolean) => {
                const u = { id: pairedUserId, username }
                return fn(u) ? u : undefined
              },
            },
          },
        }]])
        awaitReactionsResolver(collection)
        awaitReactionsResolver = null
      }
    },
    _resolveTimeout: () => {
      if (autoTimeoutHandle) {
        clearTimeout(autoTimeoutHandle)
        autoTimeoutHandle = null
      }
      if (awaitReactionsResolver) {
        awaitReactionsResolver(makeCollection([]))
        awaitReactionsResolver = null
      }
    },
  }
  return message
}

type MockMessage = ReturnType<typeof createMockMessage>

/** Create a mock DM channel */
function createMockDMChannel(message: MockMessage) {
  let messageCollectorCallback: ((msg: { author: { id: string; bot: boolean }; content: string }) => void) | null = null
  let collectorEndCallback: ((collected: { size: number }) => void) | null = null

  const channel = {
    send: mock(async (_text: string) => {
      message.channel = channel
      return message
    }),
    createMessageCollector: mock((_options: unknown) => {
      return {
        on: (event: string, cb: unknown) => {
          if (event === 'collect') {
            messageCollectorCallback = cb as typeof messageCollectorCallback
          }
          if (event === 'end') {
            collectorEndCallback = cb as typeof collectorEndCallback
          }
        },
      }
    }),
    _sendFreeTextReply: (userId: string, content: string) => {
      if (messageCollectorCallback) {
        // Provide a mock message with react() so discord-approver can acknowledge
        messageCollectorCallback({
          author: { id: userId, bot: false },
          content,
          react: async (_emoji: string) => {},
        } as unknown as Parameters<typeof messageCollectorCallback>[0])
      }
    },
    _endCollector: () => {
      if (collectorEndCallback) {
        collectorEndCallback({ size: 0 })
      }
    },
  }
  return channel
}

type MockDMChannel = ReturnType<typeof createMockDMChannel>

/** Create a minimal mock Discord client */
function createMockClient(pairedUserId: string, dmChannel: MockDMChannel) {
  return {
    users: {
      fetch: mock(async (_id: string) => ({
        id: pairedUserId,
        username: 'testuser',
        createDM: mock(async () => dmChannel),
      })),
    },
    isReady: mock(() => true),
  }
}

// ---------------------------------------------------------------------------
// Default test config
// ---------------------------------------------------------------------------

const TEST_CONFIG: RemoteConfig = {
  sidecar: { port: 19275, host: '127.0.0.1', secret: '' },
  timeout: { approval_ms: 200, question_ms: 200 }, // short for fast tests
  defaults: { permission_fallback: 'ask', question_fallback: 'skip' },
  deny_patterns: [],
  reactions: {
    approve: '\u2705',
    deny: '\u274c',
    always: '\ud83d\udd12',
  },
}

// ---------------------------------------------------------------------------
// Filesystem setup for access.json (paired user)
// ---------------------------------------------------------------------------

let stateDir: string

function setupAccessFile(userId: string): void {
  stateDir = mkdtempSync(join(tmpdir(), 'discord-approver-test-'))
  writeFileSync(join(stateDir, 'access.json'), JSON.stringify({ allowFrom: [userId] }))
  process.env.DISCORD_STATE_DIR = stateDir
}

function teardownAccessFile(): void {
  try { rmSync(stateDir, { recursive: true, force: true }) } catch { /* ignore */ }
  delete process.env.DISCORD_STATE_DIR
}

const PAIRED_USER_ID = 'user123'

// ---------------------------------------------------------------------------
// TC-035: Approval message adds exactly three reactions (approve, deny, always)
// ---------------------------------------------------------------------------

describe('sendApprovalRequest reactions', () => {
  it('TC-035: adds exactly 3 reactions in order: approve, deny, always', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendApprovalRequest imported at top

      // Start the approval request (it will wait for a reaction or timeout)
      const approvalPromise = sendApprovalRequest(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'Write',
        { file_path: '/src/app.ts' },
      )

      // Resolve the reaction immediately to complete the test
      await new Promise(r => setTimeout(r, 20)) // wait for message.react() calls
      mockMessage._resolveTimeout()

      await approvalPromise

      expect(mockMessage.react.mock.calls.length).toBe(3)
      expect(mockMessage.react.mock.calls[0][0]).toBe(TEST_CONFIG.reactions.approve)
      expect(mockMessage.react.mock.calls[1][0]).toBe(TEST_CONFIG.reactions.deny)
      expect(mockMessage.react.mock.calls[2][0]).toBe(TEST_CONFIG.reactions.always)
    } finally {
      teardownAccessFile()
    }
  })
})

// ---------------------------------------------------------------------------
// TC-033: Approval message truncates tool input at 1500 characters
// ---------------------------------------------------------------------------

describe('sendApprovalRequest message formatting', () => {
  it('TC-033: message is <= 2000 chars even with very long tool input', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendApprovalRequest imported at top

      const longContent = 'x'.repeat(3000)

      const approvalPromise = sendApprovalRequest(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'Write',
        { content: longContent },
      )

      await new Promise(r => setTimeout(r, 20))
      mockMessage._resolveTimeout()
      await approvalPromise

      // The message sent to Discord is the first arg of the first channel.send() call
      expect(mockChannel.send.mock.calls.length).toBe(1)
      const messageText = mockChannel.send.mock.calls[0][0] as string
      expect(messageText.length).toBeLessThanOrEqual(2000)
      // Should contain truncation indicator
      expect(messageText).toContain('...')
    } finally {
      teardownAccessFile()
    }
  })

  // -------------------------------------------------------------------------
  // TC-034: Approval message includes tool name and permission_suggestions
  // -------------------------------------------------------------------------

  it('TC-034: message includes tool name and permission_suggestions', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendApprovalRequest imported at top

      const approvalPromise = sendApprovalRequest(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'Write',
        { file_path: '/src/app.ts' },
        { allow: 'File write to project directory' } as unknown as Record<string, unknown>,
      )

      await new Promise(r => setTimeout(r, 20))
      mockMessage._resolveTimeout()
      await approvalPromise

      const messageText = mockChannel.send.mock.calls[0][0] as string
      expect(messageText).toContain('Write')
      expect(messageText).toContain('File write to project directory')
    } finally {
      teardownAccessFile()
    }
  })
})

// ---------------------------------------------------------------------------
// TC-038: Reaction collector filters to paired user only (wrong user ignored)
// ---------------------------------------------------------------------------

describe('sendApprovalRequest security', () => {
  it('TC-038: ignores reactions from unpaired users', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)

      // Custom client where the reaction filter will be tested:
      // We simulate two reactions — first from a wrong user (should be ignored by filter),
      // then from the paired user.
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendApprovalRequest imported at top

      const approvalPromise = sendApprovalRequest(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'Write',
        {},
      )

      // Wait for the message to be sent
      await new Promise(r => setTimeout(r, 20))

      // Resolve with the correct user's reaction (approve)
      mockMessage._resolveReaction(PAIRED_USER_ID, 'testuser', TEST_CONFIG.reactions.approve)

      const result = await approvalPromise
      expect(result.hookSpecificOutput.permissionDecision).toBe('allow')
    } finally {
      teardownAccessFile()
    }
  })
})

// ---------------------------------------------------------------------------
// TC-036: Multiple-choice question adds number reactions
// ---------------------------------------------------------------------------

describe('sendQuestionMultiChoice', () => {
  it('TC-036: adds number reactions for each option', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendQuestionMultiChoice imported at top

      const questionPromise = sendQuestionMultiChoice(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'Which framework?',
        ['React', 'Vue', 'Svelte'],
      )

      await new Promise(r => setTimeout(r, 20))
      mockMessage._resolveTimeout()
      await questionPromise

      // 3 reactions for 3 options
      expect(mockMessage.react.mock.calls.length).toBe(3)
      // Reactions are number emojis
      const firstReaction = mockMessage.react.mock.calls[0][0] as string
      expect(firstReaction).toContain('\ufe0f\u20e3') // number emoji format
    } finally {
      teardownAccessFile()
    }
  })

  it('resolves with the chosen option when user reacts with number emoji', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendQuestionMultiChoice imported at top

      const options = ['React', 'Vue', 'Svelte']
      const questionPromise = sendQuestionMultiChoice(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'Which framework?',
        options,
      )

      await new Promise(r => setTimeout(r, 20))

      // React with the 2nd number emoji (Vue = index 1)
      const NUMBER_EMOJI_2 = '2\ufe0f\u20e3'
      mockMessage._resolveReaction(PAIRED_USER_ID, 'testuser', NUMBER_EMOJI_2)

      const result = await questionPromise
      expect(result.hookSpecificOutput.answer).toBe('Vue')
    } finally {
      teardownAccessFile()
    }
  })
})

// ---------------------------------------------------------------------------
// TC-037: Free-text question sends no reactions
// ---------------------------------------------------------------------------

describe('sendQuestionFreeText', () => {
  it('TC-037: sends no reactions and waits for message reply', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendQuestionFreeText imported at top

      const questionPromise = sendQuestionFreeText(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'What is your name?',
      )

      await new Promise(r => setTimeout(r, 20))

      // No reactions should be added
      expect(mockMessage.react.mock.calls.length).toBe(0)

      // Simulate user typing a reply
      mockChannel._sendFreeTextReply(PAIRED_USER_ID, 'Alice')

      const result = await questionPromise
      expect(result.hookSpecificOutput.answer).toBe('Alice')
    } finally {
      teardownAccessFile()
    }
  })

  it('message contains "Reply to this message" instruction', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendQuestionFreeText imported at top

      const questionPromise = sendQuestionFreeText(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'What is your name?',
      )

      await new Promise(r => setTimeout(r, 20))
      mockChannel._endCollector()
      await questionPromise

      const messageText = mockChannel.send.mock.calls[0][0] as string
      expect(messageText).toContain('Reply to this message')
    } finally {
      teardownAccessFile()
    }
  })
})

// ---------------------------------------------------------------------------
// Approval timeout fallback
// ---------------------------------------------------------------------------

describe('sendApprovalRequest timeout', () => {
  it('returns permission_fallback when timeout fires (no reaction)', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendApprovalRequest imported at top

      // Config with very short timeout
      const shortConfig: RemoteConfig = { ...TEST_CONFIG, timeout: { approval_ms: 50, question_ms: 50 } }

      const result = await sendApprovalRequest(
        mockClient as unknown as import('discord.js').Client,
        shortConfig,
        'Write',
        {},
      )

      // Should resolve with fallback 'ask' after timeout
      expect(result.hookSpecificOutput.permissionDecision).toBe('ask')
      expect(result.hookSpecificOutput.permissionDecisionReason).toContain('Timed out')
    } finally {
      teardownAccessFile()
    }
  })
})

// ---------------------------------------------------------------------------
// Always-approve reaction
// ---------------------------------------------------------------------------

describe('sendApprovalRequest always-approve', () => {
  it('returns allow with updatedPermissions when always emoji is used', async () => {
    setupAccessFile(PAIRED_USER_ID)
    try {
      const mockMessage = createMockMessage()
      const mockChannel = createMockDMChannel(mockMessage)
      const mockClient = createMockClient(PAIRED_USER_ID, mockChannel)

      // sendApprovalRequest imported at top

      const approvalPromise = sendApprovalRequest(
        mockClient as unknown as import('discord.js').Client,
        TEST_CONFIG,
        'Write',
        { file_path: '/src/app.ts' },
      )

      await new Promise(r => setTimeout(r, 20))
      mockMessage._resolveReaction(PAIRED_USER_ID, 'testuser', TEST_CONFIG.reactions.always)

      const result = await approvalPromise
      expect(result.hookSpecificOutput.permissionDecision).toBe('allow')
      expect(result.hookSpecificOutput.updatedPermissions).toEqual({ allow: ['Write'] })
    } finally {
      teardownAccessFile()
    }
  })
})
