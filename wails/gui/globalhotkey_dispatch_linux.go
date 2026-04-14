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

// dispatchGlobalHotkeys reads from the listener's action channel and triggers
// the corresponding capture method on the App.
func (a *App) dispatchGlobalHotkeys() {
	if a.globalHotkeyListener == nil {
		return
	}
	for action := range a.globalHotkeyListener.Actions() {
		log.Printf("snapvector: dispatching global hotkey action: %s", action)
		switch action {
		case "capture.fullscreen":
			if _, err := a.CaptureScreen(); err != nil {
				log.Printf("snapvector: global hotkey capture fullscreen error: %v", err)
			}
		case "capture.region":
			if _, err := a.CaptureRegion(); err != nil {
				log.Printf("snapvector: global hotkey capture region error: %v", err)
			}
		case "capture.allDisplays":
			if _, err := a.CaptureAllDisplays(); err != nil {
				log.Printf("snapvector: global hotkey capture all displays error: %v", err)
			}
		default:
			log.Printf("snapvector: unknown global hotkey action: %s", action)
		}
	}
}
