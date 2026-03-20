# Kratos Code Review — Universal Standard

> The floor for all code, regardless of language or framework.
> Project-specific rules in `.claude/.Arena/review-rules/` take precedence over these.

---

## The Greatness Hierarchy

Every issue you find must be tagged with one of these tiers. Evaluate in order — a failure at a lower tier is more severe than one at a higher tier.

| Tier | Name | Definition |
|------|------|-----------|
| 1 | **Correct** | Does it do what it claims? Logic is sound, no silent failures. |
| 2 | **Safe** | Could it cause harm? Security vulnerabilities, data loss, privilege escalation. |
| 3 | **Clear** | Can a new engineer understand it in 5 minutes without asking? |
| 4 | **Minimal** | Is it the simplest solution that works? No over-engineering, no dead code. |
| 5 | **Consistent** | Does it follow the project's established patterns and conventions? |
| 6 | **Resilient** | Does it handle failure gracefully? Errors, edge cases, unexpected inputs. |
| 7 | **Performant** | Is it appropriately fast for its context? No premature optimization, no obvious waste. |
| 8 | **Maintainable** | Will this code stay healthy over time? No anti-patterns that create future debt. |

Not every piece of code needs to reach tier 8. But you must evaluate all 8 tiers and be explicit about which ones a piece of code fails to reach.

---

## Severity Labels

Each finding must carry one of these labels:

| Label | Meaning | Action Required |
|-------|---------|----------------|
| `[BLOCKER]` | Prevents correctness or safety (Tier 1–2 failures), or severe Tier 8 anti-patterns (3+ copy-paste, sequential network calls) | Must be fixed. Cannot approve until resolved. |
| `[WARNING]` | Violates clarity, consistency, resilience, or maintainability (Tier 3–6, 8) | Should be fixed. Propose auto-fix. Bulk confirm. |
| `[SUGGESTION]` | Tier 7 or style improvements | Optional. Defer to end. User skips by default. |

---

## Correctness (Tier 1)

- Logic matches the stated intent
- No off-by-one errors
- No silent return of wrong values on error paths
- Null/undefined/empty handled at every boundary
- Async flows have no race conditions or unhandled rejections
- Boolean logic is not inverted or short-circuiting incorrectly
- No unreachable code paths that should be reachable
- Function contracts (inputs → outputs) are honored

---

## Safety (Tier 2)

- No unsanitized user input passed to: SQL, shell, eval, innerHTML, file paths
- No hardcoded secrets, tokens, passwords, or API keys
- No sensitive data in logs, error messages, or responses
- Authentication checks are not bypassable
- Authorization: users can only access their own data
- No insecure direct object references (IDOR)
- Dependencies are not critically vulnerable (flag if known CVE)
- File operations are path-traversal safe
- HTTP endpoints validate all inputs
- Cryptography uses standard libraries, not hand-rolled implementations

---

## Clarity (Tier 3)

- Function and variable names explain what they are, not how they work
- Functions do one thing (Single Responsibility)
- No function longer than ~50 lines without strong justification
- Nesting depth ≤ 3 without strong justification
- Magic numbers and strings have named constants
- Comments explain *why*, not *what* (code explains what)
- No misleading comments that contradict the code
- Error messages are actionable — they tell you what went wrong and how to fix it

---

## Minimalism (Tier 4)

- No unused variables, imports, or dead code
- No duplicate logic that should be a shared function
- No abstraction layer added for a single use case
- No feature flags, backwards-compat shims, or future-proofing for hypothetical requirements
- Configuration is not more complex than the problem it solves
- Dependencies are not added for things the language/stdlib already provides

---

## Consistency (Tier 5)

- Naming follows the project's established conventions (camelCase, snake_case, etc.)
- File/folder structure matches the existing layout
- Error handling follows the project's pattern (throw vs. return, Result vs. throw, etc.)
- Logging uses the project's logger, not console.log
- API responses follow the project's shape conventions
- Test structure matches the project's testing patterns

*Note: If `.claude/.Arena/review-rules/conventions.md` exists, use it as the authoritative source for project conventions.*

---

## Resilience (Tier 6)

- All external calls (HTTP, DB, filesystem) have error handling
- Timeouts and retries are appropriate for the operation
- Partial failures degrade gracefully, not catastrophically
- Invalid inputs produce meaningful errors, not crashes
- Resource cleanup happens even on error paths (close files, release locks)
- Idempotency considered for operations that may be retried

---

## Performance (Tier 7)

- No N+1 query patterns
- No unnecessary re-computation in hot paths
- No synchronous blocking in async contexts
- Large data sets use pagination or streaming, not load-everything
- Caching is appropriate (not missing where needed, not excessive where harmful)
- No busy-wait loops or excessive polling

---

## Maintainability (Tier 8)

Long-term health anti-patterns. Only flag these in **new or modified code** — do not flag pre-existing debt.

### Code Quality

- **Redundant state**: State that duplicates or can be derived from existing state; cached values that should be computed on read
- **Parameter sprawl**: Functions gaining new parameters (especially boolean flags that fork behavior) instead of restructuring
- **Copy-paste variation**: Near-duplicate code blocks (≥5 lines, ≥80% similar) that should be unified via a shared abstraction — 3+ copies is a BLOCKER
- **Leaky abstractions**: Exposing internal implementation details that callers shouldn't depend on; breaking existing encapsulation boundaries
- **Stringly-typed code**: Raw string literals where constants, enums, string unions, or branded types already exist in the codebase

### Efficiency

- **Missed concurrency**: Independent async operations run sequentially when they could be parallel — BLOCKER when it involves network/IO calls
- **Hot-path bloat**: New blocking/expensive work added to startup, per-request, or per-render paths without justification
- **Recurring no-op updates**: State/store updates inside loops, intervals, or event handlers that fire unconditionally — missing change-detection guard
- **TOCTOU checks**: Pre-checking file/resource existence before operating instead of operating directly and handling the error
- **Unbounded growth**: Data structures that grow without bound (missing TTL, LRU eviction, size cap, or cleanup)

---

## Auto-Fix Rules

Some findings can be auto-applied. Hermes must distinguish:

| Fixable | Not Fixable |
|---------|------------|
| Remove unused imports | Logic bugs |
| Add missing error handling boilerplate | Security vulnerabilities (require human review) |
| Rename to match conventions | Structural/architectural changes |
| Add null guards on obvious paths | Business logic corrections |
| Extract magic numbers to constants | Test rewrites |

**Fix UX:**
- `[BLOCKER]` → show diff, require explicit `y/n` per issue
- `[WARNING]` → group diffs, bulk approve/reject
- `[SUGGESTION]` → show at end, skip by default

---

## Rule Proposal Protocol

If Hermes observes a recurring pattern not covered by existing rules, it should:

1. Write a proposal to `.claude/.Arena/review-rules/proposals/<date>-<short-description>.md`
2. Format:
   ```markdown
   # Rule Proposal: <title>
   Observed in: <file:line>, <file:line>
   Pattern: <what keeps appearing>
   Proposed rule: <the rule in one sentence>
   Suggested tier: <1–8>
   Suggested severity: BLOCKER / WARNING / SUGGESTION
   ```
3. Mention the proposal in the review summary so the user can promote it

---

## What "Approved" Means

Approval requires:
- Zero `[BLOCKER]` findings
- All `[WARNING]` findings either fixed or explicitly overridden by user

`[SUGGESTION]` findings do not block approval.
