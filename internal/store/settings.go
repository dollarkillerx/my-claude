package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
)

// Settings are user preferences that persist across runs.
type Settings struct {
	Lang string `json:"lang,omitempty"` // en, zh or ja; empty = follow the system
}

func settingsPath() string { return filepath.Join(Home(), "settings.json") }

// LoadSettings returns the saved settings; a missing or unreadable file
// yields the defaults.
func LoadSettings() Settings {
	var s Settings
	if data, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

// SaveSettings writes the settings.
func SaveSettings(s Settings) error {
	if err := os.MkdirAll(Home(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.AtomicWrite(settingsPath(), append(data, '\n'), 0o600)
}
