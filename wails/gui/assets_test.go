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
	assertContains(t, html, `id="captureRegionButton"`)
	assertContains(t, html, `<svg><use href="#icon-region-capture" /></svg>`)
	assertContains(t, html, `id="captureAllDisplaysButton"`)
	assertContains(t, html, `<svg><use href="#icon-all-displays" /></svg>`)
	assertContains(t, html, `id="captureButton"`)
	assertContains(t, html, `<svg><use href="#icon-capture" /></svg>`)
}

func TestFrontendIncludesFileMenuActions(t *testing.T) {
	raw, err := fs.ReadFile(assets, "frontend/dist/index.html")
	if err != nil {
		t.Fatalf("read embedded index.html: %v", err)
	}

	html := string(raw)
	assertContains(t, html, `id="fileMenuButton"`)
	assertContains(t, html, `id="openDocumentButton"`)
	assertContains(t, html, `id="saveDocumentButton"`)
	assertContains(t, html, `id="saveDocumentAsButton"`)
	assertContains(t, html, `id="documentBadge"`)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q in embedded html", needle)
	}
}
