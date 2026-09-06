// Package preset manages the declarative preset catalog, lookup, and documentation synchronization.
package preset

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
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

// ResolveReadme locates an accompanying documentation or text file for the preset.
func ResolveReadme(wadsDir string, p Preset) (string, bool) {
	for _, m := range p.Mappacks {
		ext := filepath.Ext(m)
		base := strings.TrimSuffix(m, ext)
		if path, ok := ResolveFile(wadsDir, base+".txt"); ok {
			return path, true
		}
	}
	nameClean := strings.ReplaceAll(p.Name, " ", "") + ".txt"
	if path, ok := ResolveFile(wadsDir, nameClean); ok {
		return path, true
	}
	iwadBase := strings.TrimSuffix(p.IWAD, filepath.Ext(p.IWAD))
	if path, ok := ResolveFile(wadsDir, iwadBase+".txt"); ok {
		return path, true
	}
	return "", false
}

// DecodeText converts raw text bytes (e.g. from idgames .txt / README files) into valid UTF-8,
// normalizing line endings and automatically detecting DOS Code Page 437 (ASCII/ANSI art),
// Windows-1252, or standard UTF-8.
func DecodeText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}

	// Normalize CRLF to LF and bare CR to LF
	normalized := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))

	// Check if already valid UTF-8 without accidental Arabic code points from CP437 blocks
	if utf8.Valid(normalized) {
		text := string(normalized)
		hasAccidentalArabic := false
		for _, r := range text {
			if r >= 0x0600 && r <= 0x08FF {
				hasAccidentalArabic = true
				break
			}
		}
		if !hasAccidentalArabic {
			return text
		}
	}

	// Count CP437 box-drawing, shading, and block characters vs Windows-1252 punctuation
	cpScore := 0
	winScore := 0
	for _, b := range normalized {
		switch {
		case b >= 0xB0 && b <= 0xDF:
			// Box-drawing, shading blocks (░▒▓█▄▀), and borders (║═│─) in CP437
			cpScore++
		case (b >= 0x80 && b <= 0x9F):
			// Smart quotes, dashes, ellipsis, bullet, trademark in Windows-1252
			winScore++
		}
	}

	if cpScore > winScore {
		if decoded, err := charmap.CodePage437.NewDecoder().Bytes(normalized); err == nil {
			return string(decoded)
		}
	}

	if decoded, err := charmap.Windows1252.NewDecoder().Bytes(normalized); err == nil {
		return string(decoded)
	}

	if decoded, err := charmap.ISO8859_1.NewDecoder().Bytes(normalized); err == nil {
		return string(decoded)
	}

	// Fallback to CP437 as it maps all 256 byte values
	if decoded, err := charmap.CodePage437.NewDecoder().Bytes(normalized); err == nil {
		return string(decoded)
	}

	return string(normalized)
}

// ReadReadme reads and decodes an accompanying documentation file, converting CP437/Windows-1252 to UTF-8
// and normalizing carriage returns.
func ReadReadme(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return DecodeText(raw), nil
}
