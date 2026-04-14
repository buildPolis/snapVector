package gui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
		{"mod+ ", true},     // literal space in main key
		{"mod+\t", true},    // tab in main key
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

func TestCanonicalComboPropagatesValidationError(t *testing.T) {
	if _, err := CanonicalCombo("MOD+z"); err == nil || !strings.Contains(err.Error(), "lowercase") {
		t.Fatalf("expected lowercase error, got %v", err)
	}
}

func TestDefaultHotkeysAllValidate(t *testing.T) {
	for _, h := range DefaultHotkeys() {
		if err := ValidateCombo(h.Combo); err != nil {
			t.Errorf("%s: default combo %q invalid: %v", h.Action, h.Combo, err)
		}
	}
}

func newTestStore(t *testing.T) *HotkeyStore {
	t.Helper()
	return &HotkeyStore{configDir: t.TempDir()}
}

func TestStoreLoadReturnsDefaultsWhenMissing(t *testing.T) {
	s := newTestStore(t)
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if len(got) != len(ActionCatalog) {
		t.Fatalf("len = %d, want %d", len(got), len(ActionCatalog))
	}
}

func TestStoreSaveLoadRoundTrip(t *testing.T) {
	s := newTestStore(t)
	in := DefaultHotkeys()
	in[0].Combo = "shift+mod+v" // intentionally non-canonical to exercise sort
	if err := s.Save(in); err != nil {
		t.Fatalf("Save err = %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if got[0].Combo != "mod+shift+v" {
		t.Fatalf("combo = %q, want mod+shift+v (canonicalized on save)", got[0].Combo)
	}
}

func TestStoreSaveRejectsInvalidCombo(t *testing.T) {
	s := newTestStore(t)
	bad := DefaultHotkeys()
	bad[0].Combo = "Mod+z" // uppercase invalid
	err := s.Save(bad)
	if err == nil {
		t.Fatal("expected error from invalid combo, got nil")
	}
}

func TestStoreLoadFallsBackOnCorruptFile(t *testing.T) {
	s := newTestStore(t)
	path, _ := s.configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if len(got) != len(ActionCatalog) {
		t.Fatalf("expected defaults after corrupt fallback")
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("expected .corrupt backup, got %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected original removed, got %v", err)
	}
}

func TestStoreLoadDropsUnknownActionsAndFillsMissing(t *testing.T) {
	s := newTestStore(t)
	path, _ := s.configPath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	raw := `{"version":1,"bindings":[
		{"action":"tool.select","combo":"x","scope":"app"},
		{"action":"legacy.gone","combo":"mod+g","scope":"app"}
	]}`
	os.WriteFile(path, []byte(raw), 0o600)

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if got[0].Action != "tool.select" || got[0].Combo != "x" {
		t.Errorf("tool.select combo = %q, want x", got[0].Combo)
	}
	// Confirm another action kept its default (merge semantics):
	var redo *Hotkey
	for i := range got {
		if got[i].Action == "edit.redo" {
			redo = &got[i]
			break
		}
	}
	if redo == nil || redo.Combo != "mod+shift+z" {
		t.Errorf("edit.redo default not preserved, got %+v", redo)
	}
}

func TestStoreResetRemovesFile(t *testing.T) {
	s := newTestStore(t)
	path, _ := s.configPath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"version":1,"bindings":[]}`), 0o600)
	if _, err := s.Reset(); err != nil {
		t.Fatalf("Reset err = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected file removed, got %v", err)
	}
}
