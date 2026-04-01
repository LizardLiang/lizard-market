# Timestamp Validation Testing

## Overview

The Kratos test harness now includes comprehensive timestamp validation to ensure all sub-agents properly update `status.json` with correct timestamps according to the protocol defined in `references/agent-protocol.md`.

## Agents Tested

The timestamp validation tests focus on agents that actually update `status.json` during pipeline execution:

| Agent | Stage | Purpose | Test Task |
|-------|-------|---------|-----------|
| **Athena** | 1 | PRD creation | `timestamp-athena` |
| **Nemesis** | 2 | PRD review | `timestamp-nemesis` |
| **Hephaestus** | 3 | Technical specification | `timestamp-hephaestus` |
| **Apollo** | 4 | Specification review | `timestamp-apollo` |
| **Artemis** | 6 | Test planning | `timestamp-artemis` |
| **Ares** | 7 | Implementation | `timestamp-ares` |
| **Hermes** | 8 | Code review | `timestamp-hermes` |

## Validation Rules

The timestamp validator checks for:

### 1. **ISO 8601 Format Compliance**
- All timestamps must be valid ISO 8601 format
- Pattern: `YYYY-MM-DDTHH:mm:ss[.sss]Z` or `YYYY-MM-DDTHH:mm:ss[.sss]±HH:mm`
- Examples: `2026-04-01T13:25:36.503Z`, `2026-04-01T14:25:36+08:00`

### 2. **Status-Based Timestamp Requirements**
- **`ready`**: No `started` or `completed` timestamps
- **`in-progress`**: Must have `started`, no `completed`
- **`complete`**: Must have both `started` and `completed`

### 3. **Temporal Logic**
- `started` timestamp must be before `completed` timestamp
- Stage completion times should progress chronologically (warnings for out-of-order)

### 4. **Required Fields**
- Root-level `created` timestamp
- Stage-level timestamps based on status

## Running Tests

### Quick Start
```bash
# Run comprehensive timestamp authenticity test
npm run test:timestamps && npm run validate:authenticity
```

### Individual Testing
```bash
# Test all timestamp-sensitive agents
npm run test:timestamps

# Validate authenticity of test results  
npm run validate:authenticity
```

## Output Interpretation

### Successful Validation
```
✅ Timestamps valid
📊 Stage progression:
   1-prd: complete started:13:25:36 completed:13:27:42
   2-prd-review: complete started:13:27:45 completed:13:29:15
```

### Validation Errors
```
❌ Timestamp errors:
   - Stage 1-prd: Invalid 'started' timestamp format: 2026-04-01 13:25:36
   - Stage 1-prd: 'started' timestamp is not before 'completed' timestamp

⚠️ Warnings:
   - Stage 2-prd-review: 'completed' timestamp is before previous stage 1-prd
```

## Common Issues

### 1. **Incorrect Timestamp Format**
**Problem**: Using non-ISO format like `2026-04-01 13:25:36`
**Fix**: Use ISO 8601: `2026-04-01T13:25:36Z`

### 2. **Status/Timestamp Mismatch**
**Problem**: Stage marked `complete` but missing `completed` timestamp
**Fix**: Ensure agents set both status and appropriate timestamps

### 3. **Temporal Violations**
**Problem**: `started` timestamp after `completed` timestamp
**Fix**: Verify agent logic for timestamp ordering

## Integration with Pipeline

The timestamp validation is designed to work with the existing Kratos pipeline:

1. **Test Execution**: Agents run their normal workflow
2. **Status Updates**: Agents update `status.json` as usual
3. **Validation**: Post-test analysis validates timestamp compliance
4. **Reporting**: Clear pass/fail with specific error details

## Extending Tests

To add new timestamp validation tests:

1. **Add task definition** in `src/tasks.mjs`:
```javascript
{
  name: "timestamp-new-agent",
  type: "stage-X-agent",
  description: "Tests NewAgent timestamp updates",
  prompt: "Kratos, test new agent with proper timestamp tracking."
}
```

2. **Add npm script** in `package.json`:
```json
"test:timestamp-new-agent": "node src/run.mjs --task timestamp-new-agent"
```

3. **Update test suite**:
```bash
npm run test:timestamps  # Will automatically include new test
```

## Files

- `src/timestamp-validator.mjs` - Core validation logic
- `src/test-timestamps.mjs` - Comprehensive test runner
- `src/tasks.mjs` - Test task definitions (updated with timestamp tests)
- `package.json` - npm scripts for running tests

## Protocol Reference

For detailed timestamp requirements, see:
- `references/agent-protocol.md` - Agent timestamp protocol
- `references/status-json-schema.md` - Status.json structure