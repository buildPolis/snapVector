//go:build darwin

package exportdoc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func convertPNG(ctx context.Context, svg string) ([]byte, string, error) {
	return convertWithSIPS(ctx, svg, "png", "image/png")
}

func convertJPG(ctx context.Context, svg string) ([]byte, string, error) {
	return convertWithSIPS(ctx, svg, "jpeg", "image/jpeg")
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

func convertWithSIPS(ctx context.Context, svg string, targetFormat string, mimeType string) ([]byte, string, error) {
	tempDir, err := os.MkdirTemp("", "snapvector-export-*")
	if err != nil {
		return nil, "", fmt.Errorf("create export temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "inject.svg")
	outputPath := filepath.Join(tempDir, "inject."+targetFormat)
	if err := os.WriteFile(inputPath, []byte(svg), 0o600); err != nil {
		return nil, "", fmt.Errorf("write temp svg: %w", err)
	}

	cmd := exec.CommandContext(ctx, "sips", "-s", "format", targetFormat, inputPath, "--out", outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("convert svg via sips: %w (%s)", err, string(output))
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, "", fmt.Errorf("read exported asset: %w", err)
	}
	return raw, mimeType, nil
}
