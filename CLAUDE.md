# CLAUDE.md — SnapVector

跨平台螢幕截圖與向量標註工具。詳細產品需求見 `PRD.md`。

## 專案結構

本專案採**三軌平行開發 (Parallel Prototyping)** 策略，在三個獨立資料夾中以不同框架實作同一份 PRD，最終比較效能、開發體驗與 Linux (Wayland) 相容性後擇一量產。

```
snapVector/
├── PRD.md              # 產品需求規格書 (單一事實來源)
├── CLAUDE.md           # 本檔案
├── design/             # HTML/CSS 設計稿 (Skitch 風格視覺原型)
├── qt/                 # Qt (PySide6) 實作
├── tauri/              # Tauri (Rust + Web) 實作
└── wails/              # Wails (Go + Web) 實作
```

### 資料夾職責

- **`design/`**：純 HTML + CSS + SVG，產出 Skitch 風格的標註工具列、箭頭、外框、文字方塊等視覺原型。此為三種框架共用的 UI 設計參考，不含任何後端邏輯。
- **`qt/`**：使用 PySide6 + `QGraphicsScene`。用 `uv` 管理 Python 環境。主打 Wayland 原生相容性驗證。
- **`tauri/`**：使用 Rust + Web 前端。主打最小體積與效能。
- **`wails/`**：使用 Go + Web 前端。主打 CLI 整合與 AI 代理串接體驗（PRD 首選）。

## 開發守則

- **獨立開發**：三個框架資料夾彼此**互不依賴**，不共用程式碼。`design/` 是視覺參考，由各框架各自移植。
- **功能對齊**：三個實作皆需完成 PRD §2 核心功能，特別是 §2.4 CLI 雙模式與 `--base64-stdout` / `--inject-svg`。
- **比較基準**：CLI 回應延遲、GUI 冷啟動時間、打包體積、Wayland 截圖成功率。
- **繁體中文**：回覆與文件使用繁體中文；程式碼識別字與技術術語維持原文。
- **Python 環境**：`qt/` 一律使用 `uv`，禁止 pip/conda/poetry。
- **命名規範**：檔案與資料夾使用連字號或底線，不用空格。

## 工作流程

當使用者指定在特定框架下開發時，`cd` 進對應資料夾後再進行，不要跨資料夾修改。`design/` 的變更應同步通知三個框架資料夾是否需要跟進。

## Todo 工作法

- **Roadmap 與執行清單分離**：長期規劃寫在 `plan/<track>.md`，可執行細項寫在 `plan/todo.md`。
- **Done 要有證據**：只有在功能真的實作完成，且已用對應測試、建置或實際指令驗證可用後，才能把項目標成 done。
- **部分完成不算整體完成**：若某個 phase 只完成其中一段（例如只完成 SVG 注入，尚未完成 PNG/JPG/PDF/clipboard），該 phase 仍維持 in progress，並把剩餘缺口列在 `plan/todo.md`。
- **狀態需同步**：進行中的工作要同步更新 `todos` / `todo_deps` SQL 狀態，以及對應的 `plan/todo.md` 勾選狀態，避免文件與實作脫節。
- **以可運行為準**：禁止因為 stub、假資料、未接線 UI、或未實測路徑而提早宣稱完成。
