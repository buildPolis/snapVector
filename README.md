# SnapVector

跨平台螢幕截圖與向量標註工具，採三軌平行開發：

- `qt/`: Qt + PySide6，主打 Wayland 原生相容性驗證
- `tauri/`: Tauri + Rust + Web，主打體積與效能
- `wails/`: Wails + Go + Web，主打 CLI 與桌面整合，**目前唯一已經有可建置程式碼的軌道**

產品需求與單一事實來源在 `PRD.md`。

## Repository layout

```text
.
├── design/   # 共用 HTML/CSS/SVG 視覺原型
├── qt/       # Qt 軌
├── tauri/    # Tauri 軌
├── wails/    # Wails 軌
├── plan/     # 路線圖與執行 checklist
├── PRD.md
└── CLAUDE.md
```

## Current implementation status

| Track | Buildable now | Installable now | Notes |
|---|---|---|---|
| `wails/` | Yes | Local binary only | CLI、annotation/export、目前 GUI shell 已接線 |
| `qt/` | No | No | 目前 repo 內尚未建立 Python/uv 專案檔 |
| `tauri/` | No | No | 目前 repo 內尚未建立 Rust/Node 專案檔 |

## Supported operating systems

PRD 目標平台：

- macOS
- Linux (Debian/Ubuntu，需相容 Wayland)
- Windows 10/11

目前 repo 內**真正可驗證**的狀態：

| OS | Wails CLI | Wails GUI build | Notes |
|---|---|---|---|
| macOS | Yes | Yes | 需要 Screen Recording 權限 |
| Linux | Partial | Build not verified here | capture backend 仍是 stub |
| Windows | Partial | Build not verified here | capture backend 仍是 stub |

## Build prerequisites

### All platforms

- Git
- Go 1.22+

### macOS

- Xcode Command Line Tools
- Screen Recording permission for real capture

### Linux

- Go 1.22+
- 桌面環境相依套件尚未在 repo 內固定，因 Linux capture/backend 仍未完成

### Windows

- Go 1.22+
- Windows-specific capture/backend 仍未完成

## Build and run

### Wails track

`wails/` 是目前唯一可建置的實作。

```bash
cd wails
go build -o ./build/bin/snapvector .
go build -tags production -o ./build/bin/snapvector-production .
```

輸出檔：

- `./build/bin/snapvector`: CLI binary
- `./build/bin/snapvector-production`: Wails GUI binary

執行方式：

```bash
./build/bin/snapvector --version
./build/bin/snapvector --capture --base64-stdout
./build/bin/snapvector --inject-svg '[]'
./build/bin/snapvector-production
```

更多 Wails 軌細節見 `wails/README.md`。

## GitHub Actions CI/CD

repo 現在包含兩個針對 `wails/` 軌的 workflow：

- `.github/workflows/ci.yml`：在 PR / push 時跑 Linux 測試與 build，並在 macOS / Windows 做 Wails smoke build。
- `.github/workflows/release.yml`：在 `v*` tag 或手動觸發時產出 unsigned release artifacts，並發佈到 GitHub Release。

### 觸發 CI（push / pull request）

推 code 到遠端分支時，GitHub Actions 會自動跑 `ci.yml`。常見指令：

```bash
# 建新分支並推上去，會觸發 push CI
git checkout -b feature/github-actions-release
git add .
git commit -m "Add GitHub Actions CI/CD"
git push -u origin HEAD
```

如果是已存在的分支，直接：

```bash
git add .
git commit -m "Update workflow docs"
git push
```

若你在 GitHub 上對該分支開 PR，`pull_request` 事件也會再跑一次 CI。

### 觸發 Release（tag）

建立並推送 `v*` 格式的 tag 時，GitHub Actions 會自動跑 `release.yml`：

```bash
git checkout main
git pull --ff-only
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

查看目前 tag：

```bash
git tag --list
```

如果 tag 打錯，先刪本地與遠端 tag 再重打：

```bash
git tag -d v0.1.0
git push origin :refs/tags/v0.1.0
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

### 手動觸發 Release

也可以不下 tag，直接從 GitHub UI 或 `gh` CLI 手動觸發：

```bash
gh workflow run release.yml -f tag_name=v0.1.0
```

手動觸發時，workflow 會用你提供的 `tag_name` 建立或更新 GitHub Release。

目前 release workflow 的目標產物：

| Platform | Raw artifact | Package / installer |
|---|---|---|
| macOS | `.app` | `.zip` |
| Windows | `.exe` | NSIS installer `.exe` |
| Linux | raw binary | `.deb` |

第一版 **不做 signing / notarization**；macOS 簽章需求請看 `wails/docs/macos-code-signing.md`。

### Qt track

目前 `qt/` 只有文件，尚未有 `pyproject.toml` 或可執行程式碼，因此**目前不能 build 或 install**。

等 Qt 軌 bootstrapped 後，會在該資料夾補齊：

- `uv` 建立與同步環境的指令
- 開發執行方式
- 打包與安裝方式

### Tauri track

目前 `tauri/` 只有文件，尚未有 `Cargo.toml`、`package.json` 或可執行程式碼，因此**目前不能 build 或 install**。

等 Tauri 軌 bootstrapped 後，會在該資料夾補齊：

- Rust/Node toolchain 版本需求
- 開發執行方式
- 打包與安裝方式

## Install

目前 repo **未在版本庫內直接附帶正式 installer / package**。可先用本地 build，或透過 GitHub Actions release workflow 產出 release artifacts。現階段本地安裝方式如下：

### macOS / Linux

直接保留 build 產物並從 `build/bin/` 執行，或自行複製到系統 PATH。

例如：

```bash
cd wails
go build -o ./build/bin/snapvector .
install -m 755 ./build/bin/snapvector /usr/local/bin/snapvector
```

GUI binary 同理：

```bash
cd wails
go build -tags production -o ./build/bin/snapvector-production .
install -m 755 ./build/bin/snapvector-production /usr/local/bin/snapvector-production
```

### Windows

目前沒有 MSI / installer。請先 build 出 binary，再直接從 `build\\bin\\` 執行。

## Cross-platform caveats

- `wails/` 的 CLI/GUI 主體目前以 macOS 驗證最完整。
- Linux 與 Windows 的 native capture backend、packaging、installer 還在 Phase 4。
- `qt/` 與 `tauri/` 尚未進入可建置狀態，README 先誠實標示為 not bootstrapped。

## Plans and execution tracking

- 路線圖：`plan/wails.md`
- 執行清單：`plan/todo.md`

done 只在功能已接線且實際跑通後才會標記。
