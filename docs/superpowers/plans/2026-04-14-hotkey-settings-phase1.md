# Hotkey Settings Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship in-app hotkey customization for the Wails GUI — users open Preferences, record multi-key combos per action, resolve conflicts via reassign dialog, and persist to disk.

**Architecture:** All hotkey dispatch is pure frontend (keydown → canonical combo string → action function). Go side only persists JSON to `<UserConfigDir>/SnapVector/hotkeys.json` via atomic temp+rename, with a `.corrupt` fallback when the file is unreadable. A small `hotkey-utils.js` module exposes pure functions (normalize, conflict detect, display format) runnable in both browser and Node for unit tests.

**Tech Stack:** Go 1.22+ (stdlib only), vanilla JS (no bundler — `dist/` is loaded as plain script tags), Node built-in `--test` runner for frontend unit tests.

**Spec:** `docs/superpowers/specs/2026-04-14-hotkey-settings-design.md`

---

## File Structure

| File | Responsibility | Status |
|---|---|---|
| `wails/gui/hotkey.go` | `Hotkey` struct, `DefaultHotkeys()`, combo validator, action catalog | Create |
| `wails/gui/hotkey_store.go` | `LoadHotkeys` / `SaveHotkeys` / `ResetHotkeys` / `configPath`, atomic write, corrupt fallback | Create |
| `wails/gui/hotkey_test.go` | Unit tests for the two files above | Create |
| `wails/gui/app.go` | Add `GetHotkeys` / `SaveHotkeys` / `ResetHotkeys` / `DefaultHotkeys` bindings + injectable `configDir` seam for tests | Modify |
| `wails/gui/frontend/dist/hotkey-utils.js` | Pure functions: `normalize`, `sortModifiers`, `comboToDisplay`, `detectConflict`, `ACTION_CATALOG` | Create |
| `wails/gui/frontend/dist/hotkey-utils.test.js` | `node --test` suite for the utils | Create |
| `wails/gui/frontend/dist/index.html` | Preferences modal DOM, File menu entry | Modify |
| `wails/gui/frontend/dist/app.js` | `HotkeyManager`, `PreferencesModal`, wire keydown + action registry | Modify |
| `wails/gui/frontend/dist/styles.css` | Modal layout, recorder focus state | Modify |
| `wails/README.md` | Short note on config path & test commands | Modify |

---

## Conventions used by every task

- **Canonical combo**: lowercase, `+`-separated, fixed modifier order `mod → ctrl → alt → shift → <key>`. `mod` = `Cmd` on macOS (`metaKey`), `Ctrl` elsewhere (`ctrlKey`). Main key = `KeyboardEvent.key.toLowerCase()`.
- **Action IDs**: dotted strings (`tool.select`, `capture.fullscreen`, `app.preferences`).
- **Unbound**: empty string `""`.
- **Commit format**: imperative mood, scoped (`feat(hotkey):`, `test(hotkey):`). Sign-off trailer unchanged.

---

### Task 1: Hotkey types, defaults, and action catalog (Go)

**Files:**
- Create: `wails/gui/hotkey.go`

- [ ] **Step 1: Write `wails/gui/hotkey.go`**

```go
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
```

- [ ] **Step 2: Write initial tests for types and defaults**

Append to a new file `wails/gui/hotkey_test.go`:

```go
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
		{"mod+", true},       // trailing empty
		{"+z", true},         // leading empty
		{"Mod+z", true},      // uppercase rejected
		{"mod+mod+z", true},  // duplicate modifier
		{"z+mod", true},      // main key not last
		{"shift", true},      // modifier-only
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
		"shift+mod+z":  "mod+shift+z",
		"alt+ctrl+x":   "ctrl+alt+x",
		"shift+alt+mod+q": "mod+alt+shift+q",
		"mod+z":        "mod+z",
		"":             "",
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
```

- [ ] **Step 3: Run the tests**

```bash
cd wails && go test ./gui/ -run 'TestDefaultHotkeys|TestValidateCombo|TestCanonicalCombo' -v
```

Expected: all four tests PASS.

- [ ] **Step 4: Commit**

```bash
git add wails/gui/hotkey.go wails/gui/hotkey_test.go
git commit -m "$(cat <<'EOF'
feat(hotkey): add Hotkey types, defaults, and combo validator

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: HotkeyStore — file I/O with atomic write and corrupt fallback

**Files:**
- Create: `wails/gui/hotkey_store.go`
- Modify: `wails/gui/hotkey_test.go` (append store tests)

- [ ] **Step 1: Write `wails/gui/hotkey_store.go`**

```go
package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type HotkeyStore struct {
	// configDir, when empty, resolves via os.UserConfigDir at call time.
	// Tests inject a tempdir to avoid touching the real config path.
	configDir string
}

func NewHotkeyStore() *HotkeyStore { return &HotkeyStore{} }

func (s *HotkeyStore) configPath() (string, error) {
	base := s.configDir
	if base == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("locate user config dir: %w", err)
		}
		base = d
	}
	return filepath.Join(base, "SnapVector", "hotkeys.json"), nil
}

// Load returns stored hotkeys, or DefaultHotkeys() when the file is missing
// or unreadable. A corrupt file is renamed to hotkeys.json.corrupt so the
// user's data is preserved for debugging while the app keeps running.
func (s *HotkeyStore) Load() ([]Hotkey, error) {
	path, err := s.configPath()
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DefaultHotkeys(), nil
		}
		return nil, fmt.Errorf("read hotkeys: %w", err)
	}

	var file hotkeyFile
	if err := json.Unmarshal(raw, &file); err != nil {
		s.quarantine(path, raw)
		return DefaultHotkeys(), nil
	}
	if file.Version != hotkeyFileVersion {
		s.quarantine(path, raw)
		return DefaultHotkeys(), nil
	}

	// Merge: start from defaults, overwrite with stored values so new actions
	// added in later releases still appear in the UI.
	out := DefaultHotkeys()
	byAction := map[string]*Hotkey{}
	for i := range out {
		byAction[out[i].Action] = &out[i]
	}
	for _, h := range file.Bindings {
		if _, ok := byAction[h.Action]; !ok {
			continue // action no longer exists; drop it
		}
		canonical, err := CanonicalCombo(h.Combo)
		if err != nil {
			// Invalid combo in file → keep default for this action.
			continue
		}
		byAction[h.Action].Combo = canonical
		byAction[h.Action].Scope = "app"
	}
	return out, nil
}

// Save writes bindings atomically (temp + rename). Returns an error if any
// combo fails validation, so the caller can surface the message verbatim.
func (s *HotkeyStore) Save(bindings []Hotkey) error {
	path, err := s.configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir hotkey dir: %w", err)
	}

	canonical := make([]Hotkey, 0, len(bindings))
	for _, h := range bindings {
		c, err := CanonicalCombo(h.Combo)
		if err != nil {
			return fmt.Errorf("invalid combo for %s: %w", h.Action, err)
		}
		canonical = append(canonical, Hotkey{Action: h.Action, Combo: c, Scope: "app"})
	}

	payload, err := json.MarshalIndent(hotkeyFile{
		Version:  hotkeyFileVersion,
		Bindings: canonical,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hotkeys: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "hotkeys-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// Reset deletes the config file (if present) and returns the defaults.
func (s *HotkeyStore) Reset() ([]Hotkey, error) {
	path, err := s.configPath()
	if err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("remove hotkeys: %w", err)
	}
	return DefaultHotkeys(), nil
}

func (s *HotkeyStore) quarantine(path string, raw []byte) {
	_ = os.WriteFile(path+".corrupt", raw, 0o600)
	_ = os.Remove(path)
}
```

- [ ] **Step 2: Append store tests to `wails/gui/hotkey_test.go`**

```go
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
```

Add these imports at the top of `hotkey_test.go` if not already present: `"errors"`, `"io/fs"`, `"os"`, `"path/filepath"`.

- [ ] **Step 3: Run the full gui package tests**

```bash
cd wails && go test ./gui/ -v
```

Expected: all existing tests still pass, plus the 6 new Store tests.

- [ ] **Step 4: Commit**

```bash
git add wails/gui/hotkey_store.go wails/gui/hotkey_test.go
git commit -m "$(cat <<'EOF'
feat(hotkey): add HotkeyStore with atomic write and corrupt fallback

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Wire hotkey store into App bindings

**Files:**
- Modify: `wails/gui/app.go`
- Modify: `wails/gui/app_test.go` (append binding tests)

- [ ] **Step 1: Add store field and bindings to `wails/gui/app.go`**

Edit `App` struct (around line 21-34) to append after `postCaptureHold`:

```go
	hotkeyStore     *HotkeyStore
```

Edit `NewApp()` (around line 66-80) to append before the closing brace:

```go
		hotkeyStore:     NewHotkeyStore(),
```

Append these four methods at the end of `app.go` (after `fileDialogFilters`):

```go
func (a *App) GetHotkeys() ([]Hotkey, error) {
	return a.hotkeyStore.Load()
}

func (a *App) SaveHotkeys(bindings []Hotkey) error {
	return a.hotkeyStore.Save(bindings)
}

func (a *App) ResetHotkeys() ([]Hotkey, error) {
	return a.hotkeyStore.Reset()
}

func (a *App) DefaultHotkeys() []Hotkey {
	return DefaultHotkeys()
}
```

- [ ] **Step 2: Append binding tests to `wails/gui/app_test.go`**

```go
func TestAppGetHotkeysReturnsDefaultsFirstRun(t *testing.T) {
	app := NewApp()
	app.hotkeyStore = &HotkeyStore{configDir: t.TempDir()}

	got, err := app.GetHotkeys()
	if err != nil {
		t.Fatalf("GetHotkeys err = %v", err)
	}
	if len(got) != len(ActionCatalog) {
		t.Fatalf("len = %d, want %d", len(got), len(ActionCatalog))
	}
}

func TestAppSaveHotkeysPersistsAndReloads(t *testing.T) {
	app := NewApp()
	app.hotkeyStore = &HotkeyStore{configDir: t.TempDir()}

	in := DefaultHotkeys()
	in[0].Combo = "mod+alt+v"
	if err := app.SaveHotkeys(in); err != nil {
		t.Fatalf("SaveHotkeys err = %v", err)
	}
	got, err := app.GetHotkeys()
	if err != nil {
		t.Fatalf("GetHotkeys err = %v", err)
	}
	if got[0].Combo != "mod+alt+v" {
		t.Fatalf("combo = %q, want mod+alt+v", got[0].Combo)
	}
}

func TestAppResetHotkeysDeletesFile(t *testing.T) {
	app := NewApp()
	app.hotkeyStore = &HotkeyStore{configDir: t.TempDir()}

	app.SaveHotkeys(DefaultHotkeys())
	if _, err := app.ResetHotkeys(); err != nil {
		t.Fatalf("ResetHotkeys err = %v", err)
	}
}
```

- [ ] **Step 3: Run the full gui tests**

```bash
cd wails && go test ./gui/ -v
```

Expected: all tests pass; 3 new App tests included.

- [ ] **Step 4: Commit**

```bash
git add wails/gui/app.go wails/gui/app_test.go
git commit -m "$(cat <<'EOF'
feat(hotkey): bind Get/Save/Reset/Default hotkey methods on App

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: `hotkey-utils.js` — pure frontend functions + Node tests

**Files:**
- Create: `wails/gui/frontend/dist/hotkey-utils.js`
- Create: `wails/gui/frontend/dist/hotkey-utils.test.js`

Design note: UMD-style so the same file works as a `<script>` (exposes `window.SV_Hotkey`) and as `require(...)` in Node tests.

- [ ] **Step 1: Write `hotkey-utils.js`**

```js
(function (root, factory) {
  if (typeof module === "object" && module.exports) {
    module.exports = factory();
  } else {
    root.SV_Hotkey = factory();
  }
})(typeof self !== "undefined" ? self : this, function () {
  const MODIFIER_RANK = { mod: 0, ctrl: 1, alt: 2, shift: 3 };
  const CONTROL_KEYS = new Set(["escape", "enter", "backspace", "tab"]);
  const DISPLAY_MAC = { mod: "⌘", ctrl: "⌃", alt: "⌥", shift: "⇧", enter: "⏎", backspace: "⌫", escape: "⎋", arrowup: "↑", arrowdown: "↓", arrowleft: "←", arrowright: "→" };
  const DISPLAY_OTHER = { mod: "Ctrl", ctrl: "Ctrl", alt: "Alt", shift: "Shift" };

  // normalize converts a KeyboardEvent-like {key, metaKey, ctrlKey, altKey, shiftKey}
  // into a canonical combo string, or "" if only modifiers are pressed.
  // isMac controls whether metaKey maps to "mod" (mac) vs ctrlKey → "mod" (others).
  function normalize(event, isMac) {
    const key = (event.key || "").toLowerCase();
    if (!key || key === "meta" || key === "control" || key === "alt" || key === "shift") {
      return "";
    }
    const mods = [];
    if (isMac) {
      if (event.metaKey) mods.push("mod");
      if (event.ctrlKey) mods.push("ctrl");
    } else {
      if (event.ctrlKey || event.metaKey) mods.push("mod");
    }
    if (event.altKey) mods.push("alt");
    if (event.shiftKey) mods.push("shift");
    return sortModifiers([...mods, key].join("+"));
  }

  function sortModifiers(combo) {
    if (!combo) return "";
    const parts = combo.split("+");
    if (parts.length === 1) return parts[0];
    const main = parts[parts.length - 1];
    const mods = parts.slice(0, -1);
    mods.sort((a, b) => (MODIFIER_RANK[a] ?? 99) - (MODIFIER_RANK[b] ?? 99));
    return [...mods, main].join("+");
  }

  function comboToDisplay(combo, isMac) {
    if (!combo) return "Unbound";
    const parts = combo.split("+");
    if (isMac) {
      return parts
        .map((p) => DISPLAY_MAC[p] || (p.length === 1 ? p.toUpperCase() : capitalize(p)))
        .join(" ");
    }
    return parts
      .map((p) => DISPLAY_OTHER[p] || (p.length === 1 ? p.toUpperCase() : capitalize(p)))
      .join("+");
  }

  // detectConflict: given current bindings and a proposed (action, combo),
  // returns the conflicting action's name, or null if no conflict.
  function detectConflict(bindings, action, combo) {
    if (!combo) return null;
    for (const b of bindings) {
      if (b.action === action) continue;
      if (b.combo === combo) return b.action;
    }
    return null;
  }

  // isControlKey returns true if the key alone should control the recorder
  // (Enter/Esc/Backspace) rather than be captured as a binding's main key.
  function isControlKey(key) {
    return CONTROL_KEYS.has((key || "").toLowerCase());
  }

  // isRecordableMainKey returns false for modifier-only inputs and control keys
  // pressed without any modifier. Control keys WITH modifiers are recordable.
  function isRecordableMainKey(event) {
    const key = (event.key || "").toLowerCase();
    if (!key || key === "meta" || key === "control" || key === "alt" || key === "shift") {
      return false;
    }
    const hasMod = event.metaKey || event.ctrlKey || event.altKey || event.shiftKey;
    if (isControlKey(key) && !hasMod) return false;
    return true;
  }

  function capitalize(s) {
    return s.charAt(0).toUpperCase() + s.slice(1);
  }

  return { normalize, sortModifiers, comboToDisplay, detectConflict, isControlKey, isRecordableMainKey };
});
```

- [ ] **Step 2: Write `hotkey-utils.test.js`**

```js
const test = require("node:test");
const assert = require("node:assert/strict");
const { normalize, sortModifiers, comboToDisplay, detectConflict, isRecordableMainKey } = require("./hotkey-utils.js");

function ev(key, opts = {}) {
  return { key, metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, ...opts };
}

test("normalize: mac metaKey becomes mod", () => {
  assert.equal(normalize(ev("z", { metaKey: true }), true), "mod+z");
});

test("normalize: non-mac ctrlKey becomes mod", () => {
  assert.equal(normalize(ev("z", { ctrlKey: true }), false), "mod+z");
});

test("normalize: non-mac metaKey (Super/Win) also maps to mod", () => {
  assert.equal(normalize(ev("z", { metaKey: true }), false), "mod+z");
});

test("normalize: mac ctrlKey is distinct from mod", () => {
  assert.equal(normalize(ev("z", { ctrlKey: true }), true), "ctrl+z");
});

test("normalize: modifier order is canonical", () => {
  assert.equal(normalize(ev("Q", { metaKey: true, shiftKey: true, altKey: true }), true), "mod+alt+shift+q");
});

test("normalize: modifier-only returns empty", () => {
  assert.equal(normalize(ev("Meta", { metaKey: true }), true), "");
  assert.equal(normalize(ev("Shift", { shiftKey: true }), true), "");
});

test("sortModifiers is idempotent", () => {
  assert.equal(sortModifiers("mod+shift+z"), "mod+shift+z");
  assert.equal(sortModifiers("shift+mod+z"), "mod+shift+z");
  assert.equal(sortModifiers("z"), "z");
  assert.equal(sortModifiers(""), "");
});

test("comboToDisplay mac", () => {
  assert.equal(comboToDisplay("mod+shift+q", true), "⌘ ⇧ Q");
  assert.equal(comboToDisplay("v", true), "V");
  assert.equal(comboToDisplay("", true), "Unbound");
});

test("comboToDisplay non-mac", () => {
  assert.equal(comboToDisplay("mod+shift+q", false), "Ctrl+Shift+Q");
  assert.equal(comboToDisplay("v", false), "V");
});

test("detectConflict finds other action", () => {
  const bindings = [
    { action: "tool.select", combo: "v" },
    { action: "edit.undo", combo: "mod+z" },
  ];
  assert.equal(detectConflict(bindings, "tool.arrow", "mod+z"), "edit.undo");
  assert.equal(detectConflict(bindings, "tool.arrow", "a"), null);
});

test("detectConflict ignores same action", () => {
  const bindings = [{ action: "edit.undo", combo: "mod+z" }];
  assert.equal(detectConflict(bindings, "edit.undo", "mod+z"), null);
});

test("detectConflict ignores empty combo", () => {
  const bindings = [{ action: "tool.select", combo: "" }];
  assert.equal(detectConflict(bindings, "tool.arrow", ""), null);
});

test("isRecordableMainKey rejects modifier-only and bare control keys", () => {
  assert.equal(isRecordableMainKey(ev("Meta", { metaKey: true })), false);
  assert.equal(isRecordableMainKey(ev("Escape")), false);
  assert.equal(isRecordableMainKey(ev("Enter")), false);
  assert.equal(isRecordableMainKey(ev("Enter", { metaKey: true })), true); // with modifier = recordable
  assert.equal(isRecordableMainKey(ev("v")), true);
});
```

- [ ] **Step 3: Run the tests**

```bash
node --test wails/gui/frontend/dist/hotkey-utils.test.js
```

Expected: `# pass N` for all tests with no failures.

- [ ] **Step 4: Commit**

```bash
git add wails/gui/frontend/dist/hotkey-utils.js wails/gui/frontend/dist/hotkey-utils.test.js
git commit -m "$(cat <<'EOF'
feat(hotkey): add pure frontend util module with node --test coverage

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Preferences modal DOM + File menu entry

**Files:**
- Modify: `wails/gui/frontend/dist/index.html`
- Modify: `wails/gui/frontend/dist/styles.css`

- [ ] **Step 1: Add Preferences menu entry in the File dropdown**

In `index.html`, locate the `<div id="fileMenu">` block (around line 26-30) and append a new button after `saveDocumentAsButton`:

```html
                <div class="menu-separator" role="separator"></div>
                <button type="button" id="preferencesButton" class="menu-item" role="menuitem">Preferences…</button>
```

- [ ] **Step 2: Add the modal markup**

Before the closing `</body>` in `index.html` (above `<script src="./app.js"></script>`), append:

```html
    <div id="preferencesModal" class="modal-overlay is-hidden" role="dialog" aria-modal="true" aria-labelledby="preferencesTitle">
      <div class="modal-panel" role="document">
        <header class="modal-header">
          <h2 id="preferencesTitle">Preferences › Hotkeys</h2>
          <button type="button" id="preferencesClose" class="icon-btn" aria-label="Close preferences">✕</button>
        </header>
        <div class="modal-toolbar">
          <input type="search" id="preferencesFilter" class="modal-filter" placeholder="Filter actions or hotkeys…" />
          <button type="button" id="preferencesResetAll" class="ghost-btn">Reset all defaults</button>
        </div>
        <div class="modal-body" id="preferencesBody"></div>
        <footer class="modal-footer">
          <span id="preferencesStatus" class="modal-status"></span>
          <div class="modal-actions">
            <button type="button" id="preferencesCancel" class="ghost-btn">Cancel</button>
            <button type="button" id="preferencesSave" class="ghost-btn is-primary">Save</button>
          </div>
        </footer>
      </div>
      <div id="preferencesConflict" class="conflict-popover is-hidden" role="alertdialog" aria-modal="true"></div>
    </div>
```

Also load the utils script **before** app.js. Replace `<script src="./app.js"></script>` with:

```html
    <script src="./hotkey-utils.js"></script>
    <script src="./app.js"></script>
```

- [ ] **Step 3: Append modal styles to `styles.css`**

Append to the bottom of `styles.css`:

```css
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(10, 14, 24, 0.58);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
}
.modal-overlay.is-hidden { display: none; }
.modal-panel {
  background: var(--panel, #161b28);
  color: var(--text, #e6ebf5);
  width: min(720px, 92vw);
  max-height: 86vh;
  border-radius: 12px;
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.45);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}
.modal-header h2 { font-size: 16px; margin: 0; }
.modal-toolbar {
  display: flex;
  gap: 12px;
  padding: 12px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.modal-filter {
  flex: 1;
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.04);
  color: inherit;
}
.modal-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px 20px 20px;
}
.modal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}
.modal-status { font-size: 12px; opacity: 0.75; }
.modal-status.is-error { color: #ff7b7b; opacity: 1; }
.modal-actions { display: flex; gap: 8px; }
.ghost-btn.is-primary {
  background: #3d6fff;
  color: white;
  border-color: transparent;
}
.menu-separator {
  height: 1px;
  background: rgba(255, 255, 255, 0.08);
  margin: 4px 0;
}
.hotkey-group { margin-top: 16px; }
.hotkey-group-title {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  opacity: 0.6;
  margin: 0 0 8px;
}
.hotkey-row {
  display: grid;
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: 12px;
  padding: 6px 8px;
  border-radius: 6px;
}
.hotkey-row:hover { background: rgba(255, 255, 255, 0.03); }
.hotkey-field {
  min-width: 160px;
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.04);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  cursor: pointer;
  text-align: center;
}
.hotkey-field.is-recording {
  border-color: #3d6fff;
  background: rgba(61, 111, 255, 0.18);
  color: #9eb8ff;
}
.hotkey-field.is-unbound { opacity: 0.5; }
.hotkey-clear {
  background: none;
  border: 1px solid transparent;
  color: inherit;
  cursor: pointer;
  border-radius: 4px;
  padding: 2px 6px;
  opacity: 0.6;
}
.hotkey-clear:hover { opacity: 1; background: rgba(255, 255, 255, 0.08); }
.conflict-popover {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: var(--panel, #1c2230);
  border: 1px solid rgba(255, 177, 66, 0.5);
  border-radius: 10px;
  padding: 18px 20px;
  max-width: 420px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
}
.conflict-popover.is-hidden { display: none; }
.conflict-popover p { margin: 0 0 12px; line-height: 1.45; }
.conflict-popover .conflict-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
```

- [ ] **Step 4: Visual smoke test**

```bash
cd wails && wails dev
```

Manually verify: File menu shows `Preferences…`. Clicking does nothing yet (no handler wired) — expected. Hit Ctrl+C to stop.

- [ ] **Step 5: Commit**

```bash
git add wails/gui/frontend/dist/index.html wails/gui/frontend/dist/styles.css
git commit -m "$(cat <<'EOF'
feat(hotkey): scaffold Preferences modal DOM and styles

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: HotkeyManager — load, action registry, keydown dispatch

**Files:**
- Modify: `wails/gui/frontend/dist/app.js`

- [ ] **Step 1: Add module-level state and platform detection**

Near the top of `app.js` (after `const state = { ... }` ends around line 35, before `const els = { ... }`), insert:

```js
const IS_MAC = /mac|iphone|ipad|ipod/i.test(navigator.platform);

const hotkeys = {
  bindings: [],              // Array<{action, combo, scope}>
  comboToAction: new Map(),  // combo → action
  suspended: false,          // true while modal is recording
};
```

- [ ] **Step 2: Add action registry function**

Append this function at the bottom of `app.js` (before the final closing — after `numberValue`):

```js
function hotkeyActions() {
  return {
    "tool.select":          () => setTool("select"),
    "tool.arrow":           () => setTool("arrow"),
    "tool.rectangle":       () => setTool("rectangle"),
    "tool.ellipse":         () => setTool("ellipse"),
    "tool.text":            () => setTool("text"),
    "tool.blur":            () => setTool("blur"),
    "tool.crop":            () => setTool("crop"),
    "edit.undo":            () => undo(),
    "edit.redo":            () => redo(),
    "file.open":            () => openDocument(),
    "file.save":            () => saveDocument(),
    "file.saveAs":          () => saveDocumentAs(),
    "view.zoomIn":          () => changeZoom(0.1),
    "view.zoomOut":         () => changeZoom(-0.1),
    "view.zoomReset":       () => { state.zoom = 1; state.zoomAutoFit = false; state.pan = { x: 0, y: 0 }; render(); },
    "export.copy":          () => exportCurrent(true),
    "capture.fullscreen":   () => captureScreen("fullscreen"),
    "capture.region":       () => captureScreen("region"),
    "capture.allDisplays":  () => captureScreen("all-displays"),
    "app.preferences":      () => openPreferences(),
  };
}
```

- [ ] **Step 3: Add load + rebuild + dispatch helpers**

Append after `hotkeyActions`:

```js
async function loadHotkeys() {
  try {
    const bindings = await backend.getHotkeys();
    applyHotkeyBindings(bindings);
  } catch (err) {
    console.warn("loadHotkeys failed, falling back to defaults:", err);
    applyHotkeyBindings(defaultHotkeyBindings());
  }
}

function applyHotkeyBindings(bindings) {
  hotkeys.bindings = bindings.slice();
  hotkeys.comboToAction = new Map();
  for (const b of bindings) {
    if (b.combo) hotkeys.comboToAction.set(b.combo, b.action);
  }
}

function defaultHotkeyBindings() {
  // Mirrors gui/hotkey.go DefaultHotkeys(). Used only when backend is absent
  // (e.g. the mock path when running pure HTML). Keep in sync manually.
  return [
    { action: "tool.select", combo: "v", scope: "app" },
    { action: "tool.arrow", combo: "a", scope: "app" },
    { action: "tool.rectangle", combo: "r", scope: "app" },
    { action: "tool.ellipse", combo: "o", scope: "app" },
    { action: "tool.text", combo: "t", scope: "app" },
    { action: "tool.blur", combo: "b", scope: "app" },
    { action: "tool.crop", combo: "c", scope: "app" },
    { action: "edit.undo", combo: "mod+z", scope: "app" },
    { action: "edit.redo", combo: "mod+shift+z", scope: "app" },
    { action: "file.open", combo: "mod+o", scope: "app" },
    { action: "file.save", combo: "mod+s", scope: "app" },
    { action: "file.saveAs", combo: "mod+shift+s", scope: "app" },
    { action: "view.zoomIn", combo: "mod+=", scope: "app" },
    { action: "view.zoomOut", combo: "mod+-", scope: "app" },
    { action: "view.zoomReset", combo: "mod+0", scope: "app" },
    { action: "export.copy", combo: "mod+shift+c", scope: "app" },
    { action: "capture.fullscreen", combo: "mod+shift+q", scope: "app" },
    { action: "capture.region", combo: "mod+shift+w", scope: "app" },
    { action: "capture.allDisplays", combo: "mod+shift+e", scope: "app" },
    { action: "app.preferences", combo: "mod+,", scope: "app" },
  ];
}

function isTypingTarget(target) {
  if (!target) return false;
  if (target.isContentEditable) return true;
  const tag = (target.tagName || "").toUpperCase();
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}

function onGlobalKeydown(event) {
  if (hotkeys.suspended) return;
  if (event.isComposing) return;
  if (isTypingTarget(event.target)) return;
  const combo = SV_Hotkey.normalize(event, IS_MAC);
  if (!combo) return;
  const action = hotkeys.comboToAction.get(combo);
  if (!action) return;
  const handler = hotkeyActions()[action];
  if (!handler) return;
  event.preventDefault();
  handler();
}
```

- [ ] **Step 4: Extend `createBackend` with hotkey methods**

Inside `createBackend` (around line 1163), add to the real branch (after `exportDocument` line) and the mock branch.

Real branch (insert before closing `}`):

```js
      getHotkeys: () => window.go.gui.App.GetHotkeys(),
      saveHotkeys: (bindings) => window.go.gui.App.SaveHotkeys(bindings),
      resetHotkeys: () => window.go.gui.App.ResetHotkeys(),
```

Mock branch (insert inside the returned mock object):

```js
    async getHotkeys() {
      return mockHotkeys.length ? mockHotkeys.slice() : defaultHotkeyBindings();
    },
    async saveHotkeys(bindings) {
      mockHotkeys = bindings.slice();
    },
    async resetHotkeys() {
      mockHotkeys = [];
      return defaultHotkeyBindings();
    },
```

Also add at the top of the mock branch (after `let mockCounter = 1;`):

```js
  let mockHotkeys = [];
```

- [ ] **Step 5: Add `openPreferences` stub and wire init**

Append a stub for now (full modal logic comes in Task 7):

```js
function openPreferences() {
  const modal = document.getElementById("preferencesModal");
  if (modal) modal.classList.remove("is-hidden");
}
```

Modify `init()` (around line 84) to load hotkeys and register listener. Change:

```js
async function init() {
  bindUI();
  render();
}
```

to:

```js
async function init() {
  bindUI();
  await loadHotkeys();
  window.addEventListener("keydown", onGlobalKeydown);
  render();
}
```

- [ ] **Step 6: Smoke test via wails dev**

```bash
cd wails && wails dev
```

Manually verify:
1. Press `v`, `a`, `r`, `o`, `t`, `b`, `c` with focus on canvas — tool rail switches.
2. Capture a screen first (so undo stack has content), then press `Cmd+Z` / `Ctrl+Z` — action invokes (watch the inspector/canvas).
3. Focus the inspector's text input, press `v` — it inserts `v` in the text box, tool does NOT switch.

Stop with Ctrl+C.

- [ ] **Step 7: Commit**

```bash
git add wails/gui/frontend/dist/app.js
git commit -m "$(cat <<'EOF'
feat(hotkey): load bindings, dispatch keydown, skip typing targets

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Preferences modal — open, render table, cancel, save

**Files:**
- Modify: `wails/gui/frontend/dist/app.js`

- [ ] **Step 1: Add modal state**

Near the top of `app.js` (beside `hotkeys` object from Task 6), add:

```js
const prefs = {
  draft: [],           // working copy during modal session
  dirty: false,
  recordingAction: null,
  recordingBuffer: "", // combo-in-progress during recording
};
```

- [ ] **Step 2: Extend `els` with modal refs**

Append to the `els` object (around line 37-80):

```js
  preferencesButton: document.getElementById("preferencesButton"),
  preferencesModal: document.getElementById("preferencesModal"),
  preferencesBody: document.getElementById("preferencesBody"),
  preferencesClose: document.getElementById("preferencesClose"),
  preferencesCancel: document.getElementById("preferencesCancel"),
  preferencesSave: document.getElementById("preferencesSave"),
  preferencesResetAll: document.getElementById("preferencesResetAll"),
  preferencesFilter: document.getElementById("preferencesFilter"),
  preferencesStatus: document.getElementById("preferencesStatus"),
  preferencesConflict: document.getElementById("preferencesConflict"),
```

- [ ] **Step 3: Replace `openPreferences` stub with real implementation**

Replace the stub from Task 6 with:

```js
const ACTION_LABELS = {
  "tool.select": "Select tool",
  "tool.arrow": "Arrow tool",
  "tool.rectangle": "Rectangle tool",
  "tool.ellipse": "Ellipse tool",
  "tool.text": "Text tool",
  "tool.blur": "Blur tool",
  "tool.crop": "Crop tool",
  "edit.undo": "Undo",
  "edit.redo": "Redo",
  "file.open": "Open document",
  "file.save": "Save document",
  "file.saveAs": "Save document as…",
  "view.zoomIn": "Zoom in",
  "view.zoomOut": "Zoom out",
  "view.zoomReset": "Reset zoom",
  "export.copy": "Copy to clipboard",
  "capture.fullscreen": "Capture full screen",
  "capture.region": "Capture region",
  "capture.allDisplays": "Capture all displays",
  "app.preferences": "Open Preferences",
};

const ACTION_GROUPS = [
  { title: "Tools", actions: ["tool.select", "tool.arrow", "tool.rectangle", "tool.ellipse", "tool.text", "tool.blur", "tool.crop"] },
  { title: "Editing", actions: ["edit.undo", "edit.redo"] },
  { title: "File", actions: ["file.open", "file.save", "file.saveAs"] },
  { title: "View", actions: ["view.zoomIn", "view.zoomOut", "view.zoomReset"] },
  { title: "Export", actions: ["export.copy"] },
  { title: "Capture", actions: ["capture.fullscreen", "capture.region", "capture.allDisplays"] },
  { title: "App", actions: ["app.preferences"] },
];

function openPreferences() {
  prefs.draft = hotkeys.bindings.map((b) => ({ ...b }));
  prefs.dirty = false;
  prefs.recordingAction = null;
  prefs.recordingBuffer = "";
  els.preferencesStatus.textContent = "";
  els.preferencesStatus.classList.remove("is-error");
  els.preferencesFilter.value = "";
  els.preferencesModal.classList.remove("is-hidden");
  closeFileMenu();
  renderPreferences();
}

function closePreferences(force = false) {
  if (!force && prefs.dirty) {
    const ok = confirm("You have unsaved hotkey changes. Discard them?");
    if (!ok) return;
  }
  prefs.recordingAction = null;
  hotkeys.suspended = false;
  els.preferencesModal.classList.add("is-hidden");
  els.preferencesConflict.classList.add("is-hidden");
}

function renderPreferences() {
  const filter = els.preferencesFilter.value.trim().toLowerCase();
  const byAction = new Map(prefs.draft.map((b) => [b.action, b]));
  els.preferencesBody.innerHTML = "";
  for (const group of ACTION_GROUPS) {
    const rows = group.actions
      .map((action) => ({ action, binding: byAction.get(action) }))
      .filter(({ action, binding }) => {
        if (!filter) return true;
        const label = (ACTION_LABELS[action] || action).toLowerCase();
        const combo = (binding?.combo || "").toLowerCase();
        return label.includes(filter) || combo.includes(filter);
      });
    if (!rows.length) continue;
    const groupEl = document.createElement("div");
    groupEl.className = "hotkey-group";
    groupEl.innerHTML = `<h3 class="hotkey-group-title">${group.title}</h3>`;
    for (const { action, binding } of rows) {
      const row = document.createElement("div");
      row.className = "hotkey-row";
      const label = document.createElement("span");
      label.textContent = ACTION_LABELS[action] || action;
      const field = document.createElement("button");
      field.type = "button";
      field.className = "hotkey-field";
      field.dataset.action = action;
      updateFieldDisplay(field, binding.combo);
      field.addEventListener("click", () => startRecording(action, field));
      const clear = document.createElement("button");
      clear.type = "button";
      clear.className = "hotkey-clear";
      clear.textContent = "✕";
      clear.title = "Clear hotkey";
      clear.addEventListener("click", () => clearBinding(action));
      row.append(label, field, clear);
      groupEl.append(row);
    }
    els.preferencesBody.append(groupEl);
  }
}

function updateFieldDisplay(fieldEl, combo) {
  fieldEl.textContent = combo ? SV_Hotkey.comboToDisplay(combo, IS_MAC) : "Unbound";
  fieldEl.classList.toggle("is-unbound", !combo);
  fieldEl.classList.remove("is-recording");
}

function startRecording(action, fieldEl) {
  // cancel any previous recording first
  if (prefs.recordingAction) cancelRecording();
  prefs.recordingAction = action;
  prefs.recordingBuffer = "";
  hotkeys.suspended = true;
  fieldEl.classList.add("is-recording");
  fieldEl.textContent = "Press keys…";
}

function cancelRecording() {
  if (!prefs.recordingAction) return;
  prefs.recordingAction = null;
  hotkeys.suspended = false;
  renderPreferences();
}

function clearBinding(action) {
  setDraftCombo(action, "");
}

function setDraftCombo(action, combo) {
  const row = prefs.draft.find((b) => b.action === action);
  if (!row) return;
  if (row.combo === combo) return;
  row.combo = combo;
  prefs.dirty = true;
  renderPreferences();
}

async function savePreferences() {
  try {
    await backend.saveHotkeys(prefs.draft);
    applyHotkeyBindings(prefs.draft);
    prefs.dirty = false;
    closePreferences(true);
    showToast("已儲存熱鍵設定");
  } catch (err) {
    els.preferencesStatus.textContent = `儲存失敗：${err?.message || err}`;
    els.preferencesStatus.classList.add("is-error");
  }
}
```

- [ ] **Step 4: Wire modal buttons in `bindUI`**

Add to `bindUI()` (around the end, before `bindInspector()`):

```js
  els.preferencesButton.addEventListener("click", () => {
    closeFileMenu();
    openPreferences();
  });
  els.preferencesClose.addEventListener("click", () => closePreferences());
  els.preferencesCancel.addEventListener("click", () => closePreferences());
  els.preferencesSave.addEventListener("click", () => savePreferences());
  els.preferencesFilter.addEventListener("input", () => renderPreferences());
```

- [ ] **Step 5: Smoke test**

```bash
cd wails && wails dev
```

Manually verify:
1. File → `Preferences…` opens modal with all actions grouped.
2. Filter for `undo` narrows the list.
3. Cancel closes modal; no prompt if no changes.
4. Reopen; click a field — it shows "Press keys…" (recording logic comes next task, so pressing keys won't yet capture; click another field to cancel).
5. Click ✕ next to Undo → row updates to `Unbound`. Click Cancel → prompts discard.

Stop with Ctrl+C.

- [ ] **Step 6: Commit**

```bash
git add wails/gui/frontend/dist/app.js
git commit -m "$(cat <<'EOF'
feat(hotkey): open Preferences modal with grouped table and save/cancel

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Recorder — capture combo, commit, cancel, clear

**Files:**
- Modify: `wails/gui/frontend/dist/app.js`

- [ ] **Step 1: Add recorder keydown handler**

Append these functions to `app.js` (after `savePreferences`):

```js
function onRecorderKeydown(event) {
  if (!prefs.recordingAction) return;
  // Stop both preventDefault and propagation so onGlobalKeydown (bubble phase)
  // never sees this event — otherwise committing a combo could immediately
  // dispatch the previously-bound action for the same keystroke.
  event.preventDefault();
  event.stopPropagation();

  // These keys, pressed alone, control the recorder itself.
  if (!event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey) {
    if (event.key === "Escape") {
      cancelRecording();
      return;
    }
    if (event.key === "Backspace" || event.key === "Delete") {
      const action = prefs.recordingAction;
      prefs.recordingAction = null;
      hotkeys.suspended = false;
      setDraftCombo(action, "");
      return;
    }
  }

  if (!SV_Hotkey.isRecordableMainKey(event)) {
    // Ignore modifier-only presses (wait for a main key).
    return;
  }

  const combo = SV_Hotkey.normalize(event, IS_MAC);
  if (!combo) return;
  commitRecording(combo);
}

function commitRecording(combo) {
  const action = prefs.recordingAction;
  if (!action) return;
  const conflict = SV_Hotkey.detectConflict(prefs.draft, action, combo);
  if (!conflict) {
    prefs.recordingAction = null;
    hotkeys.suspended = false;
    setDraftCombo(action, combo);
    return;
  }
  showConflictDialog(action, conflict, combo);
}
```

- [ ] **Step 2: Add placeholder conflict dialog**

(Real dialog comes in Task 9. For now, stub it so recording works end-to-end.)

```js
function showConflictDialog(action, conflictAction, combo) {
  // Placeholder: auto-reassign without prompt (Task 9 replaces this).
  reassignCombo(action, conflictAction, combo);
}

function reassignCombo(action, conflictAction, combo) {
  const conflictRow = prefs.draft.find((b) => b.action === conflictAction);
  if (conflictRow) conflictRow.combo = "";
  prefs.recordingAction = null;
  hotkeys.suspended = false;
  setDraftCombo(action, combo);
}
```

- [ ] **Step 3: Register the recorder listener**

Update `init()` to include a capture-phase listener so the recorder sees the event before the global dispatcher bails out (both checks `hotkeys.suspended` but recorder needs priority):

```js
async function init() {
  bindUI();
  await loadHotkeys();
  window.addEventListener("keydown", onRecorderKeydown, true); // capture phase
  window.addEventListener("keydown", onGlobalKeydown);
  render();
}
```

- [ ] **Step 4: Smoke test**

```bash
cd wails && wails dev
```

Manually verify:
1. Open Preferences. Click Undo field. Press `Cmd+Shift+U`. Field shows `⌘ ⇧ U`.
2. Click Redo field. Press `Esc` — field returns to its prior value.
3. Click Open field. Press `Backspace` (alone) — field shows `Unbound`.
4. Click Select tool field. Press `V` — field shows `V`.
5. Click Arrow tool field. Press `V` — (Task 8 placeholder) Select tool silently loses its V, Arrow gets V.
6. Save. Close modal. Verify new bindings take effect.

Stop with Ctrl+C.

- [ ] **Step 5: Commit**

```bash
git add wails/gui/frontend/dist/app.js
git commit -m "$(cat <<'EOF'
feat(hotkey): capture key combos in recorder with Esc cancel and Backspace clear

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Conflict reassign dialog

**Files:**
- Modify: `wails/gui/frontend/dist/app.js`

- [ ] **Step 1: Replace `showConflictDialog` with interactive dialog**

Replace the Task 8 placeholder with:

```js
function showConflictDialog(action, conflictAction, combo) {
  const dialog = els.preferencesConflict;
  const comboDisplay = SV_Hotkey.comboToDisplay(combo, IS_MAC);
  const conflictLabel = ACTION_LABELS[conflictAction] || conflictAction;
  const actionLabel = ACTION_LABELS[action] || action;
  dialog.innerHTML = `
    <p>⚠️  <strong>${comboDisplay}</strong> is already bound to <em>${conflictLabel}</em>.</p>
    <p>Reassign it to <em>${actionLabel}</em>? The previous binding will become unbound.</p>
    <div class="conflict-actions">
      <button type="button" class="ghost-btn" data-conflict="cancel">Cancel</button>
      <button type="button" class="ghost-btn is-primary" data-conflict="reassign">Reassign</button>
    </div>
  `;
  dialog.classList.remove("is-hidden");
  const onClick = (event) => {
    const choice = event.target.closest("[data-conflict]")?.dataset.conflict;
    if (!choice) return;
    dialog.classList.add("is-hidden");
    dialog.removeEventListener("click", onClick);
    if (choice === "reassign") {
      reassignCombo(action, conflictAction, combo);
    } else {
      cancelRecording();
    }
  };
  dialog.addEventListener("click", onClick);
}
```

- [ ] **Step 2: Smoke test**

```bash
cd wails && wails dev
```

Manually verify:
1. Open Preferences. Click Arrow tool field. Press `V` (already bound to Select).
2. Dialog appears: "⌘ V is already bound to Select tool. Reassign…"
3. Click Cancel → Arrow tool keeps original `A`, dialog closes.
4. Click Arrow tool field again. Press `V`. Dialog shows. Click Reassign.
5. Arrow tool now shows `V`; Select tool shows `Unbound`.

Stop with Ctrl+C.

- [ ] **Step 3: Commit**

```bash
git add wails/gui/frontend/dist/app.js
git commit -m "$(cat <<'EOF'
feat(hotkey): prompt reassign dialog on binding conflict

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Reset-all, persisted Preferences shortcut, and final polish

**Files:**
- Modify: `wails/gui/frontend/dist/app.js`
- Modify: `wails/README.md`

- [ ] **Step 1: Wire Reset-all handler**

In `bindUI()`, append:

```js
  els.preferencesResetAll.addEventListener("click", async () => {
    if (!confirm("Reset all hotkeys to defaults?")) return;
    try {
      const defaults = await backend.resetHotkeys();
      prefs.draft = defaults.map((b) => ({ ...b }));
      prefs.dirty = false;
      applyHotkeyBindings(defaults);
      renderPreferences();
      showToast("已還原為預設熱鍵");
    } catch (err) {
      els.preferencesStatus.textContent = `還原失敗：${err?.message || err}`;
      els.preferencesStatus.classList.add("is-error");
    }
  });
```

- [ ] **Step 2: Dismiss modal on backdrop click, but not inner panel**

In `bindUI()`, append:

```js
  els.preferencesModal.addEventListener("click", (event) => {
    if (event.target === els.preferencesModal) closePreferences();
  });
```

- [ ] **Step 3: Update `wails/README.md`**

Append a new section (find an appropriate place after the existing content):

```markdown
## Hotkeys

Defaults cover tools, editing, capture, zoom, and export actions.
Customize via **File → Preferences…** (or press `Cmd+,` / `Ctrl+,`).

Settings live at:

- macOS: `~/Library/Application Support/SnapVector/hotkeys.json`
- Linux: `~/.config/SnapVector/hotkeys.json`
- Windows: `%APPDATA%\SnapVector\hotkeys.json`

Delete the file (or click **Reset all defaults** in Preferences) to restore defaults.

### Frontend unit tests

```bash
node --test wails/gui/frontend/dist/hotkey-utils.test.js
```
```

- [ ] **Step 4: Full verification run**

```bash
cd wails && go test ./... && cd .. && node --test wails/gui/frontend/dist/hotkey-utils.test.js
```

Expected: all Go tests pass, all node tests pass.

- [ ] **Step 5: Manual acceptance checklist**

Run `cd wails && wails dev`. Confirm each item:

- [ ] File → `Preferences…` opens modal; `Cmd+,` / `Ctrl+,` also opens it
- [ ] Change Undo to `mod+shift+u` → Save → quit app → relaunch → new binding still active
- [ ] Record `mod+shift+q` on Capture region → reassign dialog appears citing Capture fullscreen → Reassign → Capture fullscreen becomes Unbound
- [ ] During recording: Esc keeps original, Backspace clears, modifier-only does nothing
- [ ] Focus the inspector text field, press `v` → "v" enters the input; tool rail unchanged
- [ ] Start Chinese IME composition, press Enter to commit Chinese → no hotkey fires
- [ ] Manually delete `hotkeys.json` → relaunch → defaults restored
- [ ] Manually write `{not json` into `hotkeys.json` → relaunch → defaults load, `hotkeys.json.corrupt` created
- [ ] Reset all defaults button restores every binding
- [ ] Cancel with unsaved changes prompts for discard; confirming closes without writing

Stop with Ctrl+C when done.

- [ ] **Step 6: Commit**

```bash
git add wails/gui/frontend/dist/app.js wails/README.md
git commit -m "$(cat <<'EOF'
feat(hotkey): reset-all, backdrop dismiss, and README docs

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review Notes

**Spec coverage:**
- Architecture → Tasks 1-6.
- JSON schema + version field + merge semantics → Task 2 (`Load`).
- Canonical combo + modifier order + `mod` alias → Task 1 (`CanonicalCombo`) + Task 4 (`normalize`, `sortModifiers`).
- Phase 1 default bindings (including `Cmd+Shift+Q/W/E`) → Task 1 `DefaultHotkeys()` + Task 6 `defaultHotkeyBindings()` mirror.
- Go binding surface → Task 3.
- Modal UX + filter + Save/Cancel semantics → Task 7.
- Recording with Esc/Backspace/🗙 → Task 8.
- Conflict reassign dialog → Task 9.
- Error handling table (corrupt file, IME, typing target, modifier-only, control keys, save failure) → covered across Tasks 2, 4, 6, 7, 8.
- Reset all defaults → Task 10.
- Manual acceptance checklist → Task 10 step 5.

**Not in scope (explicitly noted in spec):**
- Global/system-wide hotkeys (Phase 2).
- Tool-rail tooltip live sync (Phase 2).
- Import/export, profiles, multi-key sequences.

**Type consistency check:**
- `Hotkey.Combo` (Go) ⇄ `binding.combo` (JS): canonicalized on Save in Go and in the recorder's `normalize` path.
- Action IDs appear identically in `ActionCatalog` (Go), `hotkeyActions()`, `ACTION_LABELS`, `ACTION_GROUPS`, and `defaultHotkeyBindings()` — all 20 strings.
- `showConflictDialog`, `reassignCombo`, `cancelRecording`, `setDraftCombo` signatures match between Task 8 placeholder and Task 9 real dialog.
- `SV_Hotkey` namespace: Task 4 exports `normalize`, `sortModifiers`, `comboToDisplay`, `detectConflict`, `isControlKey`, `isRecordableMainKey` — all six consumed in Tasks 6/8.
