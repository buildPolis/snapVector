package gui

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"snapvector/annotation"
	"snapvector/capture"
	"snapvector/svgdoc"
)

func TestExportDocumentSVGMatchesSharedComposerAndKeepsCJKText(t *testing.T) {
	app := NewApp()
	rawPNG := mustPNG(t)
	payload := `[{"id":"text-1","type":"text","x":24,"y":32,"text":"繁體中文測試","variant":"outline","fontSize":28,"maxWidth":260}]`

	result, err := app.ExportDocument(payload, base64.StdEncoding.EncodeToString(rawPNG), 160, 120, "svg", false)
	if err != nil {
		t.Fatalf("ExportDocument returned error: %v", err)
	}

	annotations, err := annotation.ParsePayload(payload)
	if err != nil {
		t.Fatalf("ParsePayload returned error: %v", err)
	}
	expectedSVG, err := svgdoc.Compose(rawPNG, 160, 120, annotations)
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}

	if result.Format != "svg" {
		t.Fatalf("format = %q, want svg", result.Format)
	}
	if result.MimeType != "image/svg+xml" {
		t.Fatalf("mimeType = %q, want image/svg+xml", result.MimeType)
	}
	if result.AnnotationCount != 1 {
		t.Fatalf("annotationCount = %d, want 1", result.AnnotationCount)
	}
	if result.SVG != expectedSVG {
		t.Fatalf("SVG output mismatch between GUI export path and shared composer")
	}
	if !bytes.Contains([]byte(result.SVG), []byte("繁體中文測試")) {
		t.Fatalf("expected CJK text in SVG output")
	}
}

func TestExportDocumentPNGCopiesConvertedBytes(t *testing.T) {
	app := NewApp()
	rawPNG := mustPNG(t)
	payload := `[{"id":"rect-1","type":"rectangle","x":12,"y":18,"width":80,"height":44}]`

	var gotClipboard []byte
	var gotClipboardFormat string
	var gotConvertFormat string

	app.convertExporter = func(_ context.Context, svg string, format string) ([]byte, string, error) {
		if svg == "" {
			t.Fatal("expected non-empty svg input")
		}
		gotConvertFormat = format
		return []byte("png-bytes"), "image/png", nil
	}
	app.writeClipboard = func(_ context.Context, raw []byte, format string) error {
		gotClipboard = append([]byte(nil), raw...)
		gotClipboardFormat = format
		return nil
	}

	result, err := app.ExportDocument(payload, base64.StdEncoding.EncodeToString(rawPNG), 160, 120, "png", true)
	if err != nil {
		t.Fatalf("ExportDocument returned error: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(result.Base64)
	if err != nil {
		t.Fatalf("Base64 decode returned error: %v", err)
	}

	if gotConvertFormat != "png" {
		t.Fatalf("convert format = %q, want png", gotConvertFormat)
	}
	if gotClipboardFormat != "png" {
		t.Fatalf("clipboard format = %q, want png", gotClipboardFormat)
	}
	if !bytes.Equal(decoded, []byte("png-bytes")) {
		t.Fatalf("decoded base64 = %q, want png-bytes", string(decoded))
	}
	if !bytes.Equal(gotClipboard, []byte("png-bytes")) {
		t.Fatalf("clipboard bytes = %q, want png-bytes", string(gotClipboard))
	}
	if !result.CopiedToClipboard {
		t.Fatal("expected CopiedToClipboard to be true")
	}
}

func TestCaptureRegionUsesCapturerMetadata(t *testing.T) {
	rawPNG := mustPNG(t)
	app := NewApp()
	app.newCapturer = func() capture.Capturer {
		return fakeCapturer{
			fullPNG: rawPNG,
			fullMeta: capture.Meta{
				DisplayID: "display-1",
				Width:     160,
				Height:    120,
			},
			regionPNG: rawPNG,
			regionMeta: capture.Meta{
				X:      18,
				Y:      24,
				Width:  96,
				Height: 72,
			},
		}
	}

	result, err := app.CaptureRegion()
	if err != nil {
		t.Fatalf("CaptureRegion returned error: %v", err)
	}

	if result.Format != "png" || result.MimeType != "image/png" {
		t.Fatalf("unexpected result format: %+v", result)
	}
	if result.CaptureRegion["x"] != 18 || result.CaptureRegion["y"] != 24 {
		t.Fatalf("captureRegion = %+v, want x=18 y=24", result.CaptureRegion)
	}
	if result.CaptureRegion["width"] != 96 || result.CaptureRegion["height"] != 72 {
		t.Fatalf("captureRegion = %+v, want 96x72", result.CaptureRegion)
	}
	if result.Display["width"] != 96 || result.Display["height"] != 72 {
		t.Fatalf("display = %+v, want 96x72", result.Display)
	}
	if result.Base64 == "" {
		t.Fatal("expected base64 capture data")
	}
}

type fakeCapturer struct {
	fullPNG    capture.PNG
	fullMeta   capture.Meta
	fullErr    error
	regionPNG  capture.PNG
	regionMeta capture.Meta
	regionErr  error
}

func (f fakeCapturer) CaptureFullScreen(context.Context) (capture.PNG, capture.Meta, error) {
	return f.fullPNG, f.fullMeta, f.fullErr
}

func (f fakeCapturer) CaptureInteractiveRegion(context.Context) (capture.PNG, capture.Meta, error) {
	return f.regionPNG, f.regionMeta, f.regionErr
}

func mustPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}
