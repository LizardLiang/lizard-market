# Arena Document Templates

These templates define the structure for Metis Arena documents. Each document MUST start with YAML frontmatter containing the fields shown.

## Frontmatter Fields (Required for ALL Arena Documents)

```yaml
---
created: {ISO_TIMESTAMP}        # When created
updated: {ISO_TIMESTAMP}        # When last updated
author: metis                   # Always "metis"
git_hash: {CURRENT_HASH}       # Current git commit hash (required for staleness detection)
analysis_scope: full            # "full" | "partial" | "quick"
confidence: high                # "high" | "medium" | "low" (not percentages)
stale_after: {ISO_TIMESTAMP}    # created + 30 days
verification_status: unverified # "unverified" initially
---
```

Without `git_hash`, the staleness detection system will NOT work.

---

## Confidence Scoring

Calculate confidence for each document using these criteria:

| Level | Coverage | Consistency | Validation | Conflicts |
|-------|----------|-------------|------------|-----------|
| **HIGH** | >80% of files examined | Pattern in >90% of instances | Cross-checked with 2+ sources | Zero conflicts |
| **MEDIUM** | 40-80% of files | Pattern in 60-90% of instances | Found in code only | Minor variations |
| **LOW** | <40% of files | Inconsistent or unclear | Assumption-based | Multiple competing patterns |

Track these as you research: files examined vs total, pattern frequency, cross-validation sources, conflicting evidence.

---

## project-overview.md

```markdown
---
created: {TIMESTAMP}
updated: {TIMESTAMP}
author: metis
git_hash: {HASH}
analysis_scope: full
confidence: {high|medium|low}
stale_after: {TIMESTAMP}
verification_status: unverified
---

# Project Overview

**Confidence**: {High|Medium|Low}
**Last Verified**: {DATE}
**Coverage**: {N}% of project files examined

## Summary
[What this project is and does]

## Quick Facts
| Aspect | Details |
|--------|---------|
| **Name** | [Project name] |
| **Type** | [Web app, CLI, library, etc.] |
| **Primary Language** | [Language] |
| **Framework** | [Main framework] |
| **Git Hash** | {HASH} |
| **Last Analyzed** | {DATE} |

## Key Directories
- `src/` - [Purpose]
- `tests/` - [Purpose]

## Entry Points
- [Main entry point and purpose]

## Update History
- **{TIMESTAMP}** (Metis): Initial Arena documentation
```

---

## tech-stack.md

```markdown
---
created: {TIMESTAMP}
updated: {TIMESTAMP}
author: metis
git_hash: {HASH}
analysis_scope: full
confidence: {high|medium|low}
stale_after: {TIMESTAMP}
verification_status: unverified
---

# Tech Stack

**Confidence**: {High|Medium|Low}
**Last Verified**: {DATE}
**Source**: package.json, requirements.txt, etc.
**Coverage**: All dependency manifests examined

## Languages
| Language | Version | Usage | Confidence |
|----------|---------|-------|------------|
| [Language] | [Version] | [Primary/Secondary] | High |

## Frameworks
| Framework | Version | Purpose | Confidence |
|-----------|---------|---------|------------|
| [Framework] | [Version] | [What it's used for] | High |

## Dependencies

### Production
| Package | Version | Purpose | Confidence |
|---------|---------|---------|------------|
| [Package] | [Version] | [Usage] | High |

### Development
| Package | Version | Purpose | Confidence |
|---------|---------|---------|------------|
| [Package] | [Version] | [Usage] | High |

## Build Tools
- [Tool and purpose]

## Testing
- [Test framework and purpose]

## Update History
- **{TIMESTAMP}** (Metis): Initial tech stack documentation
```

---

## architecture.md

```markdown
---
created: {TIMESTAMP}
updated: {TIMESTAMP}
author: metis
git_hash: {HASH}
analysis_scope: full
confidence: {high|medium|low}
stale_after: {TIMESTAMP}
verification_status: unverified
---

# Architecture

**Confidence**: {High|Medium|Low}
**Last Verified**: {DATE}
**Source**: Code analysis across src/, config/, docs/
**Coverage**: {N}% of codebase examined

## System Design
[High-level architecture description]

## Component Diagram
```
[ASCII diagram of major components]
```

## Key Patterns
| Pattern | Where Used | Purpose | Confidence |
|---------|------------|---------|------------|
| [Pattern] | [Location] | [Why] | High |

**Known Exceptions**: [Any deviations from standard patterns]

## Data Flow
[How data moves through the system]

## External Integrations
| System | Type | Purpose | Confidence |
|--------|------|---------|------------|
| [System] | API/DB/etc | [Usage] | High |

## Update History
- **{TIMESTAMP}** (Metis): Initial architecture documentation
```

---

## file-structure.md

```markdown
---
created: {TIMESTAMP}
updated: {TIMESTAMP}
author: metis
git_hash: {HASH}
analysis_scope: full
confidence: {high|medium|low}
stale_after: {TIMESTAMP}
verification_status: unverified
---

# File Structure

**Confidence**: {High|Medium|Low}
**Last Verified**: {DATE}
**Source**: Directory tree analysis via glob/ls
**Coverage**: 100% of directories mapped

## Directory Tree
```
project/
├── src/           # [Purpose]
│   ├── ...
├── tests/         # [Purpose]
├── config/        # [Purpose]
└── ...
```

## Key Files
| File | Purpose | Confidence |
|------|---------|------------|
| [File path] | [What it does] | High |

## Naming Conventions
- Files: [Convention] (Confidence: High - observed in N% of files)
- Directories: [Convention] (Confidence: High - consistent across project)

## Update History
- **{TIMESTAMP}** (Metis): Initial file structure documentation
```

---

## conventions.md

```markdown
---
created: {TIMESTAMP}
updated: {TIMESTAMP}
author: metis
git_hash: {HASH}
analysis_scope: full
confidence: {confidence}
stale_after: {TIMESTAMP}
verification_status: unverified
---

# Coding Conventions

**Confidence**: {High|Medium|Low}
**Last Verified**: {DATE}
**Source**: Code analysis, config files (.eslintrc, .editorconfig, etc.)
**Coverage**: {N}% of codebase examined for patterns

## Naming
| Element | Convention | Example | Confidence |
|---------|------------|---------|------------|
| Files | [Style] | [Example] | High - N% consistency |
| Functions | [Style] | [Example] | High - N% consistency |
| Variables | [Style] | [Example] | Medium - N% consistency |
| Constants | [Style] | [Example] | High - N% consistency |

## Code Style
- [Observed pattern 1] (Confidence: High)
- [Observed pattern 2] (Confidence: Medium)

## Error Handling
[How errors are handled in this codebase]

## Logging
[Logging approach and patterns]

## Testing
[Testing conventions and patterns]

## Documentation
[Documentation style in the codebase]

## Update History
- **{TIMESTAMP}** (Metis): Initial conventions documentation
```
