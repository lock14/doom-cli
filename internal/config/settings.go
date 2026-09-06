// Package config provides platform-idiomatic path resolution and user settings for doom-cli.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig represents persistent user configuration for doom-cli.
type AppConfig struct {
	Theme     string `json:"theme,omitempty"`
	NerdFonts bool   `json:"nerd_fonts,omitempty"`
}

// LoadConfig reads the user configuration file if present, returning defaults if not found.
func LoadConfig(paths *Paths) (*AppConfig, error) {
	cfg := &AppConfig{}
	if paths == nil || paths.ConfigFile == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveConfig writes the user configuration file, creating parent directories as necessary.
func SaveConfig(paths *Paths, cfg *AppConfig) error {
	if paths == nil || paths.ConfigFile == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(paths.ConfigFile, data, 0o644)
}
