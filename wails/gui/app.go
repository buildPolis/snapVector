package gui

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"snapvector/annotation"
	"snapvector/capture"
	"snapvector/clipboarddoc"
	"snapvector/exportdoc"
	"snapvector/svgdoc"
)

type App struct {
	ctx             context.Context
	newCapturer     func() capture.Capturer
	convertExporter func(context.Context, string, string) ([]byte, string, error)
	writeClipboard  func(context.Context, []byte, string) error
}

type CaptureResult struct {
	Format        string         `json:"format"`
	MimeType      string         `json:"mimeType"`
	Base64        string         `json:"base64"`
	Display       map[string]any `json:"display,omitempty"`
	CaptureRegion map[string]any `json:"captureRegion,omitempty"`
}

type ExportResult struct {
	Format            string         `json:"format"`
	MimeType          string         `json:"mimeType"`
	Base64            string         `json:"base64,omitempty"`
	SVG               string         `json:"svg,omitempty"`
	AnnotationCount   int            `json:"annotationCount"`
	Canvas            map[string]any `json:"canvas"`
	CaptureRegion     map[string]any `json:"captureRegion,omitempty"`
	CopiedToClipboard bool           `json:"copiedToClipboard,omitempty"`
}

func NewApp() *App {
	return &App{
		newCapturer:     capture.NewPlatformCapturer,
		convertExporter: exportdoc.Convert,
		writeClipboard:  clipboarddoc.Write,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(context.Context) {}

func (a *App) CaptureScreen() (*CaptureResult, error) {
	return a.captureWith(a.newCapturer().CaptureFullScreen)
}

func (a *App) CaptureRegion() (*CaptureResult, error) {
	return a.captureWith(a.newCapturer().CaptureInteractiveRegion)
}

func (a *App) captureWith(run func(context.Context) (capture.PNG, capture.Meta, error)) (*CaptureResult, error) {
	ctx, cancel := a.captureContext()
	defer cancel()

	raw, meta, err := run(ctx)
	if err != nil {
		return nil, err
	}

	result := &CaptureResult{
		Format:   "png",
		MimeType: "image/png",
		Base64:   base64.StdEncoding.EncodeToString(raw),
	}
	if display := displayData(meta); len(display) > 0 {
		result.Display = display
	}
	if region := captureRegionData(meta); len(region) > 0 {
		result.CaptureRegion = region
	}

	return result, nil
}

func (a *App) ExportDocument(payload string, captureBase64 string, width int, height int, format string, copyToClipboard bool) (*ExportResult, error) {
	annotations, err := annotation.ParsePayload(payload)
	if err != nil {
		return nil, err
	}

	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "svg"
	}

	raw, err := base64.StdEncoding.DecodeString(captureBase64)
	if err != nil {
		return nil, fmt.Errorf("decode capture base64: %w", err)
	}

	svg, err := svgdoc.Compose(raw, width, height, annotations)
	if err != nil {
		return nil, err
	}

	result := &ExportResult{
		Format:          format,
		AnnotationCount: len(annotations),
		Canvas: map[string]any{
			"width":  width,
			"height": height,
		},
		CaptureRegion: map[string]any{
			"x":      0,
			"y":      0,
			"width":  width,
			"height": height,
		},
	}

	ctx, cancel := a.captureContext()
	defer cancel()

	if format == "svg" {
		result.MimeType = "image/svg+xml"
		result.SVG = svg
		if copyToClipboard {
			if err := a.writeClipboard(ctx, []byte(svg), "svg"); err != nil {
				return nil, err
			}
			result.CopiedToClipboard = true
		}
		return result, nil
	}

	converted, mimeType, err := a.convertExporter(ctx, svg, format)
	if err != nil {
		return nil, err
	}

	result.MimeType = mimeType
	result.Base64 = base64.StdEncoding.EncodeToString(converted)
	if copyToClipboard {
		if err := a.writeClipboard(ctx, converted, format); err != nil {
			return nil, err
		}
		result.CopiedToClipboard = true
	}

	return result, nil
}

func (a *App) captureContext() (context.Context, context.CancelFunc) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, 15*time.Second)
}

func displayData(meta capture.Meta) map[string]any {
	display := map[string]any{}
	if meta.DisplayID != "" {
		display["id"] = meta.DisplayID
	}
	if meta.Width > 0 {
		display["width"] = meta.Width
	}
	if meta.Height > 0 {
		display["height"] = meta.Height
	}
	if meta.X != 0 || meta.Y != 0 {
		display["x"] = meta.X
		display["y"] = meta.Y
	}
	if meta.ScaleFactor > 0 {
		display["scaleFactor"] = meta.ScaleFactor
	}
	return display
}

func captureRegionData(meta capture.Meta) map[string]any {
	if meta.Width <= 0 || meta.Height <= 0 {
		return nil
	}

	return map[string]any{
		"x":      meta.X,
		"y":      meta.Y,
		"width":  meta.Width,
		"height": meta.Height,
	}
}
