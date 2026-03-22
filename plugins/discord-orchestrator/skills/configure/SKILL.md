---
name: configure
description: Set up the Discord orchestrator — save bot token and configure ops channel
user_invocable: true
---

# Discord Orchestrator Setup

Help the user set up the Discord orchestrator.

## Steps

1. Ask for the Discord bot token if not already saved
2. Save it to `~/.discord-orchestrator/.env` as `DISCORD_BOT_TOKEN=<token>` with 0600 permissions
3. Ask for the ops channel ID (the Discord channel where admin commands will go)
4. Update `~/.discord-orchestrator/projects.json` with the `ops_channel` value
5. Confirm setup is complete and explain how to start the bot: `cd plugins/discord-orchestrator && bun server.ts`

## Notes

- The bot token file must have 0600 permissions (owner read/write only)
- Create `~/.discord-orchestrator/` directory if it doesn't exist
- Never display the full bot token back to the user
