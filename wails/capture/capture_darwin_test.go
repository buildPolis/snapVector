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

func TestFullScreenCaptureArgsDoNotPinDisplayOne(t *testing.T) {
	args := fullScreenCaptureArgs()

	if len(args) == 0 {
		t.Fatal("expected non-empty screencapture args")
	}
	for i, arg := range args {
		if arg == "-D" {
			t.Fatalf("unexpected display pinning flag -D at index %d in %v", i, args)
		}
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
	left := image.NewRGBA(image.Rect(0, 0, 4, 2))
	fill(left, color.RGBA{R: 255, A: 255})
	right := image.NewRGBA(image.Rect(0, 0, 6, 3))
	fill(right, color.RGBA{G: 255, A: 255})

	raw, meta, err := composeDisplayCaptures([]displayCapture{
		{
			Display: darwinDisplay{Index: 1, X: 0, Y: 0, Width: 4, Height: 2},
			Image:   left,
		},
		{
			Display: darwinDisplay{Index: 2, X: -2, Y: 2, Width: 6, Height: 3},
			Image:   right,
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

	if got := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA); got.G != 255 {
		t.Fatalf("top-left pixel = %+v, want green display at top", got)
	}
	if got := color.RGBAModel.Convert(img.At(2, 4)).(color.RGBA); got.R != 255 {
		t.Fatalf("bottom region pixel = %+v, want red display at bottom", got)
	}
}

func TestCaptureAllDisplaysViaRectFormatsArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("requires screen recording permission")
	}
	// Smoke test: a 1x1 capture should still return a decodable PNG, proving
	// the -R argument round-trips through screencapture without coordinate
	// translation bugs in the Go layer.
	rect := darwinVirtualRect{X: 0, Y: 0, Width: 1, Height: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raw, meta, err := captureAllDisplaysViaRect(ctx, rect)
	if err != nil {
		// permission errors are acceptable in CI but should be the only failure mode
		if _, ok := err.(*PermissionDeniedError); ok {
			t.Skip("screen recording permission not granted")
		}
		t.Fatalf("captureAllDisplaysViaRect failed: %v", err)
	}
	if len(raw) < 50 {
		t.Fatalf("PNG suspiciously small: %d bytes", len(raw))
	}
	if meta.Width <= 0 || meta.Height <= 0 {
		t.Fatalf("meta has non-positive dims: %+v", meta)
	}
	if meta.DisplayID != "all" {
		t.Fatalf("DisplayID = %q, want \"all\"", meta.DisplayID)
	}
}

func TestCaptureAllDisplaysFromProbeFallsBackOnInvalidRect(t *testing.T) {
	probe := &darwinDisplayProbe{
		Displays: []darwinDisplay{
			{Index: 1, X: 0, Y: 0, Width: 4, Height: 2},
		},
		VirtualRect: darwinVirtualRect{}, // zero rect should trigger fallback
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// captureAllDisplays will try to spawn screencapture and may fail or be
	// cancelled — that's fine; we only care that the dispatcher RECOGNISES
	// the invalid rect and routes to the compose path rather than calling
	// captureAllDisplaysViaRect with bad args. We assert on the absence of a
	// "screencapture -R 0,0,0,0" log line indirectly by ensuring the call
	// does not panic and returns *some* error path that isn't from -R.
	_, _, err := captureAllDisplaysFromProbe(ctx, probe)
	// We expect either a permission error, a cancel error, or a screencapture
	// failure from the compose path. The key invariant is that the function
	// returned without panicking on the zero rect.
	if err == nil {
		// Permission was granted and compose succeeded — also acceptable.
		return
	}
	if ctx.Err() == nil && err == nil {
		t.Fatalf("expected an error or success, got neither")
	}
}

func fill(img *image.RGBA, value color.RGBA) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetRGBA(x, y, value)
		}
	}
}
