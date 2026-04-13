# SnapVector Wails Plan

## Objective

Establish the Wails track as the primary SnapVector implementation candidate by first building a correct CLI foundation, then layering annotation/export, GUI, and cross-platform support in phases that stay aligned with `PRD.md`.

## Current state

- `wails/` has no Go module or application code yet.
- A prior Phase 1 CLI PoC draft exists at `docs/superpowers/plans/2026-04-14-wails-phase1-cli-poc.md`.
- That draft is a useful starting point, but it should be treated as superseded because it depends on several incorrect or risky assumptions:
  - `screencapture` stdout streaming for PNG capture
  - GUI panic stub counted as dual-mode completion
  - PRD JSON codes reused as shell exit codes
  - unsupported-platform errors collapsed into retryable permission failures

## Plan overview

We will use a four-phase roadmap:

1. **Phase 1: Corrected CLI foundation**
2. **Phase 2: Annotation and export core**
3. **Phase 3: Wails GUI shell**
4. **Phase 4: Cross-platform backends and packaging**

Each phase should only claim the PRD surface it truly implements.

## Phase 1: Corrected CLI foundation

### Goal

Ship a working `snapvector --capture --base64-stdout` PoC on macOS with canonical JSON stdout responses and a clean package structure that later phases can extend.

### Scope

In scope:

- Go module bootstrap under `wails/`
- CLI entrypoint and argv dispatch
- canonical JSON response schema for stdout
- `--capture --base64-stdout`
- `--help` and `--version`
- `--inject-svg` accepted but returning structured not-yet-implemented error
- darwin capture backend
- linux/windows stub backends with accurate error semantics
- tests, smoke checks, benchmark script, and Wails-track README

Out of scope:

- annotation rendering
- SVG composition and export
- PNG/JPG/PDF export
- clipboard support
- Wails frontend/app shell
- Wayland and Windows native capture implementations

### Architecture

Suggested package layout:

```text
wails/
├── go.mod
├── main.go
├── cli/
│   ├── cli.go
│   ├── flags.go
│   ├── response.go
│   ├── help.go
│   ├── response_test.go
│   ├── flags_test.go
│   └── cli_test.go
├── capture/
│   ├── capture.go
│   ├── capture_darwin.go
│   ├── capture_linux.go
│   ├── capture_windows.go
│   └── capture_darwin_test.go
├── gui/
│   └── gui.go
├── scripts/
│   └── bench-cli.sh
└── README.md
```

### Critical implementation rules

- The PRD JSON `code` field is part of the stdout API contract, not the shell exit code contract.
- Phase 1 should use shell-safe process exit codes (`0` on success, `1` on failure).
- The darwin backend should not assume stdout capture from `screencapture`; use a temp file and load bytes back into memory.
- Unsupported platform, permission denied, and generic capture failure must be distinct internal error cases so `retryable` is truthful.
- The GUI branch may remain a stub in Phase 1, but documentation must not claim full F4.1 completion unless it becomes a usable path.

### Phase 1 task breakdown

#### Task 1 — Bootstrap module and command routing

- Initialize `wails/go.mod`
- Add `main.go`
- Route CLI invocations to `cli.Run`
- Keep a GUI placeholder package that is explicit about being non-product in Phase 1

#### Task 2 — Canonical CLI response layer

- Define `CLIResponse` and `ErrorPayload`
- Encode only JSON to stdout
- Add tests for ok/error shapes and required field behavior
- Define status-code taxonomy aligned to PRD ranges

#### Task 3 — Flag parsing and command dispatch

- Parse `--capture`, `--base64-stdout`, `--inject-svg`, `--help`, `--version`
- Reject invalid combinations as usage errors
- Return structured not-implemented response for `--inject-svg`

#### Task 4 — Darwin capture backend

- Implement full-screen capture using temp-file `screencapture`
- Read PNG bytes back into memory
- Base64-encode for `data.base64`
- Surface decoded image dimensions as optional metadata when available
- Add darwin tests that verify the returned data is decodable PNG

#### Task 5 — Error mapping and exit behavior

- Map internal failures to truthful PRD JSON codes
- Keep process exit code independent and shell-safe
- Ensure unsupported platform is not reported as retryable permission denial

#### Task 6 — Verification and docs

- Run `go build`, `go test`, and `go vet`
- Add smoke-test commands
- Add benchmark wrapper and record measured baseline
- Document what Phase 1 does and does not satisfy from the PRD

## Phase 2: Annotation and export core

### Goal

Implement the rendering/composition core that both CLI and GUI will share conceptually, while keeping code ownership inside the Wails track.

### Scope

- `--inject-svg` payload validation
- annotation model types
- SVG symbol/token alignment with `design/`
- single-file SVG output
- raster/PDF flatten export
- blur region support with consistent semantics
- clipboard output

### Notes

- This phase should land before serious GUI tool work so the CLI and GUI share the same geometry/output contract.

## Phase 3: Wails GUI shell

### Goal

Turn the Wails track into a usable desktop annotation app backed by the already-built CLI/rendering primitives.

### Scope

- Wails app bootstrap
- region-selection overlay
- annotation canvas
- text/arrow/shape/blur tools
- direct manipulation handles
- undo/redo
- export and clipboard entry points

## Phase 4: Cross-platform backends and packaging

### Goal

Complete the evaluation track for Linux/Windows support and end-user distribution.

### Scope

- Wayland/XDG portal capture
- Windows native capture
- packaging artifacts
- install/distribution workflow
- benchmark comparison across platforms and against other tracks

## Execution notes

- Treat `docs/superpowers/plans/2026-04-14-wails-phase1-cli-poc.md` as legacy reference, not source of truth.
- Use `/plan/wails.md` as the current plan of record for the Wails track.
- Update this file when a phase boundary, architecture decision, or PRD interpretation changes.
