package preset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}
	if len(cat.Presets) != 32 {
		t.Errorf("expected 32 presets, got %d", len(cat.Presets))
	}
}

func TestFind(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	// Exact match
	p := cat.Find("Eviternity II")
	if p == nil || p.Name != "Eviternity II" {
		t.Errorf("expected Eviternity II, got %v", p)
	}

	// Case-insensitive
	p = cat.Find("sunlust")
	if p == nil || p.Name != "Sunlust" {
		t.Errorf("expected Sunlust, got %v", p)
	}

	// Prefix
	p = cat.Find("Alien Vend")
	if p == nil || p.Name != "Alien Vendetta" {
		t.Errorf("expected Alien Vendetta, got %v", p)
	}
}

func TestResolveFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wads_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create spaced file
	spacedFile := filepath.Join(tmpDir, "Eviternity II.wad")
	if err := os.WriteFile(spacedFile, []byte("wad"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test normalized matching
	path, found := ResolveFile(tmpDir, "eviternityii.wad")
	if !found || filepath.Base(path) != "Eviternity II.wad" {
		t.Errorf("expected to find Eviternity II.wad, got %s, found=%v", path, found)
	}

	// Create alias file
	gdFile := filepath.Join(tmpDir, "gd.wad")
	if err := os.WriteFile(gdFile, []byte("wad"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	path, found = ResolveFile(tmpDir, "gdturbo.wad")
	if !found || filepath.Base(path) != "gd.wad" {
		t.Errorf("expected to find gd.wad for gdturbo.wad, got %s, found=%v", path, found)
	}
}

func TestPresetParityAndInvariants(t *testing.T) {
	rootDir := filepath.Join("..", "..")
	presetsPath := filepath.Join(rootDir, "data", "presets.json")
	if _, err := os.Stat(presetsPath); os.IsNotExist(err) {
		t.Skip("data/presets.json not found from relative path, skipping parity test")
	}

	cat, err := LoadCatalog(presetsPath)
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	// 1. Check duplicate IWADs in mappacks
	baseIWADs := map[string]bool{
		"DOOM.WAD": true, "DOOM2.WAD": true, "PLUTONIA.WAD": true,
		"TNT.WAD": true, "HERETIC.WAD": true, "HEXEN.WAD": true,
	}
	for _, p := range cat.Presets {
		iwadUpper := strings.ToUpper(p.IWAD)
		for _, m := range p.Mappacks {
			mUpper := strings.ToUpper(m)
			if mUpper == iwadUpper || baseIWADs[mUpper] {
				t.Errorf("Preset '%s' includes base IWAD '%s' in mappacks list", p.Name, m)
			}
		}
	}

	// 2. Parity with Linux options.json
	linuxFile := filepath.Join(rootDir, "DoomRunner", "linux", "options.json")
	if data, err := os.ReadFile(linuxFile); err == nil {
		var obj struct {
			Presets []DoomRunnerPreset `json:"presets"`
		}
		if err := json.Unmarshal(data, &obj); err != nil {
			t.Errorf("failed to unmarshal %s: %v", linuxFile, err)
		} else {
			expected := BuildLinuxPresets(cat)
			expectedJSON, _ := json.Marshal(expected)
			actualJSON, _ := json.Marshal(obj.Presets)
			if string(expectedJSON) != string(actualJSON) {
				t.Errorf("%s presets are out of sync with data/presets.json. Run 'doom presets build'", linuxFile)
			}
		}
	}

	// 3. Parity with Windows options.json
	winFile := filepath.Join(rootDir, "DoomRunner", "windows", "options.json")
	if data, err := os.ReadFile(winFile); err == nil {
		var obj struct {
			Presets []DoomRunnerPreset `json:"presets"`
		}
		if err := json.Unmarshal(data, &obj); err != nil {
			t.Errorf("failed to unmarshal %s: %v", winFile, err)
		} else {
			expected := BuildWindowsPresets(cat)
			expectedJSON, _ := json.Marshal(expected)
			actualJSON, _ := json.Marshal(obj.Presets)
			if string(expectedJSON) != string(actualJSON) {
				t.Errorf("%s presets are out of sync with data/presets.json. Run 'doom presets build'", winFile)
			}
		}
	}

	// 4. Parity with README.md table
	readmeFile := filepath.Join(rootDir, "README.md")
	if data, err := os.ReadFile(readmeFile); err == nil {
		expectedTable := GenerateReadmeTable(cat)
		if !strings.Contains(string(data), expectedTable) {
			t.Errorf("%s presets table is out of sync with data/presets.json. Run 'doom presets build'", readmeFile)
		}
	}
}

func TestResolveReadme_And_ParseReadme(t *testing.T) {
	tmpDir := t.TempDir()

	txtContent := `===========================================================================
Title                   : Ancient Aliens
Filename                : aaliens.zip
Release date            : May 8, 2016
Author                  : Paul "skillsaw" DeBruyne
Description             : A 32-level megawad.
New levels              : 32
===========================================================================`

	txtPath := filepath.Join(tmpDir, "aaliens_v1_2.txt")
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		t.Fatalf("failed writing test txt: %v", err)
	}

	p := Preset{
		Name:     "Ancient Aliens",
		Engine:   "uzdoom",
		IWAD:     "doom2.wad",
		Mappacks: []string{"aaliens_v1_2.wad"},
	}

	resolved, ok := ResolveReadme(tmpDir, p)
	if !ok || filepath.Base(resolved) != "aaliens_v1_2.txt" {
		t.Fatalf("expected to resolve aaliens_v1_2.txt, got %s, ok=%v", resolved, ok)
	}

	info := ParseReadme(resolved)
	if info.Author != `Paul "skillsaw" DeBruyne` {
		t.Errorf("expected skillsaw, got %q", info.Author)
	}
	if info.ReleaseDate != "May 8, 2016" {
		t.Errorf("expected May 8, 2016, got %q", info.ReleaseDate)
	}
	if info.MapCount != "32" {
		t.Errorf("expected 32, got %q", info.MapCount)
	}
}
