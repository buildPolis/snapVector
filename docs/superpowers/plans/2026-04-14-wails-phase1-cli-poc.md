# SnapVector Wails — Phase 1 (CLI PoC) Implementation Plan

> **Superseded:** Use `/plan/wails.md` as the current source of truth for the Wails track. This draft is kept as historical reference only.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a working `snapvector --capture --base64-stdout` binary on macOS with the canonical CLI JSON contract, build-tagged platform capture shims, and a project skeleton that every later phase extends.

**Architecture:** Single Go module at `wails/`. `main.go` inspects `os.Args`, dispatches to `cli.Run` for any `--…` flag or falls through to `gui.Run` (stubbed in this phase). All stdout is a single `CLIResponse` JSON document emitted by `json.NewEncoder`. Capture is abstracted as `type Capturer interface { CaptureFullScreen(ctx) (PNGBytes, CaptureMeta, error) }`; Phase 1 provides a real darwin implementation via `/usr/sbin/screencapture` (zero cgo) and stubs for linux/windows that return `1201 permission/unsupported`. No Wails dependency is imported yet — the `wails.Run` branch is a panic stub so cold-start benchmarks reflect the real CLI path.

**Tech Stack:** Go 1.22+, standard library only for Phase 1 (`encoding/json`, `encoding/base64`, `os/exec`, `flag`, `testing`). Benchmark via `hyperfine`. No cgo.

**Scope note — follow-up plans (NOT in this phase):**
- Phase 2: Annotation engine + SVG composer + `--inject-svg` + PNG/JPG/PDF/clipboard export.
- Phase 3: Wails GUI shell, region-selection overlay, annotation canvas, undo/redo, tray + hotkey.
- Phase 4: Wayland D-Bus capture, Windows GDI capture, dmg/exe/AppImage packaging.

---

## File Structure

```
wails/
├── go.mod                          # module snapvector, go 1.22
├── main.go                         # argv dispatch (cli vs gui)
├── cli/
│   ├── cli.go                      # Run(args, stdout, stderr) int
│   ├── flags.go                    # flag.FlagSet wiring
│   ├── response.go                 # CLIResponse, ErrorPayload, codes
│   ├── response_test.go
│   ├── flags_test.go
│   └── cli_test.go                 # integration: Run() emits valid JSON
├── capture/
│   ├── capture.go                  # Capturer interface, PNGBytes, Meta
│   ├── capture_darwin.go           # build: darwin — /usr/sbin/screencapture
│   ├── capture_darwin_test.go      # build: darwin
│   ├── capture_linux.go            # build: linux — stub returns 1201
│   └── capture_windows.go          # build: windows — stub returns 1201
├── gui/
│   └── gui.go                      # Run() panics "not built in phase 1"
├── scripts/
│   └── bench-cli.sh                # hyperfine wrapper
└── README.md                       # track README (commands, benchmarks)
```

**Responsibilities:**
- `main.go`: 15 lines. Parse `os.Args[1:]` — if first non-binary arg starts with `--`, call `cli.Run`; else `gui.Run`. Exit with returned int.
- `cli/response.go`: Single source of truth for the JSON wire format. No other file formats stdout.
- `cli/flags.go`: Declares `--capture`, `--base64-stdout`, `--inject-svg`, `--help`, `--version`. Rejects unknown flags with code 1000.
- `cli/cli.go`: Command dispatch. Phase 1 implements `--capture`; `--inject-svg` returns 1301 not-implemented (Phase 2 lights it up).
- `capture/capture.go`: `Capturer` interface + `NewPlatformCapturer()` factory selected by build tag.
- `capture/capture_darwin.go`: Shells out to `/usr/sbin/screencapture -x -t png -`, reads stdout → PNG bytes. No cgo, no ScreenCaptureKit in Phase 1 (revisit in Phase 4).
- `gui/gui.go`: Panic stub. Keeps import graph clean so CLI builds without `github.com/wailsapp/wails/v2`.

---

## Task 1: Bootstrap module + argv dispatch

**Files:**
- Create: `wails/go.mod`
- Create: `wails/main.go`
- Create: `wails/gui/gui.go`
- Create: `wails/cli/cli.go`

- [ ] **Step 1.1: Initialize module**

Run from `wails/`:

```bash
cd wails && go mod init snapvector
```

Expected: creates `go.mod` with `module snapvector\ngo 1.22` (or whatever `go env GOVERSION` reports; bump to 1.22 if lower).

- [ ] **Step 1.2: Write `wails/main.go`**

```go
package main

import (
	"os"

	"snapvector/cli"
	"snapvector/gui"
)

func main() {
	if isCLIInvocation(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
	}
	gui.Run()
}

func isCLIInvocation(args []string) bool {
	for _, a := range args {
		if len(a) >= 2 && a[0] == '-' && a[1] == '-' {
			return true
		}
	}
	return false
}
```

- [ ] **Step 1.3: Write `wails/gui/gui.go`**

```go
package gui

// Run is a stub until Phase 3 wires Wails. Keeping the Wails import out of
// Phase 1 guarantees the CLI cold-start benchmark reflects CLI code only.
func Run() {
	panic("gui.Run is not implemented in Phase 1 — build with --tags gui in Phase 3")
}
```

- [ ] **Step 1.4: Write minimal `wails/cli/cli.go` so `main.go` compiles**

```go
package cli

import "io"

// Run is the entry point for headless mode. Returns the process exit code.
// Populated in Task 4.
func Run(args []string, stdout, stderr io.Writer) int {
	_ = args
	_ = stdout
	_ = stderr
	return 0
}
```

- [ ] **Step 1.5: Verify build**

Run: `cd wails && go build ./...`
Expected: exit 0, no output.

- [ ] **Step 1.6: Commit**

```bash
git add wails/go.mod wails/main.go wails/gui/gui.go wails/cli/cli.go
git commit -m "feat(wails): bootstrap module with argv dispatch stub"
```

---

## Task 2: `CLIResponse` wire format + codes

**Files:**
- Create: `wails/cli/response.go`
- Create: `wails/cli/response_test.go`

- [ ] **Step 2.1: Write failing test `wails/cli/response_test.go`**

```go
package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteOK_EmitsCanonicalShape(t *testing.T) {
	var buf bytes.Buffer
	WriteOK(&buf, map[string]any{
		"format":   "png",
		"mimeType": "image/png",
		"base64":   "AAAA",
	})

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v (raw=%q)", err, buf.String())
	}
	if got["status"] != "ok" {
		t.Fatalf("status = %v, want ok", got["status"])
	}
	if got["code"].(float64) != 0 {
		t.Fatalf("code = %v, want 0", got["code"])
	}
	if _, ok := got["error"]; ok {
		t.Fatal("ok response must not carry error field")
	}
	data := got["data"].(map[string]any)
	if data["format"] != "png" || data["mimeType"] != "image/png" || data["base64"] != "AAAA" {
		t.Fatalf("data payload mismatched: %v", data)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatal("json.Encoder output should end with newline")
	}
}

func TestWriteError_EmitsCanonicalShape(t *testing.T) {
	var buf bytes.Buffer
	WriteError(&buf, CodePermissionDenied, "Screen capture permission denied", true, nil)

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["status"] != "error" {
		t.Fatalf("status = %v, want error", got["status"])
	}
	if got["code"].(float64) != float64(CodePermissionDenied) {
		t.Fatalf("code = %v, want %d", got["code"], CodePermissionDenied)
	}
	if _, ok := got["data"]; ok {
		t.Fatal("error response must not carry data field")
	}
	errObj := got["error"].(map[string]any)
	if errObj["message"] != "Screen capture permission denied" {
		t.Fatalf("message = %v", errObj["message"])
	}
	if errObj["retryable"] != true {
		t.Fatalf("retryable = %v, want true", errObj["retryable"])
	}
}

func TestCodeRanges(t *testing.T) {
	cases := []struct {
		code int
		lo   int
		hi   int
		name string
	}{
		{CodeUsage, 1000, 1099, "usage"},
		{CodeCaptureFailed, 1100, 1199, "capture"},
		{CodePermissionDenied, 1200, 1299, "permission"},
		{CodeInjectInvalid, 1300, 1399, "inject"},
		{CodeExportFailed, 1400, 1499, "export"},
	}
	for _, c := range cases {
		if c.code < c.lo || c.code > c.hi {
			t.Errorf("%s code %d not in [%d,%d]", c.name, c.code, c.lo, c.hi)
		}
	}
}
```

- [ ] **Step 2.2: Run test to verify it fails**

Run: `cd wails && go test ./cli/...`
Expected: FAIL — `WriteOK`, `WriteError`, code constants undefined.

- [ ] **Step 2.3: Implement `wails/cli/response.go`**

```go
package cli

import (
	"encoding/json"
	"io"
)

// Canonical status codes. Ranges defined in PRD §2.4.
const (
	CodeOK               = 0
	CodeUsage            = 1000
	CodeCaptureFailed    = 1100
	CodePermissionDenied = 1201
	CodeInjectInvalid    = 1301
	CodeExportFailed     = 1401
)

type CLIResponse struct {
	Status string         `json:"status"`
	Code   int            `json:"code"`
	Data   map[string]any `json:"data,omitempty"`
	Error  *ErrorPayload  `json:"error,omitempty"`
}

type ErrorPayload struct {
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func WriteOK(w io.Writer, data map[string]any) error {
	return json.NewEncoder(w).Encode(CLIResponse{
		Status: "ok",
		Code:   CodeOK,
		Data:   data,
	})
}

func WriteError(w io.Writer, code int, message string, retryable bool, details map[string]any) error {
	return json.NewEncoder(w).Encode(CLIResponse{
		Status: "error",
		Code:   code,
		Error: &ErrorPayload{
			Message:   message,
			Retryable: retryable,
			Details:   details,
		},
	})
}
```

- [ ] **Step 2.4: Run test to verify pass**

Run: `cd wails && go test ./cli/...`
Expected: PASS.

- [ ] **Step 2.5: Commit**

```bash
git add wails/cli/response.go wails/cli/response_test.go
git commit -m "feat(wails/cli): canonical CLIResponse + status code taxonomy"
```

---

## Task 3: Capture interface + darwin implementation

**Files:**
- Create: `wails/capture/capture.go`
- Create: `wails/capture/capture_darwin.go`
- Create: `wails/capture/capture_linux.go`
- Create: `wails/capture/capture_windows.go`
- Create: `wails/capture/capture_darwin_test.go`

- [ ] **Step 3.1: Write `wails/capture/capture.go`**

```go
package capture

import "context"

// PNG holds raw PNG bytes — never a data URL, never base64.
type PNG []byte

// Meta carries optional display / region metadata surfaced via the CLI data.display field.
type Meta struct {
	DisplayID   string
	X, Y        int
	Width       int
	Height      int
	ScaleFactor float64
}

// Capturer abstracts the platform-native full-screen grab.
// Region capture is a Phase 3 concern and deliberately excluded here.
type Capturer interface {
	CaptureFullScreen(ctx context.Context) (PNG, Meta, error)
}

// Unsupported is returned by stub platforms so the CLI layer can map it to code 1201.
type Unsupported struct{ Platform string }

func (e *Unsupported) Error() string { return "screen capture not supported on " + e.Platform }
```

- [ ] **Step 3.2: Write `wails/capture/capture_linux.go`**

```go
//go:build linux

package capture

import "context"

func NewPlatformCapturer() Capturer { return stubCapturer{platform: "linux"} }

type stubCapturer struct{ platform string }

func (s stubCapturer) CaptureFullScreen(ctx context.Context) (PNG, Meta, error) {
	_ = ctx
	return nil, Meta{}, &Unsupported{Platform: s.platform}
}
```

- [ ] **Step 3.3: Write `wails/capture/capture_windows.go`**

```go
//go:build windows

package capture

import "context"

func NewPlatformCapturer() Capturer { return stubCapturer{platform: "windows"} }

type stubCapturer struct{ platform string }

func (s stubCapturer) CaptureFullScreen(ctx context.Context) (PNG, Meta, error) {
	_ = ctx
	return nil, Meta{}, &Unsupported{Platform: s.platform}
}
```

- [ ] **Step 3.4: Write failing darwin test `wails/capture/capture_darwin_test.go`**

```go
//go:build darwin

package capture

import (
	"bytes"
	"context"
	"image/png"
	"testing"
	"time"
)

func TestDarwinCapturer_ReturnsDecodablePNG(t *testing.T) {
	if testing.Short() {
		t.Skip("capture test requires screen recording permission")
	}
	cap := NewPlatformCapturer()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, meta, err := cap.CaptureFullScreen(ctx)
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if len(data) < 100 {
		t.Fatalf("suspiciously small PNG: %d bytes", len(data))
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not valid PNG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		t.Fatalf("PNG has empty bounds %v", b)
	}
	if meta.Width != b.Dx() || meta.Height != b.Dy() {
		t.Logf("meta=%+v decoded=%dx%d (informational)", meta, b.Dx(), b.Dy())
	}
}
```

- [ ] **Step 3.5: Run test to confirm failure**

Run: `cd wails && go test ./capture/...`
Expected: FAIL — `NewPlatformCapturer` undefined on darwin.

- [ ] **Step 3.6: Implement `wails/capture/capture_darwin.go`**

```go
//go:build darwin

package capture

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os/exec"
)

// darwinCapturer shells out to /usr/sbin/screencapture, Apple's supported, zero-cgo path.
// -x silences the shutter sound, -t png forces PNG, "-" streams to stdout.
func NewPlatformCapturer() Capturer { return &darwinCapturer{} }

type darwinCapturer struct{}

func (d *darwinCapturer) CaptureFullScreen(ctx context.Context) (PNG, Meta, error) {
	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-t", "png", "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, Meta{}, fmt.Errorf("screencapture: %w (stderr=%q)", err, stderr.String())
	}
	raw := stdout.Bytes()
	if len(raw) == 0 {
		// screencapture returns exit 0 even when permission is denied; empty
		// stdout is the canonical signal.
		return nil, Meta{}, &Unsupported{Platform: "darwin-permission"}
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, Meta{}, fmt.Errorf("decoded check: %w", err)
	}
	b := img.Bounds()
	return PNG(raw), Meta{Width: b.Dx(), Height: b.Dy()}, nil
}
```

- [ ] **Step 3.7: Run test to verify pass**

Run: `cd wails && go test ./capture/...`
Expected: PASS (requires Screen Recording permission for the running terminal; if missing, macOS will prompt — re-run after granting).

- [ ] **Step 3.8: Commit**

```bash
git add wails/capture/
git commit -m "feat(wails/capture): Capturer interface with darwin screencapture backend"
```

---

## Task 4: Flag parsing + `--capture --base64-stdout` wiring

**Files:**
- Create: `wails/cli/flags.go`
- Create: `wails/cli/flags_test.go`
- Modify: `wails/cli/cli.go`
- Create: `wails/cli/cli_test.go`

- [ ] **Step 4.1: Write failing flags test `wails/cli/flags_test.go`**

```go
package cli

import "testing"

func TestParseFlags_CaptureBase64(t *testing.T) {
	got, err := parseFlags([]string{"--capture", "--base64-stdout"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Capture || !got.Base64Stdout {
		t.Fatalf("flags = %+v", got)
	}
}

func TestParseFlags_InjectSVG(t *testing.T) {
	got, err := parseFlags([]string{"--inject-svg", "[]"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.InjectSVG != "[]" {
		t.Fatalf("inject payload = %q", got.InjectSVG)
	}
}

func TestParseFlags_Unknown(t *testing.T) {
	if _, err := parseFlags([]string{"--nope"}); err == nil {
		t.Fatal("expected error on unknown flag")
	}
}

func TestParseFlags_CaptureWithoutStdout(t *testing.T) {
	// Phase 1 only supports base64-stdout output. Bare --capture must error
	// with a usage code rather than silently writing nothing.
	if _, err := parseFlags([]string{"--capture"}); err == nil {
		t.Fatal("expected error when --base64-stdout missing")
	}
}
```

- [ ] **Step 4.2: Run — expect FAIL**

Run: `cd wails && go test ./cli/ -run TestParseFlags`
Expected: FAIL — `parseFlags` undefined.

- [ ] **Step 4.3: Implement `wails/cli/flags.go`**

```go
package cli

import (
	"errors"
	"flag"
	"io"
)

type flags struct {
	Capture      bool
	Base64Stdout bool
	InjectSVG    string
	Help         bool
	Version      bool
}

func parseFlags(args []string) (flags, error) {
	var f flags
	fs := flag.NewFlagSet("snapvector", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&f.Capture, "capture", false, "")
	fs.BoolVar(&f.Base64Stdout, "base64-stdout", false, "")
	fs.StringVar(&f.InjectSVG, "inject-svg", "", "")
	fs.BoolVar(&f.Help, "help", false, "")
	fs.BoolVar(&f.Version, "version", false, "")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if f.Capture && !f.Base64Stdout {
		return f, errors.New("--capture requires --base64-stdout in this build")
	}
	return f, nil
}
```

- [ ] **Step 4.4: Run flags test to verify pass**

Run: `cd wails && go test ./cli/ -run TestParseFlags`
Expected: PASS.

- [ ] **Step 4.5: Write failing integration test `wails/cli/cli_test.go`**

```go
package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"testing"
)

func TestRun_CaptureBase64Stdout_EmitsValidPNG(t *testing.T) {
	if testing.Short() {
		t.Skip("requires screen recording permission")
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--capture", "--base64-stdout"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
		Data   struct {
			Format   string `json:"format"`
			MimeType string `json:"mimeType"`
			Base64   string `json:"base64"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "ok" || resp.Code != 0 {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Data.Format != "png" || resp.Data.MimeType != "image/png" {
		t.Fatalf("wrong format: %+v", resp.Data)
	}
	raw, err := base64.StdEncoding.DecodeString(resp.Data.Base64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("decoded bytes are not PNG: %v", err)
	}
}

func TestRun_UnknownFlag_EmitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--nope"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "error" || resp.Code != 1000 {
		t.Fatalf("got %+v, want code=1000", resp)
	}
}

func TestRun_InjectSVG_NotYetImplemented(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--inject-svg", "[]"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit for unimplemented command")
	}
	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "error" || resp.Code != 1301 {
		t.Fatalf("got %+v, want code=1301", resp)
	}
}
```

- [ ] **Step 4.6: Run — expect FAIL**

Run: `cd wails && go test ./cli/ -run TestRun`
Expected: FAIL — `Run` still the stub.

- [ ] **Step 4.7: Replace `wails/cli/cli.go` with real implementation**

```go
package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"time"

	"snapvector/capture"
)

const versionString = "0.1.0-phase1"

func Run(args []string, stdout, stderr io.Writer) int {
	_ = stderr // Phase 1 never writes to stderr; everything goes via CLIResponse.

	f, err := parseFlags(args)
	if err != nil {
		_ = WriteError(stdout, CodeUsage, err.Error(), false, nil)
		return CodeUsage
	}

	switch {
	case f.Help:
		_ = WriteOK(stdout, map[string]any{"help": helpText()})
		return 0
	case f.Version:
		_ = WriteOK(stdout, map[string]any{"version": versionString})
		return 0
	case f.Capture && f.Base64Stdout:
		return runCapture(stdout)
	case f.InjectSVG != "":
		_ = WriteError(stdout, CodeInjectInvalid, "--inject-svg not implemented until Phase 2", false, nil)
		return CodeInjectInvalid
	default:
		_ = WriteError(stdout, CodeUsage, "no command given; try --help", false, nil)
		return CodeUsage
	}
}

func runCapture(stdout io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cap := capture.NewPlatformCapturer()
	raw, meta, err := cap.CaptureFullScreen(ctx)
	if err != nil {
		var unsup *capture.Unsupported
		if errors.As(err, &unsup) {
			_ = WriteError(stdout, CodePermissionDenied, err.Error(), true,
				map[string]any{"platform": unsup.Platform})
			return CodePermissionDenied
		}
		_ = WriteError(stdout, CodeCaptureFailed, err.Error(), false, nil)
		return CodeCaptureFailed
	}
	data := map[string]any{
		"format":   "png",
		"mimeType": "image/png",
		"base64":   base64.StdEncoding.EncodeToString(raw),
	}
	if meta.Width > 0 && meta.Height > 0 {
		data["display"] = map[string]any{
			"width":  meta.Width,
			"height": meta.Height,
		}
	}
	_ = WriteOK(stdout, data)
	return 0
}

func helpText() string {
	return "snapvector --capture --base64-stdout   emit current screen as JSON/PNG base64\n" +
		"snapvector --inject-svg <json>          (Phase 2)\n" +
		"snapvector --version\n"
}
```

- [ ] **Step 4.8: Run full test suite**

Run: `cd wails && go test ./...`
Expected: all PASS.

- [ ] **Step 4.9: Smoke test the real binary**

Run: `cd wails && go build -o ./build/bin/snapvector . && ./build/bin/snapvector --capture --base64-stdout | head -c 200`
Expected: JSON starting with `{"status":"ok","code":0,"data":{"format":"png","mimeType":"image/png","base64":"iVBORw0…`.

- [ ] **Step 4.10: Commit**

```bash
git add wails/cli/flags.go wails/cli/flags_test.go wails/cli/cli.go wails/cli/cli_test.go
git commit -m "feat(wails/cli): --capture --base64-stdout with canonical JSON output"
```

---

## Task 5: Benchmark baseline + track README

**Files:**
- Create: `wails/scripts/bench-cli.sh`
- Create: `wails/README.md`
- Modify: `wails/.gitignore` (create if missing)

- [ ] **Step 5.1: Write `wails/scripts/bench-cli.sh`**

```bash
#!/usr/bin/env bash
# Cold-start benchmark for the CLI capture path. Phase 1 budget: p50 < 100ms
# on an M-series Mac as per PRD §3. Runs are warmed then measured.
set -euo pipefail

BIN="${BIN:-./build/bin/snapvector}"
if [[ ! -x "$BIN" ]]; then
  echo "building $BIN" >&2
  go build -o "$BIN" .
fi

if ! command -v hyperfine >/dev/null; then
  echo "hyperfine not installed — brew install hyperfine" >&2
  exit 2
fi

hyperfine --warmup 3 --runs 20 \
  "$BIN --capture --base64-stdout > /dev/null"
```

- [ ] **Step 5.2: Make it executable and dry-run**

```bash
chmod +x wails/scripts/bench-cli.sh
cd wails && ./scripts/bench-cli.sh
```

Expected: hyperfine reports mean / min / max; record p50 in the commit message. Typical M1 baseline: 150–250ms (dominated by `screencapture` itself). If above 100ms, record the number — Phase 4 will revisit with ScreenCaptureKit.

- [ ] **Step 5.3: Write `wails/README.md`**

```markdown
# SnapVector — Wails track

Go + Wails implementation of the SnapVector CLI and (later) GUI.
Spec: `../PRD.md`. Work rules: `./CLAUDE.md`.

## Build

```
cd wails
go build -o ./build/bin/snapvector .
```

## CLI usage (Phase 1)

```
./build/bin/snapvector --capture --base64-stdout   # emits canonical JSON
./build/bin/snapvector --version
./build/bin/snapvector --help
```

Output obeys the PRD §2.4 JSON contract; `stdout` carries a single
`CLIResponse` document terminated by a newline.

## Benchmark

```
./scripts/bench-cli.sh
```

PRD p50 budget: <100ms. Record the measurement in every PR touching the
capture or CLI paths.

## Platform status

| platform | capture | notes |
|---|---|---|
| darwin  | ✅ `/usr/sbin/screencapture` | Requires Screen Recording permission. |
| linux   | 🚧 stub → 1201 | Wayland portal lands in Phase 4. |
| windows | 🚧 stub → 1201 | GDI backend lands in Phase 4. |

## Phases

1. **Phase 1 (this plan):** CLI skeleton + darwin capture + JSON contract.
2. Phase 2: annotation engine, `--inject-svg`, PNG/JPG/PDF/SVG export, clipboard.
3. Phase 3: Wails GUI shell, region-select overlay, annotation canvas, undo/redo, tray + hotkey.
4. Phase 4: Wayland D-Bus capture, Windows GDI, dmg/exe/AppImage packaging.
```

- [ ] **Step 5.4: Write `wails/.gitignore`**

```
/build/
/vendor/
*.test
*.out
```

- [ ] **Step 5.5: Verify the binary still ships a clean JSON line**

Run:

```bash
cd wails && ./build/bin/snapvector --version
```

Expected: one-line JSON `{"status":"ok","code":0,"data":{"version":"0.1.0-phase1"}}`.

- [ ] **Step 5.6: Commit**

```bash
git add wails/scripts/bench-cli.sh wails/README.md wails/.gitignore
git commit -m "docs(wails): Phase 1 README + hyperfine benchmark script"
```

---

## Task 6: Close-out — phase 1 self-verification

- [ ] **Step 6.1: PRD coverage walk**

Confirm, one by one, against `../PRD.md`:

- §2.4 F4.1 dual-mode binary — argv dispatch present (`main.go`).
- §2.4 F4.2 machine-readable output — every `stdout` byte produced by `WriteOK`/`WriteError`.
- §2.4 F4.3 `--capture --base64-stdout` — working on darwin, stubbed elsewhere with retryable error 1201.
- §2.4 F4.4 `--inject-svg` — accepted as a flag and returns structured error 1301 (scheduled for Phase 2).
- §2.4 canonical JSON schema — status/code/data/error shape enforced by `response_test.go`; code ranges asserted by `TestCodeRanges`.
- §3 performance — benchmark script present; record baseline in PR.

- [ ] **Step 6.2: Run full suite one more time**

Run: `cd wails && go vet ./... && go test ./...`
Expected: all PASS; `go vet` silent.

- [ ] **Step 6.3: Final commit (if anything changed) and tag**

```bash
git status
# if clean, create the phase tag:
git tag -a wails-phase1 -m "Wails Phase 1 PoC: CLI + darwin capture"
```

---

## Out-of-Scope Reminders

These are **deliberately deferred** and must not be snuck into Phase 1 PRs:

- SVG composition, annotation rendering, `--inject-svg` payload parsing — Phase 2.
- PNG/JPG/PDF export, clipboard — Phase 2.
- Any `github.com/wailsapp/wails/v2` import, frontend scaffolding, region-select overlay — Phase 3.
- `github.com/godbus/dbus/v5`, Windows GDI, `screencapturekit` cgo bridges, installer packaging — Phase 4.

Keeping Phase 1 dependency-free is what makes the cold-start benchmark meaningful.
