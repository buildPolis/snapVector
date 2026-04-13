//go:build darwin

package capture

import (
	"bytes"
	"context"
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
