//go:build linux

package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxDesktopEntryIncludesExecutableAndIcon(t *testing.T) {
	entry := linuxDesktopEntry("/tmp/Snap Vector/snapvector")

	assertDesktopContains(t, entry, `Exec="/tmp/Snap Vector/snapvector" %U`)
	assertDesktopContains(t, entry, `TryExec="/tmp/Snap Vector/snapvector"`)
	assertDesktopContains(t, entry, `Icon=snapvector`)
	assertDesktopContains(t, entry, `StartupWMClass=Snapvector`)
}

func TestSyncLinuxDesktopIntegrationWritesDesktopAssets(t *testing.T) {
	tmpDir := t.TempDir()
	execPath := filepath.Join(tmpDir, "build", "bin", "snapvector")
	if err := os.MkdirAll(filepath.Dir(execPath), 0o755); err != nil {
		t.Fatalf("mkdir executable dir: %v", err)
	}
	if err := os.WriteFile(execPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	dataDir := filepath.Join(tmpDir, "xdg-data")
	t.Setenv("XDG_DATA_HOME", dataDir)
	err := syncLinuxDesktopIntegration(linuxDesktopDeps{
		executable:  func() (string, error) { return execPath, nil },
		userHomeDir: func() (string, error) { return tmpDir, nil },
		mkdirAll:    os.MkdirAll,
		writeFile:   os.WriteFile,
	})
	if err != nil {
		t.Fatalf("syncLinuxDesktopIntegration returned error: %v", err)
	}

	iconPath := filepath.Join(dataDir, "icons", "hicolor", "512x512", "apps", linuxIconFileName)
	iconBytes, err := os.ReadFile(iconPath)
	if err != nil {
		t.Fatalf("read icon: %v", err)
	}
	if len(iconBytes) == 0 {
		t.Fatal("expected embedded icon bytes to be written")
	}
	if _, err := os.ReadFile(filepath.Join(dataDir, "pixmaps", linuxIconFileName)); err != nil {
		t.Fatalf("read pixmaps icon: %v", err)
	}

	desktopPath := filepath.Join(dataDir, "applications", linuxDesktopFileName)
	desktopBytes, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("read desktop entry: %v", err)
	}
	desktopEntry := string(desktopBytes)
	assertDesktopContains(t, desktopEntry, "Name=SnapVector")
	assertDesktopContains(t, desktopEntry, "StartupWMClass=Snapvector")
	assertDesktopContains(t, desktopEntry, "Icon=snapvector")
	assertDesktopContains(t, desktopEntry, `Exec="`+execPath+`" %U`)
}

func assertDesktopContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q in %q", needle, haystack)
	}
}
