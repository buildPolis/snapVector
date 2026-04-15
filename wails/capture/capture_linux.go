//go:build linux

package capture

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

func NewPlatformCapturer() Capturer { return linuxCapturer{} }

type linuxCapturer struct{}

// CaptureFullScreen takes a screenshot of the entire screen (non-interactive).
func (linuxCapturer) CaptureFullScreen(ctx context.Context) (PNG, Meta, error) {
	return captureViaPortal(ctx, false)
}

// CaptureAllDisplays captures all connected displays.
// The XDG portal captures the full virtual screen, so this behaves
// identically to CaptureFullScreen on Linux.
func (linuxCapturer) CaptureAllDisplays(ctx context.Context) (PNG, Meta, error) {
	return captureViaPortal(ctx, false)
}

// CaptureInteractiveRegion lets the user select a screen region.
func (linuxCapturer) CaptureInteractiveRegion(ctx context.Context) (PNG, Meta, error) {
	return captureViaPortal(ctx, true)
}

// captureViaPortal uses org.freedesktop.portal.Screenshot over D-Bus.
// When the portal is unavailable it falls back to gnome-screenshot or grim.
func captureViaPortal(ctx context.Context, interactive bool) (PNG, Meta, error) {
	if shouldSkipPortal(os.Getenv("XDG_SESSION_TYPE"), hasGnomeScreenshot()) {
		log.Printf("snapvector capture: X11 session with gnome-screenshot available — skipping portal (GNOME 46 portal requires parent_window we don't plumb)")
		return fallbackScreenshot(ctx, interactive)
	}
	raw, meta, err := portalScreenshot(ctx, interactive)
	if err == nil {
		return raw, meta, nil
	}
	log.Printf("snapvector capture: portal screenshot failed: %v — trying fallback", err)
	return fallbackScreenshot(ctx, interactive)
}

// shouldSkipPortal returns true when the XDG Desktop Portal Screenshot call
// is going to be denied (or at best waste a round-trip) and a native tool is
// available to do the job directly. Today that means: X11 sessions whose
// portal (GNOME 46+) refuses requests without a parent_window handle that
// we don't currently plumb through from the Wails WebKit window.
func shouldSkipPortal(sessionType string, hasNativeTool bool) bool {
	return sessionType == "x11" && hasNativeTool
}

func hasGnomeScreenshot() bool {
	_, err := exec.LookPath("gnome-screenshot")
	return err == nil
}

// portalScreenshot calls org.freedesktop.portal.Screenshot.Screenshot via D-Bus.
func portalScreenshot(ctx context.Context, interactive bool) (PNG, Meta, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, Meta{}, fmt.Errorf("connect session bus: %w", err)
	}
	defer conn.Close()

	// Generate a unique token for matching the Response signal.
	token := fmt.Sprintf("snapvector_%d", time.Now().UnixNano())
	senderName := strings.Replace(conn.Names()[0], ".", "_", -1)
	senderName = strings.TrimPrefix(senderName, ":")
	responsePath := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + senderName + "/" + token)

	// Subscribe to the Response signal before making the call.
	sigCh := make(chan *dbus.Signal, 1)
	conn.Signal(sigCh)
	matchRule := fmt.Sprintf("type='signal',interface='org.freedesktop.portal.Request',member='Response',path='%s'", responsePath)
	if err := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule).Err; err != nil {
		return nil, Meta{}, fmt.Errorf("add match rule: %w", err)
	}
	defer conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, matchRule)

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	opts := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"interactive":  dbus.MakeVariant(interactive),
	}

	call := obj.CallWithContext(ctx, "org.freedesktop.portal.Screenshot.Screenshot", 0, "", opts)
	if call.Err != nil {
		return nil, Meta{}, fmt.Errorf("portal Screenshot call: %w", call.Err)
	}

	// Wait for the Response signal with a timeout.
	// The channel may receive unrelated D-Bus signals (e.g. NameAcquired);
	// loop until we get the one matching our request path.
	for {
		select {
		case sig := <-sigCh:
			if sig.Path != responsePath {
				continue
			}
			return handlePortalResponse(sig)
		case <-ctx.Done():
			return nil, Meta{}, fmt.Errorf("portal screenshot timed out: %w", ctx.Err())
		}
	}
}

func handlePortalResponse(sig *dbus.Signal) (PNG, Meta, error) {
	if len(sig.Body) < 2 {
		return nil, Meta{}, fmt.Errorf("portal response has %d args, expected ≥2", len(sig.Body))
	}

	response, ok := sig.Body[0].(uint32)
	if !ok {
		return nil, Meta{}, fmt.Errorf("portal response code is not uint32")
	}
	if response == 1 {
		return nil, Meta{}, fmt.Errorf("interactive capture cancelled")
	}
	if response == 2 {
		return nil, Meta{}, &PermissionDeniedError{Platform: "linux", Stderr: "portal returned permission denied (response=2)"}
	}
	if response != 0 {
		return nil, Meta{}, fmt.Errorf("portal returned error response: %d", response)
	}

	results, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return nil, Meta{}, fmt.Errorf("portal results is not a map")
	}

	uriVariant, ok := results["uri"]
	if !ok {
		return nil, Meta{}, fmt.Errorf("portal results missing 'uri' key")
	}
	uri, ok := uriVariant.Value().(string)
	if !ok {
		return nil, Meta{}, fmt.Errorf("portal uri is not a string")
	}

	return readScreenshotURI(uri)
}

// readScreenshotURI reads a file:// URI, decodes PNG dimensions, and returns the data.
func readScreenshotURI(uri string) (PNG, Meta, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("parse screenshot uri: %w", err)
	}
	path := parsed.Path
	if path == "" {
		return nil, Meta{}, fmt.Errorf("empty path in screenshot uri: %s", uri)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("read screenshot file: %w", err)
	}
	if len(raw) == 0 {
		return nil, Meta{}, fmt.Errorf("screenshot file is empty: %s", path)
	}

	// Portal may return JPEG or PNG; convert to PNG if needed.
	if !bytes.HasPrefix(raw, []byte{0x89, 'P', 'N', 'G'}) {
		raw, err = convertToPNG(raw)
		if err != nil {
			return nil, Meta{}, fmt.Errorf("convert screenshot to png: %w", err)
		}
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, Meta{}, fmt.Errorf("decode screenshot png config: %w", err)
	}

	scaleFactor := getScaleFactor()

	// Clean up the temp file created by the portal.
	os.Remove(path)

	return PNG(raw), Meta{
		Width:       cfg.Width,
		Height:      cfg.Height,
		ScaleFactor: scaleFactor,
	}, nil
}

// convertToPNG re-encodes image data (e.g. JPEG from portal) as PNG.
func convertToPNG(data []byte) ([]byte, error) {
	img, _, err := decodeAnyImage(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fallbackScreenshot tries gnome-screenshot or grim when the portal is unavailable.
func fallbackScreenshot(ctx context.Context, interactive bool) (PNG, Meta, error) {
	tmpDir, err := os.MkdirTemp("", "snapvector-capture-*")
	if err != nil {
		return nil, Meta{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	outPath := filepath.Join(tmpDir, "capture.png")

	// Try gnome-screenshot first (X11 / XWayland).
	if path, err := exec.LookPath("gnome-screenshot"); err == nil {
		args := []string{"--file=" + outPath}
		if interactive {
			args = append(args, "--area")
		}
		started := time.Now()
		cmd := exec.CommandContext(ctx, path, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		elapsed := time.Since(started)
		log.Printf("snapvector capture: gnome-screenshot %v -> elapsed=%s stderr=%q err=%v",
			args, elapsed.Round(time.Millisecond), strings.TrimSpace(stderr.String()), runErr)
		if runErr != nil {
			return nil, Meta{}, classifyLinuxError(runErr, stderr.String())
		}
		return readPNGFile(outPath)
	}

	// Try grim (Wayland).
	if path, err := exec.LookPath("grim"); err == nil {
		args := []string{outPath}
		if interactive {
			// grim + slurp for region selection
			if slurpPath, slurpErr := exec.LookPath("slurp"); slurpErr == nil {
				slurpCmd := exec.CommandContext(ctx, slurpPath)
				slurpOut, slurpRunErr := slurpCmd.Output()
				if slurpRunErr != nil {
					return nil, Meta{}, fmt.Errorf("slurp region select failed: %w", slurpRunErr)
				}
				geometry := strings.TrimSpace(string(slurpOut))
				args = []string{"-g", geometry, outPath}
			}
		}
		cmd := exec.CommandContext(ctx, path, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if runErr := cmd.Run(); runErr != nil {
			return nil, Meta{}, classifyLinuxError(runErr, stderr.String())
		}
		return readPNGFile(outPath)
	}

	return nil, Meta{}, &UnsupportedPlatformError{
		Platform: "linux (no screenshot tool found: install gnome-screenshot or grim)",
	}
}

func readPNGFile(path string) (PNG, Meta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("read capture file: %w", err)
	}
	if len(raw) == 0 {
		return nil, Meta{}, fmt.Errorf("capture file is empty")
	}

	// Fallback tools may produce PNG or other formats.
	if !bytes.HasPrefix(raw, []byte{0x89, 'P', 'N', 'G'}) {
		var convErr error
		raw, convErr = convertToPNG(raw)
		if convErr != nil {
			return nil, Meta{}, fmt.Errorf("convert capture to png: %w", convErr)
		}
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, Meta{}, fmt.Errorf("decode capture png: %w", err)
	}

	return PNG(raw), Meta{
		Width:       cfg.Width,
		Height:      cfg.Height,
		ScaleFactor: getScaleFactor(),
	}, nil
}

func classifyLinuxError(runErr error, stderr string) error {
	lowered := strings.ToLower(stderr)
	if strings.Contains(lowered, "permission") || strings.Contains(lowered, "denied") ||
		strings.Contains(lowered, "not authorized") {
		return &PermissionDeniedError{Platform: "linux", Stderr: strings.TrimSpace(stderr)}
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr) == "" {
		return fmt.Errorf("interactive capture cancelled")
	}
	return fmt.Errorf("screenshot failed: %w (stderr=%q)", runErr, strings.TrimSpace(stderr))
}

// getScaleFactor reads GDK_SCALE or defaults to 1.0.
func getScaleFactor() float64 {
	if s := os.Getenv("GDK_SCALE"); s != "" {
		if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 {
			return f
		}
	}
	return 1.0
}
