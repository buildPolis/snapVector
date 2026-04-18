//go:build linux

package gui

import (
	"log"
)

// globalHotkeyListenerHandle is the concrete type on Linux.
type globalHotkeyListenerHandle = *GlobalHotkeyListener

// startGlobalHotkeys initializes and starts the global hotkey listener on Linux.
// It loads hotkey bindings from the store and registers eligible ones with the
// XDG GlobalShortcuts portal. If the portal is unavailable, it logs a warning
// and returns nil (graceful degradation).
func (a *App) startGlobalHotkeys() {
	bindings, err := a.hotkeyStore.Load()
	if err != nil {
		log.Printf("snapvector: failed to load hotkeys for global registration: %v", err)
		return
	}

	listener := NewGlobalHotkeyListener()
	if err := listener.Start(bindings); err != nil {
		log.Printf("snapvector: global hotkeys unavailable (portal may not be supported): %v", err)
		return
	}

	a.globalHotkeyListener = listener
	go a.dispatchGlobalHotkeys()
	log.Printf("snapvector: global hotkey listener started")
}

// stopGlobalHotkeys stops the listener if active.
func (a *App) stopGlobalHotkeys() {
	if a.globalHotkeyListener != nil {
		a.globalHotkeyListener.Stop()
	}
}

func (a *App) reapplyGlobalHotkeys() {
	a.stopGlobalHotkeys()
	a.globalHotkeyListener = nil
	a.startGlobalHotkeys()
}

// dispatchGlobalHotkeys reads from the listener's action channel and forwards
// each action to the frontend via a Wails event. The frontend owns the actual
// capture flow so a single code path handles both in-app shortcuts and global
// hotkeys; doing the capture in Go here would strand the resulting PNG
// outside the webview state.
func (a *App) dispatchGlobalHotkeys() {
	if a.globalHotkeyListener == nil {
		return
	}
	for action := range a.globalHotkeyListener.Actions() {
		log.Printf("snapvector: dispatching global hotkey action: %s", action)
		a.forwardHotkeyAction(action)
	}
}
