# qt/ — Qt (PySide6) 實作 PRD

> 本文件之規格事實來源為專案根目錄 `../PRD.md`。此處僅描述 `qt/` 軌如何達成該規格。

## 定位

以 **Qt (PySide6)** 實作根 PRD §2 全部核心功能。本軌的戰略價值為驗證 **Linux Wayland 原生相容性**——Qt 底層自動處理 X11/Wayland 雙堆疊，`QScreen` 封裝跨平台截圖，可繞過 Wails/Tauri 必須手刻 D-Bus 的技術負債（根 PRD §5、§P2）。

## 對應根 PRD 條目

全數實作：§2.1 螢幕擷取、§2.2 向量標註、§2.3 檔案匯出、§2.4 AI 代理 CLI。非功能性需求 §3 依 Qt 特性合理取值。

## 核心實作點

### 螢幕擷取（F1.1、F1.2）

- 使用 `QGuiApplication.screens()` 列舉所有螢幕。
- 使用 `QScreen.grabWindow(0)` 擷取全螢幕，Qt 自動處理 Wayland 權限。
- 區域擷取在 GUI 模式下透過全螢幕半透明遮罩 + 滑鼠拖曳實作。

### 向量標註（F2.1、F2.2、F2.3）

- 使用 `QGraphicsScene` + `QGraphicsItem` 子類別實作箭頭、矩形、橢圓、文字方塊。
- 幾何參數與配色**照搬** `../design/symbols.svg`，不重新設計。
- SVG 匯出：用 `QSvgGenerator` 將 scene 繪製到 SVG，結合底圖 Base64 合成單一檔案。
- CJK 輸入：使用 `QLineEdit` / `QTextEdit` 原生元件，Qt 已內建處理各平台 IME。

### 雙模式（F4.1）

`main.py` 結構：

```python
def main():
    if is_cli_mode(sys.argv):
        return run_cli(sys.argv)      # 不建立 QApplication GUI loop
    return run_gui(sys.argv)
```

- CLI 路徑允許建立 `QGuiApplication` 以使用 `QScreen` 等核心元件，但**不**進入 `app.exec()` event loop。
- 盡可能避免載入 `QtWidgets` 以縮短冷啟動。

### CLI JSON 輸出（F4.2、F4.3、F4.4）

- 遵循根 `../PRD.md` §2.4 中的 **Canonical CLI JSON response schema** 與 **Canonical `--inject-svg` payload schema**，不得自行改動頂層欄位或 annotation 欄位命名。
- `--capture --base64-stdout`：擷取螢幕 → PNG Base64 → 包裝 JSON 輸出至 stdout。
- `--inject-svg`：讀取 JSON 陣列 → 用 `QSvgRenderer` 合成標註 → 輸出合成後 SVG。

## 打包

- 評估 `pyside6-deploy`（Qt 官方）與 `briefcase`（Python 原生）。
- 產出 Windows `.exe`、macOS `.dmg`、Linux AppImage。
- 回報二進位體積供三軌比較。

## 範圍外

- **不**實作 Web 前端、**不**嵌入 WebView。
- **不**使用 PyQt6（授權考量），統一使用 PySide6（LGPL）。

## 驗收標準

1. 三平台（Windows、macOS、Debian Wayland）GUI 模式可擷取、標註、匯出 SVG 與 PNG。
2. Debian Wayland 下 CLI 截圖**無需額外設定**即可成功（Qt 原生支援的關鍵驗證）。
3. CLI `--capture --base64-stdout` 的 JSON 輸出可被 jq 解析且 Base64 可還原為合法 PNG。
4. `--inject-svg` 接受的 JSON schema 與 wails/tauri 兩軌**完全一致**，且以根 `../PRD.md` §2.4 的 canonical schema 為唯一依據。
