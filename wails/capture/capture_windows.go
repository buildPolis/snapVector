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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	region, err := selectRegionViaPS(ctx)
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

// selectRegionViaPS runs an embedded PowerShell overlay that shows the current
// screen, lets the user draw a rubber-band selection rectangle, and returns the
// selected rectangle in absolute virtual-screen coordinates.
func selectRegionViaPS(ctx context.Context) (image.Rectangle, error) {
	tempDir, err := os.MkdirTemp("", "snapvector-region-*")
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	scriptPath := filepath.Join(tempDir, "region_select.ps1")
	if err := os.WriteFile(scriptPath, []byte(psRegionScript), 0o600); err != nil {
		return image.Rectangle{}, fmt.Errorf("write region script: %w", err)
	}

	cmd := exec.CommandContext(ctx,
		"powershell.exe", "-STA", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	log.Printf("snapvector region-select: output=%q stderr=%q err=%v", output, strings.TrimSpace(stderr.String()), runErr)

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return image.Rectangle{}, nil // cancelled
		}
		return image.Rectangle{}, fmt.Errorf("region select script failed: %w", runErr)
	}

	parts := strings.Split(output, ",")
	if len(parts) != 4 {
		return image.Rectangle{}, fmt.Errorf("unexpected region-select output: %q", output)
	}

	vals := make([]int, 4)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return image.Rectangle{}, fmt.Errorf("parse region coord %q: %w", p, err)
		}
		vals[i] = v
	}

	x, y, w, h := vals[0], vals[1], vals[2], vals[3]
	if w <= 0 || h <= 0 {
		return image.Rectangle{}, nil // zero-size selection
	}
	return image.Rect(x, y, x+w, y+h), nil
}

var (
	user32         = syscall.NewLazyDLL("user32.dll")
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

// psRegionScript is a PowerShell script that:
//  1. Takes a screenshot of the entire virtual screen as background.
//  2. Shows a borderless full-screen overlay with a crosshair cursor.
//  3. Lets the user drag a red rubber-band selection rectangle.
//  4. Prints "x,y,w,h" (absolute virtual-screen coordinates) to stdout.
//  5. Exits with code 1 on Escape or zero-size selection.
const psRegionScript = `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$vs = [System.Windows.Forms.SystemInformation]::VirtualScreen

# Capture current screen content as the overlay background.
$bmp = New-Object System.Drawing.Bitmap($vs.Width, $vs.Height)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($vs.X, $vs.Y, 0, 0, $bmp.Size)
$g.Dispose()

$form = New-Object System.Windows.Forms.Form
$form.FormBorderStyle = [System.Windows.Forms.FormBorderStyle]::None
$form.Bounds = $vs
$form.TopMost = $true
$form.BackgroundImage = $bmp
$form.BackgroundImageLayout = [System.Windows.Forms.ImageLayout]::None
$form.Cursor = [System.Windows.Forms.Cursors]::Cross

$script:sx = 0; $script:sy = 0
$script:ex = 0; $script:ey = 0
$script:dragging = $false
$script:cancelled = $false

$form.Add_MouseDown({
    $script:sx = $_.X
    $script:sy = $_.Y
    $script:dragging = $true
})

$form.Add_MouseMove({
    if ($script:dragging) {
        $form.Refresh()
        $g2 = $form.CreateGraphics()
        $px = [Math]::Min($script:sx, $_.X)
        $py = [Math]::Min($script:sy, $_.Y)
        $pw = [Math]::Abs($_.X - $script:sx)
        $ph = [Math]::Abs($_.Y - $script:sy)
        $pen = New-Object System.Drawing.Pen([System.Drawing.Color]::Red, 2)
        $g2.DrawRectangle($pen, $px, $py, $pw, $ph)
        $pen.Dispose()
        $g2.Dispose()
    }
})

$form.Add_MouseUp({
    $script:ex = $_.X
    $script:ey = $_.Y
    $script:dragging = $false
    $form.Close()
})

$form.Add_KeyDown({
    if ($_.KeyCode -eq [System.Windows.Forms.Keys]::Escape) {
        $script:cancelled = $true
        $form.Close()
    }
})

[void]$form.ShowDialog()
$form.Dispose()
$bmp.Dispose()

if ($script:cancelled) { exit 1 }

$x = [Math]::Min($script:sx, $script:ex)
$y = [Math]::Min($script:sy, $script:ey)
$w = [Math]::Abs($script:ex - $script:sx)
$h = [Math]::Abs($script:ey - $script:sy)

if ($w -eq 0 -or $h -eq 0) { exit 1 }

# Output in absolute virtual-screen coordinates.
Write-Output "$($x + $vs.X),$($y + $vs.Y),$w,$h"
`
