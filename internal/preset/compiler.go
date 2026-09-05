// Package preset manages the declarative preset catalog and DoomRunner compilation.
package preset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DoomRunnerPreset represents a preset entry in DoomRunner's options.json.
type DoomRunnerPreset struct {
	AdditionalArgs       string            `json:"additional_args"`
	AlternativePaths     map[string]string `json:"alternative_paths"`
	CompatibilityOptions map[string]int    `json:"compatibility_options"`
	EnvVars              map[string]string `json:"env_vars"`
	LoadMapsAfterMods    bool              `json:"load_maps_after_mods"`
	Mods                 []string          `json:"mods"`
	Name                 string            `json:"name"`
	SelectedIWAD         string            `json:"selected_IWAD"`
	SelectedConfig       string            `json:"selected_config"`
	SelectedEngine       string            `json:"selected_engine"`
	SelectedMappacks     []string          `json:"selected_mappacks"`
}

// BuildLinuxPresets converts the catalog presets to DoomRunner Linux preset entries.
func BuildLinuxPresets(cat *Catalog) []DoomRunnerPreset {
	var result []DoomRunnerPreset
	for _, p := range cat.Presets {
		enginePath := fmt.Sprintf("__HOME__/.local/bin/%s", p.Engine)
		iwadPath := fmt.Sprintf("__HOME__/.local/share/games/uzdoom/%s", p.IWAD)
		mappacks := []string{}
		for _, m := range p.Mappacks {
			mappacks = append(mappacks, fmt.Sprintf("__HOME__/.local/share/games/uzdoom/%s", m))
		}

		result = append(result, DoomRunnerPreset{
			AdditionalArgs: p.AdditionalArgs,
			AlternativePaths: map[string]string{
				"config_dir":     "",
				"demo_dir":       "",
				"save_dir":       "",
				"screenshot_dir": "",
			},
			CompatibilityOptions: map[string]int{
				"compat_mode":  -1,
				"compatflags1": 0,
				"compatflags2": 0,
			},
			EnvVars:           map[string]string{},
			LoadMapsAfterMods: p.LoadMapsAfterMods,
			Mods:              []string{},
			Name:              p.Name,
			SelectedIWAD:      iwadPath,
			SelectedConfig:    "",
			SelectedEngine:    enginePath,
			SelectedMappacks:  mappacks,
		})
	}
	return result
}

// BuildWindowsPresets converts the catalog presets to DoomRunner Windows preset entries.
func BuildWindowsPresets(cat *Catalog) []DoomRunnerPreset {
	var result []DoomRunnerPreset
	for _, p := range cat.Presets {
		engineMeta, ok := cat.Engines[p.Engine]
		engineID := ""
		if ok {
			engineID = engineMeta.WindowsID
		}
		iwadPath := fmt.Sprintf("E:/Doom WADS/%s", p.IWAD)
		mappacks := []string{}
		for _, m := range p.Mappacks {
			mappacks = append(mappacks, fmt.Sprintf("E:/Doom WADS/%s", m))
		}

		result = append(result, DoomRunnerPreset{
			AdditionalArgs: p.AdditionalArgs,
			AlternativePaths: map[string]string{
				"config_dir":     "",
				"demo_dir":       "",
				"save_dir":       "",
				"screenshot_dir": "",
			},
			CompatibilityOptions: map[string]int{
				"compat_mode":  -1,
				"compatflags1": 0,
				"compatflags2": 0,
			},
			EnvVars:           map[string]string{},
			LoadMapsAfterMods: p.LoadMapsAfterMods,
			Mods:              []string{},
			Name:              p.Name,
			SelectedIWAD:      iwadPath,
			SelectedConfig:    "",
			SelectedEngine:    engineID,
			SelectedMappacks:  mappacks,
		})
	}
	return result
}

// GenerateReadmeTable constructs the markdown table for README.md.
func GenerateReadmeTable(cat *Catalog) string {
	lines := []string{
		"| Megawad / Expansion | Engine | Compatibility / Details |",
		"| :--- | :--- | :--- |",
	}
	for _, p := range cat.Presets {
		engDisplay := "DSDA-Doom"
		if p.Engine == "uzdoom" {
			engDisplay = "UZDoom"
		}
		lines = append(lines, fmt.Sprintf("| **%s** | %s | %s |", p.Name, engDisplay, p.Description))
	}
	return strings.Join(lines, "\n")
}

// CompileOptionsFiles compiles data/presets.json into DoomRunner Linux and Windows options.json files.
func CompileOptionsFiles(rootDir string) error {
	cat, err := LoadCatalog(filepath.Join(rootDir, "data", "presets.json"))
	if err != nil {
		return err
	}

	linuxFile := filepath.Join(rootDir, "DoomRunner", "linux", "options.json")
	if data, err := os.ReadFile(linuxFile); err == nil {
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err == nil {
			obj["presets"] = BuildLinuxPresets(cat)
			encoded, _ := json.MarshalIndent(obj, "", "    ")
			_ = os.WriteFile(linuxFile, append(encoded, '\n'), 0644)
		}
	}

	winFile := filepath.Join(rootDir, "DoomRunner", "windows", "options.json")
	if data, err := os.ReadFile(winFile); err == nil {
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err == nil {
			obj["presets"] = BuildWindowsPresets(cat)
			encoded, _ := json.MarshalIndent(obj, "", "    ")
			_ = os.WriteFile(winFile, append(encoded, '\n'), 0644)
		}
	}

	readmeFile := filepath.Join(rootDir, "README.md")
	if content, err := os.ReadFile(readmeFile); err == nil {
		table := GenerateReadmeTable(cat)
		pattern := regexp.MustCompile(`(## Preconfigured Presets\n\n[^\n]+\n\n)(\|[\s\S]+?)(\n\n---)`)
		updated := pattern.ReplaceAllString(string(content), fmt.Sprintf("${1}%s${3}", table))
		_ = os.WriteFile(readmeFile, []byte(updated), 0644)
	}

	return nil
}
