/**
 * PreToolUse deny pattern matcher.
 *
 * Performs simple substring matching against tool_name + stringified tool_input.
 * Patterns are case-sensitive substrings — no regex, no glob. This intentional
 * simplicity keeps patterns predictable and avoids ReDoS risks.
 *
 * Config note: deny patterns are read from remote-config.json at startup and on
 * SIGHUP (Unix) or per-request (Windows). They cannot be modified via Discord
 * messages — this prevents injection attacks where a malicious message tries to
 * weaken deny rules (see tech-spec section 5 Security Considerations).
 */

/**
 * Build a single searchable string from tool name + tool input for pattern matching.
 * Combines tool_name and JSON-stringified tool_input into one string.
 */
function buildSearchTarget(toolName: string, toolInput: Record<string, unknown>): string {
  try {
    return toolName + ' ' + JSON.stringify(toolInput)
  } catch {
    return toolName
  }
}

/**
 * Check if the tool invocation matches any deny pattern.
 *
 * @param toolName - The tool name (e.g., "Bash", "Write")
 * @param toolInput - The tool input object
 * @param patterns - Array of deny patterns (substrings)
 * @returns The matched pattern string, or null if no match
 */
export function matchDenyPattern(
  toolName: string,
  toolInput: Record<string, unknown>,
  patterns: string[],
): string | null {
  if (patterns.length === 0) return null

  const target = buildSearchTarget(toolName, toolInput)

  for (const pattern of patterns) {
    if (pattern.length === 0) continue
    if (target.includes(pattern)) {
      return pattern
    }
  }

  return null
}