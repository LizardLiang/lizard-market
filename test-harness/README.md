# Kratos Test Harness

Node.js SDK test harness for comprehensive Kratos plugin testing and performance evaluation.

## Features

- **Agent Performance Testing**: Validate individual agent behavior and response quality
- **Pipeline Integration Testing**: Test full 9-stage Kratos pipeline workflows  
- **Timestamp Compliance Validation**: Ensure proper `status.json` timestamp handling
- **Comprehensive Reporting**: Detailed test results, token usage, and error analysis

## Quick Start

```bash
# Install dependencies
npm install

# Run all tests
npm test

# Run specific test type
npm run test:research-metis     # Test Metis agent
npm run test:implementation     # Test full pipeline
npm run test:timestamps         # Test timestamp compliance
```

## Test Categories

### 1. Agent Testing
Individual agent behavior validation:

| Test | Agent | Purpose |
|------|-------|---------|
| `research-metis` | Metis | Project/code exploration |
| `research-mimir` | Mimir | External research |
| `research-clio` | Clio | Git history analysis |
| `debug` | Hades | Root-cause debugging |
| `brainstorming` | Prometheus | Strategic planning |

### 2. Pipeline Testing
Full workflow validation:

| Test | Type | Description |
|------|------|-------------|
| `implementation` | Full Pipeline | Complete 9-stage feature development |

### 3. Timestamp Validation
Validates that agents use authentic timestamps instead of fabricated ones:

| Test Type | Purpose |
|-----------|---------|
| `test:timestamps` | Run all timestamp-sensitive agents |
| `validate:authenticity` | Validate timestamp authenticity in results |

## Available Commands

### Core Testing
```bash
npm test                        # Run all basic tests
npm run test:implementation     # Full pipeline test
npm run test:debug             # Debug workflow test
```

### Research Agents
```bash
npm run test:research-metis    # Project exploration
npm run test:research-mimir    # External research  
npm run test:research-clio     # Git analysis
npm run test:brainstorming     # Strategic planning
```

### Timestamp Validation
```bash
npm run test:timestamps        # Test all agents for timestamp authenticity
npm run validate:authenticity  # Validate existing test results for authentic timestamps
```

## Output Structure

Test results are stored in `results/` with this structure:

```
results/
└── YYYY-MM-DDTHH-mm-ss-<id>/     # Test run directory
    ├── report.json                 # Summary statistics
    ├── run-meta.json              # Run metadata
    └── <test-name>/               # Individual test results
        ├── messages.jsonl         # Raw conversation stream
        ├── summary.json           # Test metrics
        └── transcript.md          # Human-readable conversation
```

## Key Metrics

Each test captures:

- **Duration**: Wall-clock time for completion
- **Token Usage**: Input/output token consumption  
- **Agent Spawns**: Number of subagents utilized
- **Error Analysis**: Failures and warning classification
- **Status Compliance**: Timestamp and status validation

## Timestamp Validation

The harness includes comprehensive timestamp validation to ensure agents properly update `status.json`:

- **ISO 8601 compliance**: All timestamps must be valid ISO format
- **Status consistency**: Timestamps must match status fields
- **Temporal logic**: Started < completed timestamp ordering
- **Progression tracking**: Chronological stage advancement

See [TIMESTAMP-TESTING.md](TIMESTAMP-TESTING.md) for detailed documentation.

## Configuration

The test harness uses:

- **Model**: `claude-sonnet-4-6` (configurable in `src/run.mjs`)
- **Plugin Path**: Auto-detected from parent directory structure
- **Test Project**: Isolated temporary project per test run

## Development

### Adding New Tests

1. **Define test** in `src/tasks.mjs`:
```javascript
{
  name: "my-test",
  type: "test-category", 
  description: "Test description",
  prompt: "Test prompt for agent execution"
}
```

2. **Add npm script** in `package.json`:
```json
"test:my-test": "node src/run.mjs --task my-test"
```

3. **Run test**:
```bash
npm run test:my-test
```

### Extending Validation

To add custom validation logic:

1. Create validator in `src/`
2. Add npm script for validation
3. Integrate with main test runner if needed

## Dependencies

- **@anthropic-ai/claude-agent-sdk**: Claude Code SDK for agent interaction
- **Node.js**: ES modules support (v16+)

## Architecture

The test harness creates isolated environments for each test:

1. **Temporary Project**: Clean test project in `results/<run>/test-project/`
2. **Agent Execution**: Full Kratos plugin loaded with all agents/skills
3. **Stream Capture**: Complete message stream recording for analysis  
4. **Post-Processing**: Automated validation and metric extraction

This ensures tests are reproducible and don't interfere with each other or the main plugin development.