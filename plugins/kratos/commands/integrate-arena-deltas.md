---
name: integrate-arena-deltas
description: Integrate feature deltas into Master Arena after feature merges to main
---

# Integrate Arena Deltas Command

After a feature merges to main, this command integrates the feature's arena-deltas.md into the Master Arena documents.

---

## Usage

```bash
# After merging feature to main
$ git merge feature/payments
$ git push

# Integrate deltas (manual, user-initiated)
$ /kratos:integrate-arena-deltas payments

# Or with full feature path
$ /kratos:integrate-arena-deltas feature/payments
```

---

## Behavior

### Step 1: Validate Prerequisites

```yaml
Checks:
  - Feature delta file exists: .claude/feature/{name}/arena-deltas.md
  - Currently on main branch (or specified base branch)
  - Feature branch has been merged (commits are in main)
  - No uncommitted changes in Arena files

If any check fails:
  - Show clear error message
  - Exit without changes
```

### Step 2: Read Feature Deltas

```python
delta_file = f".claude/feature/{feature_name}/arena-deltas.md"
delta_content = read_file(delta_file)

# Parse delta sections
deltas = parse_delta_sections(delta_content)
# Returns: {
#   "external_research": {...},
#   "codebase_discoveries": {...},
#   "architecture_validation": {...},
#   "implementation_details": {...},
#   "code_review_notes": {...},
#   "integration_checklist": {...}
# }
```

### Step 3: Update Master Arena Documents

For each Arena document that needs updates:

#### Update tech-stack.md

```python
if deltas["integration_checklist"]["tech-stack.md"]:
    arena_file = ".claude/.Arena/tech-stack.md"
    
    # Read current content
    current_content = read_file(arena_file)
    
    # Add new dependencies
    for dep in deltas["codebase_discoveries"]["dependencies_added"]:
        add_dependency_to_arena(arena_file, dep)
    
    # Update metadata
    update_arena_metadata(arena_file, {
        "updated": current_timestamp(),
        "last_updated_by": "kratos-integration",
        "source": f"feature/{feature_name} deltas"
    })
    
    # Add to update history
    append_to_section(arena_file, "Update History", 
        f"- **{timestamp()}** (Integration): Added dependencies from {feature_name} feature"
    )
```

#### Update architecture.md

```python
if deltas["integration_checklist"]["architecture.md"]:
    arena_file = ".claude/.Arena/architecture.md"
    
    # Add new services/patterns
    for service in deltas["codebase_discoveries"]["architecture_changes"]:
        add_architecture_element(arena_file, service)
    
    # Update metadata
    update_arena_metadata(arena_file, {...})
    
    # Add to update history
    append_to_section(arena_file, "Update History",
        f"- **{timestamp()}** (Integration): Added {feature_name} architectural changes"
    )
```

#### Update file-structure.md

```python
if deltas["integration_checklist"]["file-structure.md"]:
    arena_file = ".claude/.Arena/file-structure.md"
    
    # Add new directories
    for directory in deltas["codebase_discoveries"]["new_directories"]:
        add_directory_to_structure(arena_file, directory)
    
    # Update key files if needed
    for file in deltas["codebase_discoveries"]["new_files"]:
        if is_key_file(file):
            add_to_key_files(arena_file, file)
    
    # Update metadata and history
    update_arena_metadata(arena_file, {...})
    append_to_section(arena_file, "Update History", ...)
```

#### Update conventions.md

```python
if deltas["integration_checklist"]["conventions.md"]:
    arena_file = ".claude/.Arena/conventions.md"
    
    # Add any new conventions discovered
    for convention in deltas["implementation_details"]["conventions"]:
        add_convention(arena_file, convention)
    
    # Update metadata and history
    update_arena_metadata(arena_file, {...})
    append_to_section(arena_file, "Update History", ...)
```

#### Update project-overview.md (if needed)

```python
if deltas["integration_checklist"]["project-overview.md"]:
    arena_file = ".claude/.Arena/project-overview.md"
    
    # Only update if feature significantly changes project scope
    if is_significant_change(deltas):
        update_project_scope(arena_file, deltas)
    
    # Always update metadata
    update_arena_metadata(arena_file, {
        "updated": current_timestamp(),
        "git_hash": current_git_hash()
    })
```

### Step 4: Update All Arena Git Hashes

```python
# Update git_hash in all Arena documents to current commit
current_hash = git("rev-parse HEAD")

for arena_file in glob(".claude/.Arena/*.md"):
    update_arena_metadata(arena_file, {
        "git_hash": current_hash,
        "updated": current_timestamp()
    })
```

### Step 5: Commit Arena Changes

```python
# Stage all Arena updates
git("add .claude/.Arena/")

# Create descriptive commit
commit_message = f"""Arena: Integrate {feature_name} feature discoveries

Integrated from: .claude/feature/{feature_name}/arena-deltas.md

Changes:
{format_changes_summary(deltas)}

Updated Arena documents:
{list_updated_files()}
"""

git(f'commit -m "{commit_message}"')

print(f"✅ Arena updated with {feature_name} discoveries")
print(f"   Commit: {git('rev-parse --short HEAD')}")
```

### Step 6: Delete Feature Delta File

```python
# Delta has been integrated, remove it
delta_file = f".claude/feature/{feature_name}/arena-deltas.md"
os.remove(delta_file)

print(f"🗑️ Removed {delta_file} (now in Master Arena)")
```

### Step 7: Summary Output

```
✅ ARENA INTEGRATION COMPLETE

Feature: {feature_name}
Base Arena Hash: {old_hash}
Updated Arena Hash: {new_hash}

Documents Updated:
  ✓ tech-stack.md - Added {N} dependencies
  ✓ architecture.md - Documented {N} new services
  ✓ file-structure.md - Added {N} directories
  ○ conventions.md - No changes needed
  ○ project-overview.md - No changes needed

Changes Committed:
  Commit: {commit_hash}
  Message: "Arena: Integrate {feature_name} feature discoveries"

Cleanup:
  ✓ Removed {delta_file}

Next Steps:
  - Arena is now current for main branch
  - Future features will use updated Arena
  - Consider running /kratos:refresh-arena monthly
```

---

## Error Handling

### Delta File Not Found

```
❌ ERROR: Feature delta file not found

Expected: .claude/feature/{feature_name}/arena-deltas.md
Status: Not found

Possible causes:
  1. Feature name is incorrect
  2. Feature didn't create delta file (no discoveries)
  3. Delta already integrated and removed

Try:
  - Check feature name spelling
  - List available features: ls .claude/feature/
  - Verify feature had arena-deltas.md before merge
```

### Not on Main Branch

```
❌ ERROR: Not on main branch

Current branch: {current_branch}
Expected: main

Action required:
  $ git checkout main
  $ git pull
  $ /kratos:integrate-arena-deltas {feature_name}
```

### Uncommitted Changes

```
❌ ERROR: Uncommitted changes in Arena

Modified Arena files:
  {list_modified_files()}

Action required:
  Commit or stash changes before integration
  $ git stash
  $ /kratos:integrate-arena-deltas {feature_name}
```

---

## Dry Run Mode

```bash
# Preview what would be integrated without making changes
$ /kratos:integrate-arena-deltas payments --dry-run

📋 DRY RUN: Integration Preview

Feature: payments
Delta File: .claude/feature/payments/arena-deltas.md

Planned Changes:

tech-stack.md:
  + Add dependency: stripe@12.3.0
  + Add dependency: @stripe/stripe-js@1.54.0

architecture.md:
  + Add service: Payment Processing Service
  + Add pattern: Webhook handler for Stripe events

file-structure.md:
  + Add directory: src/payments/ (15 files)
  + Add key file: src/payments/stripe-client.ts

conventions.md:
  No changes needed

project-overview.md:
  No changes needed

Metadata Updates:
  - All Arena files git_hash: abc123de → xyz789ab
  - Updated timestamp: 2026-02-11T16:30:00Z

Commit Message:
  "Arena: Integrate payments feature discoveries"

To execute: /kratos:integrate-arena-deltas payments
```

---

## Integration Strategies

### Conflict Resolution

If delta contradicts existing Arena content:

```python
if has_conflict(arena_content, delta_content):
    print(f"⚠️ Conflict detected in {arena_file}")
    print(f"   Arena says: {arena_value}")
    print(f"   Delta says: {delta_value}")
    print()
    print("Resolution: Using delta value (most recent)")
    print("   Reason: Feature delta is from actual implementation")
    
    # Update Arena with delta value
    resolve_conflict(arena_file, delta_value, reason="feature_implementation")
    
    # Document in Update History
    append_to_section(arena_file, "Update History",
        f"- **{timestamp()}** (Integration): Resolved conflict - updated from '{arena_value}' to '{delta_value}' based on {feature_name} implementation"
    )
```

### Additive vs Replacement

```python
# Additive changes (add to existing)
- New dependencies
- New directories
- New patterns

# Replacement changes (override existing)
- Version updates
- Removed dependencies
- Refactored services

# Handle appropriately
if change_type == "additive":
    append_to_arena(arena_file, new_content)
elif change_type == "replacement":
    replace_in_arena(arena_file, old_content, new_content)
```

---

## Remember

- **Manual only** - No automatic integration on merge
- **User-initiated** - User runs command when ready
- **Transparent** - Show exactly what's being changed
- **Safe** - Validate prerequisites before making changes
- **Traceable** - Commit with detailed message
- **Clean** - Remove delta file after integration
