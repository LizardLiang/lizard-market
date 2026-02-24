---
description: Install Kratos memory hooks globally for session tracking
---

# Kratos: Install Hooks

You are the Kratos hook installer. Your job is to help users install the memory hooks globally so Kratos can track sessions across all projects.

---

## What This Does

The Kratos memory system uses Claude Code hooks to automatically:
- Start a memory session when you open a project
- Track which features you're working on
- Record agent spawns and file changes
- End the session with a summary when you close Claude

After installation, when you start a new session, Kratos will show you context about your last session if there's an incomplete feature.

---

## Installation Process

### Step 1: Check Current Status

First, check if hooks are already installed:

```bash
node plugins/kratos/hooks/install.cjs --status
```

### Step 2: Install Hooks

Run the installer:

```bash
node plugins/kratos/hooks/install.cjs
```

This will:
1. Create `~/.claude/hooks/kratos/` directory
2. Copy hook files and memory system
3. Update `~/.claude/settings.json` with hook configuration

### Step 3: Verify Installation

Run status again to confirm:

```bash
node plugins/kratos/hooks/install.cjs --status
```

You should see:
```
Status: FULLY OPERATIONAL
```

---

## Uninstallation

To remove hooks (database is preserved):

```bash
node plugins/kratos/hooks/install.cjs --uninstall
```

To completely remove all Kratos data:
```bash
node plugins/kratos/hooks/install.cjs --uninstall
rm -rf ~/.kratos
```

---

## Requirements

- **Python 3**: Required for the memory system
- **Node.js**: Already available (you're using Claude Code)

If Python is not available, memory features will be disabled but hooks will still be installed.

---

## File Locations

| Location | Purpose |
|----------|---------|
| `~/.claude/hooks/kratos/` | Hook scripts |
| `~/.claude/settings.json` | Hook configuration |
| `~/.kratos/memory.db` | Session database |
| `~/.kratos/active-session.json` | Current session tracking |

---

## Troubleshooting

### "Python not found"

Install Python 3:
- **Windows**: https://www.python.org/downloads/
- **macOS**: `brew install python3`
- **Linux**: `apt install python3` or `yum install python3`

### "Permission denied"

Make sure you have write access to `~/.claude/` directory.

### Hooks not triggering

1. Restart Claude Code after installation
2. Check that `~/.claude/settings.json` contains the hooks configuration
3. Run `--status` to verify installation

---

## Usage After Installation

Once installed, Kratos memory is automatic. You can use:

- `/kratos:recall` - See your last session context
- `/kratos:recall --global` - See sessions across all projects
- Start any session - Context is injected automatically if there's incomplete work

---

**Ready to install? Run the installer command above.**
