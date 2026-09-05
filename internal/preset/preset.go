// Package preset manages the declarative preset catalog, lookup, and DoomRunner options compilation.
package preset

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed data/presets.json
var embeddedPresetsJSON []byte

// Catalog represents the root structure of presets.json.
type Catalog struct {
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
	Name              string   `json:"name"`
	Engine            string   `json:"engine"`
	IWAD              string   `json:"iwad"`
	Mappacks          []string `json:"mappacks"`
	Category          string   `json:"category"`
	Compatibility     string   `json:"compatibility"`
	Description       string   `json:"description"`
	DownloadURLs      []string `json:"download_urls,omitempty"`
	AdditionalArgs    string   `json:"additional_args,omitempty"`
	LoadMapsAfterMods bool     `json:"load_maps_after_mods,omitempty"`
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

// ReadmeInfo contains parsed metadata from an accompanying WAD text file.
type ReadmeInfo struct {
	Path        string
	Filename    string
	Author      string
	ReleaseDate string
	MapCount    string
}

var (
	authorRegex = regexp.MustCompile(`(?i)^authors?\s*(?:\(s\))?\s*:\s*(.+)`)
	dateRegex   = regexp.MustCompile(`(?i)^(?:release\s+date|date)\s*:\s*(.+)`)
	mapsRegex   = regexp.MustCompile(`(?i)^(?:new\s+levels|levels|map\s*#?)\s*:\s*(.+)`)
)

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

// ParseReadme reads up to 150 lines of an idgames text file and extracts key metadata.
func ParseReadme(path string) ReadmeInfo {
	info := ReadmeInfo{
		Path:     path,
		Filename: filepath.Base(path),
	}
	f, err := os.Open(path)
	if err != nil {
		return info
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() && lineCount < 150 {
		lineCount++
		text := strings.TrimSpace(scanner.Text())
		if info.Author == "" {
			if m := authorRegex.FindStringSubmatch(text); len(m) > 1 {
				val := strings.TrimSpace(m[1])
				if val != "" && !strings.HasPrefix(val, "None") {
					info.Author = val
				}
			}
		}
		if info.ReleaseDate == "" {
			if m := dateRegex.FindStringSubmatch(text); len(m) > 1 {
				val := strings.TrimSpace(m[1])
				if val != "" {
					info.ReleaseDate = val
				}
			}
		}
		if info.MapCount == "" {
			if m := mapsRegex.FindStringSubmatch(text); len(m) > 1 {
				val := strings.TrimSpace(m[1])
				if val != "" && !strings.EqualFold(val, "none") && !strings.EqualFold(val, "no") {
					info.MapCount = val
				}
			}
		}
	}
	return info
}
