//go:build !linux

package gui

// startGlobalHotkeys is a no-op on non-Linux platforms.
// macOS and Windows do not use the D-Bus GlobalShortcuts portal.
func (a *App) startGlobalHotkeys() {}

// stopGlobalHotkeys is a no-op on non-Linux platforms.
func (a *App) stopGlobalHotkeys() {}

// globalHotkeyListenerHandle is nil on non-Linux platforms.
type globalHotkeyListenerHandle = *struct{}
