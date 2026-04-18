//go:build darwin

package gui

import "log"

type globalHotkeyListenerHandle = *GlobalHotkeyListener

func (a *App) startGlobalHotkeys() {
	bindings, err := a.hotkeyStore.Load()
	if err != nil {
		log.Printf("snapvector: failed to load hotkeys for global registration: %v", err)
		return
	}
	listener, err := newDarwinGlobalHotkeyListener(bindings)
	if err != nil {
		log.Printf("snapvector: global hotkeys unavailable: %v", err)
		return
	}
	a.globalHotkeyListener = listener
	listener.Start()
	go a.dispatchGlobalHotkeys()
	log.Printf("snapvector: macOS global hotkey listener started")
}

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

func (a *App) dispatchGlobalHotkeys() {
	if a.globalHotkeyListener == nil {
		return
	}
	for action := range a.globalHotkeyListener.Actions() {
		log.Printf("snapvector: dispatching global hotkey action: %s", action)
		a.forwardHotkeyAction(action)
	}
}
