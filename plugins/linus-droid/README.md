# Linus-Droid

> "Talk is cheap. Show me the code." — Linus Torvalds

A Claude Code plugin that channels Linus Torvalds' legendary code review style. Brutally honest, technically precise, zero diplomatic fluff.

Built using the [oh-my-claude-sisyphus](https://github.com/Yeachan-Heo/oh-my-claude-sisyphus) architecture pattern with specialized **Skills** and **Agents**.

## Commands

| Command | Description |
|---------|-------------|
| `/linus-droid:judge <path>` | Full review of file/directory |
| `/linus-droid:judge-commit [hash]` | Review changed files in a commit |
| `/linus-droid:judge-staged` | Review staged changes before commit |
| `/linus-droid:judge-block` | Review pasted code block |
| `/linus-droid:scan` | Project-wide duplication scan |
| `/linus-droid:roast <path>` | Maximum brutality mode |

## Git Integration

### Review a Commit

```bash
# Review last commit
/linus-droid:judge-commit

# Review specific commit
/linus-droid:judge-commit abc123
```

### Review Before Committing

```bash
# Stage your changes
git add .

# Review staged changes
/linus-droid:judge-staged

# If approved, commit
git commit -m "your message"
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       LINUS-DROID                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  SKILLS (Behavioral Modes)                                   │
│  ┌─────────────┐ ┌─────────────┐ ┌──────────────────┐       │
│  │ linus-mode  │ │ code-taste  │ │ duplication-hunter│       │
│  └─────────────┘ └─────────────┘ └──────────────────┘       │
│                                                              │
│  AGENTS (Spawned via Task tool)                              │
│  ┌─────────────────┐ ┌─────────────────┐ ┌────────────────┐ │
│  │ project-scanner │ │ duplication-    │ │ linus-reviewer │ │
│  │    (Haiku)      │ │ detector        │ │    (Opus)      │ │
│  │                 │ │   (Sonnet)      │ │                │ │
│  └─────────────────┘ └─────────────────┘ └────────────────┘ │
│                                                              │
│  COMMANDS (User Interface)                                   │
│  ┌────────┐ ┌──────────────┐ ┌────────────┐ ┌───────┐      │
│  │ judge  │ │ judge-commit │ │judge-staged│ │ roast │      │
│  └────────┘ └──────────────┘ └────────────┘ └───────┘      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Workflow

```
/linus-droid:judge-commit
         │
         ▼
┌─────────────────────┐
│ Get changed files   │ ← git diff --name-only
└─────────────────────┘
         │
         ▼
┌─────────────────────┐  ┌─────────────────────┐
│  project-scanner    │  │ duplication-detector │
│     (Haiku)         │  │     (Sonnet)         │
│  [PARALLEL]         │  │  [PARALLEL]          │
└─────────────────────┘  └─────────────────────┘
         │                        │
         └────────────┬───────────┘
                      ▼
         ┌─────────────────────┐
         │ Apply code-taste    │
         │ Deliver verdict     │
         └─────────────────────┘
```

## Installation

### Local Development

```bash
claude --plugin-dir /path/to/linus-droid
```

### From GitHub

```bash
/plugin install username/linus-droid@linus-droid
```

## Skills

| Skill | Purpose |
|-------|---------|
| **linus-mode** | Core persona - brutally honest, technically precise |
| **code-taste** | Good taste principles from Linus' TED 2016 talk |
| **duplication-hunter** | Methodology for finding copy-paste crimes |

## Agents

| Agent | Model | Role |
|-------|-------|------|
| **project-scanner** | Haiku | Fast project mapping |
| **duplication-detector** | Sonnet | Similarity analysis |
| **linus-reviewer** | Opus | Deep architectural review |

## The Linus Philosophy

> "Sometimes you can see a problem in a different way and rewrite it so that a special case goes away and becomes the normal case, and that's good code."

> "If you need more than 3 levels of indentation, you're screwed anyway."

> "Bad code isn't bad because it doesn't work. Bad code is bad because it's hard to understand."

## Plugin Structure

```
linus-droid/
├── .claude-plugin/
│   └── plugin.json
├── agents/
│   ├── linus-reviewer.md
│   ├── project-scanner.md
│   └── duplication-detector.md
├── commands/
│   ├── judge.md
│   ├── judge-commit.md
│   ├── judge-staged.md
│   ├── judge-block.md
│   ├── scan.md
│   └── roast.md
├── skills/
│   ├── linus-mode.md
│   ├── code-taste.md
│   └── duplication-hunter.md
└── README.md
```

## License

MIT

---

*"Complexity is the enemy of security."* — Linus Torvalds
