package exportdoc

import (
	"bytes"
	"context"
	"image/png"
	"testing"
)

func TestConvertRejectsUnsupportedFormat(t *testing.T) {
	_, _, err := Convert(context.Background(), "<svg/>", "gif")
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestConvertPNGProducesValidImage(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40"><rect width="40" height="40" fill="#E53935"/></svg>`
	raw, mime, err := Convert(context.Background(), svg, "png")
	if err != nil {
		t.Fatalf("Convert png: %v", err)
	}
	if mime != "image/png" {
		t.Fatalf("mime = %q, want image/png", mime)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 40 {
		t.Fatalf("size = %dx%d, want 40x40", b.Dx(), b.Dy())
	}
}
