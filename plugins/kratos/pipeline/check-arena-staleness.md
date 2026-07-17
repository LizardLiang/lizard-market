---
name: check-arena-staleness
description: Check if Arena is stale and prompt user for refresh decision
---

# Check Arena Staleness Command

This command checks Master Arena staleness based on git commits and prompts the user interactively.

---

## Usage

```bash
# Called automatically at pipeline start
# Or manually:
/kratos:check-arena-staleness
```

---

## Behavior

### Step 1: Read Arena Metadata

```bash
# Extract git hash from Arena
ARENA_HASH=$(grep "git_hash:" .claude/.Arena/project-overview.md | awk '{print $2}')
ARENA_DATE=$(grep "updated:" .claude/.Arena/project-overview.md | awk '{print $2}')
```

### Step 2: Calculate Staleness

```bash
CURRENT_HASH=$(git rev-parse HEAD)
COMMITS_BEHIND=$(git rev-list --count ${ARENA_HASH}..${CURRENT_HASH})
DAYS_OLD=$(( ($(date +%s) - $(date -d "$ARENA_DATE" +%s)) / 86400 ))
```

### Step 3: Determine Severity

```yaml
if commits_behind <= 10:
  severity = "FRESH"
  action = "proceed_without_prompt"

elif commits_behind <= 50:
  severity = "WARNING"
  action = "show_interactive_prompt"

else:
  severity = "CRITICAL"
  action = "show_strong_prompt"
```

### Step 4: Analyze Changed Files

```bash
# Get list of changed files
CHANGED_FILES=$(git diff --name-only ${ARENA_HASH}..${CURRENT_HASH})

# Categorize staleness by Arena section
if echo "$CHANGED_FILES" | grep -q "package.json\|requirements.txt\|Cargo.toml"; then
  STALE_SECTIONS+=("tech-stack.md - dependency changes detected")
fi

if echo "$CHANGED_FILES" | grep -qE "^src/"; then
  # Count directory changes
  NEW_DIRS=$(git diff --name-only ${ARENA_HASH}..${CURRENT_HASH} | grep "^src/" | cut -d'/' -f1-2 | sort -u | wc -l)
  if [ "$NEW_DIRS" -gt 5 ]; then
    STALE_SECTIONS+=("file-structure.md - significant directory changes")
    STALE_SECTIONS+=("architecture.md - potential architectural changes")
  fi
fi
```

### Step 5: Show Interactive Prompt (if needed)

If severity is **WARNING** or **CRITICAL**, use `AskUserQuestion` to present the staleness info and let the user decide:

```
AskUserQuestion(
  question: "ARENA STALENESS DETECTED (<severity>)\n\nYour Master Arena is outdated:\n  Last Updated: <arena_date> (<days_old> days ago)\n  Commits Behind: <commits_behind>\n  Git Hash: <arena_hash_short> → <current_hash_short>\n\nStale Sections:\n  <list stale sections from Step 4>\n\nImpact if you continue without refresh:\n  - Agents will use outdated project context\n  - Risk of design decisions based on old assumptions\n  - Higher token cost (~30-50K extra tokens for verification)\n\nRefresh now takes ~2-3 minutes and ~75K tokens.",
  options: [
    "Refresh Arena Now (Recommended) — Run Metis to update Master Arena before continuing",
    "Continue with Stale Arena — Proceed anyway, agents will verify more",
    "Show Detailed Report — See exactly what changed in those commits"
  ]
)
```

Handle the user's choice as described below.

---

## User Choice Handling

### Choice: "Refresh Arena Now"

Spawn Metis to refresh the Arena:

```
Task(
  subagent_type: "kratos:metis",
  model: "opus",
  prompt: "MISSION: Refresh Master Arena
TARGET: Full project analysis
MODE: FULL_RESEARCH
GIT_HASH: <current_hash>

Analyze the codebase and update ALL Arena documents:
- project-overview.md
- tech-stack.md
- architecture.md
- file-structure.md
- conventions.md

Calculate confidence scores for each section.
Update git_hash metadata to current commit.",
  description: "metis - refresh arena"
)
```

After Metis completes, report refresh summary and return `"proceed_with_fresh_arena"`.

### Choice: "Continue with Stale Arena"

Inform the user of mitigation:
```
Proceeding with stale Arena (as requested).

Mitigation strategy:
  - Feature deltas will capture discoveries
  - You can refresh anytime with /kratos:inquiry (Metis research mode)
```

Return `"proceed_with_stale_arena"`.

### Choice: "Show Detailed Report"

Display the detailed staleness report (see below), then re-prompt the user with the same AskUserQuestion (minus the report option).

---

## Detailed Staleness Report

```
📊 ARENA STALENESS REPORT

Last Arena Update: 2025-11-10 10:30 AM (commit: abc123de)
Current Main Branch: 2026-02-11 04:00 PM (commit: xyz789ab)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📅 TIME ANALYSIS
  Days Since Update: 93 days
  Commits Behind: 127 commits
  Average: 1.4 commits/day

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📝 CHANGES DETECTED

  package.json (MODIFIED 6 times):
    ⚠️ Affects: tech-stack.md
    Changes:
      - Added: stripe@12.3.0, socket.io@4.6.0
      - Updated: react@17.0.2 → 18.2.0
      - Removed: lodash@4.17.21
  
  src/ directory (58 files changed):
    ⚠️ Affects: file-structure.md, architecture.md
    New directories:
      - src/payments/ (15 files)
      - src/notifications/ (8 files)
      - src/websocket/ (6 files)
    Deleted:
      - src/legacy-auth/ (removed in refactor)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 ARENA SECTION STATUS

  project-overview.md         [✓ FRESH]
    - No significant changes to project scope
  
  tech-stack.md               [⚠️ STALE - HIGH PRIORITY]
    - 3 new dependencies
    - 1 major version upgrade (React 17→18)
    - 1 dependency removed
  
  architecture.md             [⚠️ STALE - MEDIUM PRIORITY]
    - 3 new services (payments, notifications, websocket)
    - 1 service removed (legacy-auth)
  
  file-structure.md           [⚠️ STALE - HIGH PRIORITY]
    - 58 files changed in src/
    - 3 new top-level directories
    - 1 directory removed
  
  conventions.md              [✓ MOSTLY FRESH]
    - Minor: New error handling pattern in payments/

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🎯 RECOMMENDATION

Arena refresh is STRONGLY RECOMMENDED due to:
  - High-priority sections outdated (tech-stack, file-structure)
  - Significant architectural changes (3 new services)
  - Major dependency upgrade (React 18)

Estimated Refresh Cost:
  Time: 2-3 minutes
  Tokens: ~75K tokens (~$0.15 in Normal mode, based on typical Opus token usage for a FULL_RESEARCH Metis spawn at standard API pricing)

Proceeding without refresh:
  - Agents will spend extra tokens verifying outdated claims
  - Risk of design decisions based on old architecture
  - Estimated extra cost during pipeline: ~30-50K tokens

Net savings from refreshing: ~$0.05-$0.10 + reduced risk

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Press Enter to return to options...
```

---

## Integration Points

### Called From

- `/kratos:main` command (before starting feature pipeline)
- `/kratos:refresh-arena` command (manual check)
- Kratos orchestrator at pipeline initialization

### Returns

- `"proceed_with_fresh_arena"` - Arena was refreshed, continue normally
- `"proceed_with_stale_arena"` - User chose to continue, set enhanced verification
- `"fresh"` - No staleness detected, proceed normally

---

## Configuration

```yaml
# Thresholds
staleness:
  fresh_threshold: 10        # commits
  warning_threshold: 50      # commits
  time_warning_days: 30
  time_critical_days: 90

# Verification multipliers when stale
verification:
  stale_multiplier: 2.0      # Double verification rate
  trust_threshold: "medium"  # Only trust medium+ confidence
```

---

## Remember

- **Always respect user choice** - Never force refresh
- **Be transparent** - Show exact costs and impacts
- **Track metrics** - Log user choices for analytics
- **No automation** - Only run when explicitly called
