//go:build !linux && !darwin && !windows

package gui

func (a *App) startGlobalHotkeys()     {}
func (a *App) stopGlobalHotkeys()      {}
func (a *App) reapplyGlobalHotkeys()   {}

type globalHotkeyListenerHandle = *struct{}
