//go:build windows

package capture

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"strconv"
	"time"

	"syscall"
	"unsafe"

	"github.com/kbinani/screenshot"
)

func NewPlatformCapturer() Capturer { return windowsCapturer{} }

type windowsCapturer struct{}

func (windowsCapturer) CaptureFullScreen(ctx context.Context) (PNG, Meta, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, Meta{}, fmt.Errorf("no active displays found")
	}

	idx := displayIndexUnderCursor(n)
	bounds := screenshot.GetDisplayBounds(idx)

	started := time.Now()
	img, err := screenshot.CaptureDisplay(idx)
	log.Printf("snapvector capture: CaptureDisplay(%d) elapsed=%s err=%v", idx, time.Since(started).Round(time.Millisecond), err)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("capture display %d: %w", idx, err)
	}

	raw, err := encodePNG(img)
	if err != nil {
		return nil, Meta{}, err
	}
	return PNG(raw), Meta{
		DisplayID:   strconv.Itoa(idx),
		X:           bounds.Min.X,
		Y:           bounds.Min.Y,
		Width:       bounds.Dx(),
		Height:      bounds.Dy(),
		ScaleFactor: 1.0,
	}, nil
}

func (windowsCapturer) CaptureAllDisplays(ctx context.Context) (PNG, Meta, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, Meta{}, fmt.Errorf("no active displays found")
	}

	type displayCapture struct {
		bounds image.Rectangle
		img    *image.RGBA
	}
	captures := make([]displayCapture, 0, n)

	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureDisplay(i)
		if err != nil {
			return nil, Meta{}, fmt.Errorf("capture display %d: %w", i, err)
		}
		captures = append(captures, displayCapture{bounds: bounds, img: img})
	}

	// Compute virtual bounding box.
	minX, minY := captures[0].bounds.Min.X, captures[0].bounds.Min.Y
	maxX, maxY := captures[0].bounds.Max.X, captures[0].bounds.Max.Y
	for _, c := range captures[1:] {
		if c.bounds.Min.X < minX {
			minX = c.bounds.Min.X
		}
		if c.bounds.Min.Y < minY {
			minY = c.bounds.Min.Y
		}
		if c.bounds.Max.X > maxX {
			maxX = c.bounds.Max.X
		}
		if c.bounds.Max.Y > maxY {
			maxY = c.bounds.Max.Y
		}
	}

	canvas := image.NewRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	for _, c := range captures {
		dest := image.Rect(
			c.bounds.Min.X-minX,
			c.bounds.Min.Y-minY,
			c.bounds.Max.X-minX,
			c.bounds.Max.Y-minY,
		)
		draw.Draw(canvas, dest, c.img, c.img.Bounds().Min, draw.Src)
	}

	raw, err := encodePNG(canvas)
	if err != nil {
		return nil, Meta{}, err
	}
	return PNG(raw), Meta{
		DisplayID: "all",
		X:         minX,
		Y:         minY,
		Width:     maxX - minX,
		Height:    maxY - minY,
	}, nil
}

func (windowsCapturer) CaptureInteractiveRegion(ctx context.Context) (PNG, Meta, error) {
	region, err := selectRegionNative(ctx)
	if err != nil {
		return nil, Meta{}, err
	}
	if region.Empty() {
		return nil, Meta{}, fmt.Errorf("interactive capture cancelled")
	}

	img, err := screenshot.CaptureRect(region)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("capture region %v: %w", region, err)
	}

	raw, err := encodePNG(img)
	if err != nil {
		return nil, Meta{}, err
	}
	return PNG(raw), Meta{
		X:           region.Min.X,
		Y:           region.Min.Y,
		Width:       region.Dx(),
		Height:      region.Dy(),
		ScaleFactor: 1.0,
	}, nil
}

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procGetCursorPos = user32.NewProc("GetCursorPos")
)

type winPOINT struct{ X, Y int32 }

// displayIndexUnderCursor returns the index of the display that currently
// contains the mouse cursor, falling back to 0 (primary display).
func displayIndexUnderCursor(n int) int {
	var pt winPOINT
	ret, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return 0
	}
	cursor := image.Point{X: int(pt.X), Y: int(pt.Y)}
	for i := 0; i < n; i++ {
		if cursor.In(screenshot.GetDisplayBounds(i)) {
			return i
		}
	}
	return 0
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

