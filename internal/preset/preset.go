// Package preset manages the declarative preset catalog, lookup, and documentation synchronization.
package preset

import (
	_ "embed"
	"encoding/json"
	"os"
	"strings"
)

//go:embed data/presets.json
var embeddedPresetsJSON []byte

// Catalog represents the root structure of presets.json.
type Catalog struct {
	Schema   string                  `json:"$schema,omitempty"`
	Metadata map[string]string       `json:"metadata,omitempty"`
	Engines  map[string]EngineConfig `json:"engines"`
	Presets  []Preset                `json:"presets"`
}

// EngineConfig describes an engine definition for execution and argument handling.
type EngineConfig struct {
	Name        string   `json:"name"`
	Binary      string   `json:"binary,omitempty"`
	Family      string   `json:"family,omitempty"`
	ArgsStyle   string   `json:"args_style,omitempty"` // "boom" (-file/-deh) or "zdoom" (-file)
	DefaultArgs []string `json:"default_args,omitempty"`
	Description string   `json:"description,omitempty"`
}

// EngineMeta is an alias for EngineConfig for backward compatibility with presets.json.
type EngineMeta = EngineConfig

// EffectiveArgsStyle returns "boom" or "zdoom", inferring from engine name or family if not explicitly set.
func (e EngineConfig) EffectiveArgsStyle() string {
	style := strings.ToLower(strings.TrimSpace(e.ArgsStyle))
	if style == "boom" || style == "zdoom" {
		return style
	}
	name := strings.ToLower(e.Name)
	fam := strings.ToLower(e.Family)
	if strings.Contains(name, "dsda") ||
		strings.Contains(name, "boom") ||
		strings.Contains(name, "woof") ||
		strings.Contains(name, "crispy") ||
		strings.Contains(name, "chocolate") ||
		strings.Contains(name, "mbf") ||
		strings.Contains(fam, "boom") ||
		strings.Contains(fam, "mbf") {
		return "boom"
	}
	return "zdoom"
}

// Preset defines a single Doom mapset configuration.
type Preset struct {
	Name              string   `json:"name"`
	Engine            string   `json:"engine"`
	IWAD              string   `json:"iwad"`
	Mappacks          []string `json:"mappacks"`
	Category          string   `json:"category"`
	Compatibility     string   `json:"compatibility"`
	Description       string   `json:"description"`
	Author            string   `json:"author,omitempty"`
	ReleaseDate       string   `json:"release_date,omitempty"`
	DownloadURLs      []string `json:"download_urls,omitempty"`
	AdditionalArgs    string   `json:"additional_args,omitempty"`
	LoadMapsAfterMods bool     `json:"load_maps_after_mods,omitempty"`
	Custom            bool     `json:"-"`
}

// WadLaunchOptions defines user launch preferences and overrides for a specific preset/wad.
type WadLaunchOptions struct {
	Engine         string   `json:"engine,omitempty"`
	IWAD           string   `json:"iwad,omitempty"`
	AdditionalArgs string   `json:"additional_args,omitempty"`
	ExtraFiles     []string `json:"extra_files,omitempty"`
}

// Clone creates a deep copy of the catalog.
func (c *Catalog) Clone() *Catalog {
	if c == nil {
		return nil
	}
	cloned := &Catalog{
		Schema:   c.Schema,
		Metadata: make(map[string]string, len(c.Metadata)),
		Engines:  make(map[string]EngineConfig, len(c.Engines)),
		Presets:  make([]Preset, len(c.Presets)),
	}
	for k, v := range c.Metadata {
		cloned.Metadata[k] = v
	}
	for k, v := range c.Engines {
		engCopy := v
		if len(v.DefaultArgs) > 0 {
			engCopy.DefaultArgs = append([]string(nil), v.DefaultArgs...)
		}
		cloned.Engines[k] = engCopy
	}
	for i, p := range c.Presets {
		pCopy := p
		if len(p.Mappacks) > 0 {
			pCopy.Mappacks = append([]string(nil), p.Mappacks...)
		}
		if len(p.DownloadURLs) > 0 {
			pCopy.DownloadURLs = append([]string(nil), p.DownloadURLs...)
		}
		cloned.Presets[i] = pCopy
	}
	return cloned
}

// MergeEngines merges user-defined engine configurations into the catalog.
func (c *Catalog) MergeEngines(engines map[string]EngineConfig) {
	if c.Engines == nil {
		c.Engines = make(map[string]EngineConfig)
	}
	for k, v := range engines {
		if v.Name == "" {
			v.Name = k
		}
		c.Engines[k] = v
	}
}

// MergePresets appends or updates presets in the catalog.
func (c *Catalog) MergePresets(presets []Preset) {
	for _, newP := range presets {
		cleanName := strings.TrimSpace(newP.Name)
		if cleanName == "" {
			continue
		}
		newP.Name = cleanName
		newP.Custom = true

		idx := -1
		for i, existing := range c.Presets {
			if strings.EqualFold(existing.Name, cleanName) {
				idx = i
				break
			}
		}

		if idx >= 0 {
			// Update existing preset fields if specified
			if newP.Engine != "" {
				c.Presets[idx].Engine = newP.Engine
			}
			if newP.IWAD != "" {
				c.Presets[idx].IWAD = newP.IWAD
			}
			if len(newP.Mappacks) > 0 {
				c.Presets[idx].Mappacks = append([]string(nil), newP.Mappacks...)
			}
			if newP.Category != "" {
				c.Presets[idx].Category = newP.Category
			}
			if newP.Compatibility != "" {
				c.Presets[idx].Compatibility = newP.Compatibility
			}
			if newP.Description != "" {
				c.Presets[idx].Description = newP.Description
			}
			if newP.Author != "" {
				c.Presets[idx].Author = newP.Author
			}
			if newP.ReleaseDate != "" {
				c.Presets[idx].ReleaseDate = newP.ReleaseDate
			}
			if newP.AdditionalArgs != "" {
				c.Presets[idx].AdditionalArgs = newP.AdditionalArgs
			}
			c.Presets[idx].Custom = true
		} else {
			c.Presets = append(c.Presets, newP)
		}
	}
}

// ApplyLaunchOptions applies per-wad launch options overrides to matching presets.
func (c *Catalog) ApplyLaunchOptions(options map[string]WadLaunchOptions) {
	for presetName, opt := range options {
		for i := range c.Presets {
			if strings.EqualFold(c.Presets[i].Name, presetName) {
				if opt.Engine != "" {
					c.Presets[i].Engine = opt.Engine
				}
				if opt.IWAD != "" {
					c.Presets[i].IWAD = opt.IWAD
				}
				if opt.AdditionalArgs != "" {
					existing := strings.TrimSpace(c.Presets[i].AdditionalArgs)
					if existing == "" {
						c.Presets[i].AdditionalArgs = opt.AdditionalArgs
					} else {
						c.Presets[i].AdditionalArgs = existing + " " + opt.AdditionalArgs
					}
				}
				if len(opt.ExtraFiles) > 0 {
					c.Presets[i].Mappacks = append(c.Presets[i].Mappacks, opt.ExtraFiles...)
				}
				break
			}
		}
	}
}

// LoadCatalog loads the preset catalog from an external file if provided/existing, or falls back to embedded data.
func LoadCatalog(customPath string) (*Catalog, error) {
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

	var catalog Catalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}

// LoadLayeredCatalog loads embedded base presets, merges optional user presets file,
// applies custom engines, custom presets, and per-wad launch options.
func LoadLayeredCatalog(
	customPresetsPath, userPresetsPath string,
	customEngines map[string]EngineConfig,
	customPresets []Preset,
	launchOptions map[string]WadLaunchOptions,
) (*Catalog, error) {
	base, err := LoadCatalog(customPresetsPath)
	if err != nil {
		return nil, err
	}
	cat := base.Clone()

	// Ensure default engines have rich metadata if missing
	if cat.Engines == nil {
		cat.Engines = make(map[string]EngineConfig)
	}
	if dsda, ok := cat.Engines["dsda-doom"]; !ok || dsda.Description == "" {
		dsda.Name = "dsda-doom"
		dsda.Description = "DSDA-Doom (MBF21 / Speedrun)"
		dsda.ArgsStyle = "boom"
		cat.Engines["dsda-doom"] = dsda
	}
	if uz, ok := cat.Engines["uzdoom"]; !ok || uz.Description == "" {
		uz.Name = "uzdoom"
		uz.Description = "UZDoom (Software-Plus / Advanced)"
		uz.ArgsStyle = "zdoom"
		cat.Engines["uzdoom"] = uz
	}

	// Layer drop-in user presets file if exists
	if userPresetsPath != "" {
		if fi, err := os.Stat(userPresetsPath); err == nil && !fi.IsDir() {
			if userCat, err := LoadCatalog(userPresetsPath); err == nil {
				cat.MergeEngines(userCat.Engines)
				cat.MergePresets(userCat.Presets)
			}
		}
	}

	// Layer user configuration
	if len(customEngines) > 0 {
		cat.MergeEngines(customEngines)
	}
	if len(customPresets) > 0 {
		cat.MergePresets(customPresets)
	}
	if len(launchOptions) > 0 {
		cat.ApplyLaunchOptions(launchOptions)
	}

	return cat, nil
}

// Find searches for a preset by exact name, case-insensitive match, or prefix.
func (c *Catalog) Find(target string) *Preset {
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
