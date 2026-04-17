package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const preferencesFileVersion = 1

type Preferences struct {
	ExportDirectory string `json:"exportDirectory"`
}

type preferencesFile struct {
	Version     int         `json:"version"`
	Preferences Preferences `json:"preferences"`
}

type PreferencesStore struct {
	configDir   string
	userHomeDir func() (string, error)
}

func NewPreferencesStore() *PreferencesStore {
	return &PreferencesStore{userHomeDir: os.UserHomeDir}
}

func (s *PreferencesStore) DefaultPreferences() Preferences {
	homeDir := ""
	if s.userHomeDir != nil {
		if dir, err := s.userHomeDir(); err == nil {
			homeDir = strings.TrimSpace(dir)
		}
	}
	if homeDir == "" {
		return Preferences{}
	}
	return Preferences{
		ExportDirectory: filepath.Join(homeDir, "Downloads"),
	}
}

func (s *PreferencesStore) configPath() (string, error) {
	base := s.configDir
	if base == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("locate user config dir: %w", err)
		}
		base = d
	}
	return filepath.Join(base, "SnapVector", "preferences.json"), nil
}

func (s *PreferencesStore) Load() (Preferences, error) {
	path, err := s.configPath()
	if err != nil {
		return Preferences{}, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s.DefaultPreferences(), nil
		}
		return Preferences{}, fmt.Errorf("read preferences: %w", err)
	}

	var file preferencesFile
	if err := json.Unmarshal(raw, &file); err != nil {
		s.quarantine(path, raw)
		return s.DefaultPreferences(), nil
	}
	if file.Version != preferencesFileVersion {
		s.quarantine(path, raw)
		return s.DefaultPreferences(), nil
	}

	return s.normalizePreferences(file.Preferences), nil
}

func (s *PreferencesStore) Save(preferences Preferences) error {
	path, err := s.configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir preferences dir: %w", err)
	}

	normalized := s.normalizePreferences(preferences)
	if normalized.ExportDirectory != "" {
		if err := os.MkdirAll(normalized.ExportDirectory, 0o755); err != nil {
			return fmt.Errorf("create export folder: %w", err)
		}
		info, err := os.Stat(normalized.ExportDirectory)
		if err != nil {
			return fmt.Errorf("stat export folder: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("export folder is not a directory: %s", normalized.ExportDirectory)
		}
	}

	payload, err := json.MarshalIndent(preferencesFile{
		Version:     preferencesFileVersion,
		Preferences: normalized,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal preferences: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "preferences-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp: %w", err)
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

func (s *PreferencesStore) Reset() (Preferences, error) {
	path, err := s.configPath()
	if err != nil {
		return Preferences{}, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Preferences{}, fmt.Errorf("remove preferences: %w", err)
	}
	return s.DefaultPreferences(), nil
}

func (s *PreferencesStore) quarantine(path string, raw []byte) {
	_ = os.WriteFile(path+".corrupt", raw, 0o600)
	_ = os.Remove(path)
}

func (s *PreferencesStore) normalizePreferences(preferences Preferences) Preferences {
	normalized := s.DefaultPreferences()
	normalized.ExportDirectory = strings.TrimSpace(preferences.ExportDirectory)
	if normalized.ExportDirectory != "" {
		normalized.ExportDirectory = filepath.Clean(normalized.ExportDirectory)
		return normalized
	}
	return s.DefaultPreferences()
}
