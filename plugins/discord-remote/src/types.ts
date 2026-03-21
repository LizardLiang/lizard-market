/**
 * Shared TypeScript types for discord-remote plugin.
 */

// ---------------------------------------------------------------------------
// Configuration types
// ---------------------------------------------------------------------------

export type SidecarConfig = {
  port: number
  host: string
  secret: string
}

export type TimeoutConfig = {
  approval_ms: number
  question_ms: number
}

export type DefaultsConfig = {
  permission_fallback: 'ask' | 'allow' | 'deny'
  question_fallback: 'skip'
}

export type ReactionsConfig = {
  approve: string
  deny: string
  always: string
}

export type RemoteConfig = {
  sidecar: SidecarConfig
  timeout: TimeoutConfig
  defaults: DefaultsConfig
  deny_patterns: string[]
  reactions: ReactionsConfig
}

// ---------------------------------------------------------------------------
// Hook request/response types
// ---------------------------------------------------------------------------

export type HookEventName = 'PermissionRequest' | 'PreToolUse' | 'AskUserQuestion'

export type HookRequest = {
  hook_event_name: HookEventName
  session_id?: string
  tool_name: string
  tool_input: Record<string, unknown>
  permission_suggestions?: Record<string, unknown>
}

// PermissionRequest response
export type PermissionResponse = {
  hookSpecificOutput: {
    hookEventName: 'PermissionRequest'
    permissionDecision: 'allow' | 'deny' | 'ask'
    permissionDecisionReason?: string
    updatedPermissions?: {
      allow: string[]
    }
  }
}

// PreToolUse response (blocked)
export type PreToolBlockResponse = {
  hookSpecificOutput: {
    hookEventName: 'PreToolUse'
    permissionDecision: 'block'
    reason: string
  }
}

// PreToolUse response (pass-through)
export type PreToolPassResponse = Record<string, never>

export type PreToolResponse = PreToolBlockResponse | PreToolPassResponse

// AskUserQuestion response
export type QuestionResponse = {
  hookSpecificOutput: {
    hookEventName: 'AskUserQuestion'
    answer: string
  }
}

export type HookResponse = PermissionResponse | PreToolResponse | QuestionResponse

// ---------------------------------------------------------------------------
// Pending request types (in-memory sidecar state)
// ---------------------------------------------------------------------------

export type PendingRequestType = 'permission' | 'question'

export type PendingRequest = {
  id: string
  type: PendingRequestType
  tool_name: string
  tool_input: Record<string, unknown>
  message_id: string
  channel_id: string
  created_at: number
  timeout_ms: number
  resolve: (response: HookResponse) => void
  reject: (error: Error) => void
  timer: ReturnType<typeof setTimeout>
}