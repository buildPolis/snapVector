# SnapVector — Wails track

Go implementation of the SnapVector CLI, annotation/export pipeline, and the current Wails GUI shell.

## Build

```bash
cd wails
go build -o ./build/bin/snapvector .
go build -tags production -o ./build/bin/snapvector-production .
```

- `snapvector` is the normal Go build that still supports all CLI workflows.
- `snapvector-production` is the real Wails GUI build. On macOS this now links `UniformTypeIdentifiers` from code, so the production tag build no longer needs manual `CGO_LDFLAGS`.

### Windows

No system dependencies are required for building. Go and Wails CLI are sufficient.

```bash
# Dev mode (run from wails/ directory)
wails dev

# Production build
wails build -platform windows/amd64

# Production build + NSIS installer (.exe setup wizard)
wails build -nsis -platform windows/amd64
```

Requires NSIS for the `-nsis` flag:

```bash
winget install NSIS.NSIS
```

**WebView2 strategy** — configure in `wails.json`:

| Strategy | Description | Installer size impact |
|---|---|---|
| `DownloadBootstrapper` | Downloads WebView2 at install time | minimal |
| `EmbedBootstrapper` | Bundles the downloader (recommended for Win10) | +~1.5 MB |
| `OfflineInstaller` | Full offline WebView2 bundle | +~150 MB |
| `Skip` | Assumes WebView2 already installed | none (risky on Win10) |

Windows 11 ships WebView2 pre-installed. For Windows 10 use `EmbedBootstrapper`.

### Linux (Ubuntu 24.04+)

Ubuntu 24.04 ships `webkit2gtk-4.1` instead of `4.0`. Wails requires the `webkit2_41` build tag.
This is already set in `wails.json` via `"build:tags"`, so `wails dev` / `wails build` pick it up automatically.

**System dependencies:**

```bash
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev \
  librsvg2-bin xclip
```

| Package | Purpose |
|---|---|
| `libgtk-3-dev` | GTK3 headers for Wails GUI (build only) |
| `libwebkit2gtk-4.1-dev` | WebKit headers for Wails GUI (build only) |
| `librsvg2-bin` | `rsvg-convert` for SVG→PNG/JPG/PDF export |
| `xclip` | Clipboard write (X11); use `wl-clipboard` on Wayland |

> **Note:** `-dev` packages are only needed on the **build machine**. End users running the pre-built binary only need the runtime libraries (`libgtk-3-0t64`, `libwebkit2gtk-4.1-0` — pre-installed on Ubuntu Desktop) plus `librsvg2-bin` and `xclip`:
>
> ```bash
> sudo apt-get install -y librsvg2-bin xclip
> ```

**Dev mode:**

```bash
source ~/.nvm/nvm.sh && nvm use 24  # if using nvm
wails dev                            # webkit2_41 tag is auto-applied
```

On first GUI launch, SnapVector also installs a user-level desktop entry and icon
under `~/.local/share/applications/` and `~/.local/share/icons/` so GNOME/KDE
can associate the running window with the correct app icon more reliably.

**Production build:**

```bash
wails build -tags "webkit2_41 production"
```

## CLI usage

```bash
./build/bin/snapvector --capture --base64-stdout
./build/bin/snapvector --version
./build/bin/snapvector --help
./build/bin/snapvector --inject-svg '[]'
./build/bin/snapvector --inject-svg '[]' --output-format png
./build/bin/snapvector --inject-svg '[]' --output-format jpg
./build/bin/snapvector --inject-svg '[]' --output-format pdf
./build/bin/snapvector --inject-svg '[]' --output-format png --copy-to-clipboard
```

CLI stdout always emits a single JSON document that follows the PRD top-level contract.

`--inject-svg` captures the current screen, validates the canonical annotation payload, and returns a single-file SVG document with the screenshot embedded as base64 plus symbol-backed annotations.

## Implementation notes

- The macOS capture backend shells out to `/usr/sbin/screencapture`.
- The Linux capture backend uses D-Bus `org.freedesktop.portal.Screenshot` with `gnome-screenshot`/`grim` fallback.
- The Windows capture backend uses `kbinani/screenshot` (Win32 GDI, CGo-free). Interactive region selection runs an embedded PowerShell script that renders the screen as a full-screen overlay with a rubber-band selector.
- Windows clipboard writes PNG/JPG via `System.Drawing.Image` and SVG/PDF via `Clipboard.SetData` with MIME-type format names; both paths run PowerShell with `-STA` (required for COM clipboard).
- Windows SVG export uses `oksvg` + `rasterx` (pure Go). PDF export wraps the rasterised PNG in a `gopdf` page. `<feGaussianBlur>` is not supported — blur annotations are silently skipped.
- JSON `code` values are the machine-readable API contract; shell exit codes remain `0` or `1`.
- The Wails GUI production build on macOS links `UniformTypeIdentifiers` from code, so `go build -tags production` works without extra linker flags.
- Linux global hotkeys use D-Bus `org.freedesktop.portal.GlobalShortcuts` (requires `xdg-desktop-portal` ≥ 1.17). Falls back gracefully if the portal is unavailable.
- Windows global hotkeys are a no-op (same as macOS); only in-app keyboard shortcuts work.

## Current coverage

Implemented now:

- canonical `--inject-svg` payload parsing and validation
- symbol-backed SVG output for arrow, rectangle, ellipse, text, and blur
- single-file SVG with embedded base screenshot
- macOS and Linux export conversion for `svg`, `png`, `jpg`, and `pdf`
- macOS and Linux clipboard output for `svg`, `png`, `jpg`, and `pdf`
- `captureRegion` metadata in CLI responses when geometry is known
- Wails GUI bootstrap through the same binary entrypoint
- static HTML/CSS/JS GUI shell with capture canvas, tool rail, selection, resize handles, inspector, undo/redo, zoom, and pan
- GUI-side payload preview that mirrors CLI `--inject-svg` semantics

Still not shipped:

- desktop-wide region-selection overlay before capture
- fully verified native Wails end-to-end export interaction from the visible app window
- Windows export: blur annotations (oksvg does not support `<feGaussianBlur>`)

## GUI usage

```bash
./build/bin/snapvector-production
```

Current verification:

- production GUI build launches successfully from the same binary shape
- capture is handed into the canvas on startup
- rectangle, blur, and text flows were exercised in-browser against the real frontend logic
- CJK text survives GUI state, inspector input, and live payload preview
- undo/redo, move/resize, zoom, payload preview, export toast, and clipboard toast were exercised
- `gui.App.ExportDocument` now has tests that prove the GUI export path matches the shared SVG composer output

## Benchmark

```bash
./scripts/bench-cli.sh
```

Record measured latency instead of assuming PRD compliance.

## Platform status

| platform | status | notes |
|---|---|---|
| darwin | ✅ Phase 1 backend | Requires Screen Recording permission. |
| linux | ✅ Phase 1 backend | D-Bus portal capture, `rsvg-convert` export, `xclip`/`wl-copy` clipboard, global hotkeys via portal. |
| windows | ✅ Phase 1 backend | GDI capture, PowerShell region overlay, `oksvg` export, PowerShell clipboard. Blur annotations not rendered in export (oksvg limitation). |

## Distribution & packaging

### Runtime dependencies by platform

| Platform | Feature | Dependency | Pre-installed? |
|---|---|---|---|
| Windows | GUI | WebView2 Runtime | Win11 ✅ / Win10 ⚠️ |
| Windows | Region capture | PowerShell + .NET Forms | ✅ all modern Windows |
| macOS | Screenshot / clipboard | `swift` (Xcode CLT) | ⚠️ needs `xcode-select --install` |
| macOS | Export | `sips`, `cupsfilter` | ✅ system built-in |
| macOS | GUI | WebKit | ✅ system built-in |
| Linux | GUI | WebKit2GTK (`libwebkit2gtk-4.1`) | ⚠️ needs apt install |
| Linux | Screenshot | `gnome-screenshot` or `grim` | ⚠️ distro-dependent |
| Linux | Export | `rsvg-convert` (`librsvg2-bin`) | ⚠️ needs apt install |
| Linux | Clipboard | `xclip` or `wl-clipboard` | ⚠️ needs apt install |

### Cross-compilation

| Target | From Windows | From macOS | From Linux |
|---|---|---|---|
| `windows/amd64` | ✅ | ✅ (`GOOS=windows`) | ✅ (`GOOS=windows`) |
| `darwin/amd64` | ❌ CGo required | ✅ | ❌ CGo required |
| `linux/amd64` | ❌ CGo required | ❌ CGo required | ✅ |

macOS and Linux builds require CGo (macOS: UniformTypeIdentifiers framework; Linux: X11/webkit). Each platform must be built on its own OS.

### macOS code signing & notarization

Unsigned macOS builds are blocked by Gatekeeper. Users see _"cannot be opened because the developer cannot be verified"_.

**Requirements:** Apple Developer Program membership ($99 USD/year).

```bash
# 1. Sign the .app
codesign --deep --force --options runtime \
  --sign "Developer ID Application: Your Name (TEAMID)" \
  snapvector.app

# 2. Submit to Apple notary service
xcrun notarytool submit snapvector.zip \
  --apple-id "your@email.com" \
  --team-id "TEAMID" \
  --password "app-specific-password" \
  --wait

# 3. Staple the notarization ticket into the .app
xcrun stapler staple snapvector.app
```

Apple typically responds within 5 minutes. Without notarization, users can still bypass Gatekeeper manually via **System Settings → Privacy & Security → Open Anyway**, or:

```bash
xattr -dr com.apple.quarantine snapvector.app
```

### Windows NSIS installer

`wails build -nsis` generates a standard Windows setup wizard (`.exe`) that:
- installs the binary to `C:\Program Files\snapvector\`
- creates Start Menu and optional desktop shortcut
- registers an uninstaller in Add/Remove Programs
- handles WebView2 bootstrap automatically

### Linux packaging

No native installer support. Recommended options:
- Ship a raw binary with a README listing `apt install librsvg2-bin xclip`
- Package as `.deb` / `.rpm` with `Depends:` listing runtime libraries
- Distribute via Flatpak (bundles all runtime deps)

#### Build a `.deb`

```bash
source ~/.nvm/nvm.sh && nvm use 24
./scripts/package-deb.sh
```

The package is written to `build/packages/` and installs:
- `/usr/bin/snapvector`
- `/usr/share/applications/snapvector.desktop`
- `/usr/share/icons/hicolor/512x512/apps/snapvector.png`

Useful overrides:

```bash
VERSION=0.1.0 MAINTAINER="Your Name <you@example.com>" ./scripts/package-deb.sh
DEPENDS="libgtk-3-0t64, libwebkit2gtk-4.1-0, librsvg2-bin, xclip" ./scripts/package-deb.sh
```

#### Upgrade flow (from a previous `.deb` or raw-binary install)

```bash
source ~/.nvm/nvm.sh && nvm use 24
./scripts/package-deb.sh
sudo dpkg -i build/packages/snapvector_0.1.0-phase1_amd64.deb
rm -f ~/.local/share/applications/snapvector.desktop ~/.local/bin/snapvector
update-desktop-database ~/.local/share/applications
```

The `rm` step clears the user-level `.desktop` and stale `~/.local/bin`
binary that would otherwise **shadow** the system-level entry installed by
the `.deb` (they share the same app-id, so GNOME Shell picks the user-level
one and hides the new version). On first launch the binary regenerates a
spec-compliant `~/.local/share/applications/snapvector.desktop` pointing at
the new `/usr/bin/snapvector`. WebView asset cache is invalidated
automatically via a size+mtime fingerprint — no need to `rm -rf
~/.cache/snapvector`. See [`linux_install.md`](./linux_install.md) §8 for
background.

## Hotkeys

Defaults cover tools, editing, capture, zoom, and export actions.
Customize via **File → Preferences…** (or press `Cmd+,` / `Ctrl+,`).

Settings live at:

- macOS: `~/Library/Application Support/SnapVector/hotkeys.json`
- Linux: `~/.config/SnapVector/hotkeys.json`
- Windows: `%APPDATA%\SnapVector\hotkeys.json`

Delete the file (or click **Reset all defaults** in Preferences) to restore defaults.

### Frontend unit tests

```bash
node --test wails/gui/frontend/dist/hotkey-utils.test.js
```
