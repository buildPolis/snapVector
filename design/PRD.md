# design/ — Skitch 風格視覺原型 PRD

> 本文件之規格事實來源為專案根目錄 `../PRD.md`。此處僅描述 `design/` 軌的實作範圍與驗收標準。

## 定位

`design/` 為**純 HTML + CSS + SVG** 的視覺原型資料夾，產出 Skitch 風格標註的視覺語言，作為 `../qt/`、`../tauri/`、`../wails/` 三個實作軌的共同 UI 參考來源。本軌**不涉及**任何截圖邏輯、CLI、後端程式碼。

## 對應根 PRD 條目

- 根 PRD §2.2 **向量標註引擎**（F2.1 Skitch 風格、F2.2 SVG `<symbol>` 獨立元件系統、F2.3 CJK 輸入體驗）。
- 根 PRD §2.3 **檔案匯出與整合**（F3.1 相容 Inkscape 的獨立 SVG 視覺樣板，以及 PNG / JPG / PDF 匯出所依據的視覺基準）。

## 交付物

| 檔案 | 用途 |
|---|---|
| `index.html` | 主畫面原型：假截圖底圖 + 標註工具列 + 五種標註示範（箭頭、矩形、橢圓、文字方塊、blur 區域）。 |
| `symbols.svg` | SVG `<symbol>` 元件庫。定義紅色白邊箭頭、矩形、橢圓、文字方塊、blur 區域 baseline 與工具圖示，供三軌移植時直接引用或參照，且需維持可被 Inkscape 開啟與編輯的結構。 |
| `components/arrow.html` | 單獨展示箭頭元件（多角度、多長度測試）。 |
| `components/blur.html` | blur 區域元件（不同強度與圓角半徑示範）。 |
| `components/frame.html` | 矩形與橢圓外框。 |
| `components/text.html` | 文字方塊與 CJK 輸入測試區（繁中、日文、韓文實測）。 |
| `styles.css` | 共用樣式（配色 token、字型 fallback）。 |

## 視覺規範（Skitch 對齊）

- **主色**：紅色 `#E53935` 系，對比白邊 `stroke-width: 3px` 外描邊。
- **箭頭**：粗線條的**直線**箭頭，箭頭尖端為對稱三角形，整體具白邊外描邊以保證在深色背景可辨識；不要做成彎箭頭或彎尾塊狀輪廓。
- **幾何外框**：空心、粗紅線、白邊描邊。
- **文字方塊**：紅底白字或紅邊白底兩款變體。
- **blur 區域**：圓角矩形，預設 `cornerRadius: 18`、`blurRadius: 12`，需明顯表達「正在模糊底圖」而非單純半透明遮罩。
- **字型 fallback**：`-apple-system, "Noto Sans CJK TC", "PingFang TC", "Microsoft JhengHei", sans-serif`。
- **互動語意**：箭頭以滑鼠左鍵拖拉建立與縮放，baseline 示意需能明確對應 start / end points，而非自由曲線。

## 範圍外

- **禁止**任何 JavaScript 後端互動邏輯（僅允許純展示用的 hover/focus）。
- **禁止**引入任何 CSS/JS 框架（React/Vue/Tailwind/Bootstrap 全部禁用）。
- 不實作截圖擷取、不處理剪貼簿、不輸出合成 SVG 檔案。

## 驗收標準

1. 在 Chrome、Safari、Firefox 三者開啟 `index.html`，視覺一致且接近 Evernote Skitch 的辨識度。
2. `symbols.svg` 可獨立在瀏覽器開啟並正確渲染所有 `<symbol>`，並可在 Inkscape 開啟、編輯、另存且主要視覺不走樣。
3. `components/text.html` 可實際輸入繁體中文、日文、韓文，無排版跑版。
4. 三軌（qt/tauri/wails）開發者依據 `symbols.svg` 與 `components/blur.html` 的幾何參數即可完成該軌標註 UI，無需再反覆詢問配色、blur 強度或尺寸。
