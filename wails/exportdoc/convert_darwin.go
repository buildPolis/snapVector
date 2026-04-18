//go:build darwin

package exportdoc

import (
	"bytes"
	_ "embed"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed bin/resvg
var resvgBinary []byte

var (
	resvgOnce sync.Once
	resvgPath string
	resvgErr  error
)

func ensureResvg() (string, error) {
	resvgOnce.Do(func() {
		dir, err := os.MkdirTemp("", "snapvector-resvg-*")
		if err != nil {
			resvgErr = fmt.Errorf("create resvg temp dir: %w", err)
			return
		}
		p := filepath.Join(dir, "resvg")
		if err := os.WriteFile(p, resvgBinary, 0o755); err != nil {
			resvgErr = fmt.Errorf("extract resvg binary: %w", err)
			return
		}
		resvgPath = p
	})
	return resvgPath, resvgErr
}

func convertPNG(ctx context.Context, svg string) ([]byte, string, error) {
	raw, err := convertWithResvg(ctx, svg)
	if err != nil {
		return nil, "", err
	}
	return raw, "image/png", nil
}

func convertJPG(ctx context.Context, svg string) ([]byte, string, error) {
	pngData, err := convertWithResvg(ctx, svg)
	if err != nil {
		return nil, "", fmt.Errorf("convert svg to png for jpg: %w", err)
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
	tempDir, err := os.MkdirTemp("", "snapvector-export-*")
	if err != nil {
		return nil, "", fmt.Errorf("create export temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "inject.svg")
	outputPath := filepath.Join(tempDir, "inject.pdf")
	if err := os.WriteFile(inputPath, []byte(svg), 0o600); err != nil {
		return nil, "", fmt.Errorf("write temp svg: %w", err)
	}

	cmd := exec.CommandContext(ctx, "cupsfilter", "-m", "application/pdf", inputPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("convert svg to pdf: %w", err)
	}
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		return nil, "", fmt.Errorf("write temp pdf: %w", err)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, "", fmt.Errorf("read exported pdf: %w", err)
	}
	return raw, "application/pdf", nil
}

func convertWithResvg(ctx context.Context, svg string) ([]byte, error) {
	bin, err := ensureResvg()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin, "-", "-c")
	cmd.Stdin = bytes.NewReader([]byte(svg))
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("resvg render: %w (stderr=%q)", err, stderr.String())
	}
	return out.Bytes(), nil
}
