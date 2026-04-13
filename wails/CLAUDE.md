# CLAUDE.md — wails/

Wails (Go + Web) 軌工作守則。規格見 `./PRD.md`。本軌為量產首選候選，所有實作以「AI 代理 CLI 優先」為最高指導原則。

## 環境

- **Go**：1.22+（透過 `go.mod` 鎖定）。
- **Wails CLI**：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`。
- **前端**：預設 `npm`；若直接複用 `../design/` 純 HTML 可省略 Node 依賴。
- **Linux 建置依賴**（列出供打包腳本引用，不自動安裝）：
  - `gcc`、`pkg-config`
  - `libwebkit2gtk-4.1-dev`
  - `libgtk-3-dev`

## 指令速查

```
wails dev                                         # GUI 開發模式
wails build                                       # 打包
go run . --capture --base64-stdout                # CLI 截圖（dev 階段）
./build/bin/snapvector --inject-svg '<json>'      # CLI 標註注入
hyperfine './build/bin/snapvector --capture --base64-stdout > /dev/null'
```

## 檔案結構

```
wails/
├── go.mod
├── main.go           # argv 分派
├── cli.go            # headless 流程
├── app.go            # App struct（前端 binding + CLI 共用）
├── capture.go        # 平台條件編譯的截圖包裝
├── svg_io.go         # SVG 合成與匯出
└── frontend/         # 前端（從 ../design/ 起步）
```

## 實作約束

- **CLI 優先**：任何新功能先在 Go 側實作並通過 CLI 驗證（`go run . --new-flag ...`），再綁定到前端 Wails binding。AI 代理介面永遠先行。
- **CLI 絕對不呼叫 `wails.Run`**：CLI 路徑完全跳過 WebView 初始化。
- **JSON 合約**：所有 stdout 輸出透過單一 `CLIResponse` struct 與 `json.NewEncoder` 輸出。**禁止** `fmt.Println` / `fmt.Printf` 直接對 stdout 寫文字。錯誤訊息也要走 JSON（stderr 亦然，保持可解析）。
- **平台差異**：`capture_windows.go` / `capture_darwin.go` / `capture_linux.go` 透過 `//go:build` tag 分檔，不要在同一函式內大量 `runtime.GOOS` 判斷。
- **Wayland D-Bus**：使用 `github.com/godbus/dbus/v5` 呼叫 `org.freedesktop.portal.Screenshot`；對照 Tauri 軌的 `ashpd`，本軌是手刻層級的驗證。

## 設計稿移植

- `frontend/` 直接複製 `../design/` 內容起步，`symbols.svg` 保留原樣作為單一事實來源。
- 若引入 Vite，維持標註的 SVG `<symbol>` 結構不變，以便 Go 端合成邏輯能重用相同幾何資料。

## Benchmark 紀律

- 每次 PR 合併前量測 `hyperfine` 的 CLI 冷啟動延遲，記錄於 commit message。
- 回歸超過 10% 視為效能 bug，須回修。

## 禁忌

- 不要引入 cgo 依賴除非**非用不可**（每增加一項 cgo 依賴都會傷害跨平台打包與冷啟動）。
- 不要在 Go 端引入 HTML template 渲染——SVG 合成走字串拼接或 `encoding/xml`。
- 不要複製根 `../PRD.md` 大段內容到本檔案。
