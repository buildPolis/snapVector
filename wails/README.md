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

- The macOS capture backend currently shells out to `/usr/sbin/screencapture`.
- JSON `code` values are the machine-readable API contract; shell exit codes remain `0` or `1`.
- Linux and Windows currently return structured unsupported-platform errors for capture.
- The Wails GUI production build on macOS links `UniformTypeIdentifiers` from code, so `go build -tags production` works without extra linker flags.

## Current coverage

Implemented now:

- canonical `--inject-svg` payload parsing and validation
- symbol-backed SVG output for arrow, rectangle, ellipse, text, and blur
- single-file SVG with embedded base screenshot
- macOS export conversion for `svg`, `png`, `jpg`, and `pdf`
- macOS clipboard output for `svg`, `png`, `jpg`, and `pdf`
- `captureRegion` metadata in CLI responses when geometry is known
- Wails GUI bootstrap through the same binary entrypoint
- static HTML/CSS/JS GUI shell with capture canvas, tool rail, selection, resize handles, inspector, undo/redo, zoom, and pan
- GUI-side payload preview that mirrors CLI `--inject-svg` semantics

Still not shipped:

- desktop-wide region-selection overlay before capture
- fully verified native Wails end-to-end export interaction from the visible app window
- Linux/Windows native capture backends

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
| linux | 🚧 stub | Returns structured unsupported-platform error. |
| windows | 🚧 stub | Returns structured unsupported-platform error. |
