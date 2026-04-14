package gui

import (
	"fmt"
	"sort"
	"strings"
)

type Hotkey struct {
	Action string `json:"action"`
	Combo  string `json:"combo"`
	Scope  string `json:"scope"`
}

type hotkeyFile struct {
	Version  int      `json:"version"`
	Bindings []Hotkey `json:"bindings"`
}

const hotkeyFileVersion = 1

// ActionCatalog is the closed set of actions that can receive hotkeys.
// Ordering drives the default Preferences modal grouping.
var ActionCatalog = []string{
	"tool.select", "tool.arrow", "tool.rectangle", "tool.ellipse",
	"tool.text", "tool.blur", "tool.crop",
	"edit.undo", "edit.redo",
	"file.open", "file.save", "file.saveAs",
	"view.zoomIn", "view.zoomOut", "view.zoomReset",
	"export.copy",
	"capture.fullscreen", "capture.region", "capture.allDisplays",
	"app.preferences",
}

func DefaultHotkeys() []Hotkey {
	defaults := map[string]string{
		"tool.select":          "v",
		"tool.arrow":           "a",
		"tool.rectangle":       "r",
		"tool.ellipse":         "o",
		"tool.text":            "t",
		"tool.blur":            "b",
		"tool.crop":            "c",
		"edit.undo":            "mod+z",
		"edit.redo":            "mod+shift+z",
		"file.open":            "mod+o",
		"file.save":            "mod+s",
		"file.saveAs":          "mod+shift+s",
		"view.zoomIn":          "mod+=",
		"view.zoomOut":         "mod+-",
		"view.zoomReset":       "mod+0",
		"export.copy":          "mod+shift+c",
		"capture.fullscreen":   "mod+shift+q",
		"capture.region":       "mod+shift+w",
		"capture.allDisplays":  "mod+shift+e",
		"app.preferences":      "mod+,",
	}
	out := make([]Hotkey, 0, len(ActionCatalog))
	for _, action := range ActionCatalog {
		out = append(out, Hotkey{Action: action, Combo: defaults[action], Scope: "app"})
	}
	return out
}

// ValidateCombo returns nil for "" (unbound) or a canonical combo string.
// It does NOT mutate; callers that want canonical form should call CanonicalCombo.
func ValidateCombo(combo string) error {
	if combo == "" {
		return nil
	}
	parts := strings.Split(combo, "+")
	seen := map[string]bool{}
	for i, p := range parts {
		if p == "" {
			return fmt.Errorf("empty segment at index %d", i)
		}
		if p != strings.ToLower(p) {
			return fmt.Errorf("segment %q must be lowercase", p)
		}
		if isModifier(p) {
			if i == len(parts)-1 {
				return fmt.Errorf("combo cannot end with modifier %q", p)
			}
			if seen[p] {
				return fmt.Errorf("duplicate modifier %q", p)
			}
			seen[p] = true
			continue
		}
		if i != len(parts)-1 {
			return fmt.Errorf("main key %q must be last segment", p)
		}
	}
	return nil
}

// CanonicalCombo returns the combo with modifiers in canonical order.
// Invalid combos propagate as errors.
func CanonicalCombo(combo string) (string, error) {
	if combo == "" {
		return "", nil
	}
	if err := ValidateCombo(combo); err != nil {
		return "", err
	}
	parts := strings.Split(combo, "+")
	main := parts[len(parts)-1]
	mods := parts[:len(parts)-1]
	sort.SliceStable(mods, func(i, j int) bool {
		return modifierRank(mods[i]) < modifierRank(mods[j])
	})
	return strings.Join(append(mods, main), "+"), nil
}

func isModifier(s string) bool {
	switch s {
	case "mod", "ctrl", "alt", "shift":
		return true
	}
	return false
}

func modifierRank(s string) int {
	switch s {
	case "mod":
		return 0
	case "ctrl":
		return 1
	case "alt":
		return 2
	case "shift":
		return 3
	}
	return 99
}
