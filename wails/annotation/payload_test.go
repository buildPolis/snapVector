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
