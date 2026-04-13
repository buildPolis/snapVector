# wails/ — Wails (Go + Web) 實作 PRD

> 本文件之規格事實來源為專案根目錄 `../PRD.md`。此處僅描述 `wails/` 軌如何達成該規格。

## 定位

以 **Wails (Go 後端 + Web 前端)** 實作根 PRD §2 全部核心功能。本軌為根 PRD §5 決策結論中的**量產首選候選**，戰略重心放在**與 AI 代理的 CLI 整合體驗**——Go 在高併發 CLI 與後端的成熟度，讓 LLM tool-calling 的延遲與穩定性最佳化。

## 對應根 PRD 條目

全數實作：§2.1 螢幕擷取、§2.2 向量標註、§2.3 檔案匯出、§2.4 AI 代理 CLI。**§3 非功能性需求** 的指標（CLI < 100 ms、GUI 冷啟動 < 1 s、二進位 ~15 MB）為本軌重點 benchmark 對象。

## 核心實作點

### 專案結構

- 根 `main.go` 作為雙模式分派入口。
- `app.go` 定義 `App` struct，方法**同時**供前端 binding 與 CLI 共用，避免邏輯重複。
- `frontend/` 為前端資產，從 `../design/` 起步移植。

### 螢幕擷取（F1.1、F1.2）

- **Windows**：`github.com/kbinani/screenshot` 或 Win32 API 包裝。
- **macOS**：同上 crate；必要時透過 cgo 呼叫 `CGWindowListCreateImage`。
- **Linux Wayland**：`github.com/godbus/dbus/v5` 呼叫 XDG Desktop Portal `org.freedesktop.portal.Screenshot` 介面，觸發系統原生權限授權。
- **Linux X11** fallback：`screenshot` crate 即可。

### 向量標註（F2.1、F2.2、F2.3、F2.4）

- 前端照 `../design/` 移植，SVG `<symbol>` 直接複用 `symbols.svg`。
- `blur` 區域在 Go 端合成時需對底圖套用裁切 + blur filter，預設 `blurRadius` / `cornerRadius` 對齊 `../design/` baseline。
- SVG 合成邏輯放在 Go 端（`svg_io.go`），確保 CLI `--inject-svg` 與 GUI 匯出走同一條路徑。
- SVG 匯出必須可在 Inkscape 開啟、編輯、另存且主要視覺不走樣；PNG / JPG / PDF 皆由同一合成結果導出，其中 JPG 以白底扁平化，PDF 為單頁分享用輸出。
- CJK 輸入交由 WebView2 / WKWebView / WebKitGTK 的原生 IME 處理。

### 雙模式（F4.1）

`main.go` 結構：

```go
func main() {
    if isCLI(os.Args) {
        os.Exit(runCLI(os.Args))  // 完全不呼叫 wails.Run
    }
    wails.Run(&options.App{...})
}
```

- CLI 路徑**絕對不**呼叫 `wails.Run`，避免 WebView 啟動污染延遲。
- 使用 `flag` 或 `spf13/cobra` 處理參數；若為單純 flags，`flag` 套件已足夠。

### CLI JSON 輸出（F4.2、F4.3、F4.4）

- Go 端資料結構與 JSON encoder 必須嚴格對齊根 `../PRD.md` §2.4 的 **Canonical CLI JSON response schema**，不得由本軌單獨增減頂層欄位。
- 禁止散落的 `fmt.Println`；所有 stdout 輸出走 `json.NewEncoder(os.Stdout).Encode(resp)`。
- `--capture --base64-stdout`、`--inject-svg` schema 與 qt/tauri 兩軌完全一致，且以根 `../PRD.md` §2.4 為單一事實來源。

## 打包

- `wails build` 產出原生二進位。
- Linux 額外包 AppImage（封裝 `libwebkit2gtk-4.1` 依賴）。
- 回報二進位體積與 CLI 冷啟動延遲，作為三軌決勝基準。

## Benchmark 重點（本軌決勝指標）

| 指標 | 目標 | 量測方式 |
|---|---|---|
| CLI 冷啟動延遲 | < 100 ms | `hyperfine './snapvector --capture --base64-stdout > /dev/null'` |
| GUI 冷啟動 | < 1 s | 目測 + 腳本量測第一個 IPC 訊息 |
| 二進位體積 | ~15 MB | `ls -lh` |
| Base64 輸出吞吐 | 4K 螢幕 < 300 ms | benchmark 腳本 |

## 驗收標準

1. 三平台 GUI 模式可擷取、標註、匯出 SVG、PNG、JPG 與 PDF。
2. Debian Wayland 首次啟動可觸發 XDG Portal 權限彈窗，授權後穩定截圖。
3. Claude Code 能透過 `--capture --base64-stdout` 與 `--inject-svg` 成功執行端到端視覺任務。
4. Linux AppImage 在乾淨 Debian 容器中可直接執行。
5. CLI 冷啟動延遲實測符合 < 100 ms 目標。
6. SVG 匯出可在 Inkscape 開啟、編輯、另存且主要視覺不走樣；JPG 為白底扁平化；PDF 為單頁分享用文件。
