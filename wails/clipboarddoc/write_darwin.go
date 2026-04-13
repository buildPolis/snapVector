//go:build darwin

package clipboarddoc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	scriptPath := filepath.Join(tempDir, "write_clipboard.swift")
	if err := os.WriteFile(scriptPath, []byte(swiftClipboardWriter), 0o600); err != nil {
		return fmt.Errorf("write clipboard script: %w", err)
	}

	cmd := exec.CommandContext(ctx, "swift", scriptPath, dataPath, pasteboardType(format))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write clipboard via swift: %w (%s)", err, string(output))
	}

	return nil
}

func pasteboardType(format string) string {
	switch format {
	case "svg":
		return "public.svg-image"
	case "png":
		return "public.png"
	case "jpg":
		return "public.jpeg"
	case "pdf":
		return "com.adobe.pdf"
	default:
		return "public.data"
	}
}

const swiftClipboardWriter = `import AppKit
import Foundation

let path = CommandLine.arguments[1]
let type = CommandLine.arguments[2]
let data = try Data(contentsOf: URL(fileURLWithPath: path))
let board = NSPasteboard.general
board.clearContents()
guard board.setData(data, forType: NSPasteboard.PasteboardType(type)) else {
  fputs("failed to write pasteboard\n", stderr)
  exit(1)
}
`
