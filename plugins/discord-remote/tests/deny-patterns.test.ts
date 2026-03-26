/**
 * Unit tests for src/deny-patterns.ts
 *
 * Uses bun:test. Tests matchDenyPattern() in isolation.
 */

import { describe, it, expect } from 'bun:test'
import { matchDenyPattern } from '../src/deny-patterns.js'

// ---------------------------------------------------------------------------
// TC-010: Deny pattern match blocks the tool
// ---------------------------------------------------------------------------

describe('matchDenyPattern', () => {
  it('TC-010: returns matched pattern when tool input matches', () => {
    const patterns = ['rm -rf /', 'DROP TABLE']
    const result = matchDenyPattern('Bash', { command: 'rm -rf /tmp' }, patterns)
    expect(result).toBe('rm -rf /')
  })

  // -------------------------------------------------------------------------
  // TC-011: Deny pattern no match passes through
  // -------------------------------------------------------------------------

  it('TC-011: returns null when no pattern matches', () => {
    const patterns = ['rm -rf /']
    const result = matchDenyPattern('Bash', { command: 'ls -la' }, patterns)
    expect(result).toBeNull()
  })

  // -------------------------------------------------------------------------
  // TC-012: Deny pattern match is case-sensitive
  // -------------------------------------------------------------------------

  it('TC-012: matching is case-sensitive (DROP TABLE does not match drop table)', () => {
    const patterns = ['DROP TABLE']
    const result = matchDenyPattern('Bash', { command: 'drop table users' }, patterns)
    expect(result).toBeNull()
  })

  // -------------------------------------------------------------------------
  // TC-013: Deny pattern matches against concatenated tool name + tool input
  // -------------------------------------------------------------------------

  it('TC-013: matches against concatenated tool name and stringified tool input', () => {
    const patterns = ['chmod 777']
    const result = matchDenyPattern('Bash', { command: 'chmod 777 /etc/passwd' }, patterns)
    expect(result).toBe('chmod 777')
  })

  // -------------------------------------------------------------------------
  // Additional edge cases
  // -------------------------------------------------------------------------

  it('returns null for empty patterns array', () => {
    const result = matchDenyPattern('Bash', { command: 'rm -rf /' }, [])
    expect(result).toBeNull()
  })

  it('skips empty pattern strings', () => {
    const patterns = ['', 'rm -rf /']
    const result = matchDenyPattern('Bash', { command: 'ls' }, patterns)
    expect(result).toBeNull()
  })

  it('matches pattern in tool name itself', () => {
    // The search target is "ToolName <stringified input>"
    const patterns = ['DangerousTool']
    const result = matchDenyPattern('DangerousTool', { arg: 'safe' }, patterns)
    expect(result).toBe('DangerousTool')
  })

  it('matches second pattern when first does not match', () => {
    const patterns = ['not-this', 'DROP TABLE']
    const result = matchDenyPattern('SQL', { query: 'DROP TABLE users' }, patterns)
    expect(result).toBe('DROP TABLE')
  })

  it('handles nested tool input object', () => {
    const patterns = ['secret']
    const result = matchDenyPattern('Bash', { args: { nested: 'secret-key' } }, patterns)
    expect(result).toBe('secret')
  })

  it('handles tool input that cannot be stringified (circular ref avoided)', () => {
    // matchDenyPattern should not throw on unusual inputs
    const patterns = ['foo']
    const result = matchDenyPattern('Bash', {}, patterns)
    expect(result).toBeNull()
  })
})
