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
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
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
	if !ensureScreenCaptureAccess() {
		return nil, Meta{}, &PermissionDeniedError{
			Platform: "darwin",
			Stderr:   "Screen Recording permission not granted; a system dialog was requested",
		}
	}
	_ = ctx // CG calls are synchronous and non-cancellable; ctx.Deadline is advisory.

	// Validate IDs up front so a bad input fails fast without spinning up
	// goroutines that would just error out.
	for _, display := range displays {
		if display.cgDisplayID == 0 {
			return nil, Meta{}, fmt.Errorf("display %d has no CGDisplayID (not from cgListDisplays)", display.Index)
		}
	}

	// Capture displays in parallel AND bypass the ImageIO PNG encode/decode
	// round-trip. cgCaptureDisplayRGBA returns native image.RGBA bytes
	// directly from CoreGraphics, which composeDisplayCaptures consumes
	// without further decoding. Earlier attempts that ran
	// cgCaptureDisplayPNG in goroutines actually *regressed* latency because
	// ImageIO's PNG encode is CPU-bound, so three parallel goroutines all
	// fought for cores. Skipping that encode is what makes parallel capture
	// net-positive: each goroutine is now just the window-server read plus
	// a memcpy into Go's Pix slice.
	type captureResult struct {
		capture displayCapture
		err     error
	}
	results := make([]captureResult, len(displays))
	var wg sync.WaitGroup
	batchStarted := time.Now()
	for i, display := range displays {
		wg.Add(1)
		go func(i int, display darwinDisplay) {
			defer wg.Done()
			started := time.Now()
			img, err := cgCaptureDisplayRGBA(display.cgDisplayID)
			log.Printf("snapvector capture: CGDisplayCreateImage id=%d index=%d -> elapsed=%s err=%v",
				display.cgDisplayID, display.Index, time.Since(started).Round(time.Millisecond), err)
			if err != nil {
				results[i].err = err
				return
			}
			// Keep display.Width/Height in POINTS; the image retains its native
			// backing pixel size via img.Bounds(). composeDisplayCaptures uses
			// points for layout and resamples the image when its pixel
			// dimensions differ from (points × targetScale).
			results[i].capture = displayCapture{
				Display: display,
				Image:   img,
			}
		}(i, display)
	}
	wg.Wait()
	log.Printf("snapvector capture: parallel RGBA capture of %d display(s) wall-clock elapsed=%s",
		len(displays), time.Since(batchStarted).Round(time.Millisecond))

	captures := make([]displayCapture, 0, len(displays))
	for _, r := range results {
		if r.err != nil {
			if !cgPreflightScreenCapture() {
				return nil, Meta{}, &PermissionDeniedError{
					Platform: "darwin",
					Stderr:   "CGPreflightScreenCaptureAccess returned false mid-capture",
				}
			}
			return nil, Meta{}, r.err
		}
		captures = append(captures, r.capture)
	}
	return composeDisplayCaptures(captures)
}

func captureDisplay(ctx context.Context, display darwinDisplay) (PNG, Meta, error) {
	_ = ctx
	if !ensureScreenCaptureAccess() {
		return nil, Meta{}, &PermissionDeniedError{
			Platform: "darwin",
			Stderr:   "Screen Recording permission not granted; a system dialog was requested",
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

	// display.X/Y live in points (unified across displays); the single-display
	// Meta contract has always been pixel-space, so multiply back by this
	// display's own scale. There is no cross-display ambiguity here because
	// only one display is in play.
	scale := display.ScaleFactor
	if scale <= 0 {
		scale = 1
	}
	return PNG(raw), Meta{
		DisplayID:   fmt.Sprintf("%d", display.Index),
		X:           int(math.Round(float64(display.X) * scale)),
		Y:           int(math.Round(float64(display.Y) * scale)),
		Width:       cfg.Width,
		Height:      cfg.Height,
		ScaleFactor: display.ScaleFactor,
	}, nil
}

func composeDisplayCaptures(captures []displayCapture) (PNG, Meta, error) {
	if len(captures) == 0 {
		return nil, Meta{}, fmt.Errorf("no display captures to compose")
	}

	// Layout is computed in POINTS (CGDisplayBounds' unified coord space).
	// The canvas is rendered at targetScale = max(ScaleFactor) so the
	// highest-DPI display keeps its native pixels intact and lower-DPI
	// displays are upsampled to match. Using any single display's scale (as
	// the earlier implementation did) left mixed-DPI layouts overlapping or
	// gap-ridden depending on which display sat at the origin.
	minX := captures[0].Display.X
	minY := captures[0].Display.Y
	maxX := captures[0].Display.X + captures[0].Display.Width
	maxY := captures[0].Display.Y + captures[0].Display.Height
	targetScale := captures[0].Display.ScaleFactor

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
		if bot := capture.Display.Y + capture.Display.Height; bot > maxY {
			maxY = bot
		}
		if s := capture.Display.ScaleFactor; s > targetScale {
			targetScale = s
		}
	}
	if targetScale <= 0 {
		targetScale = 1
	}

	canvasW := int(math.Round(float64(maxX-minX) * targetScale))
	canvasH := int(math.Round(float64(maxY-minY) * targetScale))
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))

	drawStarted := time.Now()
	for _, capture := range captures {
		pxX := int(math.Round(float64(capture.Display.X-minX) * targetScale))
		pxY := int(math.Round(float64(capture.Display.Y-minY) * targetScale))
		tw := int(math.Round(float64(capture.Display.Width) * targetScale))
		th := int(math.Round(float64(capture.Display.Height) * targetScale))
		dest := image.Rect(pxX, pxY, pxX+tw, pxY+th)

		src := capture.Image
		if sb := src.Bounds(); sb.Dx() != tw || sb.Dy() != th {
			resized := image.NewRGBA(image.Rect(0, 0, tw, th))
			// ApproxBiLinear over CatmullRom: cubic is ~3× slower and the
			// quality gain is invisible on a screen screenshot that gets
			// zoomed out to fit the canvas. For the 1x→2x retina upsample
			// this is also an integer ratio, so any resampler degenerates
			// to nearest anyway.
			xdraw.ApproxBiLinear.Scale(resized, resized.Bounds(), src, sb, xdraw.Src, nil)
			src = resized
		}

		draw.Draw(canvas, dest, src, src.Bounds().Min, draw.Src)
	}
	log.Printf("snapvector capture: compose draw %dx%d px elapsed=%s",
		canvasW, canvasH, time.Since(drawStarted).Round(time.Millisecond))

	encodeStarted := time.Now()
	var buf bytes.Buffer
	// BestSpeed (zlib level 1) over DefaultCompression (level 6): 3–5× faster
	// encode for ~20–30% larger PNG. The composed PNG is a transient IPC
	// payload between Go and the webview; file size matters less than
	// wall-clock latency to first render.
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buf, canvas); err != nil {
		return nil, Meta{}, fmt.Errorf("encode composed png: %w", err)
	}
	log.Printf("snapvector capture: compose encode %d bytes elapsed=%s",
		buf.Len(), time.Since(encodeStarted).Round(time.Millisecond))

	return PNG(buf.Bytes()), Meta{
		DisplayID: "all",
		X:         int(math.Round(float64(minX) * targetScale)),
		Y:         int(math.Round(float64(minY) * targetScale)),
		Width:     canvasW,
		Height:    canvasH,
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

// ensureScreenCaptureAccess checks preflight and, if permission is not
// granted, triggers a system dialog via CGRequestScreenCaptureAccess. The
// Request call is what registers the app in the Privacy panel — without
// calling it at least once, preflight stays false forever and TCC has no
// record of the app. Safe to call on every capture: it returns quickly and
// only shows a dialog the first time.
func ensureScreenCaptureAccess() bool {
	if cgPreflightScreenCapture() {
		return true
	}
	if cgRequestScreenCapture() {
		return true
	}
	log.Printf("snapvector capture: Screen Recording permission not granted; system dialog requested")
	return false
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
