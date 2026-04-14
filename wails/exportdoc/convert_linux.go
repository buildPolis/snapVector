//go:build linux

package exportdoc

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
)

func convertPNG(ctx context.Context, svg string) ([]byte, string, error) {
	rsvg, err := exec.LookPath("rsvg-convert")
	if err != nil {
		return nil, "", fmt.Errorf("rsvg-convert not found: install librsvg2-bin (apt install librsvg2-bin)")
	}

	tmpDir, err := os.MkdirTemp("", "snapvector-export-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.svg")
	outPath := filepath.Join(tmpDir, "output.png")

	if err := os.WriteFile(inPath, []byte(svg), 0o600); err != nil {
		return nil, "", fmt.Errorf("write svg: %w", err)
	}

	cmd := exec.CommandContext(ctx, rsvg, "-f", "png", "-o", outPath, inPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("rsvg-convert png: %w (stderr=%q)", err, stderr.String())
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		return nil, "", fmt.Errorf("read png output: %w", err)
	}
	return raw, "image/png", nil
}

func convertJPG(ctx context.Context, svg string) ([]byte, string, error) {
	// rsvg-convert does not output JPEG directly; convert via PNG → Go JPEG encoder.
	pngData, _, err := convertPNG(ctx, svg)
	if err != nil {
		return nil, "", fmt.Errorf("convert to png for jpg: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, "", fmt.Errorf("decode png for jpg conversion: %w", err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		return nil, "", fmt.Errorf("encode jpg: %w", err)
	}
	return buf.Bytes(), "image/jpeg", nil
}

func convertPDF(ctx context.Context, svg string) ([]byte, string, error) {
	rsvg, err := exec.LookPath("rsvg-convert")
	if err != nil {
		return nil, "", fmt.Errorf("rsvg-convert not found: install librsvg2-bin (apt install librsvg2-bin)")
	}

	tmpDir, err := os.MkdirTemp("", "snapvector-export-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.svg")
	outPath := filepath.Join(tmpDir, "output.pdf")

	if err := os.WriteFile(inPath, []byte(svg), 0o600); err != nil {
		return nil, "", fmt.Errorf("write svg: %w", err)
	}

	cmd := exec.CommandContext(ctx, rsvg, "-f", "pdf", "-o", outPath, inPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("rsvg-convert pdf: %w (stderr=%q)", err, stderr.String())
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		return nil, "", fmt.Errorf("read pdf output: %w", err)
	}
	return raw, "application/pdf", nil
}
