package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_NonExistent(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	tmpDir := t.TempDir()
	p := &Paths{
		ConfigFile: filepath.Join(tmpDir, "nested", "config.json"),
	}

	cfg := &AppConfig{
		Theme:         "cyberpunk",
		NerdFonts:     true,
		WadsDir:       "/custom/wads",
		WadsDirs:      []string{"/extra/wads1", "/extra/wads2"},
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
	if len(loaded.WadsDirs) != 2 || loaded.WadsDirs[0] != "/extra/wads1" {
		t.Errorf("unexpected WadsDirs: %v", loaded.WadsDirs)
	}
	if loaded.BinDir != "/custom/bin" {
		t.Errorf("expected loaded BinDir '/custom/bin', got %q", loaded.BinDir)
	}
	if loaded.SoundFontsDir != "/custom/soundfonts" {
		t.Errorf("expected loaded SoundFontsDir '/custom/soundfonts', got %q", loaded.SoundFontsDir)
	}
}

func TestConfigPreservesUnknownKeysAndCreatesBackup(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	p := &Paths{ConfigFile: configPath}

	initial := "\xef\xbb\xbf{\n  \"theme\": \"blood\",\n" +
		"  \"x_custom_future\": {\"keep\": \"me\"},\n  \"engines\": {}\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.LegacyEntityKeys) != 1 || cfg.LegacyEntityKeys[0] != "engines" {
		t.Fatalf("expected LegacyEntityKeys=[engines], got %v", cfg.LegacyEntityKeys)
	}

	cfg.NerdFonts = true
	if err := SaveConfig(p, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	rawAfter, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(rawAfter), `"x_custom_future"`) || !strings.Contains(string(rawAfter), `"keep": "me"`) {
		t.Fatalf("unknown key lost after SaveConfig:\n%s", string(rawAfter))
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	foundBackup := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "config.json.bak.") {
			foundBackup = true
		}
	}
	if !foundBackup {
		t.Fatal("expected SaveConfig to create a .bak.<timestamp> backup when overwriting")
	}
}
