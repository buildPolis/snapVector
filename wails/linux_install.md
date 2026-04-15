# SnapVector Linux 安裝指南

本文針對 `wails/` 軌，說明 Linux 上的建置、安裝、啟動與 `.deb` 打包流程。

## 1. 建置需求

Ubuntu 24.04+：

```bash
sudo apt-get install -y \
  libgtk-3-dev \
  libwebkit2gtk-4.1-dev \
  librsvg2-bin \
  xclip
```

另外需要：

- Go
- Wails CLI
- Node.js / npm

若使用 `nvm`：

```bash
source ~/.nvm/nvm.sh
nvm use 24
```

## 2. 建置正式版

在 `wails/` 目錄下：

```bash
wails build -clean -tags "webkit2_41 production"
```

輸出檔案：

```bash
build/bin/snapvector
```

## 3. 使用 raw binary 安裝

建議安裝到 `~/.local/bin`：

```bash
mkdir -p ~/.local/bin
install -m 755 build/bin/snapvector ~/.local/bin/snapvector
```

第一次啟動：

```bash
~/.local/bin/snapvector
```

首次啟動會自動建立 user-level Linux 桌面整合檔案：

- `~/.local/share/applications/snapvector.desktop`
- `~/.local/share/icons/hicolor/512x512/apps/snapvector.png`
- `~/.local/share/pixmaps/snapvector.png`

之後建議從應用程式選單或桌面捷徑啟動，不要直接反覆用 `./build/bin/snapvector`。

## 4. 建立桌面捷徑

若需要桌面捷徑，可建立：

```bash
DESKTOP_DIR="$(xdg-user-dir DESKTOP 2>/dev/null || echo "$HOME/Desktop")"
cat > "$DESKTOP_DIR/SnapVector.desktop" <<EOF
#!/usr/bin/env xdg-open
[Desktop Entry]
Version=1.0
Terminal=false
Type=Application
Name=SnapVector
Exec=$HOME/.local/bin/snapvector %U
TryExec=$HOME/.local/bin/snapvector
Icon=snapvector
StartupWMClass=Snapvector
StartupNotify=true
Comment=Cross-platform screenshot and vector annotation tool
Categories=Graphics;Utility;
EOF

chmod +x "$DESKTOP_DIR/SnapVector.desktop"
gio set "$DESKTOP_DIR/SnapVector.desktop" metadata::trusted true
```

上面會自動偵測 Desktop 目錄，不必手動猜測 `~/桌面` 或 `~/Desktop`。

## 5. Dock / App Grid 注意事項

- 若安裝後 Dock 還是顯示舊圖示，先取消固定舊項目，再從新的 launcher 重新開啟並固定。
- 若應用程式列表沒立即刷新，可重新登入 GNOME session。
- 可手動刷新部分桌面資料庫：

```bash
update-desktop-database ~/.local/share/applications
gtk-update-icon-cache -f -t ~/.local/share/icons/hicolor
```

## 6. 建立 `.deb`

專案已提供打包腳本：

```bash
./scripts/package-deb.sh
```

輸出位置：

```bash
build/packages/snapvector_0.1.0-phase1_amd64.deb
```

可覆寫常用欄位：

```bash
MAINTAINER="Your Name <you@example.com>" ./scripts/package-deb.sh
VERSION=0.1.0 ./scripts/package-deb.sh
DEPENDS="libgtk-3-0t64, libwebkit2gtk-4.1-0, librsvg2-bin, xclip" ./scripts/package-deb.sh
```

`.deb` 會安裝：

- `/usr/bin/snapvector`
- `/usr/share/applications/snapvector.desktop`
- `/usr/share/icons/hicolor/512x512/apps/snapvector.png`

## 7. 安裝 `.deb`

```bash
sudo dpkg -i build/packages/snapvector_0.1.0-phase1_amd64.deb
sudo apt-get install -f
```

安裝完成後可直接用：

```bash
gtk-launch snapvector
```

或從應用程式選單搜尋 **SnapVector**。
