---
name: context-optimizer
description: Context preparation system that reduces token usage by curating context once per pipeline run
---

# Context Optimizer

This system implements Superpowers-style context optimization for the Kratos pipeline. Instead of each agent reading all files independently, the context optimizer curates context once and provides agents only what they need.

## Core Principles

1. **Context Preparation Phase**: Controller reads files once, extracts relevant sections
2. **Task-Specific Curation**: Each agent gets only relevant context 
3. **Session Isolation**: Agents get curated context, not full session history
4. **Model Optimization**: Right model for task complexity

## Context Preparation Pipeline

### Phase 1: Global Context Collection

Read common files once at pipeline start:

```bash
# References (read once, shared across agents)
- plugins/kratos/references/agent-protocol.md
- plugins/kratos/references/arena-protocol.md  
- plugins/kratos/references/status-json-schema.md

# Templates (read once, extract structure)
- plugins/kratos/templates/*.md

# Arena (read once, cache by domain)
- .claude/.Arena/*.md

# Project context (read once)
- README.md
- package.json / go.mod / Cargo.toml
- Key architecture files
```

### Phase 2: Agent-Specific Context Curation

Create curated context packages for each agent type:

**Athena (PRD Creation)**:
- PRD template structure
- Business requirements protocols
- Project overview from Arena
- User requirements (passed in)

**Hephaestus (Tech Spec)**:
- Tech spec template structure  
- Architecture patterns from Arena
- Athena's PRD (file content)
- Technology constraints

**Ares (Implementation)**:
- Implementation templates
- Coding standards from Arena
- Hephaestus spec (file content)
- Test plan from Artemis

## Implementation Strategy

### 1. Context Cache Structure

```
.claude/temp/context-cache/
├── global/
│   ├── references.json        # All reference docs
│   ├── templates.json        # Template structures
│   ├── arena.json           # Arena knowledge base
│   └── project.json         # Project metadata
└── agents/
    ├── athena-context.json    # Curated for Athena
    ├── hephaestus-context.json # Curated for Hephaestus
    └── ares-context.json      # Curated for Ares
```

### 2. Context Injection Pattern

Instead of:
```
Task(prompt: "Read plugins/kratos/agents/athena.md...")
```

Use:
```
Task(prompt: "CURATED_CONTEXT: {...} MISSION: Create PRD...")
```

### 3. Token Reduction Targets

**Before**: Each agent reads 20+ files = 40K+ input tokens
**After**: Each agent gets curated context = 8-15K input tokens
**Reduction**: 60-75% token savings per agent

## Context Curation Rules

### Athena Context Package
- PRD template (structure only, not full text)
- Business requirement protocols (key sections)
- Project overview (summary, not full Arena)
- User requirements (provided by orchestrator)
- **Size target**: 3-5K tokens vs 15K+ currently

### Hephaestus Context Package  
- Tech spec template (structure only)
- Architecture patterns (relevant sections from Arena)
- PRD content (Athena's output)
- Technology constraints (from project analysis)
- **Size target**: 5-8K tokens vs 20K+ currently

### Ares Context Package
- Implementation templates (structure only) 
- Coding standards (key rules, not full docs)
- Tech spec content (Hephaestus output)
- Test plan content (Artemis output)
- **Size target**: 6-10K tokens vs 25K+ currently

## Model Optimization

Align model selection with task complexity:

**Simple/Mechanical Tasks** → Haiku:
- File scanning (Metis initial scan)
- Template completion (simple Ares tasks)
- Status updates

**Standard Tasks** → Sonnet:  
- Implementation (Ares)
- Test planning (Artemis)
- Risk analysis (Cassandra)

**Complex/Judgment Tasks** → Opus:
- Architecture (Hephaestus, Apollo)
- PRD creation/review (Athena, Nemesis) 
- Code review (Hermes)

## Integration Points

### 1. Pipeline Start Hook
Add context preparation phase before any agent spawning:

```
User says "Build X" → 
1. Run context optimizer (collect global context)
2. Create agent-specific context packages
3. Proceed with normal pipeline
```

### 2. Agent Spawning Hook  
Modify agent prompts to use curated context:

```
Before: "Read plugins/kratos/agents/athena.md for instructions..."
After: "CURATED_CONTEXT: {athena_context} MISSION: Create PRD..."
```

### 3. Cache Invalidation
Invalidate context cache when:
- Pipeline completes
- Files are modified
- User switches projects
- Arena is updated

## Expected Results

**Token Usage Reduction**:
- Current: 40-50K tokens per complex task
- Target: 12-20K tokens per complex task  
- Reduction: 60-75% savings

**Performance Improvement**:
- Faster agent startup (no file reading)
- More focused agent responses
- Better task completion rates

**Quality Maintenance**:
- Agents still get all necessary context
- Context curation ensures relevance
- No loss of functionality