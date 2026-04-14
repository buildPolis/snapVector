//go:build linux

package clipboarddoc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func writePlatform(ctx context.Context, payload []byte, format string) error {
	mimeType := formatToMIME(format)
	if mimeType == "" {
		return fmt.Errorf("unsupported clipboard format: %s", format)
	}

	// Wayland: use wl-copy if available and session is Wayland.
	if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		if wlCopy, err := exec.LookPath("wl-copy"); err == nil {
			return writeViaWlCopy(ctx, wlCopy, payload, mimeType)
		}
	}

	// X11: use xclip.
	if xclip, err := exec.LookPath("xclip"); err == nil {
		return writeViaXclip(ctx, xclip, payload, mimeType)
	}

	// Wayland fallback if not detected above but wl-copy exists.
	if wlCopy, err := exec.LookPath("wl-copy"); err == nil {
		return writeViaWlCopy(ctx, wlCopy, payload, mimeType)
	}

	return fmt.Errorf("no clipboard tool found: install xclip (X11) or wl-clipboard (Wayland)")
}

func writeViaXclip(ctx context.Context, xclipPath string, payload []byte, mimeType string) error {
	cmd := exec.CommandContext(ctx, xclipPath, "-selection", "clipboard", "-t", mimeType)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("xclip stdin pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("xclip start: %w", err)
	}

	if _, err := stdin.Write(payload); err != nil {
		return fmt.Errorf("xclip write: %w", err)
	}
	stdin.Close()

	// xclip forks to retain X selection ownership; the parent exits quickly
	// but Go's exec.Cmd.Wait may block if the forked child inherits fds.
	// Use a goroutine with timeout so we don't block indefinitely.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("xclip: %w (stderr=%q)", err, stderr.String())
		}
		return nil
	case <-time.After(3 * time.Second):
		// xclip forked successfully — the data is on the clipboard.
		return nil
	case <-ctx.Done():
		return fmt.Errorf("xclip timed out: %w", ctx.Err())
	}
}

func writeViaWlCopy(ctx context.Context, wlCopyPath string, payload []byte, mimeType string) error {
	cmd := exec.CommandContext(ctx, wlCopyPath, "--type", mimeType)
	cmd.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wl-copy: %w (stderr=%q)", err, stderr.String())
	}
	return nil
}

func formatToMIME(format string) string {
	switch format {
	case "svg":
		return "image/svg+xml"
	case "png":
		return "image/png"
	case "jpg":
		return "image/jpeg"
	case "pdf":
		return "application/pdf"
	default:
		return ""
	}
}
