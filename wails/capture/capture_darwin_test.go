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

	hasInteractive := false
	for _, arg := range args {
		if arg == "-i" {
			hasInteractive = true
			break
		}
	}
	if !hasInteractive {
		t.Fatalf("expected -i in interactive args: %v", args)
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

func fill(img *image.RGBA, value color.RGBA) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetRGBA(x, y, value)
		}
	}
}
