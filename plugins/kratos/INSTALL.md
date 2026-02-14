# Kratos Installation Guide

This guide is for AI agents (Claude Code, etc.) installing Kratos into a user's environment. Follow every step in order. Do not skip steps.

---

## Prerequisites

| Requirement | Purpose | Check Command |
|-------------|---------|---------------|
| **Claude Code** | Host CLI | `claude --version` |
| **Node.js 18+** | Hook scripts | `node --version` |
| **Go 1.21+** | Memory binary (build from source) | `go version` |
| **Git** | Clone the marketplace | `git --version` |

**Optional:**
- **Python 3** - Legacy memory system fallback. Not required if Go binary is built.

---

## Step 1: Clone the Marketplace

```bash
git clone <marketplace-repo-url> ~/ai-agents/lizard-market
```

Kratos lives at `plugins/kratos/` inside the marketplace repo.

---

## Step 2: Build the Kratos Binary

The memory system requires a compiled Go binary. Pre-built binaries exist in `plugins/kratos/bin/` for common platforms, but building from source is recommended.

### Build from Source

```bash
cd plugins/kratos/go
go build -ldflags="-s -w" -o ../bin/kratos ./cmd/kratos
```

### Or Use Pre-built Binaries

| Platform | Binary |
|----------|--------|
| Linux x86_64 | `bin/kratos-linux-amd64` |
| Linux ARM64 | `bin/kratos-linux-arm64` |
| macOS Intel | `bin/kratos-darwin-amd64` |
| macOS Apple Silicon | `bin/kratos-darwin-arm64` |
| Windows x86_64 | `bin/kratos-windows-amd64.exe` |

Copy the appropriate binary to a location in your PATH:

```bash
# Linux/macOS
cp plugins/kratos/bin/kratos-$(uname -s | tr A-Z a-z)-$(uname -m) ~/bin/kratos
chmod +x ~/bin/kratos

# Windows
copy plugins\kratos\bin\kratos-windows-amd64.exe %USERPROFILE%\bin\kratos.exe
```

### Verify

```bash
kratos --version
kratos init        # Creates ~/.kratos/memory.db
```

---

## Step 3: Install Memory Hooks

Hooks enable automatic session tracking (start, tool use, end). The kratos binary handles its own installation.

```bash
# From the plugins/kratos directory:
./bin/kratos install
```

This does three things:
1. Copies hook scripts to `~/.claude/hooks/kratos/`
2. Copies the kratos binary to `~/.claude/hooks/kratos/kratos`
3. Registers hooks in `~/.claude/settings.json`

### Verify Hook Installation

```bash
./bin/kratos status
```

Expected output: `Status: FULLY OPERATIONAL`

### What the Hooks Do

| Hook | Trigger | Action |
|------|---------|--------|
| `SessionStart` | Claude Code opens | Starts memory session, injects last session context |
| `PostToolUse` | After Task/Write/Edit | Records agent spawns and file changes |
| `Stop` | Claude Code closes | Ends session with summary |

---

## Step 4: Enable Auto-Activation

This is the critical step that makes Kratos respond when called by name.

### Option A: Project-Level (Recommended)

Add this to your project's `CLAUDE.md` (or `.claude/CLAUDE.md`):

```markdown
## Kratos Auto-Activation

When the user mentions "Kratos" anywhere in their message, invoke the `kratos:main` skill
using the Skill tool BEFORE generating any other response. This applies to:
- "Kratos, [task]" / "Hey Kratos" / "Ask Kratos" / "Summon Kratos"
- Any sentence containing "Kratos" directed at the orchestrator
- Mentioning a god-agent by name with a task (e.g., "Athena, write a PRD")
```

### Option B: Global (All Projects)

Add the same block to `~/.claude/CLAUDE.md` so Kratos activates in every project.

### Why This Is Needed

Claude Code decides whether to invoke a skill based on the **command description** in its system prompt. The `kratos:main` command description already includes trigger keywords, but adding explicit instructions to CLAUDE.md provides a redundant guarantee that Kratos always activates when addressed.

---

## Step 5: Verify the Full Installation

Run these checks in order:

### 5a. Binary Works

```bash
kratos --version
# Expected: kratos version X.X.X (or "dev")
```

### 5b. Database Initialized

```bash
kratos init
# Expected: Database initialized at ~/.kratos/memory.db (or already exists)
```

### 5c. Hooks Installed

```bash
kratos status
# Expected: Status: FULLY OPERATIONAL
```

### 5d. Auto-Activation Works

```bash
claude -p "Kratos, what can you do?"
# Expected: Kratos responds with capabilities overview (not a generic Claude response)
```

### 5e. Settings.json Has Hooks

Check that `~/.claude/settings.json` contains hook entries with `kratos` in the command paths:

```json
{
  "hooks": {
    "SessionStart": [{ "hooks": [{ "command": "node \"...kratos/session-start.cjs\"" }] }],
    "PostToolUse": [{ "matcher": "Task|Write|Edit", "hooks": [{ "command": "node \"...kratos/tool-use.cjs\"" }] }],
    "Stop": [{ "hooks": [{ "command": "node \"...kratos/session-end.cjs\"" }] }]
  }
}
```

---

## File Locations After Installation

| Location | Purpose |
|----------|---------|
| `plugins/kratos/` | Plugin source (agents, commands, skills) |
| `plugins/kratos/bin/kratos` | Compiled Go binary |
| `~/.claude/hooks/kratos/` | Installed hook scripts |
| `~/.claude/settings.json` | Hook registration |
| `~/.kratos/memory.db` | Session database (SQLite) |
| `~/.kratos/active-session.json` | Current session tracking |
| `.claude/.Arena/` | Per-project knowledge base (created by Metis) |
| `.claude/feature/*/` | Per-feature pipeline state (created by Kratos) |

---

## Uninstallation

### Remove Hooks Only (Preserve Database)

```bash
kratos uninstall
```

### Full Removal

```bash
kratos uninstall
rm -rf ~/.kratos                    # Delete memory database
```

Also remove the auto-activation block from your CLAUDE.md files.

---

## Troubleshooting

### "Kratos binary not found"

The hook scripts search these locations in order:
1. `kratos` in PATH
2. `plugins/kratos/bin/kratos` (relative to plugin)
3. `~/bin/kratos`

Ensure the binary is in one of these locations, or add it to your PATH.

### Hooks Not Triggering

1. Restart Claude Code after hook installation
2. Verify `~/.claude/settings.json` contains the hook configuration
3. Run `kratos status`

### Kratos Doesn't Activate When Called by Name

1. Verify the `commands/main.md` description contains trigger keywords ("Use when the user mentions Kratos")
2. Add the auto-activation block to your CLAUDE.md
3. Test with `claude -p "Kratos, what can you do?"`

### "Python not found" Warning

This is non-critical. The Go binary handles all memory operations. Python is only needed for the legacy memory system. You can safely ignore this warning.

### Database Errors

```bash
# Re-initialize the database
rm ~/.kratos/memory.db
kratos init
```

---

## Quick Install (Copy-Paste)

For experienced users, here's the full installation in one block:

```bash
# 1. Build binary
cd plugins/kratos/go && go build -ldflags="-s -w" -o ../bin/kratos ./cmd/kratos && cd ..

# 2. Initialize database
./bin/kratos init

# 3. Install hooks + binary to ~/.claude/hooks/kratos/
./bin/kratos install

# 4. Verify
./bin/kratos status
```

Then add the auto-activation block to your CLAUDE.md (see Step 4).