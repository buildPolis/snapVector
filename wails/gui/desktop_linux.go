//go:build linux

package gui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	linuxDesktopFileName = "snapvector.desktop"
	linuxIconFileName    = "snapvector.png"
	linuxIconName        = "snapvector"
	linuxStartupWMClass  = "Snapvector"
)

type linuxDesktopDeps struct {
	executable  func() (string, error)
	userHomeDir func() (string, error)
	mkdirAll    func(string, os.FileMode) error
	writeFile   func(string, []byte, os.FileMode) error
}

func ensureLinuxDesktopIntegration() {
	if err := syncLinuxDesktopIntegration(linuxDesktopDeps{
		executable:  os.Executable,
		userHomeDir: os.UserHomeDir,
		mkdirAll:    os.MkdirAll,
		writeFile:   os.WriteFile,
	}); err != nil {
		log.Printf("snapvector: failed to install desktop integration: %v", err)
	}
}

func syncLinuxDesktopIntegration(deps linuxDesktopDeps) error {
	execPath, err := deps.executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("normalize executable path: %w", err)
	}
	if filepath.Base(execPath) != "snapvector" {
		return nil
	}

	dataDir, err := linuxUserDataDir(deps)
	if err != nil {
		return err
	}

	applicationsDir := filepath.Join(dataDir, "applications")
	iconsDir := filepath.Join(dataDir, "icons", "hicolor", "512x512", "apps")
	pixmapsDir := filepath.Join(dataDir, "pixmaps")
	if err := deps.mkdirAll(applicationsDir, 0o755); err != nil {
		return fmt.Errorf("create applications dir: %w", err)
	}
	if err := deps.mkdirAll(iconsDir, 0o755); err != nil {
		return fmt.Errorf("create icons dir: %w", err)
	}
	if err := deps.mkdirAll(pixmapsDir, 0o755); err != nil {
		return fmt.Errorf("create pixmaps dir: %w", err)
	}

	iconPath := filepath.Join(iconsDir, linuxIconFileName)
	if err := deps.writeFile(iconPath, appIcon, 0o644); err != nil {
		return fmt.Errorf("write icon: %w", err)
	}
	if err := deps.writeFile(filepath.Join(pixmapsDir, linuxIconFileName), appIcon, 0o644); err != nil {
		return fmt.Errorf("write pixmaps icon: %w", err)
	}

	desktopPath := filepath.Join(applicationsDir, linuxDesktopFileName)
	desktopEntry := linuxDesktopEntry(execPath)
	if err := deps.writeFile(desktopPath, []byte(desktopEntry), 0o755); err != nil {
		return fmt.Errorf("write desktop entry: %w", err)
	}
	refreshLinuxDesktopCaches(applicationsDir, filepath.Join(dataDir, "icons", "hicolor"))

	return nil
}

func linuxUserDataDir(deps linuxDesktopDeps) (string, error) {
	if xdgDataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgDataHome != "" {
		return xdgDataHome, nil
	}

	homeDir, err := deps.userHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	if strings.TrimSpace(homeDir) == "" {
		return "", fmt.Errorf("resolve user home dir: empty path")
	}

	return filepath.Join(homeDir, ".local", "share"), nil
}

func linuxDesktopEntry(execPath string) string {
	return strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Version=1.0",
		"Name=SnapVector",
		"Comment=Cross-platform screenshot and vector annotation tool",
		"Terminal=false",
		"Categories=Graphics;Utility;",
		"StartupNotify=true",
		"StartupWMClass=" + linuxStartupWMClass,
		"Exec=" + xdgExecQuote(execPath) + " %U",
		"TryExec=" + execPath,
		"Icon=" + linuxIconName,
		"",
	}, "\n")
}

// xdgExecQuote escapes a path for use in a .desktop Exec= field per the
// XDG Desktop Entry Specification. Paths with none of the spec's
// reserved characters are returned verbatim; otherwise the full path is
// wrapped in double quotes and the four inner metacharacters (" ` $ \)
// are backslash-escaped. Returning the path unchanged when no escaping
// is needed matters because shells like GNOME and Unity reject entries
// whose Exec= is wrapped in unnecessary quotes (the quotes are parsed
// as part of the argv, not as shell syntax).
func xdgExecQuote(p string) string {
	const xdgReserved = " \t\n\"`$\\><~|&;*?#()"
	if !strings.ContainsAny(p, xdgReserved) {
		return p
	}
	escaper := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"`", "\\`",
		`$`, `\$`,
	)
	return `"` + escaper.Replace(p) + `"`
}

func refreshLinuxDesktopCaches(applicationsDir, iconThemeDir string) {
	for _, cmd := range [][]string{
		{"update-desktop-database", applicationsDir},
		{"gtk-update-icon-cache", "-f", "-t", iconThemeDir},
	} {
		bin, err := exec.LookPath(cmd[0])
		if err != nil {
			continue
		}
		if err := exec.Command(bin, cmd[1:]...).Run(); err != nil {
			log.Printf("snapvector: %s failed: %v", cmd[0], err)
		}
	}
}
