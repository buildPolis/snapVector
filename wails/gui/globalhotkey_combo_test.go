package gui

import (
	"testing"
)

func TestParseCombo(t *testing.T) {
	tests := []struct {
		combo   string
		mods    []string
		mainKey string
	}{
		{"mod+shift+q", []string{"mod", "shift"}, "q"},
		{"alt+f4", []string{"alt"}, "f4"},
		{"mod+shift+e", []string{"mod", "shift"}, "e"},
		{"ctrl+c", []string{"ctrl"}, "c"},
		{"q", nil, "q"},
		{"", nil, ""},
	}
	for _, tt := range tests {
		mods, key := parseCombo(tt.combo)
		if key != tt.mainKey {
			t.Errorf("parseCombo(%q): mainKey=%q, want %q", tt.combo, key, tt.mainKey)
		}
		if len(mods) != len(tt.mods) {
			t.Errorf("parseCombo(%q): mods=%v, want %v", tt.combo, mods, tt.mods)
			continue
		}
		for i := range mods {
			if mods[i] != tt.mods[i] {
				t.Errorf("parseCombo(%q): mod[%d]=%q, want %q", tt.combo, i, mods[i], tt.mods[i])
			}
		}
	}
}

func TestFilterGlobalBindingsOnlyCapture(t *testing.T) {
	bindings := []Hotkey{
		{Action: "capture.fullscreen", Combo: "mod+shift+q"},
		{Action: "tool.arrow", Combo: "a"},
		{Action: "capture.region", Combo: "mod+shift+w"},
		{Action: "capture.allDisplays", Combo: "mod+shift+e"},
		{Action: "capture.fullscreen", Combo: ""},
		{Action: "capture.region", Combo: "q"},
	}
	out := filterGlobalBindings(bindings)
	if len(out) != 3 {
		t.Fatalf("filterGlobalBindings returned %d, want 3: %+v", len(out), out)
	}
	expected := []string{"capture.fullscreen", "capture.region", "capture.allDisplays"}
	for i, e := range expected {
		if out[i].Action != e {
			t.Errorf("out[%d].Action=%q, want %q", i, out[i].Action, e)
		}
	}
}

func TestHasModifier(t *testing.T) {
	if !hasModifier("mod+shift+q") {
		t.Error("expected mod+shift+q to have modifier")
	}
	if hasModifier("q") {
		t.Error("expected bare q to have no modifier")
	}
	if !hasModifier("alt+f4") {
		t.Error("expected alt+f4 to have modifier")
	}
}
