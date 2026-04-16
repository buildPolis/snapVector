# SnapVector Wails Plan

## Objective

Keep the Wails track as the leading SnapVector implementation candidate by
tracking what is already shipped, what is partially verified, and what still
blocks a production-ready cross-platform release.

## Current state

`wails/` is no longer a bootstrap track. It already contains working code for:

- dual-mode binary entrypoint (CLI + GUI)
- canonical CLI JSON responses
- `--capture --base64-stdout`
- `--inject-svg` payload parsing and SVG composition
- SVG / PNG / JPG / PDF export paths
- clipboard output
- Wails GUI shell with canvas, tools, inspector, undo/redo, zoom, and pan
- Linux and Windows platform backends
- Linux `.deb` packaging script
- GitHub Actions CI / release workflow definitions

## Verified now

- `go test ./...`
- `go vet ./...`
- `go build -o ./build/bin/snapvector .`
- `go build -tags production -o ./build/bin/snapvector-production .`
- `./build/bin/snapvector --version`
- `./build/bin/snapvector --capture --base64-stdout`
- `./build/bin/snapvector --inject-svg '[]'`
- `./build/bin/snapvector-production`

## Roadmap overview

The work is now best understood as:

1. **Phase 1: CLI foundation** — done
2. **Phase 2: Annotation/export core** — done
3. **Phase 3: GUI shell** — in progress
4. **Phase 4: Release hardening and distribution** — in progress

## Phase 1: CLI foundation

### Delivered

- argv dispatch between CLI and GUI
- canonical top-level JSON schema
- `--help`
- `--version`
- `--capture --base64-stdout`
- truthful CLI error taxonomy with JSON `code` separate from shell exit code

### Notes

- The CLI contract is already wired as the shared foundation for later release
  and GUI work.

## Phase 2: Annotation and export core

### Delivered

- canonical `--inject-svg` payload parsing and validation
- single-file SVG output with embedded screenshot
- symbol-backed rendering for arrow, rectangle, ellipse, text, blur, and
  numbered-circle payload semantics where implemented in the current renderer
- flattened export to `png`, `jpg`, and `pdf`
- clipboard output

### Remaining caveats

- Windows export still cannot render blur annotations because the current
  `oksvg` stack does not support `<feGaussianBlur>`.

## Phase 3: GUI shell

### Delivered

- Wails app bootstrap from the same binary shape
- screenshot handoff into the frontend canvas
- tool rail, selection model, resize handles, inspector, undo/redo, zoom, pan
- GUI-side payload preview aligned with CLI geometry semantics

### Still missing

- desktop-wide region-selection overlay before capture
- full end-to-end verification of native visible-window export flows on every
  target platform

## Phase 4: Release hardening and distribution

### Delivered

- Linux `.deb` packaging script in `wails/scripts/package-deb.sh`
- root-level GitHub Actions workflows for:
  - CI on push / pull request
  - release artifact generation on `v*` tags or manual dispatch
- unsigned release artifact strategy:
  - macOS: `.app` + `.zip`
  - Windows: raw `.exe` + NSIS installer
  - Linux: raw binary + `.deb`

### Still missing

- GitHub-hosted runner proof for every release job
- macOS signing and notarization
- optional Windows code signing
- final release artifact naming/versioning conventions after the first real
  release run
- measured benchmark records in repo docs

## Decision notes

- `docs/superpowers/plans/2026-04-14-wails-phase1-cli-poc.md` remains legacy
  reference only.
- `wails/README.md` is the most accurate implementation-status document for the
  Wails track.
- `plan/todo.md` remains the execution checklist and should only mark items done
  after the relevant build, test, or workflow has actually run successfully.
