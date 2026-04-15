//go:build darwin

package capture

import (
	"bytes"
	"context"
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
	if err != nil {
		return nil, Meta{}, err
	}
	if display, ok := displayUnderCursor(probe.Displays); ok {
		return captureDisplay(ctx, display)
	}
	if len(probe.Displays) == 1 {
		return captureDisplay(ctx, probe.Displays[0])
	}
	return captureAllDisplaysFromProbe(ctx, probe)
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

func interactiveRegionCaptureArgs() []string {
	// -s forces selection-only mode so a stray click can't fall into window
	// capture (which would otherwise grab the Wails app itself). Interactive
	// region is the one path we still fork screencapture for — CoreGraphics
	// has no equivalent of the native magnifier loupe overlay.
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

	// cgDisplayID holds the CGDirectDisplayID from CoreGraphics. It is only
	// populated by cgListDisplays; test fixtures that build darwinDisplay via
	// struct literals leave it zero, which is fine because those fixtures
	// never reach a real capture path.
	cgDisplayID uint32
}

// darwinVirtualRect is the union of all display frames in backing pixels.
// Retained for API compatibility with captureAllDisplaysViaRect; under cgo
// we no longer drive a `screencapture -R` call off it, but compose-path
// callers still find it useful.
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

// captureWithArgs forks /usr/sbin/screencapture. Only the interactive region
// path uses it now; all non-interactive captures go through cgo.
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

// captureAllDisplaysFromProbe dispatches multi-display capture. Under cgo both
// "compose" and default modes share the same implementation (CG has no
// `-R` virtual-rect equivalent), so the env var is a no-op but kept for
// backwards compatibility with existing user workarounds.
func captureAllDisplaysFromProbe(ctx context.Context, probe *darwinDisplayProbe) (PNG, Meta, error) {
	if probe == nil || len(probe.Displays) == 0 {
		return nil, Meta{}, fmt.Errorf("no displays to capture")
	}
	if os.Getenv("SNAPVECTOR_ALL_DISPLAYS_MODE") == "compose" {
		log.Printf("snapvector capture: SNAPVECTOR_ALL_DISPLAYS_MODE=compose (noop under cgo; both modes share path)")
	}
	return captureAllDisplays(ctx, probe.Displays)
}

// captureAllDisplaysViaRect is retained as a function name for test
// compatibility. Under cgo it delegates to the standard compose path — the
// rect argument is informational only.
func captureAllDisplaysViaRect(ctx context.Context, rect darwinVirtualRect) (PNG, Meta, error) {
	probe, err := probeDarwinDisplays(ctx)
	if err != nil {
		return nil, Meta{}, err
	}
	_ = rect
	return captureAllDisplays(ctx, probe.Displays)
}

func captureAllDisplays(ctx context.Context, displays []darwinDisplay) (PNG, Meta, error) {
	if !cgPreflightScreenCapture() {
		return nil, Meta{}, &PermissionDeniedError{
			Platform: "darwin",
			Stderr:   "CGPreflightScreenCaptureAccess returned false",
		}
	}
	_ = ctx // CG calls are synchronous and non-cancellable; ctx.Deadline is advisory.

	captures := make([]displayCapture, 0, len(displays))
	for _, display := range displays {
		if display.cgDisplayID == 0 {
			return nil, Meta{}, fmt.Errorf("display %d has no CGDisplayID (not from cgListDisplays)", display.Index)
		}
		started := time.Now()
		raw, err := cgCaptureDisplayPNG(display.cgDisplayID)
		log.Printf("snapvector capture: CGDisplayCreateImage id=%d index=%d -> elapsed=%s err=%v",
			display.cgDisplayID, display.Index, time.Since(started).Round(time.Millisecond), err)
		if err != nil {
			if !cgPreflightScreenCapture() {
				return nil, Meta{}, &PermissionDeniedError{
					Platform: "darwin",
					Stderr:   "CGPreflightScreenCaptureAccess returned false mid-capture",
				}
			}
			return nil, Meta{}, err
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, Meta{}, fmt.Errorf("decode display %d png: %w", display.Index, err)
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
	_ = ctx
	if !cgPreflightScreenCapture() {
		return nil, Meta{}, &PermissionDeniedError{
			Platform: "darwin",
			Stderr:   "CGPreflightScreenCaptureAccess returned false",
		}
	}
	if display.cgDisplayID == 0 {
		return nil, Meta{}, fmt.Errorf("display %d has no CGDisplayID (not from cgListDisplays)", display.Index)
	}

	started := time.Now()
	raw, err := cgCaptureDisplayPNG(display.cgDisplayID)
	log.Printf("snapvector capture: CGDisplayCreateImage id=%d index=%d -> elapsed=%s err=%v",
		display.cgDisplayID, display.Index, time.Since(started).Round(time.Millisecond), err)
	if err != nil {
		if !cgPreflightScreenCapture() {
			return nil, Meta{}, &PermissionDeniedError{
				Platform: "darwin",
				Stderr:   "CGPreflightScreenCaptureAccess returned false after capture attempt",
			}
		}
		return nil, Meta{}, err
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, Meta{}, fmt.Errorf("decode capture png: %w", err)
	}

	return PNG(raw), Meta{
		DisplayID:   fmt.Sprintf("%d", display.Index),
		X:           display.X,
		Y:           display.Y,
		Width:       cfg.Width,
		Height:      cfg.Height,
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

func probeDarwinDisplays(_ context.Context) (*darwinDisplayProbe, error) {
	displays, err := cgListDisplays()
	if err != nil {
		return nil, err
	}
	return &darwinDisplayProbe{
		Displays:    displays,
		VirtualRect: computeVirtualRect(displays),
	}, nil
}

func computeVirtualRect(displays []darwinDisplay) darwinVirtualRect {
	if len(displays) == 0 {
		return darwinVirtualRect{}
	}
	minX := displays[0].X
	minY := displays[0].Y
	maxX := displays[0].X + displays[0].Width
	maxY := displays[0].Y + displays[0].Height
	for _, d := range displays[1:] {
		if d.X < minX {
			minX = d.X
		}
		if d.Y < minY {
			minY = d.Y
		}
		if right := d.X + d.Width; right > maxX {
			maxX = right
		}
		if bot := d.Y + d.Height; bot > maxY {
			maxY = bot
		}
	}
	return darwinVirtualRect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}

func displayUnderCursor(displays []darwinDisplay) (darwinDisplay, bool) {
	for _, display := range displays {
		if display.ContainsCursor {
			return display, true
		}
	}
	return darwinDisplay{}, false
}

// classifyDarwinError maps a screencapture subprocess failure into our error
// taxonomy. Only the interactive region path still hits this; cgo-based
// captures surface their own error shape (permission via preflight, NULL
// image otherwise).
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
