# discord-remote

Remote tool approval and chat for Claude Code via Discord.
Approve or deny Claude Code's permission requests from your phone with emoji reactions, and chat with Claude from Discord — no terminal required.

Discord 遠端審批與聊天，透過 Claude Code。
透過手機上的 Discord 表情符號反應來批准或拒絕 Claude Code 的權限請求，也可以從 Discord 直接與 Claude 對話——無需終端機。

---

## Table of Contents / 目錄

- [What It Does / 功能說明](#what-it-does--功能說明)
- [Prerequisites / 前置需求](#prerequisites--前置需求)
- [Quick Start / 快速開始](#quick-start--快速開始)
- [Auto-Approve / 自動批准](#auto-approve--自動批准)
- [Configuration / 設定](#configuration--設定)
- [Skills / 技能指令](#skills--技能指令)
- [Architecture / 架構](#architecture--架構)
- [Security / 安全性](#security--安全性)
- [Timeout Behavior / 逾時行為](#timeout-behavior--逾時行為)
- [Relationship to Official Plugin / 與官方插件的關係](#relationship-to-official-plugin--與官方插件的關係)
- [Troubleshooting / 疑難排解](#troubleshooting--疑難排解)

---

## What It Does / 功能說明

### English

This plugin does **two things**:

**1. Chat with Claude from Discord** — DM the bot with tasks like "fix the bug in auth.ts" and Claude works on it, replying through Discord. Full two-way messaging, just like the official Discord channel plugin.

**2. Remote permission approval** — When Claude needs to run a tool (write a file, execute a command), instead of blocking at the terminal:

- **PermissionRequest** — Sends a Discord DM with the tool name and input summary. Three reactions:
  - ✅ Approve (one-time)
  - ❌ Deny
  - 🔒 Always Approve (auto-approves this tool for the rest of the session)
- **PreToolUse** — Checks tool invocations against a configurable deny list. Matching patterns are blocked immediately without contacting Discord.

If you don't respond within the timeout, the plugin falls back to the terminal prompt (configurable).

### 繁體中文

此插件做**兩件事**：

**1. 從 Discord 與 Claude 對話** — 私訊機器人任務如「修復 auth.ts 的 bug」，Claude 會處理並透過 Discord 回覆。完整的雙向訊息，就像官方 Discord 頻道插件一樣。

**2. 遠端權限審批** — 當 Claude 需要執行工具（寫入檔案、執行指令），不會在終端機等待：

- **權限請求（PermissionRequest）** — 發送 Discord 私訊，顯示工具名稱和輸入摘要。三個表情符號：
  - ✅ 批准（單次）
  - ❌ 拒絕
  - 🔒 永遠批准（本次工作階段自動批准此工具）
- **預檢工具（PreToolUse）** — 依據設定的拒絕清單檢查工具呼叫，符合的模式會立即攔截。

若在逾時時間內未回應，插件會回退到終端機提示（可設定）。

---

## Prerequisites / 前置需求

| Requirement / 需求 | Details / 詳細 |
|---|---|
| Claude Code >= v2.1.80 | Channels support required / 需要頻道支援 |
| Discord Bot Token | From [Discord Developer Portal](https://discord.com/developers/applications) / 從 [Discord 開發者入口](https://discord.com/developers/applications) 取得 |
| Bun | Runtime for the MCP server / MCP 伺服器的執行環境。安裝：`curl -fsSL https://bun.sh/install \| bash` |
| Node.js >= 18 | For hook scripts (cross-platform, zero npm deps) / 用於 hook 腳本（跨平台，無需 npm 套件） |
| Discord account | Your personal account to pair with the bot / 你的個人帳號，用於與機器人配對 |

---

## Quick Start / 快速開始

### Step 1: Create a Discord Bot / 步驟一：建立 Discord 機器人

**English:**

1. Go to [Discord Developer Portal](https://discord.com/developers/applications) → **New Application**
2. Navigate to **Bot** → give it a username
3. Under **Privileged Gateway Intents**, enable:
   - **Message Content Intent**
   - **Server Members Intent** (optional, for guild channels)
4. Scroll up to **Token** → **Reset Token** → copy it (shown only once)
5. Navigate to **OAuth2** → **URL Generator**:
   - Scope: `bot`
   - Bot Permissions: View Channels, Send Messages, Send Messages in Threads, Read Message History, Attach Files, Add Reactions
   - Integration type: Guild Install
6. Copy the generated URL, open it, add the bot to a server you're in
7. (Recommended) Turn off **Public Bot** toggle so only you can add it to servers

**繁體中文：**

1. 前往 [Discord 開發者入口](https://discord.com/developers/applications) → **New Application**
2. 進入 **Bot** → 設定使用者名稱
3. 在 **Privileged Gateway Intents** 下啟用：
   - **Message Content Intent**
   - **Server Members Intent**（選用，用於伺服器頻道）
4. 往上捲到 **Token** → **Reset Token** → 複製（只顯示一次）
5. 進入 **OAuth2** → **URL Generator**：
   - 範圍：`bot`
   - 機器人權限：View Channels、Send Messages、Send Messages in Threads、Read Message History、Attach Files、Add Reactions
   - 整合類型：Guild Install
6. 複製產生的 URL，打開並將機器人加入你所在的伺服器
7. （建議）關閉 **Public Bot** 開關

### Step 2: Install the Plugin / 步驟二：安裝插件

The plugin must be registered in a Claude Code marketplace. If your marketplace already has it:

插件必須在 Claude Code 市場中註冊。如果你的市場已經有了：

```
/plugin install discord-remote@<your-marketplace>
```

### Step 3: Configure the Bot Token / 步驟三：設定機器人令牌

Start a Claude Code session with the channel. During the research preview, custom channels require a special flag:

啟動帶頻道的 Claude Code 工作階段。在研究預覽期間，自訂頻道需要特殊旗標：

```bash
claude --dangerously-load-development-channels plugin:discord-remote@<your-marketplace>
```

Then configure the token / 然後設定令牌：

```
/discord-remote:configure <your-bot-token>
```

The token is saved to `~/.claude/channels/discord/.env` with `0600` permissions.

令牌會以 `0600` 權限儲存於 `~/.claude/channels/discord/.env`。

### Step 4: Restart with the Channel / 步驟四：重新啟動帶頻道

Exit and restart — the bot token is only read at boot:

退出並重新啟動——機器人令牌只在啟動時讀取：

```bash
claude --dangerously-load-development-channels plugin:discord-remote@<your-marketplace>
```

### Step 5: Pair Your Discord Account / 步驟五：配對你的 Discord 帳號

1. DM your bot on Discord — it replies with a 6-character pairing code
2. In Claude Code: `/discord-remote:access pair <code>`
3. Lock it down: `/discord-remote:access policy allowlist`

步驟：

1. 在 Discord 上私訊你的機器人——它會回覆一個 6 位配對碼
2. 在 Claude Code 中：`/discord-remote:access pair <code>`
3. 鎖定存取：`/discord-remote:access policy allowlist`

### Step 6: Test / 步驟六：測試

**Chat test / 聊天測試：** DM the bot with "hello" — Claude should reply through Discord.

**Approval test / 審批測試：** Ask Claude to write a file. You should receive a Discord DM like:

私訊機器人「hello」—— Claude 應透過 Discord 回覆。然後請 Claude 寫入檔案，你會收到：

> **[Permission Request]** `Write`
>
> **Input:**
> ```
> { "file_path": "/src/app.ts" }
> ```
>
> React: ✅ = approve | ❌ = deny | 🔒 = always approve
> _Timeout: 60s_

Tap ✅ on your phone. Claude continues.

---

## Auto-Approve / 自動批准

### English

When you tap 🔒 (always approve) on a permission request, the sidecar remembers that tool for the rest of the session. Future requests for the same tool are auto-approved instantly — no Discord DM needed.

**Safety restriction:** Dangerous tools are **never** auto-approved, even with 🔒:

| Tool | Auto-approve? | Why |
|---|---|---|
| `Read`, `Glob`, `Grep`, `Agent` | ✅ Yes | Read-only or delegated |
| `Bash` | ❌ Never | Different commands carry different risk |
| `Write` | ❌ Never | Different files carry different risk |
| `Edit` | ❌ Never | Different edits carry different risk |
| `NotebookEdit` | ❌ Never | Same as Edit |

Clicking 🔒 on `Bash` still approves **that specific call**, but the next `Bash` call will prompt again.

The auto-approve list is **in-memory only** — it resets when the session ends. No persistence across sessions.

### 繁體中文

當你點擊 🔒（永遠批准）時，側車會記住該工具直到工作階段結束。之後相同工具的請求會立即自動批准——不需 Discord 私訊。

**安全限制：** 危險工具**永遠不會**自動批准，即使點了 🔒：

| 工具 | 自動批准？ | 原因 |
|---|---|---|
| `Read`、`Glob`、`Grep`、`Agent` | ✅ 是 | 唯讀或委派 |
| `Bash` | ❌ 永不 | 不同指令有不同風險 |
| `Write` | ❌ 永不 | 不同檔案有不同風險 |
| `Edit` | ❌ 永不 | 不同編輯有不同風險 |
| `NotebookEdit` | ❌ 永不 | 同 Edit |

在 `Bash` 上點 🔒 仍會批准**該次呼叫**，但下次 `Bash` 呼叫會再次提示。

自動批准清單**僅存在記憶體中**——工作階段結束時重置。不跨工作階段持久化。

---

## Configuration / 設定

All configuration lives at `~/.claude/channels/discord/remote-config.json`.

所有設定存放於 `~/.claude/channels/discord/remote-config.json`。

```json
{
  "sidecar": {
    "port": 19275,
    "host": "127.0.0.1"
  },
  "timeout": {
    "approval_ms": 60000,
    "question_ms": 120000
  },
  "defaults": {
    "permission_fallback": "ask",
    "question_fallback": "skip"
  },
  "deny_patterns": [
    "rm -rf /",
    "git push --force"
  ],
  "reactions": {
    "approve": "✅",
    "deny": "❌",
    "always": "🔒"
  }
}
```

### Settings Reference / 設定參考

| Setting / 設定 | Default / 預設 | Description / 說明 |
|---|---|---|
| `sidecar.port` | `19275` | HTTP sidecar port. Auto-increments if busy (+10 max). / HTTP 側車埠號。忙碌時自動遞增（最多 +10）。 |
| `timeout.approval_ms` | `60000` | Time to wait for reaction on permission requests (ms). / 等待權限請求反應的時間（毫秒）。 |
| `timeout.question_ms` | `120000` | Time to wait for answer on questions (ms). / 等待問題回答的時間（毫秒）。 |
| `defaults.permission_fallback` | `"ask"` | Action when approval times out: `"ask"` (terminal), `"allow"`, `"deny"`. / 審批逾時的行為：`"ask"`（終端機）、`"allow"`、`"deny"`。 |
| `deny_patterns` | `[]` | Substrings auto-blocked on PreToolUse. / PreToolUse 自動攔截的子字串。 |
| `reactions.approve` | `✅` | Emoji for approve. / 批准的表情符號。 |
| `reactions.deny` | `❌` | Emoji for deny. / 拒絕的表情符號。 |
| `reactions.always` | `🔒` | Emoji for always approve (session-scoped). / 永遠批准的表情符號（限本次工作階段）。 |

### Config via Skill / 透過技能指令設定

```bash
/discord-remote:configure timeout 30000        # Set 30s approval timeout / 設定 30 秒審批逾時
/discord-remote:configure fallback deny        # Auto-deny on timeout / 逾時自動拒絕
/discord-remote:configure deny add "rm -rf"    # Add deny pattern / 新增拒絕模式
/discord-remote:configure deny rm "rm -rf"     # Remove deny pattern / 移除拒絕模式
/discord-remote:configure deny list            # List patterns / 列出模式
```

### Reloading Config / 重新載入設定

- **Unix/macOS/WSL**: Send `SIGHUP` to the MCP server process, or restart the session.
- **Windows**: Config reloads automatically on each request.

- **Unix/macOS/WSL**：對 MCP 伺服器程序發送 `SIGHUP`，或重新啟動工作階段。
- **Windows**：每次請求時自動重新載入設定。

---

## Skills / 技能指令

### `/discord-remote:configure`

Setup and status. Configure bot token, timeouts, fallback behavior, and deny patterns.

設定與狀態。設定機器人令牌、逾時、回退行為和拒絕模式。

### `/discord-remote:access`

Access management. Pair new Discord users, manage allowlists, set DM policy, configure guild channels.

存取管理。配對新的 Discord 使用者、管理允許清單、設定私訊政策、設定伺服器頻道。

| Command / 指令 | Effect / 效果 |
|---|---|
| `/discord-remote:access` | Show current access state / 顯示目前存取狀態 |
| `/discord-remote:access pair <code>` | Approve pairing code / 批准配對碼 |
| `/discord-remote:access deny <code>` | Discard pending code / 丟棄待處理的配對碼 |
| `/discord-remote:access allow <userId>` | Add user by snowflake ID / 以 snowflake ID 新增使用者 |
| `/discord-remote:access remove <userId>` | Remove from allowlist / 從允許清單移除 |
| `/discord-remote:access policy <mode>` | Set DM policy: `pairing`, `allowlist`, `disabled` / 設定私訊政策 |
| `/discord-remote:access group add <channelId>` | Enable a guild channel / 啟用伺服器頻道 |
| `/discord-remote:access group rm <channelId>` | Disable a guild channel / 停用伺服器頻道 |

---

## Architecture / 架構

```
┌─────────────────────────────────────────────────────────┐
│ Claude Code                                             │
│                                                         │
│  PermissionRequest hook ──stdin/stdout JSON──► hook.mjs │
│                                                  │      │
│                              POST /approve       │      │
│                              Authorization: Bearer│      │
│                                                  ▼      │
│  MCP Channel Server ◄──── HTTP Sidecar (localhost:19275)│
│  (discord.js + MCP SDK)        │                        │
│       │                        │ discord.js              │
│       │ messages ↔ Claude      │ DM + reactions          │
│       │                        ▼                        │
└───────┼────────────────── Discord API ──────────────────┘
        │                        │
        ▼                        ▼
   Discord chat              Your phone
   (talk to Claude)          (approve/deny)
```

### English

Everything runs in a **single Bun process**: the MCP channel server (for chat) and the HTTP sidecar (for approvals) share the same Discord gateway connection.

**Hook script** (`hook.mjs`): A thin Node.js HTTP client. Reads the sidecar port from a file, POSTs the permission request, waits for the response, and writes it to stdout. Uses only Node.js built-ins — zero npm dependencies, cross-platform.

**Port discovery:** The sidecar writes its port to `~/.claude/channels/discord/sidecar.port` at startup, deletes on shutdown.

**Authentication:** A shared secret at `~/.claude/channels/discord/sidecar.secret` (256-bit random, 0600 permissions). The hook sends `Authorization: Bearer <secret>`. The sidecar rejects unauthenticated requests.

**Hooks registered:** `PermissionRequest` and `PreToolUse` via the plugin's `hooks/hooks.json`. These hooks only fire for sessions that have the plugin loaded — other Claude Code sessions are unaffected.

### 繁體中文

所有元件在**單一 Bun 程序**中執行：MCP 頻道伺服器（聊天用）和 HTTP 側車（審批用）共用相同的 Discord 閘道連線。

**Hook 腳本**（`hook.mjs`）：輕量的 Node.js HTTP 客戶端。從檔案讀取側車埠號，POST 權限請求，等待回應，寫入 stdout。只使用 Node.js 內建模組——零 npm 依賴，跨平台。

**埠號發現：** 側車啟動時將埠號寫入 `~/.claude/channels/discord/sidecar.port`，關閉時刪除。

**認證：** 共享密鑰存於 `~/.claude/channels/discord/sidecar.secret`（256 位元隨機，0600 權限）。Hook 發送 `Authorization: Bearer <secret>`。側車拒絕未認證的請求。

**已註冊的 Hooks：** 透過插件的 `hooks/hooks.json` 註冊 `PermissionRequest` 和 `PreToolUse`。這些 hooks 只在載入此插件的工作階段中觸發——其他 Claude Code 工作階段不受影響。

---

## Security / 安全性

### English

- **Localhost only** — The sidecar binds to `127.0.0.1`. No external network exposure.
- **Shared secret** — A 256-bit random secret authenticates hook→sidecar communication. Generated once, stored with 0600 permissions.
- **Bot token** — Stored at `~/.claude/channels/discord/.env` with 0600 permissions.
- **User filtering** — Only the paired Discord user (first entry in `access.json` allowFrom) can approve requests. Reactions from other users are silently ignored.
- **Deny patterns** — Read-only from config file. Cannot be modified via Discord messages (prevents prompt injection).
- **DM only** — Approval requests are sent as DMs, not in group channels.
- **No state exfil** — The `assertSendable()` guard blocks sending `access.json`, `.env`, or `sidecar.secret` as file attachments.
- **Safe auto-approve** — `Bash`, `Write`, `Edit`, `NotebookEdit` are never auto-approved. Each call gets a fresh Discord prompt regardless of 🔒.

### 繁體中文

- **僅限本機** — 側車綁定 `127.0.0.1`，不暴露於外部網路。
- **共享密鑰** — 256 位元隨機密鑰認證 hook→側車通訊。產生一次，以 0600 權限儲存。
- **機器人令牌** — 以 0600 權限儲存於 `~/.claude/channels/discord/.env`。
- **使用者過濾** — 只有配對的 Discord 使用者（`access.json` allowFrom 的第一個條目）可以批准請求。其他使用者的反應會被靜默忽略。
- **拒絕模式** — 從設定檔唯讀。無法透過 Discord 訊息修改（防止提示注入攻擊）。
- **僅限私訊** — 審批請求以私訊發送，不在群組頻道中。
- **禁止狀態外洩** — `assertSendable()` 防護阻止將 `access.json`、`.env` 或 `sidecar.secret` 作為檔案附件發送。
- **安全的自動批准** — `Bash`、`Write`、`Edit`、`NotebookEdit` 永遠不會自動批准。無論是否點了 🔒，每次呼叫都會收到 Discord 提示。

---

## Timeout Behavior / 逾時行為

### English

If you don't respond within the configured timeout:

| Fallback | Behavior |
|---|---|
| `"ask"` (default) | Falls back to the terminal prompt. Claude Code asks you directly as if the hook wasn't there. |
| `"allow"` | Automatically approves. Use only in trusted environments. |
| `"deny"` | Automatically blocks. Safe, but halts progress if you're away. |

If the sidecar is unreachable (not running, crashed), the hook falls back to `"ask"` silently.

### 繁體中文

若在設定的逾時時間內未回應：

| 回退行為 | 說明 |
|---|---|
| `"ask"`（預設） | 回退到終端機提示。Claude Code 會像 hook 不存在一樣直接詢問你。 |
| `"allow"` | 自動批准。僅在信任的環境中使用。 |
| `"deny"` | 自動攔截。安全，但若你不在電腦旁會中斷進度。 |

若側車無法連線（未執行、已崩潰），hook 會靜默回退到 `"ask"`。

---

## Relationship to Official Plugin / 與官方插件的關係

### English

This plugin **forks** the official `discord` channel plugin from `anthropics/claude-plugins-official`. It includes all the messaging features of the official plugin PLUS the approval sidecar.

You do **NOT** need both plugins installed. discord-remote replaces the official Discord plugin entirely:

| Feature | Official `discord` | `discord-remote` |
|---|---|---|
| Chat with Claude via Discord | ✅ | ✅ |
| Pairing / access control | ✅ | ✅ |
| Guild channel support | ✅ | ✅ |
| File attachments | ✅ | ✅ |
| Remote permission approval | ❌ | ✅ |
| PreToolUse deny patterns | ❌ | ✅ |
| Session auto-approve | ❌ | ✅ |

Both plugins share the same state directory (`~/.claude/channels/discord/`) and `access.json`. If you switch between them, your pairing and access config carries over.

### 繁體中文

此插件**分叉**自 `anthropics/claude-plugins-official` 的官方 `discord` 頻道插件。它包含官方插件的所有訊息功能，外加審批側車。

你**不需要**同時安裝兩個插件。discord-remote 完全取代官方 Discord 插件：

| 功能 | 官方 `discord` | `discord-remote` |
|---|---|---|
| 透過 Discord 與 Claude 對話 | ✅ | ✅ |
| 配對 / 存取控制 | ✅ | ✅ |
| 伺服器頻道支援 | ✅ | ✅ |
| 檔案附件 | ✅ | ✅ |
| 遠端權限審批 | ❌ | ✅ |
| PreToolUse 拒絕模式 | ❌ | ✅ |
| 工作階段自動批准 | ❌ | ✅ |

兩個插件共用相同的狀態目錄（`~/.claude/channels/discord/`）和 `access.json`。切換時，配對和存取設定會保留。

---

## Troubleshooting / 疑難排解

### Sidecar not running / 側車未執行

Check if `~/.claude/channels/discord/sidecar.port` exists. If not, the sidecar didn't start. Common causes:
- Bot token missing or invalid → check `/discord-remote:configure`
- Discord intents not enabled → enable **Message Content Intent** in Developer Portal
- Port conflict → check `remote-config.json` for `sidecar.port`

檢查 `~/.claude/channels/discord/sidecar.port` 是否存在。若不存在，側車未啟動。常見原因：
- 機器人令牌遺失或無效 → 檢查 `/discord-remote:configure`
- Discord 意圖未啟用 → 在開發者入口啟用 **Message Content Intent**
- 埠號衝突 → 檢查 `remote-config.json` 的 `sidecar.port`

### Approval DM not appearing / 審批私訊未出現

The hook fires but no Discord message:
- Check sidecar.port exists: `cat ~/.claude/channels/discord/sidecar.port`
- Check sidecar.secret exists: `ls -la ~/.claude/channels/discord/sidecar.secret`
- Test sidecar health: `curl http://127.0.0.1:<port>/health`

Hook 觸發但沒有 Discord 訊息：
- 檢查 sidecar.port 存在：`cat ~/.claude/channels/discord/sidecar.port`
- 檢查 sidecar.secret 存在：`ls -la ~/.claude/channels/discord/sidecar.secret`
- 測試側車健康：`curl http://127.0.0.1:<port>/health`

### Reaction not detected / 反應未偵測

You react but Claude doesn't continue:
- Ensure the bot has **Add Reactions** permission in your shared server
- Only the paired user's reactions are collected — check `/discord-remote:access`
- The bot's own reactions (it adds ✅❌🔒 to the message) are filtered out

你反應了但 Claude 沒有繼續：
- 確認機器人在共享伺服器中有 **Add Reactions** 權限
- 只收集已配對使用者的反應——檢查 `/discord-remote:access`
- 機器人自己的反應（它在訊息上加的 ✅❌🔒）會被過濾掉

### Hooks not loading / Hooks 未載入

Run `/hooks` in Claude Code. If `PermissionRequest` doesn't show the discord-remote hook:
- Plugin must be installed: `/plugin install discord-remote@<marketplace>`
- Session must be started with the channel flag
- Run `claude plugin validate .` from the plugin cache directory to check for errors

在 Claude Code 中執行 `/hooks`。若 `PermissionRequest` 沒有顯示 discord-remote hook：
- 插件必須已安裝：`/plugin install discord-remote@<marketplace>`
- 工作階段必須以頻道旗標啟動
- 從插件快取目錄執行 `claude plugin validate .` 檢查錯誤

### "Used disallowed intents" error / 「Used disallowed intents」錯誤

The bot requests Gateway Intents that aren't enabled. Go to [Discord Developer Portal](https://discord.com/developers/applications) → your app → **Bot** → **Privileged Gateway Intents** and enable **Message Content Intent**.

機器人請求未啟用的閘道意圖。前往 [Discord 開發者入口](https://discord.com/developers/applications) → 你的應用 → **Bot** → **Privileged Gateway Intents** 並啟用 **Message Content Intent**。

### Other sessions affected / 其他工作階段受影響

The plugin hooks only fire for sessions that load the discord-remote channel. Other Claude Code sessions are unaffected — they don't have the hooks registered.

插件 hooks 只在載入 discord-remote 頻道的工作階段中觸發。其他 Claude Code 工作階段不受影響——它們沒有註冊這些 hooks。

---

## Known Limitations / 已知限制

- **AskUserQuestion** hooks are not supported by Claude Code's plugin hook system. Questions from Claude go to the terminal, not Discord.
- **Single session per bot** — only one Claude Code session can use the same Discord bot at a time (Discord gateway limitation).
- **Research preview** — requires `--dangerously-load-development-channels` flag until the channel is added to the approved allowlist.

- **AskUserQuestion** hooks 不被 Claude Code 的插件 hook 系統支援。Claude 的問題會到終端機，不會到 Discord。
- **每個機器人單一工作階段** — 同一時間只有一個 Claude Code 工作階段可以使用同一個 Discord 機器人（Discord 閘道限制）。
- **研究預覽** — 在頻道加入已核准允許清單之前，需要 `--dangerously-load-development-channels` 旗標。

---

## License / 授權

Apache-2.0 — same as the official Discord channel plugin.

Apache-2.0 — 與官方 Discord 頻道插件相同。
