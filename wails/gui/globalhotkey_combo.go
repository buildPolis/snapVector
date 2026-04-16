package gui

import "strings"

// GlobalHotkeyActions lists actions eligible for system-wide hotkey registration.
// Only capture actions make sense as global shortcuts — single-key tool
// shortcuts would steal keystrokes from other applications.
var GlobalHotkeyActions = []string{
	"capture.fullscreen",
	"capture.region",
	"capture.allDisplays",
}

// filterGlobalBindings returns only bindings whose action is in GlobalHotkeyActions
// and whose combo includes at least one modifier key.
func filterGlobalBindings(bindings []Hotkey) []Hotkey {
	eligible := map[string]bool{}
	for _, a := range GlobalHotkeyActions {
		eligible[a] = true
	}
	var out []Hotkey
	for _, b := range bindings {
		if !eligible[b.Action] || b.Combo == "" || !hasModifier(b.Combo) {
			continue
		}
		out = append(out, b)
	}
	return out
}

func hasModifier(combo string) bool {
	for _, p := range strings.Split(combo, "+") {
		if isModifier(p) {
			return true
		}
	}
	return false
}

// parseCombo splits "mod+shift+q" into modifier list + main key.
func parseCombo(combo string) (mods []string, mainKey string) {
	parts := strings.Split(combo, "+")
	if len(parts) == 0 {
		return nil, ""
	}
	mainKey = parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		if isModifier(p) {
			mods = append(mods, p)
		}
	}
	return mods, mainKey
}
