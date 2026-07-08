# Kratos Installation Guide

This guide covers installing Kratos into your Claude Code environment. Follow every step in order.

---

## Prerequisites

| Requirement | Purpose | Check Command |
|-------------|---------|---------------|
| **Claude Code** | Host CLI | `claude --version` |
| **Node.js 18+** | Session tracking hooks | `node --version` |
| **Go 1.21+** | Build binary from source (optional) | `go version` |

---

## Step 1: Add the Marketplace

```bash
claude plugin marketplace add https://github.com/LizardLiang/kratos
```

---

## Step 2: Install the Plugin

This registers Kratos's skills, commands, and agents so Claude Code can discover them.

```bash
# User scope — all projects on this machine (recommended)
claude plugin install kratos@kratos

# Project scope — committed alongside your project
claude plugin install kratos@kratos --scope project

# Local scope — this machine only, not committed
claude plugin install kratos@kratos --scope local
```

After installation the plugin lives at `~/.claude/plugins/cache/kratos/` (user scope).

### Verify

Type `/kratos:` in Claude Code — autocomplete should show `main`, `quick`, `review`, `inquiry`, `status`, and others.

---

## Step 3: Set Up the Binary

The binary provides pipeline state tracking, real timestamps, template retrieval, and quality-gate enforcement. It is **optional** — agents fall back to direct file edits when it is unavailable — but the pipeline works much better with it. Binaries are no longer committed to the plugin; they ship as GitHub Release assets and land in `~/.kratos/bin/`, not the plugin's own `bin/` directory.

### Option A: Automatic (Recommended)

The first Claude Code session after install downloads the binary in the background. `SessionStart` detects there's no plugin-local binary, spawns a detached downloader, and returns immediately (a hook can't block on a ~10MB download). It needs network access once; after that the binary is cached at `~/.kratos/bin/`.

Nothing to run — just start a session, wait a few seconds, then verify:

```bash
~/.kratos/bin/kratos --version
```

If it's not there yet, give it a bit longer (throttled retries every 6h on failure) or fall back to Option B or C.

### Option B: Manual Download

For offline/air-gapped machines: download the raw binary on a machine with network, then copy it over.

| Platform | Release Asset |
|----------|---------------|
| Linux x86_64 | `kratos-linux-amd64` |
| Linux ARM64 | `kratos-linux-arm64` |
| macOS Intel | `kratos-darwin-amd64` |
| macOS Apple Silicon | `kratos-darwin-arm64` |
| Windows x86_64 | `kratos-windows-amd64.exe` |

```bash
mkdir -p ~/.kratos/bin

# Linux/macOS (replace <asset> with the file for your platform, <tag> with e.g. v2.90.0)
curl -L -o ~/.kratos/bin/kratos \
  https://github.com/LizardLiang/lizard-market/releases/download/<tag>/<asset>
chmod +x ~/.kratos/bin/kratos

# Windows
curl -L -o ~/.kratos/bin/kratos.exe \
  https://github.com/LizardLiang/lizard-market/releases/download/<tag>/kratos-windows-amd64.exe
```

### Option C: Build from Source

The Go source is not shipped with the installed plugin (only runtime files are). Clone the repo and build into `~/.kratos/bin/`:

```bash
git clone https://github.com/LizardLiang/lizard-market.git
cd lizard-market/kratos-dev/go
mkdir -p ~/.kratos/bin
go build -ldflags="-s -w" -o ~/.kratos/bin/kratos ./cmd/kratos
# Windows: -o ~/.kratos/bin/kratos.exe
```

### Initialize the Database

```bash
~/.kratos/bin/kratos init
# → Database initialized at ~/.kratos/memory.db
```

---

## Step 4: Install Hooks

Hooks wire the binary into Claude Code's lifecycle events.

```bash
~/.kratos/bin/kratos install
```

This copies hook scripts to `~/.claude/hooks/kratos/` and registers all hooks in `~/.claude/settings.json`.

### What the Hooks Do

| Hook | Trigger | Action |
|------|---------|--------|
| `UserPromptSubmit` | Every user message | `hook prompt-submit` — injects session context |
| `SessionStart` | Claude Code opens | `session-start.cjs` — starts memory session |
| `PermissionRequest` | Read permission request | Auto-allows Read **only for plugin-root and `~/.kratos/` paths**; all other paths prompt normally |
| `PreToolUse` (Bash) | Any Bash call | `hook fix-pm` — rewrites `npm` to detected package manager |
| `PostToolUse` (Task/Write/Edit) | After tool completes | `tool-use.cjs` — records agent spawns and file changes |
| `SubagentStart` (kratos:*) | Any Kratos agent starts | `path-inject.cjs` — injects resolved `<kratos-bin>` path into prompt |
| `SubagentStart` (ares/hephaestus/hermes) | These agents start | `hook subagent-start` — TODO-first gate + Hermes tier checklist |
| `SubagentStart` (athena/apollo/artemis/hera/cassandra/daedalus) | These agents start | `check --init` — prerequisite validation for the agent's stage |
| `SubagentStop` (ares/hephaestus/hermes) | These agents finish | `hook subagent-stop` — deliverable verification gate |
| `SubagentStop` (athena/apollo/artemis/hera/cassandra/daedalus) | These agents finish | `check --verify` — confirms deliverable was written |
| `Stop` | Claude Code closes | `session-end.cjs` — records session summary |

### Verify Hook Installation

```bash
~/.kratos/bin/kratos status
# → Status: FULLY OPERATIONAL
```

---

## Step 5: Enable Auto-Activation

The `kratos:auto` skill handles activation automatically — it triggers on the "Kratos" keyword, god-agent names (Athena, Ares, Metis, etc.), pipeline-phase words (PRD, spec, implementation), and "continue"/"next stage" during active pipelines.

For a belt-and-suspenders guarantee in projects where the skill description alone isn't enough, add this to your project's `CLAUDE.md` (or `~/.claude/CLAUDE.md` for all projects):

```markdown
## Kratos Auto-Activation

When the user mentions "Kratos" anywhere in their message, or addresses a god-agent by name
(Athena, Ares, Metis, Apollo, Artemis, Hermes, Hephaestus, Daedalus, Clio, Mimir, Nemesis,
Themis, Hera, Hades, Cassandra, Prometheus, Ananke), invoke the `kratos:auto` skill using
the Skill tool BEFORE generating any other response.
```

---

## Step 6: Verify the Full Installation

Run these checks in order:

### 6a. Plugin Installed

Type `/kratos:` in Claude Code — autocomplete shows available commands.

### 6b. Binary Works

```bash
~/.kratos/bin/kratos --version
# → kratos version X.X.X
```

### 6c. Database Initialized

```bash
~/.kratos/bin/kratos init
# → Database initialized at ~/.kratos/memory.db (or: already exists)
```

### 6d. Hooks Installed

```bash
~/.kratos/bin/kratos status
# → Status: FULLY OPERATIONAL
```

### 6e. Auto-Activation Works

```bash
claude -p "Kratos, what can you do?"
# → Kratos responds with capabilities, not a generic Claude answer
```

### 6f. Settings.json Has Hooks

```bash
grep -A2 "SubagentStart" ~/.claude/settings.json
# → Should show path-inject.cjs and kratos binary commands
```

---

## File Locations After Installation

| Location | Purpose |
|----------|---------|
| `~/.claude/plugins/cache/kratos/` | Installed plugin (agents, commands, skills) |
| `~/.kratos/bin/kratos[.exe]` | Go binary — downloaded automatically, or built/copied manually |
| `~/.kratos/bin/.version` | Marker tracking the installed binary's version (auto-download only) |
| `~/.claude/hooks/kratos/` | Hook scripts installed by `kratos install` |
| `~/.claude/settings.json` | Hook registration (all event bindings) |
| `~/.kratos/memory.db` | SQLite session database |
| `.claude/.Arena/` | Per-project knowledge base (created by Metis on first run) |
| `.claude/feature/*/` | Per-feature pipeline state (created by Kratos) |

---

## Uninstallation

```bash
# Remove hooks only (preserves database)
~/.kratos/bin/kratos uninstall

# Full removal
~/.kratos/bin/kratos uninstall
rm -rf ~/.kratos
```

Also remove the auto-activation block from your CLAUDE.md files if you added one.

---

## Troubleshooting

### "kratos binary not found" in hook output

Hooks look for the binary in, in order:
1. `${CLAUDE_PLUGIN_ROOT}/bin/kratos` (a dev checkout's own `bin/` directory — release installs don't have this)
2. `~/.kratos/bin/kratos`

On a release install the binary lands in `~/.kratos/bin/` via the automatic background download (Step 3, Option A). If it hasn't finished yet (first session, or offline), you'll see this message until it lands — retry in a few seconds, or install manually (Option B) / build from source (Option C). Ensure the binary is executable. You do **not** need to add it to your system PATH — hooks resolve the path automatically.

### SubagentStart/Stop hooks not triggering

1. Restart Claude Code after hook installation — hooks are loaded at session start.
2. Verify `~/.claude/settings.json` contains `SubagentStart` and `SubagentStop` entries with `kratos` in the command paths.
3. Run `~/.kratos/bin/kratos status` to confirm hooks are registered.

### Hooks not triggering at all

1. Restart Claude Code after hook installation.
2. Verify `~/.claude/settings.json` has hook entries.
3. Run `~/.kratos/bin/kratos status`.

### Kratos doesn't activate when called by name

1. Confirm the `kratos:auto` skill is listed in Claude Code's available skills.
2. Add the manual CLAUDE.md block from Step 5 as a fallback.
3. Test: `claude -p "Kratos, what can you do?"`

### Database errors

```bash
rm ~/.kratos/memory.db
~/.kratos/bin/kratos init
```

---

## Quick Install (Copy-Paste)

```bash
# 1. Add marketplace + install plugin
claude plugin marketplace add https://github.com/LizardLiang/kratos
claude plugin install kratos@kratos

# 2. Start a Claude Code session — the binary downloads to ~/.kratos/bin/
#    automatically in the background. Wait a few seconds, then verify:
~/.kratos/bin/kratos --version

# 3. Initialize database + install hooks
~/.kratos/bin/kratos init && ~/.kratos/bin/kratos install

# 4. Verify
~/.kratos/bin/kratos status
```

Then add the auto-activation block to your CLAUDE.md (see Step 5) if needed.
