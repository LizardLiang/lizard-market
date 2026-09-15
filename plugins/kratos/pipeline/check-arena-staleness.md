---
name: check-arena-staleness
description: Detect a stale Arena from its frontmatter and refresh the affected shard via Metis
---

# Check Arena Staleness

Kratos runs this before a complex task (`commands/main.md` Step 1). No `.claude/.Arena/project/overview.md` → skip; Stage 0 bootstraps the Arena.

1. Read `git_hash`, `updated`, `stale_after` (`updated` + 30 days) from the frontmatter of `.claude/.Arena/project/overview.md`.
2. Compare: `git rev-parse HEAD` differs from `git_hash` → run `git diff --name-only <git_hash>..HEAD`; today is past `stale_after` → stale on time alone. Both fresh → report "Arena fresh" and continue.
3. Map changes to shards (layout: `project/overview.md`, `tech-stack/<layer>.md`, `architecture/system-design.md`, `architecture/file-structure.md`, `conventions/<domain>.md`):
   - dependency manifests changed → `tech-stack/<layer>.md`
   - more than 5 source directories added or removed → both `architecture/` shards
   - time-only staleness → `project/overview.md`
4. Tell the user which shards are stale and why (one line each), then spawn Metis:

```
Task(
  subagent_type: "kratos:metis",
  model: "sonnet",
  prompt: "MISSION: Targeted Research
MODE: TARGETED_RESEARCH
TARGET: [stale shard path(s)]
REASON: [changed files or time-based staleness]

Update only these shards. Set frontmatter git_hash to HEAD, updated to now, stale_after to now + 30 days.",
  description: "metis - refresh stale arena shard"
)
```

After Metis completes, continue with the refreshed Arena.
