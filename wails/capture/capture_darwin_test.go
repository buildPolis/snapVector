//go:build darwin

package capture

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

func TestDarwinCapturerReturnsDecodablePNG(t *testing.T) {
	if testing.Short() {
		t.Skip("capture test requires Screen Recording permission")
	}

	capturer := NewPlatformCapturer()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, meta, err := capturer.CaptureFullScreen(ctx)
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

	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("PNG has empty bounds %v", bounds)
	}
	if meta.Width != bounds.Dx() || meta.Height != bounds.Dy() {
		t.Fatalf("meta=%+v decoded=%dx%d", meta, bounds.Dx(), bounds.Dy())
	}
}

func TestInteractiveRegionCaptureArgsRemainInteractive(t *testing.T) {
	args := interactiveRegionCaptureArgs()

	want := map[string]bool{"-i": false, "-s": false}
	for _, arg := range args {
		if _, ok := want[arg]; ok {
			want[arg] = true
		}
	}
	for flag, seen := range want {
		if !seen {
			t.Fatalf("expected %q in interactive args: %v", flag, args)
		}
	}
}

func TestDisplayUnderCursorChoosesContainingDisplay(t *testing.T) {
	display, ok := displayUnderCursor([]darwinDisplay{
		{Index: 1, X: 0, Y: 0, Width: 3456, Height: 2234, ContainsCursor: false},
		{Index: 2, X: -300, Y: 2234, Width: 3840, Height: 2160, ContainsCursor: true},
	})
	if !ok {
		t.Fatal("expected a display under cursor")
	}
	if display.Index != 2 {
		t.Fatalf("display index = %d, want 2", display.Index)
	}
}

func TestComposeDisplayCapturesUsesGlobalBackingCoordinates(t *testing.T) {
	// CGDisplayBounds is top-left origin, so a display at Y=0 sits at the top
	// of the composed canvas and a display at a higher Y sits below it. The
	// earlier implementation inverted this by applying a Cocoa-style Y flip;
	// this fixture pins the corrected ordering.
	top := image.NewRGBA(image.Rect(0, 0, 4, 2))
	fill(top, color.RGBA{R: 255, A: 255})
	bottom := image.NewRGBA(image.Rect(0, 0, 6, 3))
	fill(bottom, color.RGBA{G: 255, A: 255})

	raw, meta, err := composeDisplayCaptures([]displayCapture{
		{
			Display: darwinDisplay{Index: 1, X: 0, Y: 0, Width: 4, Height: 2},
			Image:   top,
		},
		{
			Display: darwinDisplay{Index: 2, X: -2, Y: 2, Width: 6, Height: 3},
			Image:   bottom,
		},
	})
	if err != nil {
		t.Fatalf("composeDisplayCaptures returned error: %v", err)
	}
	if meta.X != -2 || meta.Y != 0 {
		t.Fatalf("meta origin = (%d,%d), want (-2,0)", meta.X, meta.Y)
	}
	if meta.Width != 6 || meta.Height != 5 {
		t.Fatalf("meta size = %dx%d, want 6x5", meta.Width, meta.Height)
	}

	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode composed png: %v", err)
	}

	// Canvas layout after normalising to (minX=-2, minY=0):
	//   rows 0-1, cols 2-5 : red (top display at Y=0)
	//   rows 2-4, cols 0-5 : green (bottom display at Y=2)
	//   rows 0-1, cols 0-1 : empty (no display covers this region)
	if got := color.RGBAModel.Convert(img.At(3, 0)).(color.RGBA); got.R != 255 {
		t.Fatalf("top display pixel = %+v, want red at top", got)
	}
	if got := color.RGBAModel.Convert(img.At(0, 4)).(color.RGBA); got.G != 255 {
		t.Fatalf("bottom display pixel = %+v, want green at bottom", got)
	}
	if got := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA); got.R != 0 || got.G != 0 || got.B != 0 || got.A != 0 {
		t.Fatalf("uncovered pixel = %+v, want transparent", got)
	}
}

func TestComposeDisplayCapturesPlacesExternalAbovePrimary(t *testing.T) {
	// Concrete regression for issue #8: MacBook primary at (0, 0) with an
	// external monitor physically above it (Y is negative under top-left
	// origin). Before the fix the external landed at the bottom of the
	// composed image; after the fix it sits at the top.
	external := image.NewRGBA(image.Rect(0, 0, 4, 3))
	fill(external, color.RGBA{B: 255, A: 255})
	primary := image.NewRGBA(image.Rect(0, 0, 4, 2))
	fill(primary, color.RGBA{R: 255, A: 255})

	raw, meta, err := composeDisplayCaptures([]displayCapture{
		{
			Display: darwinDisplay{Index: 1, X: 0, Y: 0, Width: 4, Height: 2},
			Image:   primary,
		},
		{
			Display: darwinDisplay{Index: 2, X: 0, Y: -3, Width: 4, Height: 3},
			Image:   external,
		},
	})
	if err != nil {
		t.Fatalf("composeDisplayCaptures returned error: %v", err)
	}
	if meta.X != 0 || meta.Y != -3 {
		t.Fatalf("meta origin = (%d,%d), want (0,-3)", meta.X, meta.Y)
	}
	if meta.Width != 4 || meta.Height != 5 {
		t.Fatalf("meta size = %dx%d, want 4x5", meta.Width, meta.Height)
	}

	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode composed png: %v", err)
	}

	// Top rows (0-2) must be the external (blue); bottom rows (3-4) primary (red).
	if got := color.RGBAModel.Convert(img.At(2, 1)).(color.RGBA); got.B != 255 {
		t.Fatalf("external band pixel = %+v, want blue (external at top)", got)
	}
	if got := color.RGBAModel.Convert(img.At(2, 4)).(color.RGBA); got.R != 255 {
		t.Fatalf("primary band pixel = %+v, want red (primary at bottom)", got)
	}
}

func TestCaptureAllDisplaysSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("requires screen recording permission")
	}
	// End-to-end: enumerate displays via cgo, capture each, compose. Verifies
	// the CG → ImageIO → compose pipeline round-trips to a decodable PNG
	// without needing any filesystem or subprocess.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	probe, err := probeDarwinDisplays(ctx)
	if err != nil {
		t.Fatalf("probeDarwinDisplays: %v", err)
	}
	raw, meta, err := captureAllDisplaysFromProbe(ctx, probe)
	if err != nil {
		if _, ok := err.(*PermissionDeniedError); ok {
			t.Skip("screen recording permission not granted")
		}
		t.Fatalf("captureAllDisplaysFromProbe failed: %v", err)
	}
	if len(raw) < 50 {
		t.Fatalf("PNG suspiciously small: %d bytes", len(raw))
	}
	if meta.Width <= 0 || meta.Height <= 0 {
		t.Fatalf("meta has non-positive dims: %+v", meta)
	}
	if meta.DisplayID != "all" && len(probe.Displays) > 1 {
		t.Fatalf("DisplayID = %q, want \"all\" for multi-display", meta.DisplayID)
	}
}

func TestCaptureAllDisplaysFromProbeRejectsEmpty(t *testing.T) {
	// Empty probe must not panic and must return a clean error — callers rely
	// on this guard when displays disappear between enumeration and capture.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, _, err := captureAllDisplaysFromProbe(ctx, &darwinDisplayProbe{})
	if err == nil {
		t.Fatal("expected error for empty probe, got nil")
	}

	_, _, err = captureAllDisplaysFromProbe(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil probe, got nil")
	}
}

func TestPermissionDeniedErrorShape(t *testing.T) {
	// Regression guard: the cgo rewrite must continue to surface the same
	// error shape that cli.writeCaptureError pattern-matches against.
	err := &PermissionDeniedError{Platform: "darwin", Stderr: "diagnostic text"}
	if err.Error() != "screen capture permission denied" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if err.Platform != "darwin" {
		t.Fatalf("Platform = %q", err.Platform)
	}
}

func fill(img *image.RGBA, value color.RGBA) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetRGBA(x, y, value)
		}
	}
}
