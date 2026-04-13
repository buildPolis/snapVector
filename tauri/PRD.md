# tauri/ — Tauri (Rust + Web) 實作 PRD

> 本文件之規格事實來源為專案根目錄 `../PRD.md`。此處僅描述 `tauri/` 軌如何達成該規格。

## 定位

以 **Tauri (Rust 後端 + Web 前端)** 實作根 PRD §2 全部核心功能。本軌的戰略價值為驗證**最小體積** (~10 MB) 與 Rust 後端效能上限，同時承擔 WebKitGTK + Wayland 技術地雷（根 PRD §P1、§P2、§P3）的實戰驗證。

## 對應根 PRD 條目

全數實作：§2.1 螢幕擷取、§2.2 向量標註、§2.3 檔案匯出、§2.4 AI 代理 CLI。

## 核心實作點

### 專案結構

- `src-tauri/`：Rust 後端（Cargo crate），定義 Tauri Commands 與 CLI 分派邏輯。
- `src/`：Web 前端，從 `../design/` 起步移植，可保持純 HTML 或引入輕量 bundler（Vite）視需要決定。

### 螢幕擷取（F1.1、F1.2）

- **Windows/macOS**：使用 `xcap` 或 `screenshots` crate，避免 shell out 到 `screencapture` / PowerShell。
- **Linux Wayland**：用 `ashpd` crate 包裝 XDG Desktop Portal `Screenshot` 介面，觸發系統原生權限授權；**不**自己手寫 D-Bus wire 協定。
- **Linux X11** fallback：`xcap` 即可。

### 向量標註（F2.1、F2.2、F2.3、F2.4）

- 前端完全照 `../design/` 的 HTML/CSS/SVG 移植。
- `blur` 區域由 Rust 或前端共用渲染層對底圖裁切後套用 blur；其預設 `blurRadius` / `cornerRadius` 必須對齊 `../design/` baseline。
- SVG 合成可在前端或 Rust 端完成；傾向在 Rust 端完成以便 CLI 模式共用邏輯。
- SVG 輸出必須可在 Inkscape 開啟、編輯、另存且主要視覺不走樣；PNG / JPG / PDF 皆由同一合成結果導出，其中 JPG 以白底扁平化，PDF 為單頁分享用輸出。
- CJK 輸入由 WebKitGTK / WKWebView / WebView2 的原生 IME 支援處理。

### 雙模式（F4.1）

`src-tauri/src/main.rs`：

```rust
fn main() {
    let args: Vec<String> = std::env::args().collect();
    if is_cli(&args) {
        std::process::exit(run_cli(&args));     // 完全不啟動 Tauri builder
    }
    tauri::Builder::default()...run(...);
}
```

- CLI 路徑**絕對不**初始化 `tauri::Builder`，避免 WebView 啟動成本污染延遲測量。

### CLI JSON 輸出（F4.2、F4.3、F4.4）

- 使用 `serde_json` 嚴格遵循根 `../PRD.md` §2.4 的 **Canonical CLI JSON response schema** 與 **Canonical `--inject-svg` payload schema**。
- `--capture --base64-stdout`、`--inject-svg` 行為與 qt/wails 兩軌完全一致。
- 使用 `clap` 處理參數解析。

## 打包

- `cargo tauri build` 產出 Windows `.exe` / `.msi`、macOS `.dmg`、Linux AppImage（額外配置 AppImage bundler）。
- 回報二進位體積供三軌比較。

## 風險監控

| 風險 | 對應 | 備註 |
|---|---|---|
| WebKitGTK 依賴（§P1） | AppImage 容器化打包 | 禁止要求使用者手動 `apt install` |
| Wayland 截圖權限（§P2） | `ashpd` + XDG Portal | 必須能觸發原生授權彈窗 |
| 透明視窗破圖（§P3） | Linux 放棄全螢幕透明遮罩 | 改為擷取後於應用內裁切 |

## 範圍外

- 不追求前端框架複雜度——能用純 HTML 滿足 PRD 就不引入 React/Vue。
- 不實作雲端同步、不接 OCR。

## 驗收標準

1. 三平台 GUI 模式可擷取、標註、匯出 SVG、PNG、JPG 與 PDF。
2. Debian Wayland 首次啟動能觸發 XDG Portal 權限彈窗，授權後穩定截圖。
3. CLI `--capture --base64-stdout` 回應延遲 < 100 ms（冷啟動，根 PRD §3）。
4. Linux AppImage 在未預裝 webkit2gtk 的乾淨 Debian 容器中可直接執行。
5. SVG 匯出可在 Inkscape 開啟、編輯、另存且主要視覺不走樣。
