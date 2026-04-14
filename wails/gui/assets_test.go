package gui

import (
	"io/fs"
	"strings"
	"testing"
)

func TestFrontendCaptureButtonsUseDistinctIcons(t *testing.T) {
	raw, err := fs.ReadFile(assets, "frontend/dist/index.html")
	if err != nil {
		t.Fatalf("read embedded index.html: %v", err)
	}

	html := string(raw)
	assertContains(t, html, `id="captureRegionButton" title="Capture region (OS native)">`+"\n              "+`<svg><use href="#icon-region-capture" /></svg>`)
	assertContains(t, html, `id="captureAllDisplaysButton" title="Capture all displays (in-app crop)">`+"\n              "+`<svg><use href="#icon-all-displays" /></svg>`)
	assertContains(t, html, `id="captureButton" data-tool="capture" title="Re-capture">`+"\n              "+`<svg><use href="#icon-capture" /></svg>`)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q in embedded html", needle)
	}
}
