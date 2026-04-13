# CLAUDE.md — design/

視覺原型工作守則。規格見 `./PRD.md`，專案全景見 `../CLAUDE.md`。

## 技術邊界

- **純前端**：HTML + CSS + SVG，允許極少量原生 JS 僅用於展示互動（hover、focus、切換）。
- **禁用框架**：不得引入 React、Vue、Svelte、Tailwind、Bootstrap 或任何需要 build step 的工具。輸出需能被 qt/tauri/wails 直接嵌入或參照，不留相依性。
- **無 build 步驟**：所有檔案雙擊即可在瀏覽器開啟。

## 預覽

```
python3 -m http.server 8000
# 或
uv run python -m http.server 8000
```

瀏覽 `http://localhost:8000/index.html`。

## 檔案結構慣例

```
design/
├── index.html          # 主原型
├── symbols.svg         # SVG <symbol> 元件庫（單一事實來源）
├── styles.css          # 共用 CSS token
└── components/
    ├── arrow.html
    ├── frame.html
    └── text.html
```

- 新增標註元件時，**先**在 `symbols.svg` 定義 `<symbol id="...">`，再在 `index.html` 與 `components/` 透過 `<use href="symbols.svg#...">` 引用。不要把幾何參數散落在各 HTML 檔案裡。
- 配色與尺寸 token 集中在 `styles.css` 的 `:root` CSS variables。

## CJK 字型

- `font-family` 必須明確列出 fallback chain，不依賴瀏覽器預設。
- `components/text.html` 保留一個可輸入的 `<textarea>` 與 `contenteditable` 區塊，方便實測 IME。

## 跨軌影響

- 此資料夾的變動被 qt/tauri/wails 三軌視為共同依賴。
- 修改 `symbols.svg` 的幾何或配色時，commit message 需註明 `design: ` 前綴並簡述影響面，讓三軌 session 能快速判斷是否需跟進移植。

## 禁忌

- 不要寫截圖、剪貼簿、匯出邏輯——那是三軌實作的工作。
- 不要在這裡討論 CLI、Wayland、D-Bus。
