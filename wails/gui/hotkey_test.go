package gui

import (
	"strings"
	"testing"
)

func TestDefaultHotkeysCoversCatalog(t *testing.T) {
	defaults := DefaultHotkeys()
	if len(defaults) != len(ActionCatalog) {
		t.Fatalf("DefaultHotkeys len = %d, ActionCatalog len = %d", len(defaults), len(ActionCatalog))
	}
	seen := map[string]bool{}
	for i, h := range defaults {
		if h.Action != ActionCatalog[i] {
			t.Errorf("index %d action = %q, want %q", i, h.Action, ActionCatalog[i])
		}
		if h.Scope != "app" {
			t.Errorf("action %s scope = %q, want app", h.Action, h.Scope)
		}
		if seen[h.Action] {
			t.Errorf("duplicate action %s", h.Action)
		}
		seen[h.Action] = true
	}
}

func TestDefaultHotkeysNoInternalConflicts(t *testing.T) {
	byCombo := map[string]string{}
	for _, h := range DefaultHotkeys() {
		if h.Combo == "" {
			continue
		}
		if prev, ok := byCombo[h.Combo]; ok {
			t.Errorf("combo %q bound to both %s and %s", h.Combo, prev, h.Action)
		}
		byCombo[h.Combo] = h.Action
	}
}

func TestValidateCombo(t *testing.T) {
	cases := []struct {
		combo   string
		wantErr bool
	}{
		{"", false},
		{"v", false},
		{"mod+z", false},
		{"mod+shift+q", false},
		{"mod+,", false},
		{"mod+=", false},
		{"mod+", true},
		{"+z", true},
		{"Mod+z", true},
		{"mod+mod+z", true},
		{"z+mod", true},
		{"shift", true},
	}
	for _, c := range cases {
		err := ValidateCombo(c.combo)
		if c.wantErr != (err != nil) {
			t.Errorf("ValidateCombo(%q) err=%v, wantErr=%v", c.combo, err, c.wantErr)
		}
	}
}

func TestCanonicalComboSortsModifiers(t *testing.T) {
	cases := map[string]string{
		"shift+mod+z":     "mod+shift+z",
		"alt+ctrl+x":      "ctrl+alt+x",
		"shift+alt+mod+q": "mod+alt+shift+q",
		"mod+z":           "mod+z",
		"":                "",
	}
	for in, want := range cases {
		got, err := CanonicalCombo(in)
		if err != nil {
			t.Fatalf("CanonicalCombo(%q) err=%v", in, err)
		}
		if got != want {
			t.Errorf("CanonicalCombo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCanonicalComboStripsUnknownOnInvalid(t *testing.T) {
	if _, err := CanonicalCombo("MOD+z"); err == nil || !strings.Contains(err.Error(), "lowercase") {
		t.Fatalf("expected lowercase error, got %v", err)
	}
}
