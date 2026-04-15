package annotation

import "testing"

func TestParsePayloadValidatesAndDefaults(t *testing.T) {
	annotations, err := ParsePayload(`[
		{"type":"arrow","x1":10,"y1":20,"x2":110,"y2":120},
		{"type":"text","x":12,"y":30,"text":"這裡要修正","maxWidth":60},
		{"type":"blur","x":20,"y":40,"width":100,"height":50}
	]`)
	if err != nil {
		t.Fatalf("ParsePayload returned error: %v", err)
	}
	if len(annotations) != 3 {
		t.Fatalf("annotation count = %d, want 3", len(annotations))
	}
	if annotations[0].ID == "" {
		t.Fatal("expected generated ID")
	}
	if annotations[1].Variant != "solid" {
		t.Fatalf("default text variant = %q, want solid", annotations[1].Variant)
	}
	if annotations[2].BlurRadius != DefaultBlurRadius {
		t.Fatalf("default blur radius = %v", annotations[2].BlurRadius)
	}
	if annotations[2].Feather != annotations[2].BlurRadius {
		t.Fatalf("default feather = %v, want %v", annotations[2].Feather, annotations[2].BlurRadius)
	}
}

func TestParsePayloadRejectsInvalidType(t *testing.T) {
	_, err := ParsePayload(`[{"type":"triangle"}]`)
	if err == nil {
		t.Fatal("expected invalid type error")
	}
}

func TestParsePayloadRejectsMissingFields(t *testing.T) {
	_, err := ParsePayload(`[{"type":"rectangle","x":1,"y":2}]`)
	if err == nil {
		t.Fatal("expected missing field error")
	}
}

func TestParsePayloadRejectsInvalidColor(t *testing.T) {
	_, err := ParsePayload(`[{"type":"rectangle","x":1,"y":2,"width":3,"height":4,"strokeColor":"red"}]`)
	if err == nil {
		t.Fatal("expected invalid color error")
	}
}

func TestParsePayloadRejectsInvalidGeometry(t *testing.T) {
	_, err := ParsePayload(`[{"type":"blur","x":1,"y":2,"width":0,"height":4}]`)
	if err == nil {
		t.Fatal("expected invalid geometry error")
	}
}

func TestParsePayloadNumberedCircleFull(t *testing.T) {
	annotations, err := ParsePayload(`[{"type":"numbered-circle","x":100,"y":200,"number":3,"radius":24,"strokeColor":"#2E86AB","outlineColor":"#FFFFFF","textColor":"#FEFEFE","strokeWidth":4}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(annotations) != 1 {
		t.Fatalf("count = %d, want 1", len(annotations))
	}
	a := annotations[0]
	if a.Type != TypeNumberedCircle || a.Number != 3 || a.Radius != 24 ||
		a.StrokeColor != "#2E86AB" || a.TextColor != "#FEFEFE" || a.StrokeWidth != 4 {
		t.Fatalf("unexpected annotation: %+v", a)
	}
}

func TestParsePayloadNumberedCircleDefaults(t *testing.T) {
	annotations, err := ParsePayload(`[{"type":"numbered-circle","x":50,"y":60,"number":0}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := annotations[0]
	if a.Radius != DefaultNumberedRadius {
		t.Fatalf("radius = %v, want %v", a.Radius, DefaultNumberedRadius)
	}
	if a.StrokeWidth != DefaultNumberedStrokeW {
		t.Fatalf("strokeWidth = %v, want %v", a.StrokeWidth, DefaultNumberedStrokeW)
	}
	if a.TextColor != DefaultNumberedTextColor {
		t.Fatalf("textColor = %q, want %q", a.TextColor, DefaultNumberedTextColor)
	}
	if a.StrokeColor != DefaultStrokeColor || a.OutlineColor != DefaultOutlineColor {
		t.Fatalf("unexpected default colors: stroke=%q outline=%q", a.StrokeColor, a.OutlineColor)
	}
}

func TestParsePayloadNumberedCircleRejectsNegativeNumber(t *testing.T) {
	if _, err := ParsePayload(`[{"type":"numbered-circle","x":1,"y":2,"number":-1}]`); err == nil {
		t.Fatal("expected negative number error")
	}
}

func TestParsePayloadNumberedCircleRejectsMissing(t *testing.T) {
	if _, err := ParsePayload(`[{"type":"numbered-circle","x":1,"y":2}]`); err == nil {
		t.Fatal("expected missing number error")
	}
}

func TestParsePayloadNumberedCircleRejectsBadRadius(t *testing.T) {
	if _, err := ParsePayload(`[{"type":"numbered-circle","x":1,"y":2,"number":1,"radius":1000}]`); err == nil {
		t.Fatal("expected radius out of range error")
	}
}

func TestParsePayloadNumberedCircleRejectsBadTextColor(t *testing.T) {
	if _, err := ParsePayload(`[{"type":"numbered-circle","x":1,"y":2,"number":1,"textColor":"white"}]`); err == nil {
		t.Fatal("expected invalid textColor error")
	}
}

func TestWrapTextSplitsLongText(t *testing.T) {
	lines := WrapText("abcdefgh", 30, 10)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines, got %v", lines)
	}
}

func TestWrapTextRespectsExplicitNewlines(t *testing.T) {
	lines := WrapText("第一行\n第二行", 120, 16)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %v", lines)
	}
	if lines[0] != "第一行" || lines[1] != "第二行" {
		t.Fatalf("unexpected wrapped lines: %v", lines)
	}
}
