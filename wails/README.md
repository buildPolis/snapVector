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
| `libgtk-3-dev` | GTK3 headers for Wails GUI |
| `libwebkit2gtk-4.1-dev` | WebKit headers for Wails GUI |
| `librsvg2-bin` | `rsvg-convert` for SVG→PNG/JPG/PDF export |
| `xclip` | Clipboard write (X11); use `wl-clipboard` on Wayland |

**Dev mode:**

```bash
source ~/.nvm/nvm.sh && nvm use 24  # if using nvm
wails dev                            # webkit2_41 tag is auto-applied
```

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
- JSON `code` values are the machine-readable API contract; shell exit codes remain `0` or `1`.
- The Wails GUI production build on macOS links `UniformTypeIdentifiers` from code, so `go build -tags production` works without extra linker flags.
- Linux global hotkeys use D-Bus `org.freedesktop.portal.GlobalShortcuts` (requires `xdg-desktop-portal` ≥ 1.17). Falls back gracefully if the portal is unavailable.

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
- Windows native capture backend

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
| windows | 🚧 stub | Returns structured unsupported-platform error. |

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
