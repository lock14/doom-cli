// Package config provides platform-idiomatic path resolution and user settings for doom-cli.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lock14/doom-cli/internal/preset"
)

// AppConfig represents persistent user configuration for doom-cli.
type AppConfig struct {
	Theme         string                             `json:"theme,omitempty"`
	NerdFonts     bool                               `json:"nerd_fonts,omitempty"`
	WadsDir       string                             `json:"wads_dir,omitempty"`
	BinDir        string                             `json:"bin_dir,omitempty"`
	SoundFontsDir string                             `json:"soundfonts_dir,omitempty"`
	Engines       map[string]preset.EngineConfig     `json:"engines,omitempty"`
	Presets       []preset.Preset                    `json:"presets,omitempty"`
	LaunchOptions map[string]preset.WadLaunchOptions `json:"launch_options,omitempty"`
}

// AddEngine registers or updates a custom engine in user configuration.
func (c *AppConfig) AddEngine(cfg preset.EngineConfig) {
	if c.Engines == nil {
		c.Engines = make(map[string]preset.EngineConfig)
	}
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return
	}
	cfg.Name = name
	c.Engines[name] = cfg
}

// RemoveEngine deletes a custom engine from user configuration by name.
func (c *AppConfig) RemoveEngine(name string) bool {
	if c.Engines == nil {
		return false
	}
	for k := range c.Engines {
		if strings.EqualFold(k, name) {
			delete(c.Engines, k)
			return true
		}
	}
	return false
}

// AddPreset registers or updates a custom preset in user configuration.
func (c *AppConfig) AddPreset(p preset.Preset) {
	clean := strings.TrimSpace(p.Name)
	if clean == "" {
		return
	}
	p.Name = clean
	p.Custom = true

	for i := range c.Presets {
		if strings.EqualFold(c.Presets[i].Name, clean) {
			c.Presets[i] = p
			return
		}
	}
	c.Presets = append(c.Presets, p)
}

// RemovePreset deletes a custom preset from user configuration by name.
func (c *AppConfig) RemovePreset(name string) bool {
	for i := range c.Presets {
		if strings.EqualFold(c.Presets[i].Name, name) {
			c.Presets = append(c.Presets[:i], c.Presets[i+1:]...)
			return true
		}
	}
	return false
}

// SetLaunchOptions configures launch options overrides for a specific preset/wad.
func (c *AppConfig) SetLaunchOptions(presetName string, opts preset.WadLaunchOptions) {
	if c.LaunchOptions == nil {
		c.LaunchOptions = make(map[string]preset.WadLaunchOptions)
	}
	clean := strings.TrimSpace(presetName)
	if clean == "" {
		return
	}

	// Remove existing key if casing differs
	for k := range c.LaunchOptions {
		if strings.EqualFold(k, clean) {
			delete(c.LaunchOptions, k)
			break
		}
	}
	c.LaunchOptions[clean] = opts
}

// RemoveLaunchOptions removes launch options overrides for a specific preset/wad.
func (c *AppConfig) RemoveLaunchOptions(presetName string) bool {
	if c.LaunchOptions == nil {
		return false
	}
	for k := range c.LaunchOptions {
		if strings.EqualFold(k, presetName) {
			delete(c.LaunchOptions, k)
			return true
		}
	}
	return false
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
