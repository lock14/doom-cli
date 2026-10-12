// Package config provides platform-idiomatic path resolution and user settings for doom-cli.
package config

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"

	"github.com/lock14/doom-cli/internal/fsutil"
)

// AppConfig represents persistent user settings for doom-cli.
// User-defined engines, presets, and overrides live in <ConfigDir>/catalog.d/*.json.
type AppConfig struct {
	Theme         string   `json:"theme,omitempty"`
	NerdFonts     bool     `json:"nerd_fonts,omitempty"`
	WadsDir       string   `json:"wads_dir,omitempty"`
	WadsDirs      []string `json:"wads_dirs,omitempty"`
	BinDir        string   `json:"bin_dir,omitempty"`
	SoundFontsDir string   `json:"soundfonts_dir,omitempty"`

	// LegacyEntityKeys lists any legacy v1 entity keys ("engines", "presets",
	// "launch_options") encountered in config.json so callers can emit a warning.
	LegacyEntityKeys []string `json:"-"`

	extra map[string]json.RawMessage
}

var knownConfigKeys = map[string]bool{
	"theme":          true,
	"nerd_fonts":     true,
	"wads_dir":       true,
	"wads_dirs":      true,
	"bin_dir":        true,
	"soundfonts_dir": true,
}

var legacyEntityKeyNames = []string{"engines", "presets", "launch_options"}

// LoadConfig reads the user configuration file if present, returning defaults if not found.
// Unknown top-level JSON keys are preserved in memory so SaveConfig does not strip them.
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

	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, err
	}
	for k, v := range rawMap {
		if !knownConfigKeys[k] {
			if cfg.extra == nil {
				cfg.extra = make(map[string]json.RawMessage)
			}
			cfg.extra[k] = v
		}
	}
	for _, k := range legacyEntityKeyNames {
		if _, exists := cfg.extra[k]; exists {
			cfg.LegacyEntityKeys = append(cfg.LegacyEntityKeys, k)
		}
	}

	return cfg, nil
}

// SaveConfig writes the user configuration file atomically, creating a timestamped
// backup when overwriting an existing file and preserving any unknown top-level keys.
func SaveConfig(paths *Paths, cfg *AppConfig) error {
	if paths == nil || paths.ConfigFile == "" {
		return nil
	}
	if cfg == nil {
		cfg = &AppConfig{}
	}

	merged := make(map[string]any, len(cfg.extra)+len(knownConfigKeys))
	for k, v := range cfg.extra {
		if !knownConfigKeys[k] {
			merged[k] = v
		}
	}
	if cfg.Theme != "" {
		merged["theme"] = cfg.Theme
	}
	if cfg.NerdFonts {
		merged["nerd_fonts"] = cfg.NerdFonts
	}
	if cfg.WadsDir != "" {
		merged["wads_dir"] = cfg.WadsDir
	}
	if len(cfg.WadsDirs) > 0 {
		merged["wads_dirs"] = cfg.WadsDirs
	}
	if cfg.BinDir != "" {
		merged["bin_dir"] = cfg.BinDir
	}
	if cfg.SoundFontsDir != "" {
		merged["soundfonts_dir"] = cfg.SoundFontsDir
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(merged); err != nil {
		return err
	}

	_, err := fsutil.WriteFileAtomic(paths.ConfigFile, buf.Bytes(), 0o644)
	return err
}

// ExtraKeys returns the sorted list of unknown top-level keys preserved in AppConfig.
func (c *AppConfig) ExtraKeys() []string {
	if c == nil || len(c.extra) == 0 {
		return nil
	}
	keys := make([]string, 0, len(c.extra))
	for k := range c.extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
