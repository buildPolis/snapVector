//go:build darwin

package capture

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func NewPlatformCapturer() Capturer { return darwinCapturer{} }

type darwinCapturer struct{}

func (darwinCapturer) CaptureFullScreen(ctx context.Context) (PNG, Meta, error) {
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
