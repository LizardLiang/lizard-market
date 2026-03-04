# Kratos - 戰神 (v2.5.0)

> *「我就是眾神所造之物。」* — 現在，眾神為**你**服務。

Kratos 是主要的協調者插件，指揮專業**代理人**交付功能與智慧。從快速修復到完整的 8 階段功能流水線，Kratos 一手包辦，並具備持久記憶、外部研究能力與 Git 歷史專業知識。

## 安裝

完整的安裝步驟（建置二進位檔、安裝 Hooks、設定自動啟動），請參閱 **[INSTALL.md](INSTALL.md)**。

### 快速開始

```bash
# 1. 建置二進位檔
cd plugins/kratos/go && go build -ldflags="-s -w" -o ../bin/kratos ./cmd/kratos && cd ..

# 2. 初始化資料庫並安裝 Hooks
./bin/kratos init && ./bin/kratos install

# 3. 驗證安裝
./bin/kratos status
```

接著安裝插件，並將自動啟動區塊加入你的 `CLAUDE.md`（參見 [INSTALL.md - 步驟 2](INSTALL.md#step-2-install-the-plugin-into-claude-code) 和 [步驟 5](INSTALL.md#step-5-enable-auto-activation)）。

> **注意**：二進位檔為選用。沒有它 Kratos 仍可運作 — 代理人會直接編輯檔案作為備援。安裝後，`status.json` 將記錄真實時間戳記與完整流水線歷史。

---

## 架構

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

---

## 指令

| 指令 | 用途 |
|------|------|
| `/kratos:main` | **主指令** — 處理任何請求（自動分類） |
| `/kratos:quick` | **簡單任務** — 直接路由至測試、修復、審查、除錯 |
| `/kratos:review` | **程式審查** — 標準化審查，含嚴重等級分類與自動修復 |
| `/kratos:inquiry` | **知識查詢** — 路由問題至 Metis、Clio 或 Mimir |
| `/kratos:decompose` | **功能分解** — 將功能拆解為階段（檔案、Notion、Linear）|
| `/kratos:recall` | **回復工作階段** — 上次停在哪裡？（使用持久記憶）|
| `/kratos:status` | **戰場全覽** — 所有進行中功能的狀態 |

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

建立新功能時，Kratos 遵循 8 階段神聖路徑：

```
[0] 研究（Metis，選用）
[1] PRD（Athena）
[2] PRD 審查（Athena）
[2.5] 功能分解（Daedalus，選用）
[3] 技術規格（Hephaestus）
[4] PM 審查（Athena）─┐ 平行執行
[5] SA 審查（Apollo）  ─┘
[6] 測試計畫（Artemis）
[7] 實作（Ares）
[8] 程式審查（Hermes）
```

流水線狀態記錄於 `.claude/feature/<name>/status.json`。安裝 Kratos 二進位檔後，代理人會使用 `kratos pipeline update` 寫入真實時間戳記並維護歷史記錄。未安裝時，代理人直接編輯檔案作為備援。

---

## 持久記憶

所有工作階段、代理人啟動、決策與檔案變更均記錄於 SQLite 資料庫。使用 `/kratos:recall` 從上次停止的地方繼續 — 新工作階段會自動注入上下文。

### Arena 與 Insights

- **The Arena**（`.claude/.Arena/`）：專案專屬知識 — 架構、技術堆疊、編碼慣例。
- **Insights**（`.claude/.Arena/insights/`）：Mimir 快取的外部研究成果（有效期限機制）。

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

*「循環在此終結。我們必須比這更好。」* — Kratos 透過神聖協調，引領你的專案走向勝利。
