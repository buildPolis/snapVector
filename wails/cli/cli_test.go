package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"snapvector/capture"
)

type fakeCapturer struct {
	png  capture.PNG
	meta capture.Meta
	err  error
}

func (f fakeCapturer) CaptureFullScreen(context.Context) (capture.PNG, capture.Meta, error) {
	return f.png, f.meta, f.err
}

func (f fakeCapturer) CaptureAllDisplays(context.Context) (capture.PNG, capture.Meta, error) {
	return f.png, f.meta, f.err
}

func (f fakeCapturer) CaptureInteractiveRegion(context.Context) (capture.PNG, capture.Meta, error) {
	return f.png, f.meta, f.err
}

func TestRunCaptureBase64StdoutEmitsValidPNG(t *testing.T) {
	originalFactory := newCapturer
	newCapturer = func() capture.Capturer {
		return fakeCapturer{
			png:  mustMakePNG(t),
			meta: capture.Meta{DisplayID: "1", X: 10, Y: 20, Width: 2, Height: 2, ScaleFactor: 2},
		}
	}
	defer func() {
		newCapturer = originalFactory
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--capture", "--base64-stdout"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}

	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
		Data   struct {
			Format   string `json:"format"`
			MimeType string `json:"mimeType"`
			Base64   string `json:"base64"`
			Display  struct {
				ID          string  `json:"id"`
				X           int     `json:"x"`
				Y           int     `json:"y"`
				Width       int     `json:"width"`
				Height      int     `json:"height"`
				ScaleFactor float64 `json:"scaleFactor"`
			} `json:"display"`
			CaptureRegion struct {
				X      int `json:"x"`
				Y      int `json:"y"`
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"captureRegion"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "ok" || resp.Code != 0 {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Data.Format != "png" || resp.Data.MimeType != "image/png" {
		t.Fatalf("wrong format: %+v", resp.Data)
	}
	raw, err := base64.StdEncoding.DecodeString(resp.Data.Base64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("decoded bytes are not PNG: %v", err)
	}
	if resp.Data.Display.ID != "1" || resp.Data.Display.Width != 2 || resp.Data.Display.Height != 2 {
		t.Fatalf("wrong display metadata: %+v", resp.Data.Display)
	}
	if resp.Data.Display.X != 10 || resp.Data.Display.Y != 20 {
		t.Fatalf("wrong display origin: %+v", resp.Data.Display)
	}
	if resp.Data.CaptureRegion.X != 10 || resp.Data.CaptureRegion.Y != 20 || resp.Data.CaptureRegion.Width != 2 || resp.Data.CaptureRegion.Height != 2 {
		t.Fatalf("wrong capture region metadata: %+v", resp.Data.CaptureRegion)
	}
}

func TestRunUnknownFlagEmitsUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--nope"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}

	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "error" || resp.Code != CodeUsage {
		t.Fatalf("got %+v, want code=%d", resp, CodeUsage)
	}
}

func TestRunInjectSVGEmitsSVGDocument(t *testing.T) {
	originalFactory := newCapturer
	newCapturer = func() capture.Capturer {
		return fakeCapturer{
			png:  mustMakePNG(t),
			meta: capture.Meta{X: 4, Y: 6, Width: 2, Height: 2},
		}
	}
	defer func() {
		newCapturer = originalFactory
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--inject-svg", "[]"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
		Data   struct {
			Format          string `json:"format"`
			MimeType        string `json:"mimeType"`
			SVG             string `json:"svg"`
			AnnotationCount int    `json:"annotationCount"`
			Canvas          struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"canvas"`
			CaptureRegion struct {
				X      int `json:"x"`
				Y      int `json:"y"`
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"captureRegion"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "ok" || resp.Code != 0 {
		t.Fatalf("got %+v, want success", resp)
	}
	if resp.Data.Format != "svg" || resp.Data.MimeType != "image/svg+xml" {
		t.Fatalf("unexpected format payload: %+v", resp.Data)
	}
	if resp.Data.AnnotationCount != 0 {
		t.Fatalf("expected zero annotations, got %d", resp.Data.AnnotationCount)
	}
	if resp.Data.Canvas.Width != 2 || resp.Data.Canvas.Height != 2 {
		t.Fatalf("unexpected canvas: %+v", resp.Data.Canvas)
	}
	if resp.Data.CaptureRegion.X != 4 || resp.Data.CaptureRegion.Y != 6 || resp.Data.CaptureRegion.Width != 2 || resp.Data.CaptureRegion.Height != 2 {
		t.Fatalf("unexpected capture region: %+v", resp.Data.CaptureRegion)
	}
	if !strings.Contains(resp.Data.SVG, "<svg") {
		t.Fatalf("unexpected svg payload: %q", resp.Data.SVG)
	}
	if !strings.Contains(resp.Data.SVG, "<image") {
		t.Fatalf("svg missing base image: %q", resp.Data.SVG)
	}
}

func TestRunInjectSVGCanExportPNG(t *testing.T) {
	originalFactory := newCapturer
	originalConvert := convertSVG
	originalClipboard := writeClipboard
	newCapturer = func() capture.Capturer {
		return fakeCapturer{
			png:  mustMakePNG(t),
			meta: capture.Meta{Width: 2, Height: 2},
		}
	}
	convertSVG = func(_ context.Context, svg string, format string) ([]byte, string, error) {
		if format != "png" {
			t.Fatalf("format = %q, want png", format)
		}
		if !strings.Contains(svg, "<svg") {
			t.Fatalf("expected svg input, got %q", svg)
		}
		return mustMakePNG(t), "image/png", nil
	}
	writeClipboard = func(context.Context, []byte, string) error { return nil }
	defer func() {
		newCapturer = originalFactory
		convertSVG = originalConvert
		writeClipboard = originalClipboard
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--inject-svg", "[]", "--output-format", "png"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}

	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
		Data   struct {
			Format   string `json:"format"`
			MimeType string `json:"mimeType"`
			Base64   string `json:"base64"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "ok" || resp.Code != 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Data.Format != "png" || resp.Data.MimeType != "image/png" {
		t.Fatalf("unexpected export metadata: %+v", resp.Data)
	}
	if _, err := base64.StdEncoding.DecodeString(resp.Data.Base64); err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
}

func TestRunInjectSVGCanCopySVGToClipboard(t *testing.T) {
	originalFactory := newCapturer
	originalClipboard := writeClipboard
	newCapturer = func() capture.Capturer {
		return fakeCapturer{
			png:  mustMakePNG(t),
			meta: capture.Meta{Width: 2, Height: 2},
		}
	}
	called := false
	writeClipboard = func(_ context.Context, payload []byte, format string) error {
		called = true
		if format != "svg" {
			t.Fatalf("format = %q, want svg", format)
		}
		if !strings.Contains(string(payload), "<svg") {
			t.Fatalf("expected svg payload, got %q", string(payload))
		}
		return nil
	}
	defer func() {
		newCapturer = originalFactory
		writeClipboard = originalClipboard
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--inject-svg", "[]", "--copy-to-clipboard"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !called {
		t.Fatal("expected clipboard writer to be called")
	}

	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
		Data   struct {
			CopiedToClipboard bool `json:"copiedToClipboard"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "ok" || resp.Code != 0 || !resp.Data.CopiedToClipboard {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestRunInjectSVGSamplePayloadRendersAllTypes(t *testing.T) {
	originalFactory := newCapturer
	newCapturer = func() capture.Capturer {
		return fakeCapturer{
			png:  mustMakePNG(t),
			meta: capture.Meta{Width: 1200, Height: 720},
		}
	}
	defer func() {
		newCapturer = originalFactory
	}()

	payload := `[
		{"id":"ann-arrow-1","type":"arrow","x1":96,"y1":120,"x2":312,"y2":228},
		{"id":"ann-rect-1","type":"rectangle","x":344,"y":88,"width":220,"height":132},
		{"id":"ann-ellipse-1","type":"ellipse","x":620,"y":96,"width":168,"height":124},
		{"id":"ann-blur-1","type":"blur","x":850,"y":430,"width":196,"height":116,"blurRadius":12,"cornerRadius":18},
		{"id":"ann-text-1","type":"text","x":140,"y":264,"text":"這裡要修正","variant":"solid","fontSize":24,"maxWidth":220}
	]`

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--inject-svg", payload}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}

	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
		Data   struct {
			AnnotationCount int    `json:"annotationCount"`
			SVG             string `json:"svg"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "ok" || resp.Code != 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Data.AnnotationCount != 5 {
		t.Fatalf("annotationCount = %d, want 5", resp.Data.AnnotationCount)
	}
	if !strings.Contains(resp.Data.SVG, "feGaussianBlur") {
		t.Fatalf("svg missing blur filter: %q", resp.Data.SVG)
	}
	if strings.Count(resp.Data.SVG, "<use ") < 5 {
		t.Fatalf("svg missing use nodes: %q", resp.Data.SVG)
	}
	if !strings.Contains(resp.Data.SVG, "這裡要修正") {
		t.Fatalf("svg missing text annotation: %q", resp.Data.SVG)
	}
	if !strings.Contains(resp.Data.SVG, `clip-path="url(#ann-symbol-3-ann-blur-1-clip)"`) {
		t.Fatalf("svg missing blur clip-path semantics: %q", resp.Data.SVG)
	}
	if !strings.Contains(resp.Data.SVG, `stroke="#E53935" stroke-width="6" stroke-dasharray="10 6"`) {
		t.Fatalf("svg missing blur dashed border semantics: %q", resp.Data.SVG)
	}
	if !strings.Contains(resp.Data.SVG, `stroke="#FFFFFF" stroke-width="4"`) {
		t.Fatalf("svg missing blur white outline semantics: %q", resp.Data.SVG)
	}
}

func TestRunUnsupportedPlatformIsNotRetryable(t *testing.T) {
	originalFactory := newCapturer
	newCapturer = func() capture.Capturer {
		return fakeCapturer{
			err: &capture.UnsupportedPlatformError{Platform: "linux"},
		}
	}
	defer func() {
		newCapturer = originalFactory
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--capture", "--base64-stdout"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}

	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
		Error  struct {
			Retryable bool `json:"retryable"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "error" || resp.Code != CodeUnsupportedPlatform {
		t.Fatalf("got %+v, want code=%d", resp, CodeUnsupportedPlatform)
	}
	if resp.Error.Retryable {
		t.Fatal("unsupported platform must not be retryable")
	}
}

func TestRunInjectSVGInvalidPayload(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--inject-svg", `{"bad":true}`}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}

	var resp struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout.String())
	}
	if resp.Status != "error" || resp.Code != CodeInjectInvalid {
		t.Fatalf("got %+v, want code=%d", resp, CodeInjectInvalid)
	}
}

func mustMakePNG(t *testing.T) capture.PNG {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return capture.PNG(buf.Bytes())
}
