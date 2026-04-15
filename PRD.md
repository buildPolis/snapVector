# 產品需求規格書 (PRD)

**專案名稱：** 跨平台螢幕截圖與向量標註工具 (代號：SnapVector)

**發行單位：** 築邦科技有限公司 (BuildPolis)

**文件狀態：** 核心架構草案 (Draft)

**目標作業系統：** Windows 11/10, macOS, Linux (Debian/Ubuntu - 需相容 Wayland)

---

## 1. 產品概述 (Executive Summary)

### 1.1 產品定位

一款主打「輕量、極速、零依賴」的桌面端截圖與標註軟體。具備**雙模式運行能力**：既提供一般大眾直覺的 GUI 操作介面，能使用高辨識度向量標註（Skitch 風格）並匯出相容 Inkscape 的單一 SVG，以及常見分享格式 PNG、JPG、PDF；同時提供強大的 Headless CLI 介面，能作為 Claude Code、Gemini CLI 等 AI 自主開發代理 (Autonomous Agents) 的「視覺外掛工具」。

### 1.2 目標受眾 (Target Audience)

- **軟體開發者與 AI 代理 (Agents)：** 需要透過終端機指令極速獲取螢幕畫面 (Base64)、精準注入 UI 標註，且偏好跨平台、高自由度且能產出乾淨向量原始碼的專業工作流。

- **一般大眾與辦公室白領：** 需要快速截圖並標示重點以進行溝通，追求下載後「點兩下即可使用」，不具備複雜系統環境設定能力的使用者。

### 1.3 商業與戰略價值

作為團隊切入開發者工具與大眾實用工具市場的雙棲產品。未來可將此核心截圖與標註模組，封裝為 B2B 企業解決方案的套件（如 ERP 系統的錯誤回報模組），或進一步演進為帶有本機端電腦視覺 (CV) 推論能力的智慧型助理。

---

## 2. 核心功能需求 (Functional Requirements)

### 2.1 螢幕擷取 (Screen Capture)

- **F1.1 全螢幕與區域擷取：** 支援多螢幕環境下的全畫面截圖，以及自訂滑鼠拖曳框選區域。

- **F1.2 系統權限處理：** 在 macOS 與 Linux (Wayland) 環境下，必須能優雅地觸發系統原生的截圖權限授權視窗（如 XDG Desktop Portal），確保一般大眾不會遇到黑屏或閃退。

### 2.2 向量標註引擎 (Vector Annotation Engine)

- **F2.1 Skitch 風格標註：** 內建帶有對比色白邊的粗線條紅色**直線箭頭**、幾何外框（矩形、橢圓）、文字方塊，以及可遮蔽敏感資訊的區域 blur 標註。箭頭頭部需為對稱三角形，不使用彎曲箭身。

- **F2.2 獨立元件系統 (Symbols)：** 所有標註圖形在底層皆封裝為 SVG `<symbol>`，確保重複添加標註時檔案體積不會失控膨脹，且輸出的 SVG 結構在 Inkscape 中仍可被開啟、識別與編輯。

- **F2.3 完美的中文輸入體驗：** 標註文字必須完美支援各平台的原生 CJK（中日韓）輸入法，無選字框飄移或漏字問題。

- **F2.4 區域 blur 遮蔽：** 支援以圓角矩形框選敏感資訊後套用 blur，並可調整 blur 強度。該效果在 GUI 預覽、PNG 匯出、JPG 匯出、PDF 匯出、SVG 匯出、剪貼簿輸出與 CLI `--inject-svg` 注入時都必須保留一致語義。

- **F2.5 直接操控調整：** GUI 模式下，箭頭以**滑鼠左鍵拖拉**建立，並可透過拖拉端點或 bounding handles 調整方向、長度與位置；其幾何對應 CLI `--inject-svg` 的 `x1`, `y1`, `x2`, `y2`。

### 2.3 檔案匯出與整合 (Export & Integration)

- **F3.1 獨立 SVG 匯出：** 將截圖轉換為 Base64 字串，與向量標註群組 (`<g>`) 結合成單一 `.svg` 檔案，確保在任何離線環境或瀏覽器皆可完美無損渲染。輸出的 SVG 必須可在 Inkscape 中成功**開啟、編輯、另存**，且主要視覺不走樣。若含 blur 區域，需以單一 SVG 內可離線渲染的方式表達（例如 clip + duplicate image + filter），不得依賴外部圖片或雲端服務。

- **F3.2 點陣圖匯出：** 支援將標註後的畫面扁平化 (Flatten) 並匯出為常見的 `.png` 與 `.jpg` 格式。PNG 匯出需保留 alpha；JPG 匯出因格式限制，透明區域一律以白色背景扁平化。

- **F3.3 PDF 匯出：** 支援將標註後的畫面輸出為單頁分享用 `.pdf` 文件。PDF 允許扁平化輸出，但內容視覺結果必須與 GUI 預覽一致，不要求保留可編輯向量或文字語義。

- **F3.4 剪貼簿整合：** 標註完成後，可直接將結果複製到系統剪貼簿，方便貼入通訊軟體或文件中。

### 2.4 AI 代理 CLI 整合介面 (Agent-Ready CLI)

- **F4.1 雙模式執行 (Dual-Mode Binary)：** 提供單一執行檔，能根據啟動參數 (`os.Args`) 決定啟動圖形介面 (GUI) 或進入無頭指令列模式 (Headless CLI)。

- **F4.2 面向機器的標準輸出 (Machine-Readable Output)：** 在 CLI 模式下，強制使用標準化 JSON 格式輸出執行結果至 `stdout`，確保大語言模型代理能精準解析狀態碼與資料。

- **F4.3 靜默截圖與 Base64 串流：** 支援 `--capture` 搭配 `--base64-stdout` 參數，允許 AI 代理在不儲存實體檔案、不干擾使用者畫面的情況下，極速獲取當前螢幕的 Base64 視覺資料，作為多模態模型的輸入上下文。

- **F4.4 標註渲染注入：** 允許 AI 代理透過 `--inject-svg` 參數傳入 JSON 陣列（包含座標、標註類型與文本資料），軟體需在背景將其渲染為向量標註，並回傳合成後的結果。

#### Canonical CLI JSON response schema（F4.2 單一事實來源）

所有實作軌在 CLI 模式下輸出到 `stdout` 的 JSON **必須**遵循下列結構，禁止各軌自行增減頂層欄位：

```json
{
  "status": "ok",
  "code": 0,
  "data": {
    "format": "png",
    "mimeType": "image/png",
    "base64": "iVBORw0KGgoAAAANSUhEUgAA..."
  }
}
```

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `status` | string | 必填 | 執行狀態。僅允許 `"ok"` 或 `"error"`。 |
| `code` | integer | 必填 | 機器可判斷的狀態碼。成功固定為 `0`；錯誤時為非零值。 |
| `data` | object | 成功時必填 | 成功結果內容。欄位依命令不同而變化，但必須是 JSON object，不可直接輸出裸字串或陣列。 |
| `error` | object | 失敗時必填 | 失敗資訊。成功時不得輸出。 |

錯誤回應格式：

```json
{
  "status": "error",
  "code": 1201,
  "error": {
    "message": "Screen capture permission denied",
    "retryable": true
  }
}
```

`error` 欄位定義：

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `message` | string | 必填 | 給機器與人類都可讀的錯誤摘要。 |
| `retryable` | boolean | 必填 | 表示 AI 代理是否可在提示使用者授權後重試。 |
| `details` | object | 選填 | 平台特定資訊，例如 `platform`, `portal`, `stderr`。 |

狀態碼保留區間：

| 區間 | 用途 |
|---|---|
| `0` | 成功 |
| `1000-1099` | 參數解析與 CLI 使用錯誤 |
| `1100-1199` | 螢幕擷取失敗 |
| `1200-1299` | 權限、Portal、系統 API 失敗 |
| `1300-1399` | SVG 注入、標註資料解析失敗 |
| `1400-1499` | 匯出、檔案、剪貼簿失敗 |

`--capture --base64-stdout` 成功時，`data` **至少**包含：

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `format` | string | 必填 | 固定為 `"png"`。 |
| `mimeType` | string | 必填 | 固定為 `"image/png"`。 |
| `base64` | string | 必填 | PNG 內容的 Base64 字串，不含 data URL prefix。 |
| `display` | object | 選填 | 多螢幕資訊，如 `id`, `x`, `y`, `width`, `height`, `scaleFactor`。 |
| `captureRegion` | object | 選填 | 實際擷取區域，如 `x`, `y`, `width`, `height`。 |

#### Canonical `--inject-svg` payload schema（F4.4 單一事實來源）

`--inject-svg` 的輸入為 **JSON array**，陣列中的每個元素都是一個 annotation object。所有座標均以**已擷取底圖左上角為原點**的像素值表示，`x` 向右遞增、`y` 向下遞增，不使用百分比。

通用欄位：

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `type` | string | 必填 | 僅允許 `"arrow"`、`"rectangle"`、`"ellipse"`、`"text"`、`"blur"`、`"numbered-circle"`。 |
| `id` | string | 選填 | 供呼叫端追蹤 annotation；若缺省由實作自行產生。 |
| `strokeColor` | string | 選填 | 預設為 `#E53935`。 |
| `outlineColor` | string | 選填 | 預設為 `#FFFFFF`。 |
| `strokeWidth` | number | 選填 | 預設依 `design/symbols.svg` baseline。 |

各型別欄位：

| `type` | 必填欄位 | 說明 |
|---|---|---|
| `arrow` | `x1`, `y1`, `x2`, `y2` | 定義箭頭起點與終點。 |
| `rectangle` | `x`, `y`, `width`, `height` | 空心外框。 |
| `ellipse` | `x`, `y`, `width`, `height` | 以 bounding box 定義橢圓。 |
| `text` | `x`, `y`, `text` | `x`,`y` 為文字方塊左上角。 |
| `blur` | `x`, `y`, `width`, `height` | 以圓角矩形定義 blur 區域。 |
| `numbered-circle` | `x`, `y`, `number` | `x`,`y` 為圓心；`number` 為顯示的整數序號（≥ 0）。 |

`text` 額外欄位：

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `text` | string | 必填 | 支援繁中 / 日文 / 韓文。 |
| `variant` | string | 選填 | 僅允許 `"solid"` 或 `"outline"`，分別對應紅底白字與紅邊白底。 |
| `fontSize` | number | 選填 | 預設依 baseline。 |
| `maxWidth` | number | 選填 | 超出時可換行。 |

`blur` 額外欄位：

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `blurRadius` | number | 選填 | blur 強度，預設依 baseline，建議預設為 `12`。 |
| `cornerRadius` | number | 選填 | 圓角半徑，預設依 baseline，建議預設為 `18`。 |
| `feather` | number | 選填 | 邊緣柔化量，若未提供則由實作採用與 `blurRadius` 相容的預設值。 |

`numbered-circle` 額外欄位：

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `number` | integer | 必填 | 圓圈顯示的數字，須 ≥ 0。 |
| `radius` | number | 選填 | 圓半徑，預設 `20`，範圍 6–200。 |
| `textColor` | string | 選填 | 數字顏色，預設 `#FFFFFF`。 |

`numbered-circle` 沿用通用欄位：`strokeColor`（主填色，預設 `#E53935`）、`outlineColor`（外白邊，預設 `#FFFFFF`）、`strokeWidth`（外白邊粗細，預設 `6`）。

輸入範例：

```json
[
  {
    "id": "ann-arrow-1",
    "type": "arrow",
    "x1": 96,
    "y1": 120,
    "x2": 312,
    "y2": 228
  },
  {
    "id": "ann-rect-1",
    "type": "rectangle",
    "x": 344,
    "y": 88,
    "width": 220,
    "height": 132
  },
  {
    "id": "ann-ellipse-1",
    "type": "ellipse",
    "x": 620,
    "y": 96,
    "width": 168,
    "height": 124
  },
  {
    "id": "ann-blur-1",
    "type": "blur",
    "x": 850,
    "y": 430,
    "width": 196,
    "height": 116,
    "blurRadius": 12,
    "cornerRadius": 18
  },
  {
    "id": "ann-text-1",
    "type": "text",
    "x": 140,
    "y": 264,
    "text": "這裡要修正",
    "variant": "solid",
    "fontSize": 24,
    "maxWidth": 220
  },
  {
    "id": "ann-step-1",
    "type": "numbered-circle",
    "x": 420,
    "y": 360,
    "number": 1,
    "radius": 20
  }
]
```

`--inject-svg` 成功時，`data` **至少**包含：

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `format` | string | 必填 | 固定為 `"svg"`。 |
| `mimeType` | string | 必填 | 固定為 `"image/svg+xml"`。 |
| `svg` | string | 必填 | 完整單檔 SVG 字串。 |
| `annotationCount` | integer | 必填 | 實際渲染的 annotation 數量。 |
| `canvas` | object | 選填 | 合成畫布尺寸，如 `width`, `height`。 |

規格約束：

1. 所有實作軌對同一份 `--inject-svg` payload 必須產生語義一致的結果。
2. 若 annotation 欄位缺失或 `type` 非法，必須回傳 `status="error"` 與 `1300-1399` 區間錯誤碼，不得 silent fallback。
3. GUI 匯出與 CLI `--inject-svg` 的向量幾何基準都必須對齊 `design/symbols.svg`；`blur` 的幾何與預設值則對齊 design baseline 中的 blur region tokens。
4. `blur` 不得退化為單純半透明遮罩；輸出結果必須保留實際可辨識的模糊效果。
5. SVG 匯出不得依賴外部 CSS、外部字型檔、script 或 `foreignObject` 才能完成主要渲染；需優先使用 SVG 原生元素、presentation attributes 與可離線內嵌資產，以提高 Inkscape round-trip 相容性。
6. JPG 與 PDF 匯出可扁平化，但輸出前的合成結果在幾何位置、標註顏色與 blur 區域語義上必須與 SVG / GUI 預覽對齊。

---

## 3. 非功能性需求 (Non-Functional Requirements)

- **極速回應 (Performance)：** CLI 模式下的指令回應延遲需小於 100 毫秒；GUI 模式冷啟動時間需小於 1 秒。常駐背景時的記憶體佔用需極小化。

- **大眾化安裝 (Distribution)：** 必須提供大眾友善的安裝包。Windows (`.exe`)、macOS (`.dmg`)、Debian (`.AppImage` 或 Flatpak)。禁止要求使用者開啟終端機安裝依賴套件。

- **離線與隱私 (Local-First)：** 所有截圖、Base64 轉換與 SVG / PNG / JPG / PDF 生成必須 100% 在本地端完成，無需依賴任何雲端服務。

---

## 4. Linux (Debian/Ubuntu) 環境部署與技術地雷

在跨平台桌面開發中，Linux（特別是 Debian 系）為最大技術風險區。若採用依賴 Web 技術的框架，開發團隊必須預防並克服以下三大技術天坑：

- **P1. WebKitGTK 依賴地獄 (Dependency Hell)：**

    - **問題：** Linux 沒有統一的系統級 WebView，Wails/Tauri 皆依賴 `webkit2gtk`。若使用者系統未安裝相關函式庫，軟體將無法啟動。

    - **對策：** 發佈時必須嚴格設定 `.deb` 依賴規則，或採用 AppImage/Flatpak 容器化打包技術，將環境一併封裝。

- **P2. Wayland 顯示伺服器的「截圖權限隔離」：**

    - **問題：** 現代 Debian 預設使用 Wayland。基於安全隔離，傳統的 X11 全域截圖 API 完全失效。Go 與 Rust 皆無內建完美支援 Wayland 截圖的輕量現成方案。

    - **對策：** 開發團隊必須在後端手寫 D-Bus 請求，呼叫 Linux 的 `XDG Desktop Portal` 介面，觸發系統原生的截圖授權機制。

- **P3. 透明無框視窗與系統列碎裂化：**

    - **問題：** WebKitGTK 搭配 Wayland 時，極易出現背景無法透明或滑鼠穿透失效的 Bug。

    - **對策：** 需評估在 Linux 平台上放棄「全螢幕透明遮罩」的選取模式，改由系統擷取全螢幕後，於應用程式內部滿版顯示並進行裁切與標註。

---

## 5. 技術選型與框架對比 (Architecture Decision)

為了在「完美的 Web UI 標註體驗」、「LLM 極速指令列整合」以及「Linux 原生相容性」三者間取得平衡，開發團隊將從以下三個架構中進行最終決策：

| **評估維度** | **Wails (Go + Web 前端) 🏆 首選** | **Tauri (Rust + Web 前端)** | **Qt (C++ 或 Python / PySide6)** |
|---|---|---|---|
| **LLM 調用與 CLI 效能** | **極佳。** Go 語言是開發高併發 CLI 與後端應用的業界霸主，架構極其優雅。啟動延遲趨近於零。 | **極佳。** Rust 效能頂尖。但異步處理與狀態共享的心智負擔極重。 | **良好。** Python/C++ 開發 CLI 成熟，但 Python 啟動較慢。若未來需結合 AI 推論則具優勢。 |
| **UI 開發體驗 (SVG 標註)** | **極佳。** 完全享受前端龐大的 Canvas/SVG 生態系與 CSS 樣式，完美支援中文輸入。 | **極佳。** 與 Wails 相同，由前端 Web 技術主導介面。 | **中等。** 需使用 `QGraphicsScene` 刻畫，撰寫複雜的 Skitch 標註需較高學習成本。 |
| **Linux (Wayland) 相容性** | **高風險。** 需手刻 D-Bus 呼叫，且需處理 WebKitGTK 依賴與透明視窗破圖問題。 | **高風險。** 同 Wails，依賴 WebKitGTK，需手刻底層系統呼叫。 | **極佳。** Qt 底層自動相容 X11 與 Wayland，`QScreen` 完美封裝跨平台截圖。 |
| **打包與分發 (給大眾)** | **佳。** 單一執行檔，檔案極小 (~15MB)。需謹慎處理 Linux 依賴。 | **極佳。** 體積最小 (~10MB)。需處理 Linux 依賴。 | **中等。** 跨平台一致性最高。但 Python 打包體積龐大 (~100MB+)。 |

**決策結論：**

基於本產品高度仰賴「與 AI 代理的 CLI 整合」以及「靈活的向量標註」兩大核心價值，**強烈建議選用 Wails (Go) 作為基礎框架。** 團隊應將初期研發資源集中於克服 Linux (Wayland) 的 D-Bus 截圖 API 串接與打包封裝。

---

## 6. 階段性執行計畫 (Roadmap)

### Phase 1: 核心技術驗證 (PoC) - [預估時程：2-3 週]

- **目標 1：** 使用 Go (Wails) 完成基礎雙模式切換。驗證 CLI 模式下抓取畫面並以 JSON + Base64 格式輸出至 `stdout` 的穩定性。

- **目標 2：** 在 Debian (Wayland) 環境下，成功透過 Go 呼叫 D-Bus (XDG Desktop Portal) 完成截圖。

### Phase 2: 最小可行性產品 (MVP) 開發 - [預估時程：4-6 週]

- 實作螢幕區域框選 UI (處理跨平台透明遮罩議題)。

- 完成前端 Web 向量標註工具列 (箭頭、方框、文字)。

- 實作 LLM `--inject-svg` 注入標註並合成單一 SVG 檔案的後端邏輯，並確保其匯出的 SVG 可在 Inkscape round-trip。

- 完成系統常駐列 (System Tray) 與全域快捷鍵綁定。

### Phase 3: 封測與發佈 (Beta & Release) - [預估時程：2-3 週]

- 使用 `electron-builder` 或類似工具打包 Windows `.exe`、macOS `.dmg` 與 Linux `.AppImage`，並驗證 SVG、PNG、JPG、PDF 匯出結果。

- 在 BuildPolis 內部與特定測試群體進行跨平台穩定度測試，並邀請 AI 開發代理 (如 Claude Code) 進行實際 API 串接測試。

- 建立專案 README 說明 CLI 呼叫規範，正式對外發布。
