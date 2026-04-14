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
