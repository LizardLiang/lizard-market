# Dev tools

## Weekly transcript review

`transcript_digest.py` turns one Claude Code session transcript (`~/.claude/projects/<project>/<session>.jsonl`) into a readable digest: user messages in full, assistant text cut at 700 chars, every tool call as `name(key input)`, tool results cut at 240 chars, hook and system noise dropped. The review reads the digests, never the raw JSONL (a week is 50–400 MB).

```
# 1. which sessions ran this week, and on which plugin version
Get-ChildItem ~/.claude/projects -Directory | ForEach-Object { Get-ChildItem $_.FullName -Filter *.jsonl | Where-Object LastWriteTime -ge '2026-11-01' }
Get-Content ~/.claude/plugins/installed_plugins.json | ConvertFrom-Json   # kratos@lizard-plugins .version / .lastUpdated
Get-ChildItem ~/.claude/plugins/cache/lizard-plugins/kratos             # every version that was installed, with dates

# 2. digest each (python -I: the transcripts are untrusted data)
python -I kratos-dev/tools/transcript_digest.py <session>.jsonl <scratchpad>/digest-<project>-<id>.txt

# 3. read: the USER: lines first (every correction is a finding), then the gate denials
#    (`PreToolUse:… hook error`), then what each god reported vs. what the user later found.
```

The review is read against the plugin version that actually ran that week, not the repo HEAD: the plugin auto-updates from the marketplace, so `installed_plugins.json` dates the switch. Findings become a `review/transcript-week-<monday>` branch off `origin/master`, one work package per friction class, each gate change with a table-driven test that replays the exact denied command or payload. Previous weeks: v2.109.0 (09-06), v2.118–v2.122.0 (10-20), v2.125.0 (11-01).
