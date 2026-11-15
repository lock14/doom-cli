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
		Theme:         "plasma",
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

	if loaded.Theme != "plasma" {
		t.Errorf("expected loaded theme 'plasma', got %q", loaded.Theme)
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
		"  \"x_custom_future\": {\"keep\": \"me\"}\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
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

func TestConfigCaseInsensitiveKnownKeysDeduped(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	p := &Paths{ConfigFile: configPath}

	raw := "{\n  \"Theme\": \"nord\",\n  \"Wads_Dir\": \"/data/wads\",\n  \"custom_extra\": 42\n}\n"
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Theme != "nord" || cfg.WadsDir != "/data/wads" {
		t.Fatalf("unexpected loaded config: %+v", cfg)
	}
	cfg.Theme = ""
	if err := SaveConfig(p, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	reloaded, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("reload LoadConfig: %v", err)
	}
	if reloaded.Theme != "" {
		t.Errorf("expected cleared Theme to stay empty, got %q", reloaded.Theme)
	}
}

func TestApplyConfigPathsAndProvenanceHelpers(t *testing.T) {
	t.Parallel()
	p := ResolveFor("linux")
	cfg := &AppConfig{
		WadsDir:       "/custom/wads",
		WadsDirs:      []string{"/extra/wads"},
		BinDir:        "/custom/bin",
		SoundFontsDir: "/custom/sf2",
	}
	cfg.SetNerdFonts(false)

	ApplyConfigPaths(p, cfg)
	if p.WadsDir != "/custom/wads" || p.BinDir != "/custom/bin" || p.SoundFontsDir != "/custom/sf2" {
		t.Fatalf("unexpected paths after ApplyConfigPaths: %+v", p)
	}
	if len(p.WadsDirs) != 1 || p.WadsDirs[0] != "/extra/wads" {
		t.Fatalf("unexpected WadsDirs: %v", p.WadsDirs)
	}
	if DirSource(cfg.WadsDir) != "config" || DirSource("") != "default" {
		t.Errorf("unexpected DirSource results")
	}
	if WadsDirsSource(cfg) != "config" || WadsDirsSource(&AppConfig{}) != "default" {
		t.Errorf("unexpected WadsDirsSource results")
	}
	nf, nfSrc := ResolveNerdFontsWithSource(cfg)
	if nf || nfSrc != "config" {
		t.Errorf("ResolveNerdFontsWithSource(explicit false) = (%v, %q), want (false, \"config\")", nf, nfSrc)
	}
	nfDef, nfDefSrc := ResolveNerdFontsWithSource(&AppConfig{})
	if nfDef || nfDefSrc != "default" {
		t.Errorf("ResolveNerdFontsWithSource(unset) = (%v, %q), want (false, \"default\")", nfDef, nfDefSrc)
	}
}
