# CLAUDE.md — tauri/

Tauri (Rust + Web) 軌工作守則。規格見 `./PRD.md`。

## 環境

- **Rust**：stable toolchain，透過 `rustup` 安裝。
- **Tauri CLI**：`cargo install tauri-cli --version '^2'`（使用 Tauri v2）。
- **前端套件管理**：預設 `pnpm`；若前端保持純 HTML 直接照抄 `../design/`，可省略 Node 依賴。
- **Linux 建置依賴**（僅列出，不自動安裝）：`libwebkit2gtk-4.1-dev`、`build-essential`、`libssl-dev`、`libayatana-appindicator3-dev`、`librsvg2-dev`。

## 指令速查

```
cargo tauri dev                                   # GUI 開發模式
cargo tauri build                                 # 打包
cargo run -- --capture --base64-stdout            # CLI 截圖（dev 階段）
./target/release/snapvector --inject-svg '<json>' # CLI 標註注入
```

## 檔案結構

```
tauri/
├── src-tauri/
│   ├── Cargo.toml
│   ├── tauri.conf.json
│   └── src/
│       ├── main.rs        # argv 分派（CLI vs GUI）
│       ├── cli.rs         # headless 流程
│       ├── capture.rs     # xcap / ashpd 包裝
│       ├── svg_io.rs      # SVG 合成
│       └── commands.rs    # Tauri Commands
└── src/                   # 前端（從 ../design/ 起步）
```

## 實作約束

- **CLI 絕對不初始化 Tauri Builder**：CLI 路徑需跳過 `tauri::Builder::default()`，否則延遲與體積測試失去意義。
- **D-Bus**：使用 `ashpd` crate，不要用 `zbus` 自己刻 Portal 呼叫。
- **截圖 crate**：`xcap`（跨平台）為首選；避免 shell out 到作業系統截圖工具。
- **JSON 合約**：`CliResponse` struct 使用 `serde` derive，欄位與 `../qt/`、`../wails/` 完全一致。
- **參數解析**：統一用 `clap` derive API。

## 設計稿移植

- 前端優先直接複製 `../design/` 目錄內容到 `src/`。
- 若需引入 Vite 或其他 bundler，維持 `symbols.svg` 作為單一事實來源，不要改動幾何參數或配色。

## 禁忌

- 不要在 `package.json` / Cargo.toml 引入非必要的重量級相依。
- 不要對三平台寫三套 capture 實作——用條件編譯 `#[cfg(target_os = ...)]` 在同一 module 內處理。
- 不要複製根 `../PRD.md` 大段內容到本檔案。
