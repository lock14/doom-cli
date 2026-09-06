package config

import (
	"path/filepath"
	"testing"
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
		Theme: "cyberpunk",
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
}
