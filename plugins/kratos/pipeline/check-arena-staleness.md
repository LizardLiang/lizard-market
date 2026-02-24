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

```python
if severity in ["WARNING", "CRITICAL"]:
    user_choice = AskUserQuestion(
        question=f"""
⚠️ ARENA STALENESS DETECTED ({severity})

Your Master Arena is outdated:
  📅 Last Updated: {arena_date} ({days_old} days ago)
  📊 Commits Behind: {commits_behind} commits (>{threshold} threshold)
  🔀 Git Hash: {arena_hash[:8]} → {current_hash[:8]}

Stale Sections:
{format_stale_sections(stale_sections)}

Impact if you continue without refresh:
  ⚠️ Agents will use outdated project context
  ⚠️ Risk of design decisions based on old assumptions
  ⚠️ Higher token cost during pipeline (more verification needed)
  ⚠️ Estimated extra cost: ~30-50K tokens

Refresh Arena Now ({"strongly recommended" if severity == "CRITICAL" else "recommended"}):
  ✓ Ensures accurate foundation for your feature
  ✓ Reduces verification overhead in pipeline
  ✓ Time: ~2-3 minutes
  ✓ Cost: ~75K tokens (~$0.15 in Normal mode)
  ✓ Net savings: ~$0.05-$0.10 + reduced risk

What would you like to do?
        """,
        options=[
            {
                "label": "Refresh Arena Now (Recommended)",
                "value": "refresh",
                "description": "Run Metis to update Master Arena before continuing"
            },
            {
                "label": "Continue with Stale Arena",
                "value": "continue",
                "description": "Proceed anyway - agents will verify more (higher cost)"
            },
            {
                "label": "Show Detailed Report",
                "value": "report",
                "description": f"See exactly what changed in those {commits_behind} commits"
            }
        ]
    )
    
    handle_user_choice(user_choice)
```

---

## User Choice Handling

### Choice: "refresh"

```python
print("⚔️ KRATOS: Summoning METIS to refresh Arena...")
print()

# Spawn Metis for full Arena refresh
spawn_task(
    subagent_type="kratos:metis",
    prompt="""
MISSION: Refresh Master Arena

TARGET: Full project analysis
MODE: FULL_RESEARCH
GIT_HASH: {current_hash}

Analyze the codebase and update ALL Arena documents:
- project-overview.md
- tech-stack.md
- architecture.md
- file-structure.md
- conventions.md

Calculate confidence scores for each section.
Update git_hash metadata to current commit.
    """,
    description="metis - refresh arena"
)

print("✅ Arena refreshed successfully!")
print(f"   Updated from {arena_hash[:8]} to {current_hash[:8]}")
print(f"   All 5 Arena documents are now current")
print()
return "proceed_with_fresh_arena"
```

### Choice: "continue"

```python
print("⚠️ Proceeding with stale Arena (as requested)")
print()
print("Mitigation strategy:")
print("  • Agents will double verification rates (10% → 20%)")
print("  • Feature deltas will capture discoveries")
print("  • You can refresh anytime: /kratos:refresh-arena")
print()

# Set enhanced verification for this pipeline
set_pipeline_config("verification_multiplier", 2.0)
set_pipeline_config("trust_threshold", "medium_or_higher")

return "proceed_with_stale_arena"
```

### Choice: "report"

```python
show_detailed_staleness_report(arena_hash, current_hash, changed_files)

# Ask again after showing report
return check_arena_staleness()
```

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
  Tokens: ~75K tokens (~$0.15 in Normal mode)

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
