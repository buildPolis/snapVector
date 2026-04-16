# SnapVector Wails Detailed Todo

Source of truth for roadmap: `/plan/wails.md`

This file is the execution-oriented checklist across all phases. Items are marked done only when the implemented behavior has been exercised and is currently workable.

## Current status summary

- **Phase 1:** done
- **Phase 2:** done
- **Phase 3:** in progress
- **Phase 4:** pending

## Verification status

- [x] `go test ./...`
- [x] `go vet ./...`
- [x] `go build -o ./build/bin/snapvector .`
- [x] `go build -tags production -o ./build/bin/snapvector-production .`
- [x] `./build/bin/snapvector --version`
- [x] `./build/bin/snapvector --capture --base64-stdout`
- [x] `./build/bin/snapvector --inject-svg '[]'`
- [x] `./build/bin/snapvector-production`
- [ ] `./scripts/bench-cli.sh` measured with `hyperfine` installed

## Phase 1 — Corrected CLI foundation

### Plan and baseline

- [x] Replace the old Wails draft as source of truth
- [x] Create `/plan/wails.md`
- [x] Mark `docs/superpowers/plans/2026-04-14-wails-phase1-cli-poc.md` as superseded

### Bootstrap

- [x] Initialize `wails/go.mod`
- [x] Add `wails/main.go`
- [x] Add explicit CLI vs GUI dispatch
- [x] Add `wails/gui/gui.go` placeholder

### CLI contract

- [x] Add canonical JSON response types
- [x] Keep stdout JSON-only in CLI mode
- [x] Separate JSON `code` from process exit code
- [x] Add `--help`
- [x] Add `--version`
- [x] Add `--capture`
- [x] Add `--base64-stdout`
- [x] Accept `--inject-svg`

### Capture backend

- [x] Add capture interface and metadata model
- [x] Implement darwin full-screen capture with temp-file `screencapture`
- [x] Distinguish permission-denied vs unsupported-platform vs generic capture failure
- [x] Add linux stub backend
- [x] Add windows stub backend

### Tests and docs

- [x] Add CLI response tests
- [x] Add flag parsing tests
- [x] Add CLI integration tests
- [x] Add darwin capture test
- [x] Add `wails/README.md`
- [x] Add `wails/.gitignore`
- [x] Add `wails/scripts/bench-cli.sh`

## Phase 2 — Annotation and export core

### Implemented and workable now

- [x] Parse canonical `--inject-svg` JSON array payload
- [x] Validate required fields by annotation type
- [x] Reject unsupported `type` values with structured error
- [x] Default `strokeColor`, `outlineColor`, `strokeWidth`
- [x] Default text `variant` and `fontSize`
- [x] Default blur `blurRadius`, `cornerRadius`, `feather`
- [x] Add symbol-based SVG composition module
- [x] Embed captured screenshot as base64 image in a single-file SVG
- [x] Render `arrow`
- [x] Render `rectangle`
- [x] Render `ellipse`
- [x] Render `text`
- [x] Render `blur` with actual SVG blur filter and clipped duplicate image
- [x] Return canonical JSON success payload for `--inject-svg`
- [x] Add payload parser tests
- [x] Add SVG composer tests

### Still incomplete in Phase 2

- [x] Align every SVG primitive more precisely against `design/symbols.svg` baseline metrics
- [x] Support optional `captureRegion` metadata in `--inject-svg` result when relevant
- [x] Add PNG flattened export path
- [x] Add JPG flattened export path
- [x] Add PDF export path
- [x] Add clipboard output path
- [x] Add tests for PRD sample payload end-to-end assertions
- [x] Add tests for invalid color / invalid geometry edge cases
- [x] Add tests that verify multiline text wrapping and `maxWidth` semantics
- [x] Add tests that verify blur output stays semantically consistent with CLI/GUI expectations
- [x] Document exact current Phase 2 coverage in `wails/README.md`

## Phase 3 — Wails GUI shell

### App shell

- [x] Add Wails runtime dependency intentionally
- [x] Replace `gui.Run()` panic stub with real app bootstrap
- [x] Add desktop window lifecycle wiring
- [x] Decide frontend stack for the Wails shell and scaffold it

### Capture and canvas UX

- [ ] Add region-selection overlay
- [x] Add screenshot handoff into annotation canvas
- [x] Render the captured image in the GUI canvas
- [x] Add zoom and pan behavior

### Annotation tools

- [x] Add tool rail matching the design baseline
- [x] Add arrow creation flow
- [x] Add rectangle creation flow
- [x] Add ellipse creation flow
- [x] Add text creation flow with CJK input sanity
- [x] Add blur region creation flow
- [x] Add selection box and resize handles
- [x] Add direct manipulation for existing annotations

### Editing workflow

- [x] Add selection model
- [x] Add undo
- [x] Add redo
- [x] Add property inspector
- [x] Keep GUI geometry aligned with CLI payload semantics

### GUI verification

- [x] Launch GUI successfully from the same binary
- [ ] Verify region capture flow is usable
- [x] Verify annotation editing is usable
- [x] Verify GUI-exported output matches CLI geometry expectations

## Phase 4 — Cross-platform backends and packaging

### Linux and Windows

- [ ] Implement Linux Wayland/XDG portal capture
- [ ] Handle Linux permission and portal errors with truthful `retryable` semantics
- [ ] Implement Windows native capture backend
- [ ] Add backend-specific tests where feasible

### Packaging and distribution

- [ ] Add packaging workflow for macOS *(workflow file landed locally; GitHub runner verification pending)*
- [ ] Add packaging workflow for Windows *(workflow file landed locally; GitHub runner verification pending)*
- [ ] Add packaging workflow for Linux *(workflow file landed locally; GitHub runner verification pending)*
- [ ] Document install/run flow for each platform *(release workflow/docs landed locally; end-to-end release verification pending)*

### Performance and comparison

- [ ] Record actual CLI latency baseline with `hyperfine`
- [ ] Compare Wails CLI latency against target in `PRD.md`
- [ ] Compare Wails track against Qt and Tauri tracks on the agreed criteria

## Done criteria

An item should only be changed from unchecked to checked after:

1. The implementation exists in the repository.
2. The relevant command, test, or user flow has been exercised successfully.
3. The result matches the intended PRD or plan behavior without relying on a stub.
