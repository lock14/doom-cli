package config

import (
	"path/filepath"
	"testing"

	"github.com/lock14/doom-cli/internal/preset"
)

func TestLoadConfig_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	p := &Paths{
		ConfigFile: filepath.Join(tmpDir, "config.json"),
	}

	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("unexpected error loading non-existent config: %v", err)
	}
	if cfg.Theme != "" {
		t.Errorf("expected empty theme for non-existent config, got %q", cfg.Theme)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	p := &Paths{
		ConfigFile: filepath.Join(tmpDir, "nested", "config.json"),
	}

	cfg := &AppConfig{
		Theme:         "cyberpunk",
		NerdFonts:     true,
		WadsDir:       "/custom/wads",
		BinDir:        "/custom/bin",
		SoundFontsDir: "/custom/soundfonts",
	}

	if err := SaveConfig(p, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loaded.Theme != "cyberpunk" {
		t.Errorf("expected loaded theme 'cyberpunk', got %q", loaded.Theme)
	}
	if !loaded.NerdFonts {
		t.Errorf("expected loaded NerdFonts true, got false")
	}
	if loaded.WadsDir != "/custom/wads" {
		t.Errorf("expected loaded WadsDir '/custom/wads', got %q", loaded.WadsDir)
	}
	if loaded.BinDir != "/custom/bin" {
		t.Errorf("expected loaded BinDir '/custom/bin', got %q", loaded.BinDir)
	}
	if loaded.SoundFontsDir != "/custom/soundfonts" {
		t.Errorf("expected loaded SoundFontsDir '/custom/soundfonts', got %q", loaded.SoundFontsDir)
	}
}

func TestAppConfig_ExtensibleHelpers(t *testing.T) {
	cfg := &AppConfig{}

	// 1. Engine helpers
	cfg.AddEngine(preset.EngineConfig{
		Name:        "woof",
		Binary:      "/usr/bin/woof",
		ArgsStyle:   "boom",
		Description: "Woof port",
	})
	if len(cfg.Engines) != 1 || cfg.Engines["woof"].Binary != "/usr/bin/woof" {
		t.Errorf("expected woof engine to be added, got %+v", cfg.Engines)
	}
	if !cfg.RemoveEngine("WOOF") {
		t.Error("expected RemoveEngine case-insensitive to succeed")
	}
	if len(cfg.Engines) != 0 {
		t.Errorf("expected empty engines after removal, got %d", len(cfg.Engines))
	}

	// 2. Preset helpers
	cfg.AddPreset(preset.Preset{
		Name:     "KDiZD",
		Engine:   "uzdoom",
		IWAD:     "DOOM.WAD",
		Mappacks: []string{"kdizd.pk3"},
	})
	if len(cfg.Presets) != 1 || cfg.Presets[0].Name != "KDiZD" || !cfg.Presets[0].Custom {
		t.Errorf("expected KDiZD preset to be added as custom, got %+v", cfg.Presets)
	}
	// Update existing preset
	cfg.AddPreset(preset.Preset{
		Name:     "kdizd",
		Engine:   "gzdoom",
		IWAD:     "DOOM.WAD",
		Mappacks: []string{"kdizd.pk3"},
	})
	if len(cfg.Presets) != 1 || cfg.Presets[0].Engine != "gzdoom" {
		t.Errorf("expected KDiZD to be updated in place, got %+v", cfg.Presets)
	}
	if !cfg.RemovePreset("kdizd") {
		t.Error("expected RemovePreset case-insensitive to succeed")
	}
	if len(cfg.Presets) != 0 {
		t.Errorf("expected empty presets after removal, got %d", len(cfg.Presets))
	}

	// 3. Launch options helpers
	cfg.SetLaunchOptions("Sunlust", preset.WadLaunchOptions{
		Engine:         "woof",
		AdditionalArgs: "-skill 4",
	})
	if len(cfg.LaunchOptions) != 1 || cfg.LaunchOptions["Sunlust"].Engine != "woof" {
		t.Errorf("expected Sunlust launch options to be set, got %+v", cfg.LaunchOptions)
	}
	if !cfg.RemoveLaunchOptions("sunlust") {
		t.Error("expected RemoveLaunchOptions case-insensitive to succeed")
	}
	if len(cfg.LaunchOptions) != 0 {
		t.Errorf("expected empty launch options after removal, got %d", len(cfg.LaunchOptions))
	}
}

func TestAppConfig_SaveAndLoadExtensible(t *testing.T) {
	tmpDir := t.TempDir()
	p := &Paths{
		ConfigFile: filepath.Join(tmpDir, "config.json"),
	}

	cfg := &AppConfig{
		Theme:     "sigil",
		NerdFonts: false,
	}
	cfg.AddEngine(preset.EngineConfig{
		Name:      "crispy-doom",
		Binary:    "crispy-doom",
		ArgsStyle: "boom",
	})
	cfg.AddPreset(preset.Preset{
		Name:     "MyCustomWad",
		Engine:   "crispy-doom",
		IWAD:     "DOOM2.WAD",
		Mappacks: []string{"mywad.wad"},
	})
	cfg.SetLaunchOptions("Alien Vendetta", preset.WadLaunchOptions{
		AdditionalArgs: "-skill 4 -warp 01",
	})

	if err := SaveConfig(p, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(loaded.Engines) != 1 || loaded.Engines["crispy-doom"].Name != "crispy-doom" {
		t.Errorf("failed to persist engines: %+v", loaded.Engines)
	}
	if len(loaded.Presets) != 1 || loaded.Presets[0].Name != "MyCustomWad" {
		t.Errorf("failed to persist presets: %+v", loaded.Presets)
	}
	if len(loaded.LaunchOptions) != 1 || loaded.LaunchOptions["Alien Vendetta"].AdditionalArgs != "-skill 4 -warp 01" {
		t.Errorf("failed to persist launch options: %+v", loaded.LaunchOptions)
	}
}
