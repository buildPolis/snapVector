//go:build linux

package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxDesktopEntryPlainPathHasNoQuotes(t *testing.T) {
	entry := linuxDesktopEntry("/usr/bin/snapvector")

	assertDesktopContains(t, entry, "Exec=/usr/bin/snapvector %U")
	assertDesktopContains(t, entry, "TryExec=/usr/bin/snapvector")
	assertDesktopContains(t, entry, "Icon=snapvector")
	assertDesktopContains(t, entry, "StartupWMClass=Snapvector")
	if strings.Contains(entry, `Exec="/usr/bin/snapvector"`) {
		t.Fatalf("plain path must not be double-quoted in Exec:\n%s", entry)
	}
	if strings.Contains(entry, `TryExec="`) {
		t.Fatalf("TryExec must never be double-quoted (fd.o spec is literal path):\n%s", entry)
	}
}

func TestLinuxDesktopEntryQuotesPathsWithSpaces(t *testing.T) {
	entry := linuxDesktopEntry("/tmp/Snap Vector/snapvector")

	assertDesktopContains(t, entry, `Exec="/tmp/Snap Vector/snapvector" %U`)
	// TryExec is a literal path per XDG spec — implementations call
	// access(2) on the raw string, so spaces are fine without quoting.
	assertDesktopContains(t, entry, "TryExec=/tmp/Snap Vector/snapvector")
}

func TestLinuxDesktopEntryEscapesXdgMetacharacters(t *testing.T) {
	entry := linuxDesktopEntry(`/opt/w$d/"q"/app`)

	assertDesktopContains(t, entry, `Exec="/opt/w\$d/\"q\"/app" %U`)
	assertDesktopContains(t, entry, `TryExec=/opt/w$d/"q"/app`)
}

func TestXdgExecQuote(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "/usr/bin/snapvector", "/usr/bin/snapvector"},
		{"home dotlocal", "/home/jane/.local/bin/snapvector", "/home/jane/.local/bin/snapvector"},
		{"with space", "/tmp/Snap Vector/snapvector", `"/tmp/Snap Vector/snapvector"`},
		{"with dollar", "/opt/w$d/app", `"/opt/w\$d/app"`},
		{"with backtick", "/opt/w`d/app", "\"/opt/w\\`d/app\""},
		{"with quote", `/opt/w"d/app`, `"/opt/w\"d/app"`},
		{"with backslash", `/opt/w\d/app`, `"/opt/w\\d/app"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := xdgExecQuote(c.in); got != c.want {
				t.Fatalf("xdgExecQuote(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
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
	assertDesktopContains(t, desktopEntry, "Exec="+execPath+" %U")
	assertDesktopContains(t, desktopEntry, "TryExec="+execPath)
}

func assertDesktopContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q in %q", needle, haystack)
	}
}
