package preset

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

//go:embed data/presets.json
var embeddedPresetsJSON []byte

// PresetCatalog represents the root structure of presets.json.
type PresetCatalog struct {
	Schema   string                `json:"$schema,omitempty"`
	Metadata map[string]string     `json:"metadata,omitempty"`
	Engines  map[string]EngineMeta `json:"engines"`
	Presets  []Preset              `json:"presets"`
}

// EngineMeta describes an engine configuration in presets.json.
type EngineMeta struct {
	Name             string `json:"name"`
	Family           string `json:"family"`
	LinuxPath        string `json:"linux_path"`
	LinuxConfigDir   string `json:"linux_config_dir"`
	LinuxDataDir     string `json:"linux_data_dir"`
	WindowsID        string `json:"windows_id"`
	WindowsName      string `json:"windows_name"`
	WindowsPath      string `json:"windows_path"`
	WindowsConfigDir string `json:"windows_config_dir"`
	WindowsDataDir   string `json:"windows_data_dir"`
}

// Preset defines a single Doom mapset configuration.
type Preset struct {
	Name               string   `json:"name"`
	Engine             string   `json:"engine"`
	IWAD               string   `json:"iwad"`
	Mappacks           []string `json:"mappacks"`
	Category           string   `json:"category"`
	Compatibility      string   `json:"compatibility"`
	Description        string   `json:"description"`
	DownloadURLs       []string `json:"download_urls,omitempty"`
	AdditionalArgs     string   `json:"additional_args,omitempty"`
	LoadMapsAfterMods  bool     `json:"load_maps_after_mods,omitempty"`
}

// LoadCatalog loads the preset catalog from an external file if provided/existing, or falls back to embedded data.
func LoadCatalog(customPath string) (*PresetCatalog, error) {
	var raw []byte
	var err error

	if customPath != "" {
		raw, err = os.ReadFile(customPath)
	} else if envPath := os.Getenv("DOOM_PRESETS_FILE"); envPath != "" {
		raw, err = os.ReadFile(envPath)
	}

	if len(raw) == 0 || err != nil {
		raw = embeddedPresetsJSON
	}

	var catalog PresetCatalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}

// Find searches for a preset by exact name, case-insensitive match, or prefix.
func (c *PresetCatalog) Find(target string) *Preset {
	clean := strings.TrimSpace(strings.ToLower(target))
	if clean == "" {
		return nil
	}

	// 1. Exact case-insensitive match
	for i := range c.Presets {
		if strings.ToLower(c.Presets[i].Name) == clean {
			return &c.Presets[i]
		}
	}

	// 2. Prefix match
	for i := range c.Presets {
		if strings.HasPrefix(strings.ToLower(c.Presets[i].Name), clean) {
			return &c.Presets[i]
		}
	}

	return nil
}

// NormalizeFilename strips spaces, dashes, underscores, and lowers case for fuzzy matching.
func NormalizeFilename(name string) string {
	lower := strings.ToLower(name)
	r := strings.NewReplacer(" ", "", "-", "", "_", "")
	return r.Replace(lower)
}

// ResolveFile searches for a required file within wadsDir with case-insensitivity, normalization, and known aliases.
func ResolveFile(wadsDir string, targetName string) (string, bool) {
	if wadsDir == "" || targetName == "" {
		return "", false
	}

	exact := filepath.Join(wadsDir, targetName)
	if _, err := os.Stat(exact); err == nil {
		return exact, true
	}

	targetLower := strings.ToLower(targetName)
	targetNorm := NormalizeFilename(targetName)

	entries, err := os.ReadDir(wadsDir)
	if err != nil {
		return "", false
	}

	// 1. Case-insensitive match
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.ToLower(name) == targetLower {
			return filepath.Join(wadsDir, name), true
		}
	}

	// 2. Normalized match (ignoring spaces, dashes, underscores)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if NormalizeFilename(name) == targetNorm {
			return filepath.Join(wadsDir, name), true
		}
	}

	// 3. Known aliases
	switch targetLower {
	case "gdturbo.wad":
		return ResolveFile(wadsDir, "gd.wad")
	case "gd.wad":
		return ResolveFile(wadsDir, "gdturbo.wad")
	case "doom.wad":
		return ResolveFile(wadsDir, "doom1.wad")
	}

	return "", false
}
