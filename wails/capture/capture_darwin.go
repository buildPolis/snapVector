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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func NewPlatformCapturer() Capturer { return darwinCapturer{} }

type darwinCapturer struct{}

func (darwinCapturer) CaptureFullScreen(ctx context.Context) (PNG, Meta, error) {
	displays, err := listDarwinDisplays(ctx)
	if err == nil && len(displays) > 1 {
		return captureAllDisplays(ctx, displays)
	}
	return captureWithArgs(ctx, fullScreenCaptureArgs()...)
}

func (darwinCapturer) CaptureInteractiveRegion(ctx context.Context) (PNG, Meta, error) {
	return captureWithArgs(ctx, interactiveRegionCaptureArgs()...)
}

func fullScreenCaptureArgs() []string {
	return []string{"-x", "-t", "png"}
}

func interactiveRegionCaptureArgs() []string {
	return []string{"-i", "-x", "-t", "png"}
}

type darwinDisplay struct {
	Index       int     `json:"index"`
	X           int     `json:"x"`
	Y           int     `json:"y"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	ScaleFactor float64 `json:"scaleFactor"`
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

	if err := cmd.Run(); err != nil {
		return nil, Meta{}, classifyDarwinError(err, stderr.String())
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

func listDarwinDisplays(ctx context.Context) ([]darwinDisplay, error) {
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

	var displays []darwinDisplay
	if err := json.Unmarshal(output, &displays); err != nil {
		return nil, fmt.Errorf("decode display probe output: %w (%s)", err, string(output))
	}
	if len(displays) == 0 {
		return nil, fmt.Errorf("display probe returned no active displays")
	}

	return displays, nil
}

const swiftDisplayProbe = `import AppKit
import Foundation

let screens = NSScreen.screens.enumerated().map { index, screen -> [String: Any] in
  let frame = screen.frame
  let backing = screen.convertRectToBacking(frame)
  return [
    "index": index + 1,
    "x": Int(backing.origin.x.rounded()),
    "y": Int(backing.origin.y.rounded()),
    "width": Int(backing.size.width.rounded()),
    "height": Int(backing.size.height.rounded()),
    "scaleFactor": screen.backingScaleFactor,
  ]
}

let data = try JSONSerialization.data(withJSONObject: screens, options: [])
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
