//go:build windows

package clipboarddoc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func writePlatform(ctx context.Context, payload []byte, format string) error {
	tempDir, err := os.MkdirTemp("", "snapvector-clipboard-*")
	if err != nil {
		return fmt.Errorf("create clipboard temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	dataPath := filepath.Join(tempDir, "payload")
	if err := os.WriteFile(dataPath, payload, 0o600); err != nil {
		return fmt.Errorf("write clipboard payload: %w", err)
	}

	// Build a self-contained PowerShell script that reads the temp file and
	// puts it on the clipboard using System.Windows.Forms.
	script := buildPSClipboardScript(dataPath, format)

	cmd := exec.CommandContext(ctx,
		"powershell.exe", "-STA", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", script,
	)
	// Suppress the console window that would otherwise flash when spawning
	// powershell.exe from a GUI (Wails) process.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clipboard write via powershell: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// buildPSClipboardScript returns a PowerShell one-liner that reads dataPath
// and copies it to the clipboard in the appropriate format.
func buildPSClipboardScript(dataPath, format string) string {
	// Escape the path for use inside a PowerShell double-quoted string.
	escaped := strings.ReplaceAll(dataPath, `\`, `\\`)

	switch format {
	case "png", "jpg":
		// Use System.Drawing.Image so Windows clipboard stores a proper bitmap
		// that any app can paste.
		return fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$img = [System.Drawing.Image]::FromFile("%s")
[System.Windows.Forms.Clipboard]::SetImage($img)
$img.Dispose()
`, escaped)

	case "svg":
		return buildPSRawClipboardScript(escaped, "image/svg+xml")

	case "pdf":
		return buildPSRawClipboardScript(escaped, "application/pdf")

	default:
		return buildPSRawClipboardScript(escaped, "application/octet-stream")
	}
}

// buildPSRawClipboardScript puts raw bytes onto the clipboard under a custom
// format name (MIME type string).  The data is stored as a MemoryStream so
// other applications that understand the format can retrieve it.
func buildPSRawClipboardScript(escapedPath, mimeType string) string {
	return fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$bytes = [System.IO.File]::ReadAllBytes("%s")
$ms = New-Object System.IO.MemoryStream(,$bytes)
[System.Windows.Forms.Clipboard]::SetData("%s", $ms)
$ms.Dispose()
`, escapedPath, mimeType)
}
