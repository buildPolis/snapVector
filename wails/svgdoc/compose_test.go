package svgdoc

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"snapvector/annotation"
)

func TestComposeBuildsSingleFileSVG(t *testing.T) {
	svg, err := Compose(mustPNG(t), 4, 4, []annotation.Annotation{
		{
			ID:           "arrow-1",
			Type:         annotation.TypeArrow,
			StrokeColor:  "#E53935",
			OutlineColor: "#FFFFFF",
			StrokeWidth:  10,
			X1:           1,
			Y1:           1,
			X2:           20,
			Y2:           20,
		},
		{
			ID:           "blur-1",
			Type:         annotation.TypeBlur,
			StrokeColor:  "#E53935",
			OutlineColor: "#FFFFFF",
			StrokeWidth:  10,
			X:            1,
			Y:            1,
			Width:        2,
			Height:       2,
			BlurRadius:   12,
			CornerRadius: 18,
			Feather:      12,
		},
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}
	if !strings.Contains(svg, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Fatalf("missing svg root: %q", svg)
	}
	if !strings.Contains(svg, `<image x="0" y="0"`) {
		t.Fatalf("missing base image: %q", svg)
	}
	if !strings.Contains(svg, `feGaussianBlur`) {
		t.Fatalf("missing blur filter: %q", svg)
	}
	if !strings.Contains(svg, `<symbol`) || !strings.Contains(svg, `<use`) {
		t.Fatalf("missing symbol encapsulation: %q", svg)
	}
}

func TestComposeWrapsMultilineTextWithinMaxWidth(t *testing.T) {
	svg, err := Compose(mustPNG(t), 200, 120, []annotation.Annotation{
		{
			ID:           "text-1",
			Type:         annotation.TypeText,
			StrokeColor:  "#E53935",
			OutlineColor: "#FFFFFF",
			StrokeWidth:  10,
			X:            12,
			Y:            20,
			Text:         "第一段很長的說明文字",
			Variant:      "outline",
			FontSize:     20,
			MaxWidth:     80,
		},
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}
	if strings.Count(svg, "<tspan") < 2 {
		t.Fatalf("expected wrapped tspans, got %q", svg)
	}
	if !strings.Contains(svg, `stroke="#E53935"`) {
		t.Fatalf("expected outline styling, got %q", svg)
	}
}

func TestComposeUsesBaselineGeometryRatios(t *testing.T) {
	svg, err := Compose(mustPNG(t), 400, 240, []annotation.Annotation{
		{
			ID:           "rect-1",
			Type:         annotation.TypeRectangle,
			StrokeColor:  "#E53935",
			OutlineColor: "#FFFFFF",
			StrokeWidth:  10,
			X:            10,
			Y:            20,
			Width:        220,
			Height:       140,
		},
		{
			ID:           "blur-1",
			Type:         annotation.TypeBlur,
			StrokeColor:  "#E53935",
			OutlineColor: "#FFFFFF",
			StrokeWidth:  10,
			X:            30,
			Y:            40,
			Width:        220,
			Height:       140,
			BlurRadius:   12,
			CornerRadius: 18,
			Feather:      12,
		},
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}
	if !strings.Contains(svg, `viewBox="0 0 220 140"`) {
		t.Fatalf("expected baseline rectangle/blur viewBox, got %q", svg)
	}
	if !strings.Contains(svg, `<symbol id="ann-symbol-0-rect-1" viewBox="0 0 220 140" overflow="visible">`) {
		t.Fatalf("expected overflow=visible on rectangle symbol, got %q", svg)
	}
	if !strings.Contains(svg, `x="0" y="0" width="220" height="140" rx="18"`) {
		t.Fatalf("expected rectangle at full bounds, got %q", svg)
	}
}

func TestComposeBlurKeepsSemanticStructure(t *testing.T) {
	svg, err := Compose(mustPNG(t), 220, 140, []annotation.Annotation{
		{
			ID:           "blur-1",
			Type:         annotation.TypeBlur,
			StrokeColor:  "#E53935",
			OutlineColor: "#FFFFFF",
			StrokeWidth:  10,
			X:            20,
			Y:            30,
			Width:        220,
			Height:       140,
			BlurRadius:   12,
			CornerRadius: 18,
			Feather:      12,
		},
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}

	assertContains(t, svg, `<symbol id="ann-symbol-0-blur-1" viewBox="0 0 220 140" overflow="visible">`)
	assertContains(t, svg, `<rect x="0" y="0" width="220" height="140" rx="18" fill="#FFFFFF" opacity="0.14"/>`)
	assertContains(t, svg, `<clipPath id="ann-symbol-0-blur-1-clip">`)
	assertContains(t, svg, `<filter id="ann-symbol-0-blur-1-filter"`)
	assertContains(t, svg, `<feGaussianBlur stdDeviation="12"/>`)
	assertContains(t, svg, `clip-path="url(#ann-symbol-0-blur-1-clip)"`)
	assertContains(t, svg, `filter="url(#ann-symbol-0-blur-1-filter)"`)
	assertContains(t, svg, `fill="#FFFFFF" opacity="0.14"`)
	assertContains(t, svg, `stroke="#FFFFFF" stroke-width="4"`)
	assertContains(t, svg, `stroke="#E53935" stroke-width="6" stroke-dasharray="10 6"`)
	assertContains(t, svg, `width="220" height="140"`)
	assertContains(t, svg, `x="-20" y="-30" width="220" height="140"`)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q in %q", needle, haystack)
	}
}

func TestComposeRendersNumberedCircle(t *testing.T) {
	svg, err := Compose(mustPNG(t), 100, 100, []annotation.Annotation{
		{
			ID:           "step-1",
			Type:         annotation.TypeNumberedCircle,
			StrokeColor:  "#2E86AB",
			OutlineColor: "#FFFFFF",
			TextColor:    "#FFFFFF",
			StrokeWidth:  6,
			X:            50,
			Y:            60,
			Radius:       20,
			Number:       7,
		},
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}
	if !strings.Contains(svg, `<circle`) {
		t.Fatalf("missing circle: %q", svg)
	}
	if !strings.Contains(svg, `fill="#2E86AB"`) {
		t.Fatalf("missing fill color: %q", svg)
	}
	if !strings.Contains(svg, `stroke="#FFFFFF"`) {
		t.Fatalf("missing outline color: %q", svg)
	}
	if !strings.Contains(svg, `paint-order="stroke fill"`) {
		t.Fatalf("missing paint-order: %q", svg)
	}
	if !strings.Contains(svg, `>7</text>`) {
		t.Fatalf("missing number text: %q", svg)
	}
	if !strings.Contains(svg, `text-anchor="middle"`) || !strings.Contains(svg, `dy=".35em"`) {
		t.Fatalf("text not centered (expected text-anchor=middle + dy=.35em): %q", svg)
	}
	if strings.Contains(svg, `dominant-baseline="central"`) {
		t.Fatalf("dominant-baseline=central should be removed (sips renders it wrong): %q", svg)
	}
}

func TestComposeRendersEllipseFullBounds(t *testing.T) {
	svg, err := Compose(mustPNG(t), 400, 240, []annotation.Annotation{
		{
			ID:           "oval-1",
			Type:         annotation.TypeEllipse,
			StrokeColor:  "#E53935",
			OutlineColor: "#FFFFFF",
			StrokeWidth:  10,
			X:            10,
			Y:            20,
			Width:        200,
			Height:       120,
		},
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}
	if !strings.Contains(svg, `<symbol id="ann-symbol-0-oval-1" viewBox="0 0 200 120" overflow="visible">`) {
		t.Fatalf("expected overflow=visible on ellipse symbol, got %q", svg)
	}
	if !strings.Contains(svg, `cx="100" cy="60" rx="100" ry="60"`) {
		t.Fatalf("expected ellipse to fill full bounds, got %q", svg)
	}
}

func mustPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}
