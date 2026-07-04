#!/usr/bin/env bash
# Creates a deterministic fixture project for testing one Kratos agent in isolation.
# Usage: mk-fixture.sh <dest-dir> <stage-key>
# Seeds golden upstream docs so stage N can run without live stages 0..N-1.

set -euo pipefail
DEST="${1:?dest dir required}"
STAGE="${2:?stage key required}"
mkdir -p "$DEST"
DEST="$(cd "$DEST" && pwd)"
FEATURE="rate-limit"
FDIR="$DEST/.claude/feature/$FEATURE"

mkdir -p "$DEST/src" "$FDIR"
cd "$DEST"

# --- tiny known codebase: plain-JS token API, no deps ---------------------
cat > src/server.js <<'EOF'
const http = require('http');
const { verifyToken } = require('./auth');

const server = http.createServer((req, res) => {
  if (req.url === '/api/data') {
    const token = req.headers['authorization'];
    if (!verifyToken(token)) {
      res.writeHead(401); return res.end('unauthorized');
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    return res.end(JSON.stringify({ items: [1, 2, 3] }));
  }
  res.writeHead(404); res.end();
});

module.exports = server;
if (require.main === module) server.listen(3000);
EOF

cat > src/auth.js <<'EOF'
const VALID = new Set(['alpha-token', 'beta-token']);
function verifyToken(t) {
  if (!t) return false;
  return VALID.has(t.replace('Bearer ', ''));
}
module.exports = { verifyToken };
EOF

cat > package.json <<'EOF'
{ "name": "fixture-api", "version": "1.0.0", "scripts": { "test": "node --test" } }
EOF

mkdir -p test
cat > test/auth.test.js <<'EOF'
const { test } = require('node:test');
const assert = require('node:assert');
const { verifyToken } = require('../src/auth');
test('valid token passes', () => assert.equal(verifyToken('Bearer alpha-token'), true));
test('missing token fails', () => assert.equal(verifyToken(undefined), false));
EOF

git init -q 2>/dev/null || true
git add -A && git -c user.email=h@h -c user.name=harness commit -qm "fixture baseline" 2>/dev/null || true

# --- golden upstream docs, seeded per stage under test --------------------
# ORIGINAL_USER_REQUEST for all stages:
#   "Add per-token rate limiting to /api/data: 10 requests/minute, return 429
#    with Retry-After, exempt admin tokens, in-memory only."

need_prd=false; need_spec=false; need_testplan=false; need_impl=false
case "$STAGE" in
  1-prd) ;;
  2-prd-review|3-decomposition|4-tech-spec) need_prd=true ;;
  5-spec-review-sa|6-test-plan) need_prd=true; need_spec=true ;;
  7-implementation) need_prd=true; need_spec=true; need_testplan=true ;;
  8-prd-alignment|9-review) need_prd=true; need_spec=true; need_testplan=true; need_impl=true ;;
  *) echo "unknown stage: $STAGE" >&2; exit 2 ;;
esac

if $need_prd; then cat > "$FDIR/prd.md" <<'EOF'
# PRD: Per-Token Rate Limiting

## Problem
/api/data has no request throttling; a single token can exhaust the service.

## Requirements
- AC-1: Requests to /api/data are limited to 10 per rolling 60s window per token.
- AC-2: Request 11+ within the window returns HTTP 429.
- AC-3: 429 responses include a Retry-After header with seconds until the window resets.
- AC-4: Tokens listed as admin (config array) bypass the limit entirely.
- AC-5: Unauthorized requests (401) do not consume rate-limit quota.

## Out of Scope
Persistence across restarts; distributed counters; per-IP limits.
EOF
fi

if $need_spec; then cat > "$FDIR/tech-spec.md" <<'EOF'
# Tech Spec: Per-Token Rate Limiting

## Architecture
Sliding-window counter in a Map keyed by token, module src/rate-limit.js,
wired into src/server.js after auth succeeds (AC-5).

## Interfaces
- `checkLimit(token, now=Date.now()) -> { allowed: boolean, retryAfterSec: number }` (AC-1, AC-2, AC-3)
- `ADMIN_TOKENS` exported const array, default ['admin-token'] (AC-4)

## Files
- src/rate-limit.js (new)
- src/server.js (modify: call checkLimit after verifyToken)
- test/rate-limit.test.js (new)

## Sequencing
1. rate-limit module + unit tests. 2. server wiring. 3. integration test for 429 + Retry-After.
EOF
fi

if $need_testplan; then cat > "$FDIR/test-plan.md" <<'EOF'
# Test Plan

- TC-1 (AC-1): 10 requests in window all 200.
- TC-2 (AC-2): 11th request in window is 429.
- TC-3 (AC-3): 429 carries Retry-After ~= window remainder.
- TC-4 (AC-4): admin-token issues 20 requests, all 200.
- TC-5 (AC-5): 401 request does not decrement quota for that token.
- TC-6 (AC-1): window expiry resets the counter.
EOF
fi

if $need_impl; then
  # A deliberately imperfect implementation: AC-3 header missing, AC-5 violated.
  # Lets Hera/Hermes/Cassandra be graded on whether they CATCH the gaps.
  cat > src/rate-limit.js <<'EOF'
const WINDOW_MS = 60000, LIMIT = 10;
const ADMIN_TOKENS = ['admin-token'];
const hits = new Map();
function checkLimit(token, now = Date.now()) {
  if (ADMIN_TOKENS.includes(token)) return { allowed: true, retryAfterSec: 0 };
  const arr = (hits.get(token) || []).filter(t => now - t < WINDOW_MS);
  arr.push(now);
  hits.set(token, arr);
  return { allowed: arr.length <= LIMIT, retryAfterSec: Math.ceil((arr[0] + WINDOW_MS - now) / 1000) };
}
module.exports = { checkLimit, ADMIN_TOKENS };
EOF
  cat > src/server.js <<'EOF'
const http = require('http');
const { verifyToken } = require('./auth');
const { checkLimit } = require('./rate-limit');

const server = http.createServer((req, res) => {
  if (req.url === '/api/data') {
    const token = (req.headers['authorization'] || '').replace('Bearer ', '');
    const gate = checkLimit(token); // BUG: consumes quota before auth (violates AC-5)
    if (!verifyToken(req.headers['authorization'])) {
      res.writeHead(401); return res.end('unauthorized');
    }
    if (!gate.allowed) {
      res.writeHead(429); return res.end('rate limited'); // BUG: no Retry-After (violates AC-3)
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    return res.end(JSON.stringify({ items: [1, 2, 3] }));
  }
  res.writeHead(404); res.end();
});

module.exports = server;
if (require.main === module) server.listen(3000);
EOF
  cat > "$FDIR/implementation-notes.md" <<'EOF'
# Implementation Notes
Added src/rate-limit.js sliding window; wired into server.js. Unit tests in test/.
All acceptance criteria implemented.
EOF
  git add -A && git -c user.email=h@h -c user.name=harness commit -qm "implementation (seeded gaps: AC-3, AC-5)" 2>/dev/null || true
fi

# minimal status.json so the agent has pipeline state to read/update
cat > "$FDIR/status.json" <<EOF
{
  "feature": "$FEATURE",
  "created": "2026-07-04T00:00:00Z",
  "updated": "2026-07-04T00:00:00Z",
  "stage": "$STAGE",
  "pipeline_status": "in-progress",
  "mode": "normal",
  "implementation_mode": "ares",
  "pipeline": {},
  "history": []
}
EOF

echo "fixture ready: $DEST (stage $STAGE, feature $FEATURE)"
