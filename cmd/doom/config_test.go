package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/config"
)

func TestConfigThemesAndThemeValidation(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	if err := runConfigThemes(nil, nil); err != nil {
		t.Fatalf("runConfigThemes() returned unexpected error: %v", err)
	}

	// 1. Set valid built-in theme via doom config set theme <name>
	if err := runConfigSet(nil, []string{"theme", "blood"}); err != nil {
		t.Fatalf("runConfigSet(theme, blood) error = %v", err)
	}

	paths := resolveAppPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != "blood" {
		t.Errorf("cfg.Theme = %q, expected 'blood'", cfg.Theme)
	}

	// 1b. Reset theme via "default" or ""
	if err := runConfigSet(nil, []string{"theme", "default"}); err != nil {
		t.Fatalf("runConfigSet(theme, default) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != "" {
		t.Errorf("cfg.Theme after reset to default = %q, expected empty string", cfg.Theme)
	}

	// 2. Setting unknown theme fails with helpful hint
	if err := runConfigSet(nil, []string{"theme", "nonexistent-theme"}); err == nil {
		t.Errorf("runConfigSet(theme, nonexistent-theme) expected error, got nil")
	}

	// 3. Setting custom theme file in themes directory succeeds
	customThemeDir := paths.ThemesDir
	if err := os.MkdirAll(customThemeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(themes) error = %v", err)
	}
	customThemeFile := filepath.Join(customThemeDir, "custom.json")
	customContent := `{"name": "custom", "description": "Custom theme"}`
	if err := os.WriteFile(customThemeFile, []byte(customContent), 0o600); err != nil {
		t.Fatalf("WriteFile(custom.json) error = %v", err)
	}

	if err := runConfigSet(nil, []string{"theme", "custom"}); err != nil {
		t.Fatalf("runConfigSet(theme, custom) error = %v", err)
	}

	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != "custom" {
		t.Errorf("cfg.Theme = %q, expected 'custom'", cfg.Theme)
	}

	// 4. Setting custom theme by mixed-case file path preserves original path casing
	mixedCaseFile := filepath.Join(tempDir, "MyCustomTheme.json")
	if err := os.WriteFile(mixedCaseFile, []byte(customContent), 0o600); err != nil {
		t.Fatalf("WriteFile(MyCustomTheme.json) error = %v", err)
	}
	if err := runConfigSet(nil, []string{"theme", mixedCaseFile}); err != nil {
		t.Fatalf("runConfigSet(theme, mixedCaseFile) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != mixedCaseFile {
		t.Errorf("cfg.Theme = %q, expected exact case-preserved path %q", cfg.Theme, mixedCaseFile)
	}
}

func TestParseBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bool
		wantErr  bool
	}{
		{input: "true", expected: true, wantErr: false},
		{input: "t", expected: true, wantErr: false},
		{input: "1", expected: true, wantErr: false},
		{input: "on", expected: true, wantErr: false},
		{input: "yes", expected: true, wantErr: false},
		{input: "y", expected: true, wantErr: false},
		{input: "enable", expected: true, wantErr: false},
		{input: "enabled", expected: true, wantErr: false},
		{input: "false", expected: false, wantErr: false},
		{input: "f", expected: false, wantErr: false},
		{input: "0", expected: false, wantErr: false},
		{input: "off", expected: false, wantErr: false},
		{input: "no", expected: false, wantErr: false},
		{input: "n", expected: false, wantErr: false},
		{input: "disable", expected: false, wantErr: false},
		{input: "disabled", expected: false, wantErr: false},
		{input: "maybe", expected: false, wantErr: true},
		{input: "2", expected: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := parseBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBool(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parseBool(%q) = %v, expected %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRunConfigCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	paths := resolveAppPaths()

	// 1. Naked `doom config` in non-TTY falls back to runConfigShow
	var showOut bytes.Buffer
	cfgCmd := newConfigCmd()
	cfgCmd.SetOut(&showOut)
	if err := cfgCmd.Execute(); err != nil {
		t.Fatalf("naked config command error = %v", err)
	}
	if !strings.Contains(showOut.String(), "=== Doom CLI Configuration ===") {
		t.Errorf("expected naked config in non-TTY to print configuration, got:\n%s", showOut.String())
	}

	var getOut bytes.Buffer
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&getOut)
	cfgCmd.SetArgs([]string{"get", "theme"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatalf("config get theme error = %v", err)
	}
	if strings.TrimSpace(getOut.String()) != "classic" {
		t.Errorf("config get theme with empty config = %q, want %q", strings.TrimSpace(getOut.String()), "classic")
	}

	// 2. Set nerd-fonts on
	if err := runConfigSet(nil, []string{"nerd-fonts", "on"}); err != nil {
		t.Fatalf("runConfigSet(nerd-fonts, on) error = %v", err)
	}
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !cfg.NerdFonts {
		t.Errorf("expected NerdFonts true, got false")
	}

	// 3. Set nerd-fonts off
	if err := runConfigSet(nil, []string{"nerd-fonts", "off"}); err != nil {
		t.Fatalf("runConfigSet(nerd-fonts, off) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.NerdFonts {
		t.Errorf("expected NerdFonts false, got true")
	}

	// 4. Set theme via config set
	if err := runConfigSet(nil, []string{"theme", "blood"}); err != nil {
		t.Fatalf("runConfigSet(theme, blood) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != "blood" {
		t.Errorf("expected Theme blood, got %q", cfg.Theme)
	}

	// 5. Set wads-dirs via config set
	if err := runConfigSet(nil, []string{"wads-dirs", "/tmp/w1,/tmp/w2"}); err != nil {
		t.Fatalf("runConfigSet(wads-dirs) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !reflect.DeepEqual(cfg.WadsDirs, []string{"/tmp/w1", "/tmp/w2"}) {
		t.Errorf("expected WadsDirs [/tmp/w1 /tmp/w2], got %v", cfg.WadsDirs)
	}

	// 6. Get settings
	if err := runConfigGet(nil, []string{"theme"}); err != nil {
		t.Errorf("runConfigGet(theme) error = %v", err)
	}
	if err := runConfigGet(nil, []string{"nerd-fonts"}); err != nil {
		t.Errorf("runConfigGet(nerd-fonts) error = %v", err)
	}
	if err := runConfigGet(nil, []string{"wads-dirs"}); err != nil {
		t.Errorf("runConfigGet(wads-dirs) error = %v", err)
	}
	if err := runConfigGet(nil, nil); err != nil {
		t.Errorf("runConfigGet() without args error = %v", err)
	}

	// 7. Error cases
	if err := runConfigSet(nil, []string{"invalid-key", "val"}); err == nil {
		t.Errorf("runConfigSet(invalid-key) expected error, got nil")
	}
	if err := runConfigSet(nil, []string{"nerd-fonts", "invalid-bool"}); err == nil {
		t.Errorf("runConfigSet(nerd-fonts, invalid-bool) expected error, got nil")
	}
	if err := runConfigGet(nil, []string{"invalid-key"}); err == nil {
		t.Errorf("runConfigGet(invalid-key) expected error, got nil")
	}
}

func TestGetPathsMalformedConfigFallsBackToDefaults(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	p0 := config.GetPaths()
	if err := os.MkdirAll(p0.ConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p0.ConfigFile, []byte(`{invalid json`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := resolveAppPaths()
	if got.WadsDir != p0.WadsDir {
		t.Errorf("resolveAppPaths().WadsDir = %q, want default %q on malformed config", got.WadsDir, p0.WadsDir)
	}
}

func TestCanonicalConfigKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "theme exact", input: "theme", expected: "theme", wantErr: false},
		{name: "theme with spaces", input: "  THEME  ", expected: "theme", wantErr: false},
		{name: "nerd-fonts standard", input: "nerd-fonts", expected: "nerd-fonts", wantErr: false},
		{name: "nerd_fonts underscore rejected", input: "nerd_fonts", expected: "", wantErr: true},
		{name: "wads-dir dash", input: "wads-dir", expected: "wads-dir", wantErr: false},
		{name: "wads_dir underscore rejected", input: "wads_dir", expected: "", wantErr: true},
		{name: "wads-dirs dash", input: "wads-dirs", expected: "wads-dirs", wantErr: false},
		{name: "wads_dirs underscore rejected", input: "wads_dirs", expected: "", wantErr: true},
		{name: "bin-dir dash", input: "bin-dir", expected: "bin-dir", wantErr: false},
		{name: "bin_dir underscore rejected", input: "bin_dir", expected: "", wantErr: true},
		{name: "soundfonts-dir dash", input: "soundfonts-dir", expected: "soundfonts-dir", wantErr: false},
		{name: "soundfonts_dir underscore rejected", input: "soundfonts_dir", expected: "", wantErr: true},
		{name: "unknown key", input: "unknown", expected: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalConfigKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("canonicalConfigKey(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if got != tt.expected {
				t.Errorf("canonicalConfigKey(%q) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestConfigTwoTierProvenanceAndCustomThemesList(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	// 1. Default provenance when config.json is empty
	var out bytes.Buffer
	cfgCmd := newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"show"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Theme:          classic (default)",
		"Nerd Fonts:     disabled (universal rectangular badges) (default)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected default config show to contain %q, got:\n%s", want, out.String())
		}
	}

	// 2. Set config.json values including explicit nerd-fonts = off
	if err := runConfigSet(nil, []string{"wads-dir", "/cfg/wads"}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigSet(nil, []string{"wads-dirs", "/cfg/extra1,/cfg/extra2"}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigSet(nil, []string{"theme", "amber"}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigSet(nil, []string{"nerd-fonts", "off"}); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"show"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	showStr := out.String()
	for _, want := range []string{
		"Theme:          amber (config)",
		"Nerd Fonts:     disabled (universal rectangular badges) (config)",
		"WADs Dir:       /cfg/wads (config)",
		"WADs Dirs:      /cfg/extra1, /cfg/extra2 (config)",
	} {
		if !strings.Contains(showStr, want) {
			t.Errorf("expected config show to contain %q, got:\n%s", want, showStr)
		}
	}

	out.Reset()
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"get", "wads-dir"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "/cfg/wads" {
		t.Errorf("config get wads-dir = %q, want /cfg/wads", got)
	}

	// 3. Custom theme in doom config themes
	paths := resolveAppPaths()
	if err := os.MkdirAll(paths.ThemesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(paths.ThemesDir, "neon.json"),
		[]byte(`{"name":"neon","description":"Custom neon"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"themes"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "neon") {
		t.Errorf("expected config themes to include custom theme 'neon', got:\n%s", out.String())
	}
}
