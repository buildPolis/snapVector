package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"time"

	"snapvector/annotation"
	"snapvector/capture"
	"snapvector/clipboarddoc"
	"snapvector/exportdoc"
	"snapvector/svgdoc"
)

const versionString = "0.1.0-phase1"

var newCapturer = capture.NewPlatformCapturer
var convertSVG = exportdoc.Convert
var writeClipboard = clipboarddoc.Write

func Run(args []string, stdout, stderr io.Writer) int {
	_ = stderr

	f, err := parseFlags(args)
	if err != nil {
		_ = WriteError(stdout, CodeUsage, err.Error(), false, nil)
		return 1
	}

	switch {
	case f.Help:
		_ = WriteOK(stdout, map[string]any{"help": helpText()})
		return 0
	case f.Version:
		_ = WriteOK(stdout, map[string]any{"version": versionString})
		return 0
	case f.Capture && f.Base64Stdout:
		return runCapture(stdout)
	case f.InjectSVG != "":
		return runInjectSVG(stdout, f.InjectSVG, f.OutputFormat, f.CopyToClipboard)
	default:
		_ = WriteError(stdout, CodeUsage, "no command given; try --help", false, nil)
		return 1
	}
}

func runCapture(stdout io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	capturer := newCapturer()
	raw, meta, err := capturer.CaptureFullScreen(ctx)
	if err != nil {
		return writeCaptureError(stdout, err)
	}

	data := map[string]any{
		"format":   "png",
		"mimeType": "image/png",
		"base64":   base64.StdEncoding.EncodeToString(raw),
	}

	display := displayData(meta)
	if len(display) > 0 {
		data["display"] = display
	}
	if region := captureRegionData(meta); len(region) > 0 {
		data["captureRegion"] = region
	}

	_ = WriteOK(stdout, data)
	return 0
}

func runInjectSVG(stdout io.Writer, payload string, outputFormat string, copyToClipboard bool) int {
	annotations, err := annotation.ParsePayload(payload)
	if err != nil {
		_ = WriteError(stdout, CodeInjectInvalid, err.Error(), false, nil)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	capturer := newCapturer()
	raw, meta, err := capturer.CaptureFullScreen(ctx)
	if err != nil {
		return writeCaptureError(stdout, err)
	}

	svg, err := svgdoc.Compose(raw, meta.Width, meta.Height, annotations)
	if err != nil {
		_ = WriteError(stdout, CodeExportFailed, err.Error(), false, nil)
		return 1
	}

	data := map[string]any{
		"annotationCount": len(annotations),
		"canvas": map[string]any{
			"width":  meta.Width,
			"height": meta.Height,
		},
	}
	if region := captureRegionData(meta); len(region) > 0 {
		data["captureRegion"] = region
	}

	if outputFormat == "svg" {
		data["format"] = "svg"
		data["mimeType"] = "image/svg+xml"
		data["svg"] = svg
		if copyToClipboard {
			if err := writeClipboard(ctx, []byte(svg), "svg"); err != nil {
				_ = WriteError(stdout, CodeExportFailed, err.Error(), false, nil)
				return 1
			}
			data["copiedToClipboard"] = true
		}
		_ = WriteOK(stdout, data)
		return 0
	}

	converted, mimeType, err := convertSVG(ctx, svg, outputFormat)
	if err != nil {
		_ = WriteError(stdout, CodeExportFailed, err.Error(), false, nil)
		return 1
	}
	data["format"] = outputFormat
	data["mimeType"] = mimeType
	data["base64"] = base64.StdEncoding.EncodeToString(converted)
	if copyToClipboard {
		if err := writeClipboard(ctx, converted, outputFormat); err != nil {
			_ = WriteError(stdout, CodeExportFailed, err.Error(), false, nil)
			return 1
		}
		data["copiedToClipboard"] = true
	}

	_ = WriteOK(stdout, data)
	return 0
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

	region := map[string]any{
		"x":      meta.X,
		"y":      meta.Y,
		"width":  meta.Width,
		"height": meta.Height,
	}
	return region
}

func writeCaptureError(stdout io.Writer, err error) int {
	var permissionErr *capture.PermissionDeniedError
	if errors.As(err, &permissionErr) {
		_ = WriteError(stdout, CodePermissionDenied, permissionErr.Error(), true, map[string]any{
			"platform": permissionErr.Platform,
			"stderr":   permissionErr.Stderr,
		})
		return 1
	}

	var unsupportedErr *capture.UnsupportedPlatformError
	if errors.As(err, &unsupportedErr) {
		_ = WriteError(stdout, CodeUnsupportedPlatform, unsupportedErr.Error(), false, map[string]any{
			"platform": unsupportedErr.Platform,
		})
		return 1
	}

	_ = WriteError(stdout, CodeCaptureFailed, err.Error(), false, nil)
	return 1
}
