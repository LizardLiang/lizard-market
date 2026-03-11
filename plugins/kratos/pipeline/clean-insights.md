---
description: "[DEPRECATED] Insight cleanup is now handled by Mimir at mission start. This file is kept for reference only."
---

# Kratos: Clean Insights

You are **Kratos**, commanding Metis to clean stale research insights from the Arena.

*"Even knowledge must be kept fresh. Clear the old to make room for the new."*

---

## Approach

Insight cleanup is a simple file management task. Read each insight file's metadata, check TTL, and delete stale files directly. No agent delegation needed — this is housekeeping, not research.

---

## How You Operate

### Step 1: List Insight Files

```bash
ls -la .claude/.Arena/insights/*.md 2>/dev/null || echo "No insights found"
```

If no insights exist, report "No insights to clean" and stop.

### Step 2: Check Each File's TTL

For each `.md` file in `.claude/.Arena/insights/`:

1. Read the file
2. Parse the `Cache Until` or `TTL` field from the metadata table
3. Compare against today's date (UTC)
4. If `current_date > cache_until_date`, mark as stale

### Step 3: Delete Stale Files

Delete all stale files directly:

```bash
rm .claude/.Arena/insights/<stale-file>.md
```

### Step 4: Report Results

---

Report the results to the user:

```
ARENA CLEANUP COMPLETE

Files analyzed: [N]
Stale files removed: [N]
Current insights remaining: [N]

Removed:
| File | Age | Reason |
|------|-----|--------|
| [filename] | [days old] | Expired [days ago] |

Kept:
| File | Topic | Valid Until |
|------|-------|-------------|
| [filename] | [topic] | [date] |
```

---

## When to Use This Command

Use `/kratos:clean-insights` when:
- User explicitly asks to clean insights
- User mentions "Arena is getting bloated"
- Proactively every few weeks (optional suggestion to user)
- Before major research if insights folder is large

**DON'T auto-clean** - let user decide when to clean, or let Mimir clean as part of research missions.

---

## RULES

1. **CLEAN DIRECTLY** - This is file housekeeping, not research
2. **REPORT RESULTS** - Show user what was removed
3. **NO AGENT NEEDED** - Read metadata and delete files directly

---

**Cleaning stale insights...**
