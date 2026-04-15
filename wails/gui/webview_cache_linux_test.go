//go:build linux

package gui

import (
	"os"
	"path/filepath"
	"testing"
)

// seedWebKitCacheDirs populates the two WebKit cache subdirectories
// under appCache with sentinel files, so tests can assert whether they
// survive or get wiped.
func seedWebKitCacheDirs(t *testing.T, appCache string) {
	t.Helper()
	for _, sub := range []string{"WebKitCache", "CacheStorage"} {
		dir := filepath.Join(appCache, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sentinel"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write sentinel in %s: %v", dir, err)
		}
	}
}

func assertCacheWiped(t *testing.T, appCache string) {
	t.Helper()
	for _, sub := range []string{"WebKitCache", "CacheStorage"} {
		if _, err := os.Stat(filepath.Join(appCache, sub)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be wiped, stat err=%v", sub, err)
		}
	}
}

func assertCachePresent(t *testing.T, appCache string) {
	t.Helper()
	for _, sub := range []string{"WebKitCache", "CacheStorage"} {
		if _, err := os.Stat(filepath.Join(appCache, sub, "sentinel")); err != nil {
			t.Fatalf("expected %s/sentinel to survive, err=%v", sub, err)
		}
	}
}

func readStamp(t *testing.T, appCache string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(appCache, binaryFingerprintStampFile))
	if err != nil {
		t.Fatalf("read stamp: %v", err)
	}
	return string(b)
}

func TestBustWebViewCacheIfChanged_firstRunWritesStampNoError(t *testing.T) {
	appCache := t.TempDir()

	bustWebViewCacheIfChanged(appCache, "fp-1")

	if got := readStamp(t, appCache); got != "fp-1" {
		t.Fatalf("stamp = %q, want %q", got, "fp-1")
	}
}

func TestBustWebViewCacheIfChanged_sameFingerprintPreservesCache(t *testing.T) {
	appCache := t.TempDir()
	seedWebKitCacheDirs(t, appCache)
	if err := os.WriteFile(filepath.Join(appCache, binaryFingerprintStampFile), []byte("fp-1"), 0o644); err != nil {
		t.Fatalf("seed stamp: %v", err)
	}

	bustWebViewCacheIfChanged(appCache, "fp-1")

	assertCachePresent(t, appCache)
	if got := readStamp(t, appCache); got != "fp-1" {
		t.Fatalf("stamp unexpectedly changed to %q", got)
	}
}

func TestBustWebViewCacheIfChanged_differentFingerprintWipesCache(t *testing.T) {
	appCache := t.TempDir()
	seedWebKitCacheDirs(t, appCache)
	if err := os.WriteFile(filepath.Join(appCache, binaryFingerprintStampFile), []byte("fp-old"), 0o644); err != nil {
		t.Fatalf("seed stamp: %v", err)
	}

	bustWebViewCacheIfChanged(appCache, "fp-new")

	assertCacheWiped(t, appCache)
	if got := readStamp(t, appCache); got != "fp-new" {
		t.Fatalf("stamp = %q, want %q", got, "fp-new")
	}
}

func TestBustWebViewCacheIfChanged_missingCacheDirsDoesNotError(t *testing.T) {
	appCache := filepath.Join(t.TempDir(), "not-yet-created")

	bustWebViewCacheIfChanged(appCache, "fp-1")

	if got := readStamp(t, appCache); got != "fp-1" {
		t.Fatalf("stamp = %q, want %q", got, "fp-1")
	}
}

func TestCurrentBinaryFingerprint_stableAcrossCalls(t *testing.T) {
	a, ok := currentBinaryFingerprint()
	if !ok {
		t.Skip("cannot resolve os.Executable in this environment")
	}
	b, ok := currentBinaryFingerprint()
	if !ok {
		t.Fatalf("second call returned ok=false")
	}
	if a != b {
		t.Fatalf("fingerprint not stable: %q vs %q", a, b)
	}
	if a == "" {
		t.Fatalf("empty fingerprint")
	}
}
