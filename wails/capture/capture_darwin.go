//go:build darwin

package capture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func NewPlatformCapturer() Capturer { return darwinCapturer{} }

type darwinCapturer struct{}

func (darwinCapturer) CaptureFullScreen(ctx context.Context) (PNG, Meta, error) {
	probe, err := probeDarwinDisplays(ctx)
	if err == nil {
		if display, ok := displayUnderCursor(probe.Displays); ok {
			return captureDisplay(ctx, display)
		}
		if len(probe.Displays) > 1 {
			return captureAllDisplaysFromProbe(ctx, probe)
		}
	}
	return captureWithArgs(ctx, fullScreenCaptureArgs()...)
}

func (darwinCapturer) CaptureAllDisplays(ctx context.Context) (PNG, Meta, error) {
	probe, err := probeDarwinDisplays(ctx)
	if err != nil {
		return nil, Meta{}, err
	}
	return captureAllDisplaysFromProbe(ctx, probe)
}

func (darwinCapturer) CaptureInteractiveRegion(ctx context.Context) (PNG, Meta, error) {
	return captureWithArgs(ctx, interactiveRegionCaptureArgs()...)
}

func fullScreenCaptureArgs() []string {
	return []string{"-x", "-t", "png"}
}

func interactiveRegionCaptureArgs() []string {
	// -s forces selection-only mode so a stray click can't fall into window
	// capture (which would otherwise grab the Wails app itself).
	return []string{"-i", "-s", "-x", "-t", "png"}
}

type darwinDisplay struct {
	Index          int     `json:"index"`
	X              int     `json:"x"`
	Y              int     `json:"y"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	ScaleFactor    float64 `json:"scaleFactor"`
	ContainsCursor bool    `json:"containsCursor"`
}

// darwinVirtualRect describes the bounding box of all active displays in the
// coordinate system expected by `screencapture -R` (top-left origin of the
// primary display, in points — NOT backing pixels).
type darwinVirtualRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type darwinDisplayProbe struct {
	Displays    []darwinDisplay   `json:"displays"`
	VirtualRect darwinVirtualRect `json:"virtualRect"`
}

type displayCapture struct {
	Display darwinDisplay
	Image   image.Image
}

func captureWithArgs(ctx context.Context, args ...string) (PNG, Meta, error) {
	tempDir, err := os.MkdirTemp("", "snapvector-capture-*")
	if err != nil {
		return nil, Meta{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	path := filepath.Join(tempDir, "capture.png")
	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", append(args, path)...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	started := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(started)
	log.Printf("snapvector capture: screencapture %v -> elapsed=%s stderr=%q err=%v",
		args, elapsed.Round(time.Millisecond), strings.TrimSpace(stderr.String()), runErr)
	if runErr != nil {
		return nil, Meta{}, classifyDarwinError(runErr, stderr.String())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if permissionDenied(stderr.String()) {
			return nil, Meta{}, &PermissionDeniedError{
				Platform: "darwin",
				Stderr:   strings.TrimSpace(stderr.String()),
			}
		}
		return nil, Meta{}, fmt.Errorf("read capture file: %w", err)
	}
	if len(raw) == 0 {
		if permissionDenied(stderr.String()) {
			return nil, Meta{}, &PermissionDeniedError{
				Platform: "darwin",
				Stderr:   strings.TrimSpace(stderr.String()),
			}
		}
		return nil, Meta{}, fmt.Errorf("capture file is empty")
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, Meta{}, fmt.Errorf("decode capture png: %w", err)
	}

	return PNG(raw), Meta{
		Width:  cfg.Width,
		Height: cfg.Height,
	}, nil
}

// captureAllDisplaysFromProbe is the preferred multi-display path: a single
// `screencapture -R` invocation covering the virtual bounding rect. This
// avoids N process forks and N disk round-trips that the N×`-D i` + compose
// path (captureAllDisplays) incurs.
//
// Trade-off: in mixed-DPI layouts `screencapture -R` captures at a unified
// resolution, so non-primary-DPI displays may be up/downsampled. Set
// SNAPVECTOR_ALL_DISPLAYS_MODE=compose to force the per-display fallback when
// pixel-perfect resolution on every display matters more than latency.
func captureAllDisplaysFromProbe(ctx context.Context, probe *darwinDisplayProbe) (PNG, Meta, error) {
	if os.Getenv("SNAPVECTOR_ALL_DISPLAYS_MODE") == "compose" {
		return captureAllDisplays(ctx, probe.Displays)
	}
	if probe.VirtualRect.Width <= 0 || probe.VirtualRect.Height <= 0 {
		log.Printf("snapvector capture: invalid virtualRect %+v, using compose fallback", probe.VirtualRect)
		return captureAllDisplays(ctx, probe.Displays)
	}
	png, meta, err := captureAllDisplaysViaRect(ctx, probe.VirtualRect)
	if err == nil {
		return png, meta, nil
	}
	// Permission errors should surface immediately rather than trigger a
	// fallback that will also fail (and confuse the user with two errors).
	if _, ok := err.(*PermissionDeniedError); ok {
		return nil, Meta{}, err
	}
	log.Printf("snapvector capture: -R path failed (%v), falling back to compose", err)
	return captureAllDisplays(ctx, probe.Displays)
}

func captureAllDisplaysViaRect(ctx context.Context, rect darwinVirtualRect) (PNG, Meta, error) {
	tempDir, err := os.MkdirTemp("", "snapvector-capture-*")
	if err != nil {
		return nil, Meta{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	path := filepath.Join(tempDir, "all-displays.png")
	rectArg := fmt.Sprintf("%d,%d,%d,%d", rect.X, rect.Y, rect.Width, rect.Height)
	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-t", "png", "-R", rectArg, path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	started := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(started)
	log.Printf("snapvector capture: screencapture -R %s -> elapsed=%s stderr=%q err=%v",
		rectArg, elapsed.Round(time.Millisecond), strings.TrimSpace(stderr.String()), runErr)
	if runErr != nil {
		return nil, Meta{}, classifyDarwinError(runErr, stderr.String())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if permissionDenied(stderr.String()) {
			return nil, Meta{}, &PermissionDeniedError{
				Platform: "darwin",
				Stderr:   strings.TrimSpace(stderr.String()),
			}
		}
		return nil, Meta{}, fmt.Errorf("read capture file: %w", err)
	}
	if len(raw) == 0 {
		if permissionDenied(stderr.String()) {
			return nil, Meta{}, &PermissionDeniedError{
				Platform: "darwin",
				Stderr:   strings.TrimSpace(stderr.String()),
			}
		}
		return nil, Meta{}, fmt.Errorf("capture file is empty")
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, Meta{}, fmt.Errorf("decode capture png: %w", err)
	}

	return PNG(raw), Meta{
		DisplayID: "all",
		X:         rect.X,
		Y:         rect.Y,
		Width:     cfg.Width,
		Height:    cfg.Height,
	}, nil
}

func captureAllDisplays(ctx context.Context, displays []darwinDisplay) (PNG, Meta, error) {
	tempDir, err := os.MkdirTemp("", "snapvector-capture-*")
	if err != nil {
		return nil, Meta{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	captures := make([]displayCapture, 0, len(displays))
	for _, display := range displays {
		path := filepath.Join(tempDir, fmt.Sprintf("display-%d.png", display.Index))
		cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-t", "png", "-D", fmt.Sprintf("%d", display.Index), path)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, Meta{}, classifyDarwinError(err, stderr.String())
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, Meta{}, fmt.Errorf("read display capture: %w", err)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, Meta{}, fmt.Errorf("decode display capture png: %w", err)
		}

		display.Width = img.Bounds().Dx()
		display.Height = img.Bounds().Dy()
		captures = append(captures, displayCapture{
			Display: display,
			Image:   img,
		})
	}

	return composeDisplayCaptures(captures)
}

func captureDisplay(ctx context.Context, display darwinDisplay) (PNG, Meta, error) {
	tempDir, err := os.MkdirTemp("", "snapvector-capture-*")
	if err != nil {
		return nil, Meta{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	path := filepath.Join(tempDir, fmt.Sprintf("display-%d.png", display.Index))
	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-t", "png", "-D", fmt.Sprintf("%d", display.Index), path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, Meta{}, classifyDarwinError(err, stderr.String())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("read display capture: %w", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, Meta{}, fmt.Errorf("decode display capture png: %w", err)
	}

	display.Width = cfg.Width
	display.Height = cfg.Height
	return PNG(raw), Meta{
		DisplayID:   fmt.Sprintf("%d", display.Index),
		X:           display.X,
		Y:           display.Y,
		Width:       display.Width,
		Height:      display.Height,
		ScaleFactor: display.ScaleFactor,
	}, nil
}

func composeDisplayCaptures(captures []displayCapture) (PNG, Meta, error) {
	if len(captures) == 0 {
		return nil, Meta{}, fmt.Errorf("no display captures to compose")
	}

	minX := captures[0].Display.X
	minY := captures[0].Display.Y
	maxX := captures[0].Display.X + captures[0].Display.Width
	maxY := captures[0].Display.Y + captures[0].Display.Height

	for _, capture := range captures[1:] {
		if capture.Display.X < minX {
			minX = capture.Display.X
		}
		if capture.Display.Y < minY {
			minY = capture.Display.Y
		}
		if right := capture.Display.X + capture.Display.Width; right > maxX {
			maxX = right
		}
		if top := capture.Display.Y + capture.Display.Height; top > maxY {
			maxY = top
		}
	}

	canvas := image.NewRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	for _, capture := range captures {
		x := capture.Display.X - minX
		y := maxY - (capture.Display.Y + capture.Display.Height)
		dest := image.Rect(x, y, x+capture.Display.Width, y+capture.Display.Height)
		draw.Draw(canvas, dest, capture.Image, capture.Image.Bounds().Min, draw.Src)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, Meta{}, fmt.Errorf("encode composed png: %w", err)
	}

	return PNG(buf.Bytes()), Meta{
		DisplayID: "all",
		X:         minX,
		Y:         minY,
		Width:     canvas.Bounds().Dx(),
		Height:    canvas.Bounds().Dy(),
	}, nil
}

func probeDarwinDisplays(ctx context.Context) (*darwinDisplayProbe, error) {
	tempDir, err := os.MkdirTemp("", "snapvector-displays-*")
	if err != nil {
		return nil, fmt.Errorf("create display temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	scriptPath := filepath.Join(tempDir, "list_displays.swift")
	if err := os.WriteFile(scriptPath, []byte(swiftDisplayProbe), 0o600); err != nil {
		return nil, fmt.Errorf("write display probe script: %w", err)
	}

	cmd := exec.CommandContext(ctx, "swift", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list darwin displays via swift: %w (%s)", err, string(output))
	}

	var probe darwinDisplayProbe
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("decode display probe output: %w (%s)", err, string(output))
	}
	if len(probe.Displays) == 0 {
		return nil, fmt.Errorf("display probe returned no active displays")
	}

	return &probe, nil
}

// listDarwinDisplays is a legacy thin wrapper retained for tests and any
// future caller that only needs the per-display list.
func listDarwinDisplays(ctx context.Context) ([]darwinDisplay, error) {
	probe, err := probeDarwinDisplays(ctx)
	if err != nil {
		return nil, err
	}
	return probe.Displays, nil
}

func displayUnderCursor(displays []darwinDisplay) (darwinDisplay, bool) {
	for _, display := range displays {
		if display.ContainsCursor {
			return display, true
		}
	}
	return darwinDisplay{}, false
}

// swiftDisplayProbe enumerates NSScreen.screens and emits two pieces of data:
//   1. Per-display backing-pixel rects (used by the per-display capture path
//      and the compose fallback — preserves the historical contract).
//   2. A virtualRect describing the union of all displays in the coordinate
//      system that `screencapture -R` expects (top-left origin of the primary
//      display, in POINTS not backing pixels). Pre-computing this in Swift
//      keeps Go free of NSScreen coordinate conversion.
const swiftDisplayProbe = `import AppKit
import Foundation

let cursor = NSEvent.mouseLocation
let screens = NSScreen.screens
let primary = screens.first(where: { $0.frame.origin == .zero }) ?? screens.first!
let primaryHeight = primary.frame.size.height

var displays: [[String: Any]] = []
for (index, screen) in screens.enumerated() {
  let frame = screen.frame
  let backing = screen.convertRectToBacking(frame)
  displays.append([
    "index": index + 1,
    "x": Int(backing.origin.x.rounded()),
    "y": Int(backing.origin.y.rounded()),
    "width": Int(backing.size.width.rounded()),
    "height": Int(backing.size.height.rounded()),
    "scaleFactor": screen.backingScaleFactor,
    "containsCursor": frame.contains(cursor),
  ])
}

var minX = CGFloat.infinity
var maxX = -CGFloat.infinity
var minY = CGFloat.infinity
var maxY = -CGFloat.infinity
for screen in screens {
  let f = screen.frame
  // NSScreen uses bottom-left origin; screencapture -R uses top-left origin
  // of the primary display. Invert Y around primaryHeight.
  let topY = primaryHeight - (f.origin.y + f.size.height)
  let bottomY = primaryHeight - f.origin.y
  minX = min(minX, f.origin.x)
  maxX = max(maxX, f.origin.x + f.size.width)
  minY = min(minY, topY)
  maxY = max(maxY, bottomY)
}

let virtualRect: [String: Int] = [
  "x": Int(minX.rounded()),
  "y": Int(minY.rounded()),
  "width": Int((maxX - minX).rounded()),
  "height": Int((maxY - minY).rounded()),
]

let payload: [String: Any] = [
  "displays": displays,
  "virtualRect": virtualRect,
]

let data = try JSONSerialization.data(withJSONObject: payload, options: [])
FileHandle.standardOutput.write(data)
`

func classifyDarwinError(runErr error, stderr string) error {
	if permissionDenied(stderr) {
		return &PermissionDeniedError{
			Platform: "darwin",
			Stderr:   strings.TrimSpace(stderr),
		}
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr) == "" {
		return fmt.Errorf("interactive capture cancelled")
	}

	return fmt.Errorf("screencapture failed: %w (stderr=%q)", runErr, strings.TrimSpace(stderr))
}

func permissionDenied(stderr string) bool {
	lowered := strings.ToLower(stderr)
	return strings.Contains(lowered, "permission") ||
		strings.Contains(lowered, "denied") ||
		strings.Contains(lowered, "not authorized") ||
		strings.Contains(lowered, "not permitted") ||
		strings.Contains(lowered, "screen recording")
}
