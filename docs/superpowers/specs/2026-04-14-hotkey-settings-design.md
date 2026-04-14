# 熱鍵設定 (Hotkey Settings) 設計規格

- **日期**：2026-04-14
- **軌道**：`wails/`（Go + Web）
- **相關 PRD 段**：§8 Roadmap「系統常駐列與全域快捷鍵綁定」
- **狀態**：Phase 1 設計定稿；Phase 2 另行 brainstorm

## 目標與非目標

### 目標
讓使用者在 SnapVector Wails GUI 內自訂熱鍵，支援多鍵組合（`Cmd/Ctrl/Alt/Shift + 主鍵`），並將設定持久化於跨平台標準 config 目錄。熱鍵覆蓋截圖、工具切換、編輯、檔案、檢視、匯出共數十個動作。

### 非目標（Phase 1 明確排除）
- **全域 (system-wide) 熱鍵**：僅在 SnapVector 視窗聚焦時生效。全域註冊需 CGO 與 Wayland portal 整合，延後至 Phase 2。
- 匯入/匯出熱鍵 JSON、多套 profile 切換、多鍵序列 (`mod+k mod+s`)。
- Tool-rail 按鈕 tooltip 隨設定動態更新（硬寫的 `title="Select (V)"` 在 Phase 1 允許與實際設定不同步）。

## 分期策略

| Phase | 內容 | 風險 |
|---|---|---|
| **Phase 1**（本 spec） | 設定 UI、按鍵擷取、衝突檢查、JSON 持久化、應用內熱鍵分派 | 低，純前端 + Go 檔案 I/O |
| Phase 2（另行設計） | `golang.design/x/hotkey` 整合、macOS/Windows/X11 原生註冊、Wayland `org.freedesktop.portal.GlobalShortcuts` fallback | CGO、Wayland 桌面環境差異 |

Schema 在 Phase 1 就預留 `scope` 欄位（值固定為 `"app"`），Phase 2 擴充不需要 migration。

## 架構

```
┌─────────────────── frontend (app.js) ───────────────────┐
│  HotkeyManager                PreferencesModal          │
│  ├─ bindings Map              ├─ 表格 UI                 │
│  ├─ comboToAction 反查        ├─ 錄製器                   │
│  ├─ keydown listener          ├─ 衝突 reassign 對話       │
│  └─ normalize(event)          └─ save / cancel / reset   │
│           │                          │                  │
└───────────┼──────────────────────────┼──────────────────┘
            │   Wails binding          │
            ▼                          ▼
┌──────────────────── Go (gui/app.go) ─────────────────────┐
│  HotkeyStore                                             │
│  ├─ GetHotkeys() / SaveHotkeys / ResetHotkeys            │
│  ├─ DefaultHotkeys()（純函式）                            │
│  └─ configPath()                                         │
│         → <UserConfigDir>/SnapVector/hotkeys.json        │
└──────────────────────────────────────────────────────────┘
```

**關鍵原則**：

1. **熱鍵分派純前端**。`keydown` → 正規化成 canonical 字串 → 查反查表 → 呼叫既有 action 函式。不繞經 Go，延遲 ≈ 0。
2. **Go 只負責持久化**。讀寫 JSON、回傳預設值、原子寫入（temp + rename）。
3. **Schema 預留 `scope`**。Phase 1 永遠是 `"app"`，Phase 2 擴充 `"global"` 不改 schema。

## 資料契約

### JSON schema（`<UserConfigDir>/SnapVector/hotkeys.json`）

```json
{
  "version": 1,
  "bindings": [
    { "action": "tool.select",       "combo": "v",            "scope": "app" },
    { "action": "edit.undo",         "combo": "mod+z",        "scope": "app" },
    { "action": "capture.fullscreen","combo": "mod+shift+q",  "scope": "app" }
  ]
}
```

### Canonical combo 格式

- 全小寫，`+` 分隔
- 修飾鍵固定順序：`mod → ctrl → alt → shift → <主鍵>`
- `mod` 為跨平台別名：macOS 映射到 `Cmd` (metaKey)，其他平台映射到 `Ctrl` (ctrlKey)
- 若需要**明確**區分 `Ctrl`（不跟 `mod` 聯動）：使用 `ctrl`
- 主鍵使用 `KeyboardEvent.key` 小寫值（`arrowup`、`escape`、`f1`、`,`），不用 `.code`
- 空字串 `""` = 未綁定

### Phase 1 預設綁定

| action | 預設 combo | 動作 |
|---|---|---|
| `tool.select` | `v` | 切換到選擇工具 |
| `tool.arrow` | `a` | 切換到箭頭 |
| `tool.rectangle` | `r` | 矩形 |
| `tool.ellipse` | `o` | 橢圓 |
| `tool.text` | `t` | 文字 |
| `tool.blur` | `b` | 模糊 |
| `tool.crop` | `c` | 裁切 |
| `edit.undo` | `mod+z` | 復原 |
| `edit.redo` | `mod+shift+z` | 重做 |
| `file.open` | `mod+o` | 開啟 |
| `file.save` | `mod+s` | 儲存 |
| `file.saveAs` | `mod+shift+s` | 另存 |
| `view.zoomIn` | `mod+=` | 放大 |
| `view.zoomOut` | `mod+-` | 縮小 |
| `view.zoomReset` | `mod+0` | 重設縮放 |
| `export.copy` | `mod+shift+c` | 複製 PNG 到剪貼簿 |
| `capture.fullscreen` | `mod+shift+q` | 全螢幕截圖 |
| `capture.region` | `mod+shift+w` | 區域截圖 |
| `capture.allDisplays` | `mod+shift+e` | 所有顯示器 |
| `app.preferences` | `mod+,` | 開啟設定面板 |

截圖用 `Q/W/E` 而非 `3/4/5` 的原因：避開 macOS 系統預設截圖快捷鍵，降低首次使用衝突。

### Go binding 介面

```go
type Hotkey struct {
    Action string `json:"action"`
    Combo  string `json:"combo"`
    Scope  string `json:"scope"` // Phase 1 固定為 "app"
}

func (a *App) GetHotkeys() ([]Hotkey, error)    // 讀檔；不存在則回預設
func (a *App) SaveHotkeys(h []Hotkey) error     // 原子寫入（temp + rename）
func (a *App) ResetHotkeys() ([]Hotkey, error)  // 刪檔 + 回預設
func (a *App) DefaultHotkeys() []Hotkey         // 純函式，不讀檔
```

## UX 流程

### 開啟設定

- File 選單 → `Preferences…`
- 或按熱鍵 `mod+,`（Phase 1 即可用）

### Modal 佈局

```
┌─ Preferences › Hotkeys ──────────────────────────── ✕ ┐
│  Filter: [ search ]              [Reset all defaults] │
│                                                       │
│  Tools                                                │
│  ├ Select tool            [  V          ]  [🗙]        │
│  ├ Arrow tool             [  A          ]  [🗙]        │
│  …                                                    │
│  Editing                                              │
│  ├ Undo                   [  ⌘ Z        ]  [🗙]        │
│  …                                                    │
│  Capture                                              │
│  ├ Capture full screen    [  ⌘ ⇧ Q      ]  [🗙]        │
│  …                                                    │
│                                                       │
│              [ Cancel ]  [ Save ]                     │
└───────────────────────────────────────────────────────┘
```

- 分組顯示：Tools / Editing / File / View / Export / Capture / App
- Filter 即時過濾動作名與當前 combo
- 每列：動作名稱、熱鍵欄位（點擊進入錄製）、清空按鈕 🗙

### 錄製流程

1. 點擊熱鍵欄位 → 變 `[ Press keys… ]` 藍框，`HotkeyManager.isRecording = true`，主分派暫停
2. 使用者按下組合 → 即時顯示 `⌘ ⇧ Q`（modifier-only 時維持 "Press keys…"）
3. 按下**主鍵**時立刻嘗試 commit：
   - **無衝突** → 欄位變回普通顯示（暫存，尚未寫檔）
   - **有衝突** → 彈確認框：
     ```
     ⚠️  ⌘ ⇧ Q 目前綁定給 "Capture full screen"。
         要改成綁定給 "Capture region" 嗎？
         原來的 "Capture full screen" 會變成未綁定。
         [Reassign]  [Cancel]
     ```
4. **Esc**：取消錄製，保留原值
5. **Backspace / Delete**（錄製中，無其他鍵按下時）：清空為未綁定
6. **🗙 按鈕**：直接清空

### Save / Cancel 語義

- Modal 內所有編輯是**暫存 state**
- **Save**：呼叫 `SaveHotkeys` 寫檔並重建前端 `comboToAction` 反查表
- **Cancel**：丟棄所有編輯
- 未存離開偵測：若 `dirty === true`，關閉前提示「有未儲存的變更」

### 顯示格式化

| 平台 | 顯示 | 儲存 |
|---|---|---|
| macOS | `⌘ ⇧ Q`、`⌃ ⌥`、`⏎`、`⌫` Unicode | `mod+shift+q` |
| Windows / Linux | `Ctrl+Shift+Q` | `mod+shift+q` |

## 錯誤處理與邊界情況

| 情況 | 處理 |
|---|---|
| 使用者在文字標註工具編輯中按 `v` | `isTypingInTextInput(target)` 檢查：`INPUT / TEXTAREA / [contenteditable]` 焦點時跳過分派 |
| IME 組字中（中文輸入） | `e.isComposing === true` 時跳過分派 |
| config 檔 JSON 損壞 | Go log warning，備份成 `hotkeys.json.corrupt`，回傳預設；下次 Save 建立乾淨檔 |
| config 檔不存在 | 回傳預設，不自動寫檔（首次 Save 才建立） |
| schema `version` 不符 | Phase 1 僅認 `version: 1`；其他版本視為損壞 |
| 只按修飾鍵不按主鍵 | 錄製器忽略（非合法 combo） |
| 嘗試綁 Enter/Esc/Backspace 純鍵 | 錄製器保留為控制鍵（commit/cancel/clear）；欲綁必須加修飾鍵（`mod+enter`） |
| 同一 combo 被兩 action 指定（載入時） | 後者覆蓋前者並 log warning |
| 磁碟滿 / 權限不足 | Go 回 error；modal 顯示紅字：「儲存失敗：<錯誤>」，不關閉 modal |
| 系統級搶鍵（macOS `Cmd+W` 關窗、`Cmd+Q` 退出） | 接受攔不到；錄製時若使用者輸入此類組合，顯示 ⚠️ 「此組合可能被系統攔截，Phase 2 全域註冊後才能使用」 |

## 測試策略

### Go unit (`wails/gui/hotkey_test.go`)

- `DefaultHotkeys()` 穩定回傳（快照測試）
- `SaveHotkeys` + `GetHotkeys` round-trip
- 損壞檔 fallback：寫入非法 JSON → `GetHotkeys()` 回預設且建立 `.corrupt` 備份
- 原子寫入：驗證使用 temp file + rename（同一 tempdir 下檢查 `hotkeys.json` 存在、不存在 `.tmp` 殘留）
- `configPath()` 各 GOOS 回傳正確子目錄

### 前端 JS 純函式（export 到 `window.__snapVectorTest`）

- `normalize(KeyboardEvent-like)` 三平台修飾鍵映射正確（mac meta→mod、其他 ctrl→mod）
- `comboToDisplay(combo, isMac)` 格式符合規格
- 衝突偵測：輸入新 binding list → 回傳衝突 action（若有）
- 修飾鍵順序正規化：`shift+mod+z` → `mod+shift+z`

### 手動驗收 checklist（spec 交付前逐條勾選）

- [ ] 開 modal → 改 `Undo` 為 `mod+shift+u` → Save → 關 app → 重開 → 新綁定生效
- [ ] 錄製 `mod+shift+q`（已被 fullscreen capture 佔用）→ 彈 reassign 對話 → Reassign 後 fullscreen capture 欄位變「未綁定」
- [ ] 錄製中按 Esc → 欄位保留原值
- [ ] 錄製中按 Backspace（不按其他鍵）→ 欄位變「未綁定」
- [ ] 文字標註編輯中按 `v` → 不切換工具，字母 v 輸入到文字框
- [ ] 中文輸入法組字中按 Enter → 不觸發任何熱鍵動作
- [ ] 手動刪 config 檔 → 開 app → 回到預設
- [ ] 手動把 config 寫成壞 JSON → 開 app → 回預設，`hotkeys.json.corrupt` 已生成
- [ ] `mod+,` 能開 Preferences modal
- [ ] Reset all defaults 恢復所有綁定

## 檔案異動（預期）

| 檔案 | 異動 |
|---|---|
| `wails/gui/hotkey.go` | 新檔：`Hotkey` struct、`HotkeyStore`、預設綁定清單、原子寫入 |
| `wails/gui/hotkey_test.go` | 新檔：unit tests |
| `wails/gui/app.go` | 新增 `GetHotkeys` / `SaveHotkeys` / `ResetHotkeys` / `DefaultHotkeys` binding 方法 |
| `wails/gui/frontend/dist/index.html` | 新增 Preferences modal DOM、File 選單新增 `Preferences…` |
| `wails/gui/frontend/dist/app.js` | `HotkeyManager`、`PreferencesModal`、`normalize` / `comboToDisplay`、接線既有 action 函式 |
| `wails/gui/frontend/dist/styles.css` | Modal 樣式、錄製欄位 focus 樣式 |

## 開放問題

- **Phase 1 出貨後**：tool-rail tooltip (`title="Select (V)"`) 與實際設定不同步是否該進 Phase 1.5 修？目前歸 Phase 2。
- **匯入匯出**：若 Phase 1 使用者累積很多自訂熱鍵，Phase 2 前能否純手動複製 config 檔？→ 可以，檔案格式穩定，文件寫清楚路徑。
