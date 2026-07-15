# ⚔️ Kratos — 給 Claude Code 的對抗式 Spec-Driven 流水線

> *「我就是眾神所造之物。」* — 現在，眾神為**你**服務。

![version](https://img.shields.io/badge/version-2.97.0-blue) ![Claude Code](https://img.shields.io/badge/Claude%20Code-plugin-8A2BE2) ![agents](https://img.shields.io/badge/agents-19-orange) ![pipeline](https://img.shields.io/badge/pipeline-9%20stages-green) ![license](https://img.shields.io/badge/license-MIT-lightgrey)

**別再交付 AI 垃圾。** Kratos 讓你的功能走一條真正的流水線：PM 撰寫 PRD、魔鬼代言人（**Nemesis**）逐條挑戰、架構師寫出技術規格、對齊門（**Hera**）驗證實作確實對得上你**真正**要的東西。具名代理人、由 Hooks 強制執行的評審門、跨 session 的持久記憶 — 不是又一堆 subagent。

## 為什麼用 Kratos，而不是單一「做完整功能」的 agent？

|                                    | 單一大 agent | agent 大雜燴 | **Kratos**                    |
| ---------------------------------- | :----------: | :----------: | ----------------------------- |
| 需求在動工**前**就被挑戰            |      ✗       |      ✗       | ✅ Nemesis 對抗式評審          |
| 實作對照原始需求驗證                |      ✗       |      ✗       | ✅ Hera 對齊門                 |
| 品質門**強制執行**，不只是建議      |      ✗       |      ✗       | ✅ Claude Code Hooks           |
| 跨 session 持久記憶                 |      ✗       |     少見     | ✅ SQLite + `/kratos:recall`   |

## 你實際會得到什麼 — lite vs full

Kratos 分兩層運作。**markdown 層獨立可用 — 免建置、免二進位檔、免設定。** 選用的 Go 二進位檔只是讓追蹤更精確。

|                                      | markdown 層 *(預設)* | + Go 二進位檔 *(選用)* |
| ------------------------------------ | :------------------: | :--------------------: |
| 全部 19 代理人 + 11 階段流水線       |          ✅          |           ✅          |
| 指令（`/kratos:quick`、`review`…）   |          ✅          |           ✅          |
| 強制品質門 Hooks                     |          ✅          |           ✅          |
| 流水線時間戳與階段歷史               |       檔案備援       |        ✅ 精確        |
| Session 記憶 / recall                |          —           |        ✅ SQLite      |

### 快速開始

```bash
# 1. 加入市場並安裝插件 — 這一步就能用上完整流水線
claude plugin marketplace add https://github.com/LizardLiang/kratos
claude plugin install kratos@kratos
```

就這樣 — 試試 `/kratos:quick 幫 UserService.js 加測試`。markdown 層零額外設定即可運作。

**選用 — 啟用精確追蹤與記憶**（使用 `bin/` 內附的預建二進位檔，涵蓋 Linux、macOS arm64/amd64、Windows amd64）：

```bash
cd ~/.claude/plugins/cache/kratos
./bin/kratos init && ./bin/kratos install   # 初始化資料庫 + 註冊 Hooks
./bin/kratos status                         # 驗證
```

想從原始碼建置？請參閱 **[INSTALL.md — Option B](INSTALL.md)**。接著將自動啟動區塊加入你的 `CLAUDE.md`（參見 [INSTALL.md - 步驟 5](INSTALL.md#step-5-enable-auto-activation)）。

---

## 架構

**一個請求如何流經 Kratos** — 從你的提示詞，經過 Hooks 與路由器，進入對應的代理人，最後輸出交付成果。開啟 [`kratos-agent-flow.html`](kratos-agent-flow.html) 可看可縮放的互動版本（明／暗、流程與代理人視圖）。

![Kratos 代理人指令載入流程](docs/assets/agent-flow.png)

<details>
<summary>文字版（代理人陣容）</summary>

```
                         ⚔️ KRATOS ⚔️
                      主協調者
              (記憶啟用・流水線協調)
                             │
   ┌─────────────────────────┼─────────────────────────────────────────┐
   │                         │                                         │
   ▼                         ▼                                         ▼
┌─────────┐            ┌───────────┐                             ┌───────────┐
│  METIS  │            │   CLIO    │                             │   MIMIR   │
│ 專案研究 │            │  Git 歷史  │                             │  外部研究  │
└────┬────┘            └─────┬─────┘                             └─────┬─────┘
     │                       │                                         │
     └───────────────────────┼───────────────────┐                     │
                             │                   │                     │
                             ▼                   ▼                     ▼
┌─────────┐            ┌───────────┐       ┌───────────┐         ┌───────────┐
│ ATHENA  │            │HEPHAESTUS │       │  APOLLO   │         │  HERMES   │
│ 產品管理 │            │  技術規格  │       │  架構審查  │         │  程式審查  │
└────┬────┘            └─────┬─────┘       └─────┬─────┘         └─────┬─────┘
     │                       │                   │                     │
     └───────────────────────┴─────────┬─────────┴─────────────────────┘
                                       │
                              ┌────────┴────────┐
                              │  ARES & ARTEMIS │
                              │   實作與品質保證  │
                              └────────┬────────┘
                                       │
                            ┌──────────┴──────────┐
                            │        HADES        │
                            │   除錯（按需啟用）   │
                            └─────────────────────┘
```

</details>

## 眾神陣容（代理人）

| 代理人 | 領域 | 專長 | 模型（一般模式）|
|--------|------|------|----------------|
| **Metis** | 專案知識 | 程式庫分析、Arena 文件 | Sonnet |
| **Clio** | Git 歷史 | Blame、提交記錄、貢獻者分析 | Sonnet |
| **Mimir** | 外部研究 | 網路、GitHub、最佳實踐、文件 | Sonnet |
| **Athena** | 產品管理 | PRD、PM 審查、需求定義 | Opus |
| **Daedalus** | 功能分解 | 功能階段、依賴關係、平台原生任務 | Sonnet |
| **Hephaestus** | 工程設計 | 技術規格、系統藍圖 | Opus |
| **Apollo** | 架構設計 | 系統設計、SA 審查 | Opus |
| **Artemis** | 品質保證 | 測試計畫、測試案例 | Sonnet |
| **Ares** | 實作 | 程式撰寫、Bug 修復、重構 | Sonnet |
| **Hermes** | 同儕審查 | 程式審查、品質稽核 | Opus |
| **Hades** | 除錯 | 錯誤定位、失敗證明、根因分析 | Sonnet |
| **Ananke** | 任務管理 | 個人待辦清單（二進位檔 + 檔案備援） | Sonnet |
| **Iris** | 秘書 | 每日簡報 + 日常助理 — 學習、腦力激盪、深入調查、筆記；透過個人檔案、記憶與例行事項了解使用者 | Sonnet |

---

## Hooks 與品質關卡

Kratos 內建 Claude Code Hooks，自動強制執行工作流程規範 — 執行 `./bin/kratos install` 後無需額外設定。

### SubagentStart — 待辦清單優先關卡

在 **Ares** 和 **Hephaestus** 開始工作前觸發。注入強制提示，要求代理人在呼叫任何工具前必須先撰寫編號待辦清單。

### SubagentStop — 交付成果驗證

在 **Ares** 或 **Hephaestus** 嘗試完成工作時觸發。若未達標則封鎖完成並強制繼續：

| 代理人 | 檢查項目 |
|--------|---------|
| **Ares** | 必須撰寫待辦清單、提及具體修改的檔案，並確認完成 |
| **Hephaestus** | 規格文件必須涵蓋至少 2 項：架構、資料模型、API、實作、Schema、介面 |

當 `stop_hook_active` 為 true（由 Hook 觸發的重新執行）時，關卡自動放行以避免無限迴圈。

**Ares 驗證關卡（v2.87）：** 同一個 SubagentStop Hook 會掃描 session transcript — 若程式碼檔案已修改但未執行任何測試指令，則封鎖 Ares 完成（掃描失敗時放行；若變更確實無執行面向，可宣告 `TESTS-NOT-APPLICABLE: <原因>` 豁免；僅檢查 subagent 自身的活動）。Ares 也會在 `implementation-notes.md` 中為每個任務記錄先失敗後通過的證據（修正前 RED、修正後 GREEN），由 Hera 在第 8 階段驗證。

### PreToolUse — 套件管理器自動修正

攔截所有含 `npm` 的 `Bash` 工具呼叫，並依據專案根目錄的 lockfile 自動改寫為正確的套件管理器：

| Lockfile | 偵測結果 |
|----------|---------|
| `bun.lockb` | `bun` |
| `yarn.lock` | `yarn` |
| `pnpm-lock.yaml` | `pnpm` |

若未找到其他 lockfile，`npm` 指令原樣通過。

---

## 指令

| 指令 | 用途 |
|------|------|
| `/kratos:main` | **主指令** — 處理任何請求（自動分類） |
| `/kratos:quick` | **簡單任務** — 直接路由至測試、修復、審查、除錯 |
| `/kratos:review` | **程式審查** — 標準化審查，含嚴重等級分類與自動修復 |
| `/kratos:inquiry` | **知識查詢** — 路由問題至 Metis、Clio 或 Mimir |
| `/kratos:iris` | **秘書模式** — 每日簡報、學習主題、思考點子、記筆記（管線外的日常協助）|
| `/kratos:decompose` | **功能分解** — 將功能拆解為階段（檔案、Notion、Linear）|
| `/kratos:recall` | **回復工作階段** — 上次停在哪裡？（使用持久記憶）|
| `/kratos:status` | **戰場全覽** — 所有進行中功能的狀態 |
| `/kratos:spec-archive` | **規格歸檔** — 將功能的 spec delta 併入其 living spec（實作完成後，隨時可用）|
| `/kratos:spec-backfill` | **規格回補** — 將既有已出貨的功能遷移為 living spec（一次性遷移）|

每個代理人也都有對應的內嵌指令（`/kratos:athena`、`/kratos:ares`…），可在主 session 中直接執行，無需啟動 subagent。

---

## 程式審查標準

Kratos 內建分層審查標準，由 Hermes 在每次審查中強制執行：

| 層級 | 名稱 | 檢查內容 |
|------|------|---------|
| 1 | **正確性** | 邏輯、邊界情況、靜默失敗 |
| 2 | **安全性** | 資安漏洞、注入攻擊、密鑰、驗證 |
| 3 | **清晰度** | 可讀性、命名、複雜度 |
| 4 | **精簡性** | 廢棄程式碼、過度設計 |
| 5 | **一致性** | 專案慣例 |
| 6 | **韌性** | 錯誤處理、資源清理 |
| 7 | **效能** | N+1 查詢、阻塞操作、浪費 |

規則存放於 `rules/`（全域基準）與 `.claude/.Arena/review-rules/`（專案專屬，優先級較高）。語言特定規則（React、TypeScript、Python 等）依偵測到的檔案類型自動載入。

```bash
/kratos:review src/auth.ts           # 審查單一檔案
/kratos:review --staged              # 審查已暫存的變更
/kratos:review --branch feat/login   # 審查分支差異
/kratos:review src/components/ power # 審查整個目錄（全力模式）
```

Hermes 回報 `[BLOCKER]`、`[WARNING]` 和 `[SUGGESTION]` 結果 — BLOCKER 必須在核准前解決。可自動修復的問題會以 diff 形式呈現，確認後套用。

---

## 執行模式

在任何請求前加上前綴來調整 Kratos 的效能：

| 模式 | 觸發詞 | 使用模型 |
|------|--------|---------|
| **省錢模式** | `eco:`、`budget:`、`cheap:` | Haiku/Sonnet — 最省 Token |
| **一般模式** | （預設）| Opus/Sonnet 均衡搭配 |
| **全力模式** | `power:`、`max:`、`full-power:` | 所有代理人使用 Opus |

---

## 流水線（複雜功能）

建立新功能時，Kratos 遵循 11 階段神聖路徑：

```
[0] 研究（Metis，選用）
[1] PRD（Athena）
[2] PRD 審查（Athena）
[3] 功能分解（Daedalus，選用）
[4] 決策討論（Themis，選用）← 在 Hephaestus 規格前鎖定決策
[4] 技術規格（Hephaestus）
[5] PM 審查（Athena）─┐ 平行執行
[6] SA 審查（Apollo）  ─┘
[7] 測試計畫（Artemis）
[8] 實作（Ares）
[9] PRD 對齊（Hera）
[11] 程式審查（Hermes + Cassandra）
```

流水線狀態記錄於 `.claude/feature/<name>/status.json`。安裝 Kratos 二進位檔後，代理人會使用 `kratos pipeline update` 寫入真實時間戳記並維護歷史記錄。未安裝時，代理人直接編輯檔案作為備援。

---

## 持久記憶

所有工作階段、代理人啟動、決策與檔案變更均記錄於 SQLite 資料庫。使用 `/kratos:recall` 從上次停止的地方繼續 — 新工作階段會自動注入上下文。

### Arena 與 Insights

- **The Arena**（`.claude/.Arena/`）：專案專屬知識 — 架構、技術堆疊、編碼慣例。
- **Insights**（`.claude/.Arena/insights/`）：Mimir 快取的外部研究成果（有效期限機制）。

### Living Specs（活文件規格）

Living spec 是依能力（capability）彙整的系統行為記錄，位於 `.claude/.Arena/specs/<capability>/spec.md`，跨功能持續累積。Athena 為每個功能撰寫 spec delta（`.claude/feature/<slug>/spec-delta/<capability>.md`），Hera 判定 `aligned` 後（或隨時手動）以 `/kratos:spec-archive <slug>` 併入 living spec；`/kratos:spec-view` 可檢視；`/kratos:spec-backfill` 用於回補既有功能。

---

## 使用範例

```bash
# 查詢 Git 歷史
/kratos:inquiry 上個月誰動過登入頁面？

# 查詢最佳實踐（省錢模式）
eco: Node.js 處理大型檔案上傳最有效率的方式是什麼？

# 除錯錯誤
/kratos:quick debug: TypeError: Cannot read properties of undefined

# 簡單任務
/kratos:quick 為 UserService.js 新增單元測試

# 複雜功能
/kratos:main 建立多租戶訂閱系統

# 全力模式進行重要審查
power: 審查付款處理邏輯的安全漏洞

# 繼續上次的工作
/kratos:recall
```

---

## 開發與貢獻

開發、議題回報與 PR 都在 `LizardLiang/lizard-market` monorepo 的 `plugins/kratos/` 目錄下進行。`LizardLiang/kratos`（若你正在此處閱讀）是每次發布時重新產生歷史的發布鏡像，請勿對它開 PR。

---

*「循環在此終結。我們必須比這更好。」* — Kratos 透過神聖協調，引領你的專案走向勝利。
