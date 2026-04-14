package gui

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

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

func TestCaptureRegionUsesInteractiveMetadata(t *testing.T) {
	rawPNG := mustPNG(t)
	app := NewApp()
	app.newCapturer = func() capture.Capturer {
		return fakeCapturer{
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

func TestCaptureAllDisplaysUsesComposedMetadata(t *testing.T) {
	rawPNG := mustPNG(t)
	app := NewApp()
	app.newCapturer = func() capture.Capturer {
		return fakeCapturer{
			allPNG: rawPNG,
			allMeta: capture.Meta{
				DisplayID: "all",
				X:         -300,
				Y:         0,
				Width:     3840,
				Height:    4394,
			},
		}
	}

	result, err := app.CaptureAllDisplays()
	if err != nil {
		t.Fatalf("CaptureAllDisplays returned error: %v", err)
	}

	if result.Display["id"] != "all" {
		t.Fatalf("display id = %+v, want all", result.Display)
	}
	if result.CaptureRegion["x"] != -300 || result.CaptureRegion["y"] != 0 {
		t.Fatalf("captureRegion = %+v, want x=-300 y=0", result.CaptureRegion)
	}
	if result.CaptureRegion["width"] != 3840 || result.CaptureRegion["height"] != 4394 {
		t.Fatalf("captureRegion = %+v, want 3840x4394", result.CaptureRegion)
	}
	if result.Display["width"] != 3840 || result.Display["height"] != 4394 {
		t.Fatalf("display = %+v, want 3840x4394", result.Display)
	}
}

func TestCaptureScreenUsesDisplayUnderCursorMetadata(t *testing.T) {
	rawPNG := mustPNG(t)
	app := NewApp()
	app.newCapturer = func() capture.Capturer {
		return fakeCapturer{
			fullPNG: rawPNG,
			fullMeta: capture.Meta{
				DisplayID:   "2",
				X:           -300,
				Y:           2234,
				Width:       3840,
				Height:      2160,
				ScaleFactor: 2,
			},
		}
	}

	result, err := app.CaptureScreen()
	if err != nil {
		t.Fatalf("CaptureScreen returned error: %v", err)
	}

	if result.Display["id"] != "2" {
		t.Fatalf("display id = %+v, want 2", result.Display)
	}
	if result.Display["x"] != -300 || result.Display["y"] != 2234 {
		t.Fatalf("display = %+v, want x=-300 y=2234", result.Display)
	}
	if result.CaptureRegion["width"] != 3840 || result.CaptureRegion["height"] != 2160 {
		t.Fatalf("captureRegion = %+v, want 3840x2160", result.CaptureRegion)
	}
}

func TestOpenDocumentUsesDialogAndReadsContents(t *testing.T) {
	app := NewApp()
	app.openFileDialog = func(_ context.Context, opts wailsruntime.OpenDialogOptions) (string, error) {
		if opts.Title == "" || len(opts.Filters) == 0 {
			t.Fatalf("unexpected dialog options: %+v", opts)
		}
		return "/tmp/example.sv.json", nil
	}
	app.readFile = func(path string) ([]byte, error) {
		if path != "/tmp/example.sv.json" {
			t.Fatalf("read path = %q", path)
		}
		return []byte(`{"kind":"snapvector-document","version":1}`), nil
	}

	result, err := app.OpenDocument()
	if err != nil {
		t.Fatalf("OpenDocument returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected document result")
	}
	if result.Path != "/tmp/example.sv.json" || result.Name != "example.sv.json" {
		t.Fatalf("unexpected open result: %+v", result)
	}
	if result.Contents != `{"kind":"snapvector-document","version":1}` {
		t.Fatalf("contents = %q", result.Contents)
	}
}

func TestSaveDocumentWritesToExistingPath(t *testing.T) {
	app := NewApp()
	var gotPath string
	var gotContents []byte
	var gotMode os.FileMode
	app.writeFile = func(path string, data []byte, mode os.FileMode) error {
		gotPath = path
		gotContents = append([]byte(nil), data...)
		gotMode = mode
		return nil
	}

	result, err := app.SaveDocument("/tmp/existing.sv.json", `{"annotations":[]}`)
	if err != nil {
		t.Fatalf("SaveDocument returned error: %v", err)
	}
	if result.Path != "/tmp/existing.sv.json" || result.Name != "existing.sv.json" {
		t.Fatalf("unexpected save result: %+v", result)
	}
	if gotPath != "/tmp/existing.sv.json" {
		t.Fatalf("write path = %q", gotPath)
	}
	if string(gotContents) != `{"annotations":[]}` {
		t.Fatalf("write contents = %q", string(gotContents))
	}
	if gotMode != 0o600 {
		t.Fatalf("write mode = %v, want 0600", gotMode)
	}
}

func TestSaveDocumentAsPromptsAndAppendsExtension(t *testing.T) {
	app := NewApp()
	app.saveFileDialog = func(_ context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
		if opts.DefaultFilename != "capture-01.sv.json" {
			t.Fatalf("default filename = %q", opts.DefaultFilename)
		}
		return "/tmp/capture-01", nil
	}

	var gotPath string
	app.writeFile = func(path string, data []byte, mode os.FileMode) error {
		gotPath = path
		return nil
	}

	result, err := app.SaveDocumentAs("capture-01", `{"annotations":[]}`)
	if err != nil {
		t.Fatalf("SaveDocumentAs returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected save-as result")
	}
	if result.Path != "/tmp/capture-01.sv.json" || result.Name != "capture-01.sv.json" {
		t.Fatalf("unexpected save-as result: %+v", result)
	}
	if gotPath != "/tmp/capture-01.sv.json" {
		t.Fatalf("write path = %q", gotPath)
	}
}

func TestSaveDocumentAsReplacesBareJSONExtension(t *testing.T) {
	app := NewApp()
	app.saveFileDialog = func(_ context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
		if opts.DefaultFilename != "capture-01.sv.json" {
			t.Fatalf("default filename = %q", opts.DefaultFilename)
		}
		return "/tmp/capture-01.json", nil
	}

	var gotPath string
	app.writeFile = func(path string, data []byte, mode os.FileMode) error {
		gotPath = path
		return nil
	}

	result, err := app.SaveDocumentAs("capture-01", `{"annotations":[]}`)
	if err != nil {
		t.Fatalf("SaveDocumentAs returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected save-as result")
	}
	if result.Path != "/tmp/capture-01.sv.json" || result.Name != "capture-01.sv.json" {
		t.Fatalf("unexpected save-as result: %+v", result)
	}
	if gotPath != "/tmp/capture-01.sv.json" {
		t.Fatalf("write path = %q", gotPath)
	}
}

func TestSaveDocumentAsCancelReturnsNil(t *testing.T) {
	app := NewApp()
	app.saveFileDialog = func(context.Context, wailsruntime.SaveDialogOptions) (string, error) {
		return "", nil
	}

	result, err := app.SaveDocumentAs("capture-01", `{"annotations":[]}`)
	if err != nil {
		t.Fatalf("SaveDocumentAs returned error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result on cancel, got %+v", result)
	}
}

func TestOpenDocumentPropagatesReadError(t *testing.T) {
	app := NewApp()
	app.openFileDialog = func(context.Context, wailsruntime.OpenDialogOptions) (string, error) {
		return "/tmp/example.sv.json", nil
	}
	app.readFile = func(string) ([]byte, error) {
		return nil, errors.New("boom")
	}

	_, err := app.OpenDocument()
	if err == nil || err.Error() != "read document: boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFileDialogFiltersAvoidMultiDotPatterns(t *testing.T) {
	filters := fileDialogFilters()
	if len(filters) != 1 {
		t.Fatalf("filter count = %d, want 1", len(filters))
	}
	if filters[0].Pattern != "*.json" {
		t.Fatalf("pattern = %q, want *.json", filters[0].Pattern)
	}
	if filters[0].DisplayName == "" {
		t.Fatal("expected non-empty display name")
	}
}

type fakeCapturer struct {
	fullPNG    capture.PNG
	fullMeta   capture.Meta
	fullErr    error
	allPNG     capture.PNG
	allMeta    capture.Meta
	allErr     error
	regionPNG  capture.PNG
	regionMeta capture.Meta
	regionErr  error
}

func (f fakeCapturer) CaptureFullScreen(context.Context) (capture.PNG, capture.Meta, error) {
	return f.fullPNG, f.fullMeta, f.fullErr
}

func (f fakeCapturer) CaptureAllDisplays(context.Context) (capture.PNG, capture.Meta, error) {
	return f.allPNG, f.allMeta, f.allErr
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
