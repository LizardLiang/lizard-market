# discord-remote

Remote tool approval for Claude Code via Discord.
Approve or deny Claude Code's permission requests from your phone with emoji reactions — no terminal required.

Discord 遠端審批 Claude Code 工具請求。
透過手機上的 Discord 表情符號反應來批准或拒絕 Claude Code 的權限請求，無需終端機。

---

## Table of Contents / 目錄

- [What It Does / 功能說明](#what-it-does--功能說明)
- [Prerequisites / 前置需求](#prerequisites--前置需求)
- [Quick Start / 快速開始](#quick-start--快速開始)
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

When Claude Code wants to run a tool (write a file, execute a command, etc.), instead of blocking at the terminal prompt, discord-remote:

1. **PermissionRequest** — Sends a Discord DM with the tool name and input summary. Adds three reactions:
   - ✅ Approve
   - ❌ Deny
   - 🔒 Always Approve (adds the tool to session allowlist)
2. **AskUserQuestion** — Forwards Claude's questions to Discord:
   - Multiple choice: number emoji reactions (1️⃣ 2️⃣ 3️⃣), tap your answer
   - Free text: reply to the message with your answer
3. **PreToolUse** — Checks tool invocations against a configurable deny list. Matching patterns are blocked immediately without contacting Discord.

If you don't respond within the timeout, the plugin falls back to the terminal prompt (configurable).

### 繁體中文

當 Claude Code 需要執行工具（寫入檔案、執行指令等）時，discord-remote 不會在終端機等待，而是：

1. **權限請求（PermissionRequest）** — 發送 Discord 私訊，顯示工具名稱和輸入摘要。附上三個表情符號：
   - ✅ 批准
   - ❌ 拒絕
   - 🔒 永遠批准（將該工具加入本次工作階段的允許清單）
2. **提問（AskUserQuestion）** — 將 Claude 的問題轉發到 Discord：
   - 選擇題：數字表情符號（1️⃣ 2️⃣ 3️⃣），點擊選擇答案
   - 開放式問題：直接回覆訊息
3. **預檢工具（PreToolUse）** — 依據設定的拒絕清單檢查工具呼叫，符合的模式會立即攔截，不需聯繫 Discord。

若在逾時時間內未回應，插件會回退到終端機提示（可設定）。

---

## Prerequisites / 前置需求

| Requirement / 需求 | Details / 詳細 |
|---|---|
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
7. （建議）關閉 **Public Bot** 開關，這樣只有你能將機器人加入其他伺服器

### Step 2: Install the Plugin / 步驟二：安裝插件

```bash
# Option A: Symlink into your project (recommended)
# 選項 A：建立符號連結到你的專案（推薦）
ln -s /path/to/discord-remote ~/.claude/plugins/discord-remote

# Option B: Copy
# 選項 B：複製
cp -r /path/to/discord-remote ~/.claude/plugins/discord-remote
```

### Step 3: Configure the Bot Token / 步驟三：設定機器人令牌

In a Claude Code session / 在 Claude Code 工作階段中：

```
/discord-remote:configure <your-bot-token>
```

The token is saved to `~/.claude/channels/discord/.env` with `0600` permissions.

令牌會以 `0600` 權限儲存於 `~/.claude/channels/discord/.env`。

### Step 4: Start a Session with the Channel / 步驟四：啟動帶頻道的工作階段

```bash
claude --channels plugin:discord-remote
```

### Step 5: Pair Your Discord Account / 步驟五：配對你的 Discord 帳號

1. DM your bot on Discord — it replies with a 6-character pairing code
2. In Claude Code: `/discord-remote:access pair <code>`
3. Lock it down: `/discord-remote:access policy allowlist`

步驟：

1. 在 Discord 上私訊你的機器人——它會回覆一個 6 位配對碼
2. 在 Claude Code 中：`/discord-remote:access pair <code>`
3. 鎖定存取：`/discord-remote:access policy allowlist`

### Step 6: Verify / 步驟六：驗證

```
/discord-remote:configure
```

This shows token status, sidecar status, paired users, and access configuration.

這會顯示令牌狀態、側車伺服器狀態、已配對使用者和存取設定。

### Step 7: Test It / 步驟七：測試

Ask Claude to write a file. You should receive a Discord DM with the approval request. Tap ✅ to approve.

請 Claude 寫入一個檔案。你應該會在 Discord 收到審批請求的私訊。點擊 ✅ 批准。

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
| `reactions.always` | `🔒` | Emoji for always approve. / 永遠批准的表情符號。 |

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
Claude Code (hook system / hook 系統)
    │
    │ stdin/stdout JSON
    ▼
hook.mjs (Node.js, zero npm deps / 無 npm 依賴)
    │
    │ POST /approve | /pretool | /question
    │ Authorization: Bearer <shared-secret>
    ▼
HTTP Sidecar (localhost:19275)
    │                          ┌─────────────────────┐
    │ discord.js DM + reactions│ MCP Channel Server   │
    ▼                          │ (same Bun process)   │
Discord API  ──►  Your phone   │ messages ↔ Claude    │
                               └─────────────────────┘
```

### English

The MCP server and HTTP sidecar run in the **same Bun process**. The sidecar reuses the Discord gateway connection that the MCP channel server already maintains — no extra connections.

Hook scripts are thin Node.js HTTP clients. They POST to the sidecar and relay the response back to Claude Code. They use only Node.js built-ins — zero npm dependencies, works on Windows/macOS/Linux.

**Port discovery:** The sidecar writes its actual port to `~/.claude/channels/discord/sidecar.port` at startup. Hook scripts read this file. It's deleted on shutdown.

**Authentication:** A shared secret is generated at first run and stored at `~/.claude/channels/discord/sidecar.secret` (0600 permissions). The hook sends it as `Authorization: Bearer <secret>`. The sidecar rejects requests without a valid secret.

### 繁體中文

MCP 伺服器和 HTTP 側車在**同一個 Bun 程序**中執行。側車重複使用 MCP 頻道伺服器已維護的 Discord 閘道連線——不需額外連線。

Hook 腳本是輕量的 Node.js HTTP 客戶端。它們向側車發送 POST 請求，並將回應轉發回 Claude Code。只使用 Node.js 內建模組——零 npm 依賴，可在 Windows/macOS/Linux 上運作。

**埠號發現：** 側車啟動時將實際埠號寫入 `~/.claude/channels/discord/sidecar.port`。Hook 腳本讀取此檔案。關閉時刪除。

**認證：** 首次執行時產生共享密鑰，儲存於 `~/.claude/channels/discord/sidecar.secret`（0600 權限）。Hook 以 `Authorization: Bearer <secret>` 發送。側車拒絕沒有有效密鑰的請求。

---

## Security / 安全性

### English

- **Localhost only** — The sidecar binds to `127.0.0.1`. No external network exposure.
- **Shared secret** — A 256-bit random secret authenticates hook→sidecar communication. Generated once, stored with 0600 permissions.
- **Bot token** — Stored at `~/.claude/channels/discord/.env` with 0600 permissions.
- **User filtering** — Only the paired Discord user (first entry in `access.json` allowFrom) can approve requests. Reactions from other users are silently ignored.
- **Deny patterns** — Read-only from config file. Cannot be modified via Discord messages (prevents prompt injection).
- **DM only** — Approval requests are sent as DMs, not in group channels.
- **No state exfil** — The `assertSendable()` guard (inherited from the official plugin) blocks sending `access.json`, `.env`, or `sidecar.secret` as file attachments.

### 繁體中文

- **僅限本機** — 側車綁定 `127.0.0.1`，不暴露於外部網路。
- **共享密鑰** — 256 位元隨機密鑰認證 hook→側車通訊。產生一次，以 0600 權限儲存。
- **機器人令牌** — 以 0600 權限儲存於 `~/.claude/channels/discord/.env`。
- **使用者過濾** — 只有配對的 Discord 使用者（`access.json` allowFrom 的第一個條目）可以批准請求。其他使用者的反應會被靜默忽略。
- **拒絕模式** — 從設定檔唯讀。無法透過 Discord 訊息修改（防止提示注入攻擊）。
- **僅限私訊** — 審批請求以私訊發送，不在群組頻道中。
- **禁止狀態外洩** — `assertSendable()` 防護（繼承自官方插件）阻止將 `access.json`、`.env` 或 `sidecar.secret` 作為檔案附件發送。

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

This plugin **forks** the official `discord` channel plugin from `anthropics/claude-plugins-official`. Both plugins:

- Share the same bot token and `~/.claude/channels/discord/` state directory
- Share `access.json` for access control
- Can coexist — they connect to Discord independently

discord-remote adds the **HTTP sidecar** and **hook system** on top of the channel messaging features. You can use either plugin alone, or both together.

> **Note:** With both plugins active, you'll receive both channel messages and approval DMs. This is expected behavior. The official plugin handles conversational messages; discord-remote handles permission approvals.

### 繁體中文

此插件**分叉**自 `anthropics/claude-plugins-official` 的官方 `discord` 頻道插件。兩個插件：

- 共用相同的機器人令牌和 `~/.claude/channels/discord/` 狀態目錄
- 共用 `access.json` 進行存取控制
- 可以共存——它們各自獨立連線到 Discord

discord-remote 在頻道訊息功能之上增加了 **HTTP 側車** 和 **hook 系統**。你可以單獨使用任一插件，或同時使用兩者。

> **注意：** 同時啟用兩個插件時，你會同時收到頻道訊息和審批私訊。這是預期行為。官方插件處理對話訊息；discord-remote 處理權限審批。

---

## Troubleshooting / 疑難排解

### Sidecar not running / 側車未執行

The sidecar starts only after the Discord gateway connects. Check that your bot token is valid and Claude Code shows the `discord-remote` MCP server as connected.

側車只在 Discord 閘道連線後才啟動。確認你的機器人令牌有效，且 Claude Code 顯示 `discord-remote` MCP 伺服器已連線。

### Port conflict / 埠號衝突

If port 19275 is busy, the sidecar tries 19276..19285. Check `~/.claude/channels/discord/sidecar.port` for the actual port. If all ports are busy, configure a different base port in `remote-config.json`.

若埠號 19275 被佔用，側車會嘗試 19276..19285。查看 `~/.claude/channels/discord/sidecar.port` 確認實際埠號。若所有埠號都被佔用，在 `remote-config.json` 中設定不同的基礎埠號。

### No paired user / 未配對使用者

Run `/discord-remote:access` to see the allowFrom list. If empty, DM your bot to start pairing.

執行 `/discord-remote:access` 查看允許清單。若為空，私訊你的機器人開始配對。

### Reactions not collected / 無法收集反應

The bot needs these intents enabled in the [Discord Developer Portal](https://discord.com/developers/applications) → Bot settings:
- **Message Content Intent**
- **Server Members Intent** (for guild channels)

The gateway intents `GuildMessageReactions` and `DirectMessageReactions` are already configured in the server code.

機器人需要在 [Discord 開發者入口](https://discord.com/developers/applications) → Bot 設定中啟用以下意圖：
- **Message Content Intent**
- **Server Members Intent**（用於伺服器頻道）

閘道意圖 `GuildMessageReactions` 和 `DirectMessageReactions` 已在伺服器程式碼中設定。

### Hook timeout exceeded / Hook 逾時

The hook timeout in `hooks.json` (120s for PermissionRequest, 180s for AskUserQuestion) must exceed your configured `approval_ms` / `question_ms`. If you increase the approval timeout beyond 120s, also update `hooks.json`.

`hooks.json` 中的 hook 逾時（PermissionRequest 120 秒，AskUserQuestion 180 秒）必須超過你設定的 `approval_ms` / `question_ms`。若你將審批逾時增加到 120 秒以上，也需更新 `hooks.json`。

### Approval falls back to terminal every time / 審批總是回退到終端機

Check that:
1. The sidecar is running (`/discord-remote:configure` shows status)
2. `sidecar.port` file exists and contains the correct port
3. `sidecar.secret` file exists and is readable by the hook process
4. The hook script can reach `127.0.0.1:<port>`

檢查：
1. 側車正在執行（`/discord-remote:configure` 顯示狀態）
2. `sidecar.port` 檔案存在且包含正確的埠號
3. `sidecar.secret` 檔案存在且 hook 程序可讀取
4. Hook 腳本可以連線到 `127.0.0.1:<port>`

---

## License / 授權

Apache-2.0 — same as the official Discord channel plugin.

Apache-2.0 — 與官方 Discord 頻道插件相同。
