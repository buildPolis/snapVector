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
