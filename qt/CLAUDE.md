# CLAUDE.md — qt/

Qt (PySide6) 軌工作守則。規格見 `./PRD.md`。

## 環境

- **一律使用 `uv`**：`uv init`、`uv add pyside6`、`uv run python main.py`。
- **禁用** pip、conda、poetry、pyenv。
- Python 3.14.3（對齊使用者全域設定）。
- PySide6（LGPL）**不**使用 PyQt6（GPL）。

## 指令速查

```
uv sync                                           # 安裝依賴
uv run python main.py                             # GUI 模式
uv run python main.py --capture --base64-stdout   # CLI 截圖
uv run python main.py --inject-svg '<json>'       # CLI 標註注入
```

## 入口結構

`main.py` 須先解析 `sys.argv` 判斷 CLI/GUI，再載入對應模組：

```python
if __name__ == "__main__":
    if any(flag in sys.argv for flag in ("--capture", "--inject-svg")):
        from cli import run_cli
        sys.exit(run_cli(sys.argv))
    from gui import run_gui
    sys.exit(run_gui(sys.argv))
```

CLI 路徑**不得** `import PySide6.QtWidgets`，僅 `QtGui` / `QtCore` / `QtSvg`，以壓低冷啟動。

## 實作約束

- **SVG 合成**：用 `QSvgRenderer` + `QSvgGenerator`，不要手拼字串。
- **CJK 輸入**：用 `QLineEdit` / `QTextEdit` 原生元件，不要自造 IME 處理邏輯。
- **標註視覺**：幾何與配色參數照 `../design/symbols.svg`，在 `annotations/` 下實作各 `QGraphicsItem` 子類別時直接引用該 SVG 的數值。
- **JSON schema**：與 `../wails/`、`../tauri/` 對齊，任何欄位變動需同步更新三軌。

## 檔案結構建議

```
qt/
├── pyproject.toml
├── main.py           # argv 分派
├── cli.py            # headless 流程
├── gui.py            # QApplication + 主視窗
├── capture.py        # QScreen 包裝
├── annotations/      # QGraphicsItem 子類別
└── svg_io.py         # QSvgRenderer / QSvgGenerator 合成與匯出
```

## 禁忌

- 不要引入 Qt Quick / QML（超出 PRD 範圍且徒增打包體積）。
- 不要在 CLI 路徑呼叫 `QApplication.exec()`。
- 不要重複根 `../PRD.md` 的大段內容到本軌檔案。
