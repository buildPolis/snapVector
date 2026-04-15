//go:build windows

package exportdoc

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/signintech/gopdf"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// convertPNG renders the SVG to a PNG using the pure-Go oksvg + rasterx pipeline.
//
// Known limitation: <filter>/<feGaussianBlur> is not supported by oksvg, so
// blur annotations are silently omitted in the output.  All other annotation
// types (arrow, rectangle, ellipse, text) render correctly.
func convertPNG(_ context.Context, svg string) ([]byte, string, error) {
	raw, err := renderSVGtoPNG(svg)
	if err != nil {
		return nil, "", err
	}
	return raw, "image/png", nil
}

// convertJPG renders the SVG to PNG first, then re-encodes as JPEG (quality 92).
func convertJPG(ctx context.Context, svg string) ([]byte, string, error) {
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

// convertPDF renders the SVG to PNG and wraps it in a PDF page.
func convertPDF(ctx context.Context, svg string) ([]byte, string, error) {
	pngData, _, err := convertPNG(ctx, svg)
	if err != nil {
		return nil, "", fmt.Errorf("convert to png for pdf: %w", err)
	}

	// Determine image dimensions from the PNG header.
	cfg, err := png.DecodeConfig(bytes.NewReader(pngData))
	if err != nil {
		return nil, "", fmt.Errorf("decode png config for pdf: %w", err)
	}

	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: float64(cfg.Width), H: float64(cfg.Height)},
	})
	pdf.AddPage()

	imgHolder, err := gopdf.ImageHolderByBytes(pngData)
	if err != nil {
		return nil, "", fmt.Errorf("create pdf image holder: %w", err)
	}
	if err := pdf.ImageByHolder(imgHolder, 0, 0, &gopdf.Rect{W: float64(cfg.Width), H: float64(cfg.Height)}); err != nil {
		return nil, "", fmt.Errorf("embed image in pdf: %w", err)
	}

	return pdf.GetBytesPdf(), "application/pdf", nil
}

// renderSVGtoPNG parses the SVG string and rasterises it to a PNG byte slice.
func renderSVGtoPNG(svg string) ([]byte, error) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.WarnErrorMode)
	if err != nil {
		return nil, fmt.Errorf("parse svg: %w", err)
	}

	w := int(icon.ViewBox.W)
	h := int(icon.ViewBox.H)
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("svg has invalid viewBox dimensions: %dx%d", w, h)
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := rasterx.NewScannerGV(w, h, img, img.Bounds())
	raster := rasterx.NewDasher(w, h, scanner)

	icon.SetTarget(0, 0, float64(w), float64(h))
	icon.Draw(raster, 1.0)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}
