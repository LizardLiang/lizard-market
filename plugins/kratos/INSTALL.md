# Kratos Installation Guide

This guide covers installing Kratos into your Claude Code environment. Follow every step in order.

---

## Prerequisites

| Requirement | Purpose | Check Command |
|-------------|---------|---------------|
| **Claude Code** | Host CLI | `claude --version` |
| **Node.js 18+** | Session tracking hooks | `node --version` |
| **Go 1.25.6+** | Build binary from source (optional; version from `kratos-dev/go/go.mod`) | `go version` |

---

## Step 1: Add the Marketplace

```bash
claude plugin marketplace add https://github.com/LizardLiang/kratos
```

Two repositories are involved: `LizardLiang/kratos` is the runtime-only install and marketplace mirror that this command adds, and `LizardLiang/lizard-market` is the source repository that hosts development and the GitHub Release assets (the prebuilt binaries in Step 3).

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

## Step 4: Hooks (Automatic)

Hooks wire the binary into Claude Code's lifecycle events. They ship with the plugin in `hooks/hooks.json`, and Claude Code loads them when the plugin is enabled. There is nothing to install.

**Upgrading from an old install?** Earlier versions had a `kratos install` command that copied hook scripts to `~/.claude/hooks/kratos/` and registered them in `~/.claude/settings.json`. Those legacy global hooks run beside the plugin hooks, so every event fires twice. Remove them once:

```bash
~/.kratos/bin/kratos uninstall
```

While they remain, session start prints this line:

> Kratos: legacy hook copies are still registered in ~/.claude/settings.json and run alongside the plugin's — run `kratos install` once to remove them.

The message names `kratos install`; both `kratos install` and `kratos uninstall` remove the legacy copies.

### What the Hooks Do

| Hook | Trigger | Action |
|------|---------|--------|
| `UserPromptSubmit` | Every user message | `hook prompt-submit` — detects Kratos keywords and handoffs, injects skill activation |
| `SessionStart` | Claude Code opens | `session-start.cjs` — registers the session ledger, prints `KRATOS_BIN:`, memories and pending items, downloads the binary when missing |
| `SessionEnd` | Claude Code closes | `session-end.cjs` — closes the session ledger row with a one-line summary |
| `PermissionRequest` | Read permission request | Auto-allows Read **only for plugin-root and `~/.kratos/` paths**; all other paths prompt normally |
| `PreToolUse` (Write/Edit/MultiEdit/NotebookEdit/Bash/PowerShell/Agent/Task) | Any write, Bash/PowerShell call or spawn | `hook edit-gate` — inline edit gate: Odysseus plan-only lane, Iris source-file budget; fails open for other agents |
| `PostToolUse` (Agent/Task/Write/Edit/MultiEdit) | After tool completes | `tool-use.cjs` — records agent spawns and file changes |
| `PostToolUse` (Write/Edit) | After a file write | `hook spec-delta-check` — validates a just-written spec delta |
| `SubagentStart` (kratos:*) | Any Kratos agent starts | `path-inject.cjs` — injects resolved `<kratos-bin>` path into prompt |
| `SubagentStart` (ares/hephaestus/hermes) | These agents start | `hook subagent-start` — TODO-first gate + Hermes tier checklist |
| `SubagentStart` (athena/apollo/artemis/hera/cassandra/daedalus) | These agents start | `check --init` — deliverable expectations for the agent's stage |
| `SubagentStop` (ares/hephaestus/hermes/nemesis/athena) | These agents finish | `hook subagent-stop` — deliverable verification gate |
| `SubagentStop` (athena/apollo/artemis/hera/cassandra/daedalus) | These agents finish | `check --verify` — confirms deliverable was written |
| `Stop` | Every assistant turn | `memory-sweep.cjs` — periodic memory sweep reminder |

### Verify the Hooks

Start a new Claude Code session. The session-start output shows a `KRATOS_BIN:` line when the plugin hooks run.

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

## Optional: Build/Test Allowlist for Pipeline Projects

Ares subagents auto-approve file edits (`mode: "acceptEdits"`), but Bash commands still hit permission prompts — a foreground subagent waiting on one looks like a hung pipeline. For projects that run the pipeline often, add an allowlist to the project's `.claude/settings.json` (tailor to your stack; allow only commands you'd approve every time):

```json
{
  "permissions": {
    "allow": [
      "Bash(npx tsc*)",
      "Bash(npm test*)",
      "Bash(npm run build*)",
      "Bash(go test*)",
      "Bash(git status*)",
      "Bash(git diff*)"
    ]
  }
}
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

### 6d. Hooks Active

Start a new Claude Code session. The session-start output shows a `KRATOS_BIN:` line, and no `legacy hook copies are still registered` line.

### 6e. Auto-Activation Works

```bash
claude -p "Kratos, what can you do?"
# → Kratos responds with capabilities, not a generic Claude answer
```

---

## File Locations After Installation

| Location | Purpose |
|----------|---------|
| `~/.claude/plugins/cache/kratos/` | Installed plugin (agents, commands, skills) |
| `~/.kratos/bin/kratos[.exe]` | Go binary — downloaded automatically, or built/copied manually |
| `~/.kratos/bin/.version` | Marker recording the installed binary's version, rewritten from `kratos --version` on each check (auto-download only) |
| `~/.claude/plugins/cache/kratos/.../hooks/hooks.json` | Hook registration — ships with the plugin |
| `~/.kratos/memory.db` | SQLite session database |
| `.claude/.Arena/` | Per-project knowledge base (created by Metis on first run) |
| `.claude/feature/*/` | Per-feature pipeline state (created by Kratos) |

---

## Uninstallation

```bash
# Remove the plugin (its hooks go with it)
claude plugin uninstall kratos@kratos

# Old installs only: remove legacy global hooks from ~/.claude/settings.json
~/.kratos/bin/kratos uninstall

# Full removal (database and binary)
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

1. Restart Claude Code after enabling or updating the plugin — hooks are loaded at session start.
2. Confirm the plugin is enabled: `claude plugin list` shows `kratos`.
3. Confirm the matchers in the plugin's `hooks/hooks.json` name the agent (for example `kratos:ares`).

### Hooks not triggering at all

1. Restart Claude Code after enabling the plugin.
2. Confirm the plugin is enabled: `claude plugin list` shows `kratos`.

### Hooks firing twice

An old install left legacy global hooks in `~/.claude/settings.json`. Run `~/.kratos/bin/kratos uninstall` once to remove them.

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

# 3. Initialize the database (hooks ship with the plugin — nothing to install)
~/.kratos/bin/kratos init
#    Upgrading from an old install? Remove legacy global hooks once:
#    ~/.kratos/bin/kratos uninstall

# 4. Verify
~/.kratos/bin/kratos status
```

Then add the auto-activation block to your CLAUDE.md (see Step 5) if needed.
