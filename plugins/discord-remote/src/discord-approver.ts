/**
 * Discord interaction layer for discord-remote plugin.
 *
 * Handles sending approval/question messages to Discord DMs and collecting
 * user responses via reactions or message replies. All functions are async
 * and return a Promise that resolves with the user's response or a timeout
 * fallback value.
 *
 * Security: Only the paired user (first entry in access.json allowFrom) can
 * approve or answer. Reactions and replies from other users are ignored.
 */

import { readFileSync } from 'fs'
import { join } from 'path'
import { homedir } from 'os'
import type { Client, Message, Collection, ReadonlyCollection, MessageReaction, User } from 'discord.js'
import type { RemoteConfig, PermissionResponse, QuestionResponse } from './types.js'

/**
 * Get the current state directory, respecting the DISCORD_STATE_DIR env var.
 * Evaluated at call time so tests can override the env var dynamically.
 */
function getStateDir(): string {
  return process.env.DISCORD_STATE_DIR ?? join(homedir(), '.claude', 'channels', 'discord')
}

// ---------------------------------------------------------------------------
// Access helpers
// ---------------------------------------------------------------------------

type AccessJson = {
  allowFrom?: string[]
}

/**
 * Get the primary paired user ID from access.json.
 * Returns the first entry in allowFrom, or null if none.
 */
function getPairedUserId(): string | null {
  try {
    const accessFile = join(getStateDir(), 'access.json')
    const raw = readFileSync(accessFile, 'utf8')
    const access = JSON.parse(raw) as AccessJson
    return access.allowFrom?.[0] ?? null
  } catch {
    return null
  }
}

// ---------------------------------------------------------------------------
// Message formatting
// ---------------------------------------------------------------------------

const MAX_DISCORD_LENGTH = 2000
const MAX_TOOL_INPUT_LENGTH = 1500

/**
 * Summarize tool input for display in Discord.
 * String values > 100 chars are truncated. The whole output is capped at
 * MAX_TOOL_INPUT_LENGTH chars to stay within Discord's 2000 char message limit.
 */
function summarizeToolInput(toolInput: Record<string, unknown>): string {
  const summarized: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(toolInput)) {
    if (typeof value === 'string' && value.length > 100) {
      summarized[key] = `(${value.length} chars) ${value.slice(0, 100)}...`
    } else {
      summarized[key] = value
    }
  }
  let result: string
  try {
    result = JSON.stringify(summarized, null, 2)
  } catch {
    result = String(toolInput)
  }
  if (result.length > MAX_TOOL_INPUT_LENGTH) {
    result = result.slice(0, MAX_TOOL_INPUT_LENGTH) + '\n...(truncated)'
  }
  return result
}

/**
 * Format a PermissionRequest message for Discord.
 */
function formatPermissionMessage(
  toolName: string,
  toolInput: Record<string, unknown>,
  timeoutMs: number,
  reactions: RemoteConfig['reactions'],
  permissionSuggestions?: Record<string, unknown>,
): string {
  const inputSummary = summarizeToolInput(toolInput)
  const timeoutSecs = Math.round(timeoutMs / 1000)
  const lines = [
    `**[Permission Request]** \`${toolName}\``,
    '',
    '**Input:**',
    '```',
    inputSummary,
    '```',
  ]

  if (permissionSuggestions && Object.keys(permissionSuggestions).length > 0) {
    let suggestionsText: string
    try {
      suggestionsText = JSON.stringify(permissionSuggestions)
    } catch {
      suggestionsText = String(permissionSuggestions)
    }
    lines.push('', `**Suggested permissions:** ${suggestionsText}`)
  }

  lines.push(
    '',
    `React: ${reactions.approve} = approve | ${reactions.deny} = deny | ${reactions.always} = always approve`,
    `_Timeout: ${timeoutSecs}s_`,
  )

  const msg = lines.join('\n')

  // Truncate to hard Discord limit if needed (shouldn't happen given MAX_TOOL_INPUT_LENGTH)
  return msg.length > MAX_DISCORD_LENGTH ? msg.slice(0, MAX_DISCORD_LENGTH - 3) + '...' : msg
}

/**
 * Format an AskUserQuestion message for Discord (multiple-choice).
 */
function formatQuestionMultiChoiceMessage(
  question: string,
  options: string[],
  timeoutMs: number,
): string {
  const timeoutSecs = Math.round(timeoutMs / 1000)
  const optionLines = options.map((opt, i) => `${i + 1}. ${opt}`).join('\n')
  return [
    '**[Question from Claude]**',
    '',
    question,
    '',
    optionLines,
    '',
    'React with the number of your choice.',
    `_Timeout: ${timeoutSecs}s_`,
  ].join('\n')
}

/**
 * Format an AskUserQuestion message for Discord (free-text).
 */
function formatQuestionFreeTextMessage(question: string, timeoutMs: number): string {
  const timeoutSecs = Math.round(timeoutMs / 1000)
  return [
    '**[Question from Claude]**',
    '',
    question,
    '',
    'Reply to this message with your answer.',
    `_Timeout: ${timeoutSecs}s_`,
  ].join('\n')
}

// ---------------------------------------------------------------------------
// Number emojis for multiple-choice reactions (1-9)
// ---------------------------------------------------------------------------

const NUMBER_EMOJIS = ['1\ufe0f\u20e3', '2\ufe0f\u20e3', '3\ufe0f\u20e3', '4\ufe0f\u20e3',
  '5\ufe0f\u20e3', '6\ufe0f\u20e3', '7\ufe0f\u20e3', '8\ufe0f\u20e3', '9\ufe0f\u20e3']

// ---------------------------------------------------------------------------
// Discord approver functions
// ---------------------------------------------------------------------------

/**
 * Send a PermissionRequest message to the paired user's DM and wait for reaction.
 *
 * Adds three reactions (approve, deny, always) to the message. Returns a
 * PermissionResponse based on which reaction the user picks, or a timeout
 * fallback if no response within timeoutMs.
 */
export async function sendApprovalRequest(
  client: Client,
  config: RemoteConfig,
  toolName: string,
  toolInput: Record<string, unknown>,
  permissionSuggestions?: Record<string, unknown>,
): Promise<PermissionResponse> {
  const pairedUserId = getPairedUserId()
  if (!pairedUserId) {
    throw new Error('No paired Discord user found. Run /discord-remote:configure to set up.')
  }

  const user = await client.users.fetch(pairedUserId)
  const dmChannel = await user.createDM()

  const messageText = formatPermissionMessage(
    toolName,
    toolInput,
    config.timeout.approval_ms,
    config.reactions,
    permissionSuggestions,
  )

  const message = await dmChannel.send(messageText)

  // Add reactions sequentially (avoids rate limit issues on rapid requests)
  try {
    await message.react(config.reactions.approve)
    await message.react(config.reactions.deny)
    await message.react(config.reactions.always)
  } catch (err) {
    process.stderr.write(`discord-remote: failed to add reactions: ${err}\n`)
  }

  return new Promise<PermissionResponse>((resolve) => {
    // Set up reaction collector filtered to the paired user
    const filter = (reaction: { emoji: { name: string | null } }, user: { id: string }) => {
      const emoji = reaction.emoji.name ?? ''
      return (
        user.id === pairedUserId &&
        (emoji === config.reactions.approve ||
          emoji === config.reactions.deny ||
          emoji === config.reactions.always)
      )
    }

    message
      .awaitReactions({ filter, max: 1, time: config.timeout.approval_ms, errors: [] })
      .then((collected: Collection<string, MessageReaction>) => {
        const reaction = collected.first()
        if (!reaction) {
          // Timeout — use configured fallback
          resolve(makeTimeoutPermissionResponse(config.defaults.permission_fallback))
          return
        }

        const emoji = reaction.emoji.name ?? ''
        const reactingUser = reaction.users.cache.find((u: User) => u.id === pairedUserId)
        const username = reactingUser?.username ?? pairedUserId

        if (emoji === config.reactions.always) {
          resolve({
            hookSpecificOutput: {
              hookEventName: 'PermissionRequest',
              decision: {
                behavior: 'allow',
                reason: `Always-approved via Discord by ${username}`,
                updatedPermissions: [`${toolName}(*)`],
              },
            },
          })
        } else if (emoji === config.reactions.approve) {
          resolve({
            hookSpecificOutput: {
              hookEventName: 'PermissionRequest',
              decision: {
                behavior: 'allow',
                reason: `Approved via Discord by ${username}`,
              },
            },
          })
        } else {
          resolve({
            hookSpecificOutput: {
              hookEventName: 'PermissionRequest',
              decision: {
                behavior: 'deny',
                reason: `Denied via Discord by ${username}`,
              },
            },
          })
        }
      })
      .catch(() => {
        // awaitReactions can throw on timeout if errors flag is set — we use errors: []
        // so this shouldn't happen, but handle it defensively
        resolve(makeTimeoutPermissionResponse(config.defaults.permission_fallback))
      })
  })
}

function makeTimeoutPermissionResponse(
  fallback: 'ask' | 'allow' | 'deny',
): PermissionResponse {
  return {
    hookSpecificOutput: {
      hookEventName: 'PermissionRequest',
      decision: {
        behavior: fallback,
        reason: 'Timed out waiting for Discord response, falling back to terminal',
      },
    },
  }
}

/**
 * Send a multiple-choice question to the paired user's DM and wait for reaction.
 */
export async function sendQuestionMultiChoice(
  client: Client,
  config: RemoteConfig,
  question: string,
  options: string[],
): Promise<QuestionResponse> {
  const pairedUserId = getPairedUserId()
  if (!pairedUserId) {
    throw new Error('No paired Discord user found. Run /discord-remote:configure to set up.')
  }

  // Cap options at 9 (single digit emojis)
  const cappedOptions = options.slice(0, 9)

  const user = await client.users.fetch(pairedUserId)
  const dmChannel = await user.createDM()

  const messageText = formatQuestionMultiChoiceMessage(
    question,
    cappedOptions,
    config.timeout.question_ms,
  )

  const message = await dmChannel.send(messageText)

  // Add number reactions
  try {
    for (let i = 0; i < cappedOptions.length; i++) {
      await message.react(NUMBER_EMOJIS[i])
    }
  } catch (err) {
    process.stderr.write(`discord-remote: failed to add number reactions: ${err}\n`)
  }

  return new Promise<QuestionResponse>((resolve) => {
    const validEmojis = new Set(NUMBER_EMOJIS.slice(0, cappedOptions.length))

    const filter = (reaction: { emoji: { name: string | null } }, user: { id: string }) => {
      return user.id === pairedUserId && validEmojis.has(reaction.emoji.name ?? '')
    }

    message
      .awaitReactions({ filter, max: 1, time: config.timeout.question_ms, errors: [] })
      .then((collected: Collection<string, MessageReaction>) => {
        const reaction = collected.first()
        if (!reaction) {
          resolve({ hookSpecificOutput: { hookEventName: 'AskUserQuestion', answer: '' } })
          return
        }

        const emojiIndex = NUMBER_EMOJIS.indexOf(reaction.emoji.name ?? '')
        const answer = emojiIndex >= 0 ? (cappedOptions[emojiIndex] ?? '') : ''
        resolve({ hookSpecificOutput: { hookEventName: 'AskUserQuestion', answer } })
      })
      .catch(() => {
        resolve({ hookSpecificOutput: { hookEventName: 'AskUserQuestion', answer: '' } })
      })
  })
}

/**
 * Send a free-text question to the paired user's DM and wait for a message reply.
 */
export async function sendQuestionFreeText(
  client: Client,
  config: RemoteConfig,
  question: string,
): Promise<QuestionResponse> {
  const pairedUserId = getPairedUserId()
  if (!pairedUserId) {
    throw new Error('No paired Discord user found. Run /discord-remote:configure to set up.')
  }

  const user = await client.users.fetch(pairedUserId)
  const dmChannel = await user.createDM()

  const messageText = formatQuestionFreeTextMessage(question, config.timeout.question_ms)
  const message = await dmChannel.send(messageText)

  return new Promise<QuestionResponse>((resolve) => {
    const collector = dmChannel.createMessageCollector({
      filter: (msg: Message) => msg.author.id === pairedUserId && !msg.author.bot,
      max: 1,
      time: config.timeout.question_ms,
    })

    collector.on('collect', (msg: Message) => {
      // React to acknowledge we received the answer
      void msg.react('\u2705').catch(() => {})
      resolve({ hookSpecificOutput: { hookEventName: 'AskUserQuestion', answer: msg.content } })
    })

    collector.on('end', (collected: ReadonlyCollection<string, Message>) => {
      if (collected.size === 0) {
        // Timed out
        resolve({ hookSpecificOutput: { hookEventName: 'AskUserQuestion', answer: '' } })
      }
    })

    // Make TypeScript happy — message variable used to satisfy linting
    void message
  })
}